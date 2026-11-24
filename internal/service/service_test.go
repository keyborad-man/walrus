package service

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"walrus/internal/db"
	"walrus/internal/dst"
	"walrus/internal/lua"
	"walrus/internal/runtime"
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
