package service

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
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

func TestCronDSTUpdateCheckAndAutoUpgrade(t *testing.T) {
	tmp := t.TempDir()
	store, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	// Mock Klei builds.json server returning build 700001.
	buildsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"release": []string{"700000", "700001"},
			"beta":    []string{"700002"},
		})
	}))
	defer buildsSrv.Close()

	// Capture webhook events.
	var mu sync.Mutex
	var events []string
	whSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev WebhookEvent
		if err := json.NewDecoder(r.Body).Decode(&ev); err == nil {
			mu.Lock()
			events = append(events, ev.Event)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer whSrv.Close()

	_, _ = store.CreateWebhook(&db.WebhookConfig{
		Name:     "test-hook",
		Template: "generic",
		URL:      whSrv.URL,
		Events:   []string{"*"},
		Enabled:  true,
	})

	dstDir := filepath.Join(tmp, "dst-server")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatalf("mkdir dstDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "version.txt"), []byte("700000\n"), 0o644); err != nil {
		t.Fatalf("write version.txt: %v", err)
	}

	fs := dst.NewFS(filepath.Join(tmp, "clusters"))
	driver := runtime.NewMockDriver(tmp, fs)
	steamMgr := steam.NewManager(filepath.Join(tmp, "steamcmd"), dstDir, "mock")
	steamMgr.SetBuildsURL(buildsSrv.URL)
	clusterSvc := NewClusterService(store, fs, driver)
	backupSvc := NewBackupService(filepath.Join(tmp, "backups"), store, fs, driver)
	webhookSvc := NewWebhookService(store)
	cronSvc := NewCronService(store, fs, driver, clusterSvc, backupSvc, steamMgr, webhookSvc)

	if _, err := clusterSvc.CreateCluster(CreateClusterRequest{
		ClusterID:    "dst_upd_world",
		ClusterName:  "DST Update World",
		GameMode:     "endless",
		CavesEnabled: false,
	}); err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	ctx := context.Background()
	if err := clusterSvc.StartCluster(ctx, "dst_upd_world"); err != nil {
		t.Fatalf("start cluster: %v", err)
	}
	defer func() { _ = clusterSvc.StopCluster(ctx, "dst_upd_world") }()
	time.Sleep(1500 * time.Millisecond)

	// 1. Notify-only mode should detect update, dispatch webhook, and NOT modify local version.
	notifyTask, err := store.CreateScheduledTask(&db.ScheduledTask{
		ClusterID:  "*",
		Name:       "Check DST Version",
		TaskType:   "update",
		CronExpr:   "0 * * * *",
		ParamsJSON: `{"auto_upgrade":false}`,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("create notify task: %v", err)
	}
	cronSvc.executeTaskByID(notifyTask.ID, true)

	gotNotify, _ := store.GetScheduledTask(notifyTask.ID)
	if !strings.Contains(gotNotify.LastStatus, "notify only") {
		t.Fatalf("expected notify only status, got %q", gotNotify.LastStatus)
	}
	if steamMgr.LocalVersion() != "700000" {
		t.Fatalf("expected local version unchanged in notify-only mode, got %q", steamMgr.LocalVersion())
	}

	// 2. Auto-upgrade with pre-update backup should stop, backup, update to 700001, and restart.
	autoTask, err := store.CreateScheduledTask(&db.ScheduledTask{
		ClusterID:  "*",
		Name:       "Auto Upgrade DST",
		TaskType:   "update",
		CronExpr:   "0 4 * * *",
		ParamsJSON: `{"auto_upgrade":true,"only_when_empty":false,"countdown_sec":0,"backup_before_update":true}`,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("create auto task: %v", err)
	}
	cronSvc.executeTaskByID(autoTask.ID, true)

	gotAuto, _ := store.GetScheduledTask(autoTask.ID)
	if gotAuto.LastStatus != "updated to 700001" {
		t.Fatalf("expected 'updated to 700001', got %q", gotAuto.LastStatus)
	}
	if steamMgr.LocalVersion() != "700001" {
		t.Fatalf("expected local version updated to 700001, got %q", steamMgr.LocalVersion())
	}
	backups, err := backupSvc.ListBackups("dst_upd_world")
	if err != nil || len(backups) != 1 || backups[0].Trigger != "pre_update" {
		t.Fatalf("expected 1 pre_update backup, got %+v err=%v", backups, err)
	}
	// Cluster should be restarted.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if driver.GetShardStatus("dst_upd_world", "master").State == runtime.StateRunning {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if st := driver.GetShardStatus("dst_upd_world", "master"); st.State != runtime.StateRunning {
		t.Fatalf("expected cluster restarted after DST upgrade, got %+v", st)
	}

	// 3. Running again when up to date should report up to date.
	cronSvc.executeTaskByID(autoTask.ID, true)
	gotAuto2, _ := store.GetScheduledTask(autoTask.ID)
	if gotAuto2.LastStatus != "up to date (700001)" {
		t.Fatalf("expected 'up to date (700001)', got %q", gotAuto2.LastStatus)
	}

	// Verify both webhook events arrived.
	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	joined := strings.Join(events, ",")
	mu.Unlock()
	if !strings.Contains(joined, "steam.update_available") || !strings.Contains(joined, "steam.update_complete") {
		t.Fatalf("expected steam update webhooks, got %q", joined)
	}
}

func TestCronModUpdateCheckAndAutoUpgrade(t *testing.T) {
	tmp := t.TempDir()
	store, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	// Mock Steam Workshop GetPublishedFileDetails returning time_updated = 1800000000.
	wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]any{
			"response": map[string]any{
				"publishedfiledetails": []map[string]any{
					{
						"publishedfileid": "378160973",
						"result":          1,
						"title":           "Global Positions",
						"description":     "Map share",
						"preview_url":     "https://steam.example/preview/378160973.jpg",
						"time_updated":    1800000000,
					},
				},
			},
		})
	}))
	defer wsSrv.Close()

	var mu sync.Mutex
	var events []string
	whSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var ev WebhookEvent
		if err := json.NewDecoder(r.Body).Decode(&ev); err == nil {
			mu.Lock()
			events = append(events, ev.Event)
			mu.Unlock()
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer whSrv.Close()

	_, _ = store.CreateWebhook(&db.WebhookConfig{
		Name:     "mod-hook",
		Template: "generic",
		URL:      whSrv.URL,
		Events:   []string{"*"},
		Enabled:  true,
	})

	dstDir := filepath.Join(tmp, "dst-server")
	localModsDir := filepath.Join(tmp, "local-mods")
	fs := dst.NewFS(filepath.Join(tmp, "clusters"))
	driver := runtime.NewMockDriver(tmp, fs)
	steamMgr := steam.NewManager(filepath.Join(tmp, "steamcmd"), dstDir, "mock")
	workshop := steam.NewWorkshopClient("linux")
	workshop.SetDetailsEndpoint(wsSrv.URL)
	modSvc := NewModService(dstDir, localModsDir, store, fs, steamMgr, workshop)
	clusterSvc := NewClusterService(store, fs, driver)
	clusterSvc.SetModService(modSvc)
	backupSvc := NewBackupService(filepath.Join(tmp, "backups"), store, fs, driver)
	webhookSvc := NewWebhookService(store)
	cronSvc := NewCronService(store, fs, driver, clusterSvc, backupSvc, steamMgr, webhookSvc)
	cronSvc.SetModService(modSvc)

	// Create two clusters: c1 enables workshop-378160973, c2 has no workshop mods.
	for _, cid := range []string{"c1", "c2"} {
		if _, err := clusterSvc.CreateCluster(CreateClusterRequest{
			ClusterID:    cid,
			ClusterName:  cid,
			GameMode:     "endless",
			CavesEnabled: false,
		}); err != nil {
			t.Fatalf("create cluster %s: %v", cid, err)
		}
	}
	if err := fs.WriteModOverrides("c1", "master", `return {
  ["workshop-378160973"] = { enabled = true, configuration_options = {} },
}`); err != nil {
		t.Fatalf("write c1 modoverrides: %v", err)
	}

	// Stage locally installed older version of workshop-378160973.
	modDir := filepath.Join(dstDir, "mods", "workshop-378160973")
	if err := os.MkdirAll(modDir, 0o755); err != nil {
		t.Fatalf("mkdir mod: %v", err)
	}
	if err := os.WriteFile(filepath.Join(modDir, "modinfo.lua"), []byte(`name = "Global Positions"
version = "1.7.0"`), 0o644); err != nil {
		t.Fatalf("write modinfo: %v", err)
	}

	ctx := context.Background()
	if err := clusterSvc.StartCluster(ctx, "c1"); err != nil {
		t.Fatalf("start c1: %v", err)
	}
	defer func() { _ = clusterSvc.StopCluster(ctx, "c1") }()
	time.Sleep(1500 * time.Millisecond)

	// The Workshop item gets a new version while c1 is running (startup already updated it).
	// Even if TimeUpdated was already touched (e.g. by preview backfill), InstalledTimeUpdated governs update detection.
	if err := store.UpsertModCache(&db.ModCache{
		ModID:                "workshop-378160973",
		WorkshopID:           "378160973",
		Name:                 "Global Positions",
		TimeUpdated:          1800000000,
		InstalledTimeUpdated: 1700000000,
	}); err != nil {
		t.Fatalf("upsert mod cache: %v", err)
	}

	// 1. Notify-only mode should report 1 mod update available without touching installed_time_updated.
	notifyTask, err := store.CreateScheduledTask(&db.ScheduledTask{
		ClusterID:  "c1",
		Name:       "Check Mods",
		TaskType:   "mod_update",
		CronExpr:   "0 * * * *",
		ParamsJSON: `{"auto_upgrade":false}`,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("create mod notify task: %v", err)
	}
	cronSvc.executeTaskByID(notifyTask.ID, true)
	gotNotify, _ := store.GetScheduledTask(notifyTask.ID)
	if !strings.Contains(gotNotify.LastStatus, "updates available (1 mod(s), notify only)") {
		t.Fatalf("expected notify-only mod status, got %q", gotNotify.LastStatus)
	}
	cacheBefore, _ := store.GetModCache("workshop-378160973")
	if cacheBefore.InstalledTimeUpdated != 1700000000 {
		t.Fatalf("expected InstalledTimeUpdated unchanged, got %d", cacheBefore.InstalledTimeUpdated)
	}

	// 2. Auto-upgrade with backup should backup c1 (not c2), update mod, refresh InstalledTimeUpdated, and restart c1.
	autoTask, err := store.CreateScheduledTask(&db.ScheduledTask{
		ClusterID:  "c1",
		Name:       "Auto Update Mods",
		TaskType:   "mod_update",
		CronExpr:   "0 */6 * * *",
		ParamsJSON: `{"auto_upgrade":true,"only_when_empty":false,"countdown_sec":0,"backup_before_update":true}`,
		Enabled:    true,
	})
	if err != nil {
		t.Fatalf("create mod auto task: %v", err)
	}
	cronSvc.executeTaskByID(autoTask.ID, true)
	gotAuto, _ := store.GetScheduledTask(autoTask.ID)
	if gotAuto.LastStatus != "updated 1 mod(s), restarted 1 cluster(s)" {
		t.Fatalf("unexpected auto mod update status: %q", gotAuto.LastStatus)
	}
	cacheAfter, _ := store.GetModCache("workshop-378160973")
	if cacheAfter.InstalledTimeUpdated != 1800000000 {
		t.Fatalf("expected InstalledTimeUpdated=1800000000, got %d", cacheAfter.InstalledTimeUpdated)
	}

	c1Backups, _ := backupSvc.ListBackups("c1")
	c2Backups, _ := backupSvc.ListBackups("c2")
	if len(c1Backups) != 1 || c1Backups[0].Trigger != "pre_update" {
		t.Fatalf("expected 1 pre_update backup for c1, got %+v", c1Backups)
	}
	if len(c2Backups) != 0 {
		t.Fatalf("expected 0 backups for unaffected c2, got %+v", c2Backups)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if driver.GetShardStatus("c1", "master").State == runtime.StateRunning {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if st := driver.GetShardStatus("c1", "master"); st.State != runtime.StateRunning {
		t.Fatalf("expected c1 restarted after mod update, got %+v", st)
	}

	// 3. Running again when up to date should report up to date.
	cronSvc.executeTaskByID(autoTask.ID, true)
	gotAuto2, _ := store.GetScheduledTask(autoTask.ID)
	if gotAuto2.LastStatus != "up to date (1 mod(s))" {
		t.Fatalf("expected 'up to date (1 mod(s))', got %q", gotAuto2.LastStatus)
	}

	time.Sleep(200 * time.Millisecond)
	mu.Lock()
	joined := strings.Join(events, ",")
	mu.Unlock()
	if !strings.Contains(joined, "mod.update_available") || !strings.Contains(joined, "mod.update_complete") {
		t.Fatalf("expected mod update webhooks, got %q", joined)
	}
}

func TestStartShardUpdatesOutdatedWorkshopMods(t *testing.T) {
	for _, tc := range []struct {
		name         string
		steamDown    bool
		wantInstTime int64
	}{
		{name: "updates before launch", wantInstTime: 1800000000},
		{name: "steam unreachable still launches", steamDown: true, wantInstTime: 1700000000},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tmp := t.TempDir()
			store, err := db.Open(filepath.Join(tmp, "test.db"))
			if err != nil {
				t.Fatalf("open db: %v", err)
			}
			defer store.Close()

			wsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tc.steamDown {
					w.WriteHeader(http.StatusBadGateway)
					return
				}
				_ = json.NewEncoder(w).Encode(map[string]any{
					"response": map[string]any{
						"publishedfiledetails": []map[string]any{
							{"publishedfileid": "378160973", "result": 1, "title": "Global Positions", "time_updated": 1800000000},
						},
					},
				})
			}))
			defer wsSrv.Close()

			dstDir := filepath.Join(tmp, "dst-server")
			fs := dst.NewFS(filepath.Join(tmp, "clusters"))
			driver := runtime.NewMockDriver(tmp, fs)
			steamMgr := steam.NewManager(filepath.Join(tmp, "steamcmd"), dstDir, "mock")
			workshop := steam.NewWorkshopClient("linux")
			workshop.SetDetailsEndpoint(wsSrv.URL)
			modSvc := NewModService(dstDir, filepath.Join(tmp, "local-mods"), store, fs, steamMgr, workshop)
			clusterSvc := NewClusterService(store, fs, driver)
			clusterSvc.SetModService(modSvc)

			if _, err := clusterSvc.CreateCluster(CreateClusterRequest{ClusterID: "c1", ClusterName: "c1", GameMode: "endless"}); err != nil {
				t.Fatalf("create cluster: %v", err)
			}
			if err := fs.WriteModOverrides("c1", "master", `return {
  ["workshop-378160973"] = { enabled = true, configuration_options = {} },
}`); err != nil {
				t.Fatalf("write modoverrides: %v", err)
			}
			modDir := filepath.Join(dstDir, "mods", "workshop-378160973")
			if err := os.MkdirAll(modDir, 0o755); err != nil {
				t.Fatalf("mkdir mod: %v", err)
			}
			if err := os.WriteFile(filepath.Join(modDir, "modinfo.lua"), []byte(`name = "Global Positions"`), 0o644); err != nil {
				t.Fatalf("write modinfo: %v", err)
			}
			if err := store.UpsertModCache(&db.ModCache{
				ModID:                "workshop-378160973",
				WorkshopID:           "378160973",
				Name:                 "Global Positions",
				InstalledTimeUpdated: 1700000000,
			}); err != nil {
				t.Fatalf("upsert mod cache: %v", err)
			}

			ctx := context.Background()
			if err := clusterSvc.StartShard(ctx, "c1", "master"); err != nil {
				t.Fatalf("start shard: %v", err)
			}
			defer func() { _ = clusterSvc.StopCluster(ctx, "c1") }()
			deadline := time.Now().Add(5 * time.Second)
			for time.Now().Before(deadline) && driver.GetShardStatus("c1", "master").State != runtime.StateRunning {
				time.Sleep(50 * time.Millisecond)
			}
			if st := driver.GetShardStatus("c1", "master"); st.State != runtime.StateRunning {
				t.Fatalf("expected master running, got %+v", st)
			}
			cache, _ := store.GetModCache("workshop-378160973")
			if cache.InstalledTimeUpdated != tc.wantInstTime {
				t.Fatalf("expected InstalledTimeUpdated=%d, got %d", tc.wantInstTime, cache.InstalledTimeUpdated)
			}
		})
	}
}

