package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"walrus/internal/apperr"
	"walrus/internal/db"
	"walrus/internal/dst"
	"walrus/internal/lua"
	"walrus/internal/runtime"
	"walrus/internal/steam"
)

func TestClusterAndBackupLifecycle(t *testing.T) {
	tmp := t.TempDir()
	store, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	fs := dst.NewFS(filepath.Join(tmp, "clusters"))
	driver := runtime.NewMockDriver(tmp, fs)
	clusterSvc := NewClusterService(store, fs, driver)
	backupSvc := NewBackupService(filepath.Join(tmp, "backups"), store, fs, driver)

	// Create cluster with caves
	detail, err := clusterSvc.CreateCluster(CreateClusterRequest{
		ClusterID:    "test_world",
		ClusterName:  "Test World",
		GameMode:     "endless",
		MaxPlayers:   6,
		CavesEnabled: true,
	})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if !detail.Meta.CavesEnabled || detail.CavesServerINI == nil {
		t.Fatal("expected caves to be enabled")
	}

	// Start cluster in mock mode
	ctx := context.Background()
	if err := clusterSvc.StartCluster(ctx, "test_world"); err != nil {
		t.Fatalf("start cluster: %v", err)
	}
	time.Sleep(1500 * time.Millisecond)

	st := driver.GetShardStatus("test_world", "master")
	if st.State != runtime.StateRunning {
		t.Fatalf("expected master running, got %s", st.State)
	}
	if err := clusterSvc.SendCommand("test_world", "master", `c_announce("Hello!")`); err != nil {
		t.Fatalf("send command: %v", err)
	}

	// Create manual backup
	rec, err := backupSvc.CreateBackup(ctx, "test_world", "manual", "before change", "admin")
	if err != nil {
		t.Fatalf("create backup: %v", err)
	}
	if rec.SizeBytes <= 0 {
		t.Fatalf("expected non-empty backup zip, got %d bytes", rec.SizeBytes)
	}

	// Mutate world lua
	if err := clusterSvc.WriteWorldLua("test_world", "master", "return { version=4, overrides={ world_size='huge' } }\n"); err != nil {
		t.Fatalf("write lua: %v", err)
	}

	// Restore backup -> should stop shards, create pre_restore backup, and revert lua
	if err := backupSvc.RestoreBackup(ctx, "test_world", rec.ID, "admin"); err != nil {
		t.Fatalf("restore backup: %v", err)
	}
	restoredLua, err := clusterSvc.ReadWorldLua("test_world", "master")
	if err != nil {
		t.Fatalf("read restored lua: %v", err)
	}
	if restoredLua == "return { version=4, overrides={ world_size='huge' } }\n" {
		t.Fatal("expected world lua to be reverted after restore")
	}

	backups, err := backupSvc.ListBackups("test_world")
	if err != nil {
		t.Fatalf("list backups: %v", err)
	}
	if len(backups) != 2 {
		t.Fatalf("expected 2 backups (manual + pre_restore), got %d", len(backups))
	}
}

func TestStartShardBackfillsMissingLevelDataDefaults(t *testing.T) {
	tmp := t.TempDir()
	store, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	fs := dst.NewFS(filepath.Join(tmp, "clusters"))
	driver := runtime.NewMockDriver(tmp, fs)
	clusterSvc := NewClusterService(store, fs, driver)

	if _, err := clusterSvc.CreateCluster(CreateClusterRequest{
		ClusterID:    "legacy_world",
		ClusterName:  "Legacy World",
		GameMode:     "endless",
		CavesEnabled: true,
	}); err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	// Simulate pre-fix leveldataoverride.lua files missing layout_mode / task_set.
	if err := fs.WriteLevelData("legacy_world", "master", `return {
  desc="Standard",
  id="SURVIVAL_TOGETHER",
  location="forest",
  name="Standard Forest",
  overrides={
    task_set="default",
    world_size="default",
  },
  version=4,
}
`); err != nil {
		t.Fatalf("write legacy master lua: %v", err)
	}
	if err := fs.WriteLevelData("legacy_world", "caves", `return {
  desc="The Caves",
  id="DST_CAVE",
  location="cave",
  name="The Caves",
  overrides={
    world_size="default",
  },
  version=4,
}
`); err != nil {
		t.Fatalf("write legacy caves lua: %v", err)
	}

	ctx := context.Background()
	if err := clusterSvc.StartCluster(ctx, "legacy_world"); err != nil {
		t.Fatalf("start cluster: %v", err)
	}
	defer func() { _ = clusterSvc.StopCluster(ctx, "legacy_world") }()

	masterRaw, err := fs.ReadLevelData("legacy_world", "master")
	if err != nil {
		t.Fatalf("read master lua: %v", err)
	}
	masterParsed, err := lua.ParseLevelData(masterRaw)
	if err != nil {
		t.Fatalf("parse master lua: %v", err)
	}
	if masterParsed.Overrides["layout_mode"] != "LinkNodesByKeys" {
		t.Fatalf("expected master layout_mode=LinkNodesByKeys, got %v", masterParsed.Overrides["layout_mode"])
	}

	cavesRaw, err := fs.ReadLevelData("legacy_world", "caves")
	if err != nil {
		t.Fatalf("read caves lua: %v", err)
	}
	cavesParsed, err := lua.ParseLevelData(cavesRaw)
	if err != nil {
		t.Fatalf("parse caves lua: %v", err)
	}
	if cavesParsed.Overrides["task_set"] != "cave_default" || cavesParsed.Overrides["layout_mode"] != "RestrictNodesByKey" {
		t.Fatalf("expected caves task_set=cave_default and layout_mode=RestrictNodesByKey, got %+v", cavesParsed.Overrides)
	}
}