func TestCreateClusterWithCustomWorldOverridesAndNetworkMode(t *testing.T) {
	tmp := t.TempDir()
	store, err := db.Open(filepath.Join(tmp, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	defer store.Close()

	fs := dst.NewFS(filepath.Join(tmp, "clusters"))
	driver := runtime.NewMockDriver(tmp, fs)
	clusterSvc := NewClusterService(store, fs, driver)

	detail, err := clusterSvc.CreateCluster(CreateClusterRequest{
		ClusterID:      "custom_dst_world",
		ClusterName:    "Custom DST World",
		GameMode:       "endless",
		MaxPlayers:     8,
		OfflineCluster: true,
		LanOnlyCluster: true,
		CavesEnabled:   true,
		MasterPreset:   "endless",
		MasterOverrides: map[string]any{
			"world_size":         "huge",
			"year_of_the_knight": "enabled",
			"day":                "longday",
		},
		CavesOverrides: map[string]any{
			"world_size":       "medium",
			"acidrain_enabled": "none",
		},
	})
	if err != nil {
		t.Fatalf("create cluster: %v", err)
	}
	if !detail.ClusterINI.OfflineCluster || !detail.ClusterINI.LanOnlyCluster {
		t.Fatalf("expected offline and lan_only flags set, got %+v", detail.ClusterINI)
	}

	masterView, err := clusterSvc.ReadWorldConfig("custom_dst_world", "master")
	if err != nil {
		t.Fatalf("read master world: %v", err)
	}
	if masterView.Parsed == nil {
		t.Fatal("expected parsed master leveldata")
	}
	if masterView.Parsed.Overrides["world_size"] != "huge" ||
		masterView.Parsed.Overrides["year_of_the_knight"] != "enabled" ||
		masterView.Parsed.Overrides["day"] != "longday" {
		t.Fatalf("unexpected master overrides: %+v", masterView.Parsed.Overrides)
	}
	// Preserve preset baseline overrides alongside custom patches
	if masterView.Parsed.Overrides["portalresurection"] != "always" {
		t.Fatalf("expected endless preset baseline portalresurection=always preserved, got %v", masterView.Parsed.Overrides["portalresurection"])
	}

	cavesView, err := clusterSvc.ReadWorldConfig("custom_dst_world", "caves")
	if err != nil {
		t.Fatalf("read caves world: %v", err)
	}
	if cavesView.Parsed == nil ||
		cavesView.Parsed.Overrides["world_size"] != "medium" ||
		cavesView.Parsed.Overrides["acidrain_enabled"] != "none" {
		t.Fatalf("unexpected caves overrides: %+v", cavesView.Parsed)
	}
}