func TestRedownloadWorkshopModValidation(t *testing.T) {
	tmp := t.TempDir()
	store, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	dstDir := filepath.Join(tmp, "dst-server")
	localModsDir := filepath.Join(tmp, "local-mods")
	fs := dst.NewFS(filepath.Join(tmp, "clusters"))
	steamMgr := steam.NewManager(filepath.Join(tmp, "steamcmd"), dstDir, "mock")
	workshop := steam.NewWorkshopClient("mock")
	modSvc := NewModService(dstDir, localModsDir, store, fs, steamMgr, workshop)

	// Invalid modinfo.lua should surface MODINFO_PARSE_FAILED instead of silent success.
	badDir := filepath.Join(dstDir, "mods", "workshop-2007975851")
	if err := os.MkdirAll(badDir, 0o755); err != nil {
		t.Fatalf("mkdir bad mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(badDir, "modinfo.lua"), []byte(`name = (`), 0o644); err != nil {
		t.Fatalf("write invalid modinfo: %v", err)
	}

	err = modSvc.RedownloadWorkshopMod(context.Background(), "workshop-2007975851", "zh-CN")
	code, _, _ := apperr.Extract(err, "")
	if code != apperr.CodeModInfoParseFailed {
		t.Fatalf("expected %s for invalid modinfo.lua, got code=%q err=%v", apperr.CodeModInfoParseFailed, code, err)
	}

	// Valid modinfo.lua should succeed and populate ModCache.
	if err := os.RemoveAll(badDir); err != nil {
		t.Fatalf("remove bad mod: %v", err)
	}
	if err := modSvc.RedownloadWorkshopMod(context.Background(), "workshop-378160973", "zh-CN"); err != nil {
		t.Fatalf("redownload valid mod: %v", err)
	}
	cache, err := store.GetModCache("workshop-378160973")
	if err != nil {
		t.Fatalf("get mod cache: %v", err)
	}
	if cache.Version != "1.7.4" || cache.Name == "" {
		t.Fatalf("expected cached metadata from modinfo.lua, got %+v", cache)
	}
}

func TestStartClusterAutoDownloadsMissingMods(t *testing.T) {
	tmp := t.TempDir()
	store, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	dstDir := filepath.Join(tmp, "dst-server")
	localModsDir := filepath.Join(tmp, "local-mods")
	fs := dst.NewFS(filepath.Join(tmp, "clusters"))
	driver := runtime.NewMockDriver(tmp, fs)
	steamMgr := steam.NewManager(filepath.Join(tmp, "steamcmd"), dstDir, "mock")
	workshop := steam.NewWorkshopClient("mock")
	modSvc := NewModService(dstDir, localModsDir, store, fs, steamMgr, workshop)
	clusterSvc := NewClusterService(store, fs, driver)
	clusterSvc.SetModService(modSvc)

	if _, err := clusterSvc.CreateCluster(CreateClusterRequest{
		ClusterID:    "modded_world",
		ClusterName:  "Modded World",
		GameMode:     "endless",
		CavesEnabled: true,
	}); err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	// Stage a local mod in localModsDir only (not yet mirrored into dst-server/mods).
	localSrc := filepath.Join(localModsDir, "local-custom")
	if err := os.MkdirAll(localSrc, 0o755); err != nil {
		t.Fatalf("mkdir local mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(localSrc, "modinfo.lua"), []byte(`name = "Custom Local"`), 0o644); err != nil {
		t.Fatalf("write local modinfo: %v", err)
	}

	// Configure enabled workshop mods (not downloaded yet), a disabled workshop mod, and the local mod.
	overridesLua := `return {
  ["workshop-378160973"] = { enabled = true, configuration_options = {} },
  ["workshop-1909182187"] = { enabled = true, configuration_options = {} },
  ["workshop-999999999"] = { enabled = false, configuration_options = {} },
  ["local-custom"] = { enabled = true, configuration_options = {} },
}
`
	if err := fs.WriteModOverrides("modded_world", "master", overridesLua); err != nil {
		t.Fatalf("write master modoverrides: %v", err)
	}
	if err := fs.WriteModOverrides("modded_world", "caves", overridesLua); err != nil {
		t.Fatalf("write caves modoverrides: %v", err)
	}

	ctx := context.Background()
	if err := clusterSvc.StartCluster(ctx, "modded_world"); err != nil {
		t.Fatalf("start cluster: %v", err)
	}
	defer func() { _ = clusterSvc.StopCluster(ctx, "modded_world") }()

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		ms := driver.GetShardStatus("modded_world", "master")
		cs := driver.GetShardStatus("modded_world", "caves")
		if ms.State == runtime.StateRunning && cs.State == runtime.StateRunning {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if st := driver.GetShardStatus("modded_world", "master"); st.State != runtime.StateRunning {
		t.Fatalf("expected master running after auto-downloading mods, got %+v", st)
	}
	if st := driver.GetShardStatus("modded_world", "caves"); st.State != runtime.StateRunning {
		t.Fatalf("expected caves running after auto-downloading mods, got %+v", st)
	}

	// Enabled workshop mods must be downloaded into dst-server/mods.
	for _, wid := range []string{"378160973", "1909182187"} {
		if _, err := os.Stat(filepath.Join(dstDir, "mods", "workshop-"+wid, "modinfo.lua")); err != nil {
			t.Fatalf("expected enabled workshop-%s to be auto-downloaded: %v", wid, err)
		}
	}
	// Disabled workshop mod must NOT be downloaded.
	if _, err := os.Stat(filepath.Join(dstDir, "mods", "workshop-999999999")); err == nil {
		t.Fatal("expected disabled workshop-999999999 not to be downloaded")
	}
	// Local mod must be mirrored into dst-server/mods.
	if _, err := os.Stat(filepath.Join(dstDir, "mods", "local-custom", "modinfo.lua")); err != nil {
		t.Fatalf("expected local-custom to be mirrored into dst-server/mods: %v", err)
	}
	// ModCache should be refreshed for auto-downloaded workshop mods.
	if cache, err := store.GetModCache("workshop-378160973"); err != nil || cache.Name == "" {
		t.Fatalf("expected mod cache populated for workshop-378160973, got %+v err=%v", cache, err)
	}
}

func TestStartShardAbortsWhenEnabledLocalModMissing(t *testing.T) {
	tmp := t.TempDir()
	store, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	dstDir := filepath.Join(tmp, "dst-server")
	localModsDir := filepath.Join(tmp, "local-mods")
	fs := dst.NewFS(filepath.Join(tmp, "clusters"))
	driver := runtime.NewMockDriver(tmp, fs)
	steamMgr := steam.NewManager(filepath.Join(tmp, "steamcmd"), dstDir, "mock")
	workshop := steam.NewWorkshopClient("mock")
	modSvc := NewModService(dstDir, localModsDir, store, fs, steamMgr, workshop)
	clusterSvc := NewClusterService(store, fs, driver)
	clusterSvc.SetModService(modSvc)

	if _, err := clusterSvc.CreateCluster(CreateClusterRequest{
		ClusterID:    "missing_local",
		ClusterName:  "Missing Local",
		GameMode:     "endless",
		CavesEnabled: false,
	}); err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if err := fs.WriteModOverrides("missing_local", "master", `return { ["local-nonexistent"] = { enabled = true } }`); err != nil {
		t.Fatalf("write modoverrides: %v", err)
	}

	if err := clusterSvc.StartShard(context.Background(), "missing_local", "master"); err != nil {
		t.Fatalf("StartShard should enter starting state, got %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st := driver.GetShardStatus("missing_local", "master")
		if st.State == runtime.StateCrashed {
			if st.LastError == "" {
				t.Fatal("expected LastError to explain missing local mod")
			}
			return
		}
		time.Sleep(30 * time.Millisecond)
	}
	t.Fatalf("expected shard to abort into crashed state when enabled local mod is missing, got %+v", driver.GetShardStatus("missing_local", "master"))
}

func TestGetClusterModsAndRedownloadBackfillPreviewURL(t *testing.T) {
	tmp := t.TempDir()
	store, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if err := r.ParseForm(); err != nil {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var details []map[string]any
		for k, vals := range r.PostForm {
			if len(k) > 16 && k[:16] == "publishedfileids" && len(vals) > 0 {
				id := vals[0]
				if id == "999999999" {
					// Simulate unavailable/deleted workshop item with no preview.
					details = append(details, map[string]any{
						"publishedfileid": id,
						"result":          9,
					})
					continue
				}
				details = append(details, map[string]any{
					"publishedfileid": id,
					"result":          1,
					"title":           "Steam English Title " + id,
					"description":     "Steam Description " + id,
					"preview_url":     "https://steam.example/preview/" + id + ".jpg",
					"time_updated":    1700000000,
				})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"response": map[string]any{
				"publishedfiledetails": details,
			},
		})
	}))
	defer srv.Close()

	dstDir := filepath.Join(tmp, "dst-server")
	localModsDir := filepath.Join(tmp, "local-mods")
	fs := dst.NewFS(filepath.Join(tmp, "clusters"))
	driver := runtime.NewMockDriver(tmp, fs)
	steamMgr := steam.NewManager(filepath.Join(tmp, "steamcmd"), dstDir, "mock")
	workshop := steam.NewWorkshopClient("linux")
	workshop.SetDetailsEndpoint(srv.URL)
	modSvc := NewModService(dstDir, localModsDir, store, fs, steamMgr, workshop)
	clusterSvc := NewClusterService(store, fs, driver)

	if _, err := clusterSvc.CreateCluster(CreateClusterRequest{
		ClusterID:   "preview_world",
		ClusterName: "Preview World",
		GameMode:    "endless",
	}); err != nil {
		t.Fatalf("create cluster: %v", err)
	}

	// Stage a downloaded mod with localized Chinese modinfo.lua but NO cached preview_url.
	modDir := filepath.Join(dstDir, "mods", "workshop-1185229307")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "modinfo.lua"), []byte(`
name = "史诗血量条"
author = "author"
version = "104"
description = "本地描述"
`), 0o644); err != nil {
		t.Fatalf("write modinfo: %v", err)
	}

	if err := fs.WriteModOverrides("preview_world", "master", `return {
  ["workshop-1185229307"] = { enabled = true, configuration_options = {} },
  ["workshop-999999999"] = { enabled = true, configuration_options = {} },
}`); err != nil {
		t.Fatalf("write modoverrides: %v", err)
	}

	// 1. First GetClusterMods call should batch-fetch missing preview URLs and persist them.
	res, err := modSvc.GetClusterMods(context.Background(), "preview_world", "master", "zh-CN")
	if err != nil {
		t.Fatalf("get cluster mods: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 Steam batch fetch, got %d", hits.Load())
	}
	var gotEpic *ClusterModItem
	for i := range res.Mods {
		if res.Mods[i].ModID == "workshop-1185229307" {
			gotEpic = &res.Mods[i]
		}
	}
	if gotEpic == nil {
		t.Fatal("missing workshop-1185229307 in response")
	}
	if gotEpic.PreviewURL != "https://steam.example/preview/1185229307.jpg" {
		t.Fatalf("expected backfilled preview_url, got %q", gotEpic.PreviewURL)
	}
	if gotEpic.Name != "史诗血量条" {
		t.Fatalf("expected localized modinfo name preserved, got %q", gotEpic.Name)
	}
	cache, err := store.GetModCache("workshop-1185229307")
	if err != nil || cache.PreviewURL != "https://steam.example/preview/1185229307.jpg" {
		t.Fatalf("expected preview_url persisted in mod_cache, got %+v err=%v", cache, err)
	}

	// 2. Second GetClusterMods call should hit neither cached mod nor cooled-down missing mod.
	res2, err := modSvc.GetClusterMods(context.Background(), "preview_world", "master", "zh-CN")
	if err != nil {
		t.Fatalf("second get cluster mods: %v", err)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected no additional Steam fetch when cached/cooled down, got %d", hits.Load())
	}
	if res2.Mods[0].PreviewURL != "https://steam.example/preview/1185229307.jpg" {
		t.Fatalf("expected cached preview_url on second load, got %q", res2.Mods[0].PreviewURL)
	}

	// 3. RedownloadWorkshopMod should also populate preview_url when missing.
	if err := modSvc.RedownloadWorkshopMod(context.Background(), "workshop-378160973", "zh-CN"); err != nil {
		t.Fatalf("redownload workshop mod: %v", err)
	}
	cache2, err := store.GetModCache("workshop-378160973")
	if err != nil || cache2.PreviewURL != "https://steam.example/preview/378160973.jpg" {
		t.Fatalf("expected redownload to populate preview_url, got %+v err=%v", cache2, err)
	}
}
