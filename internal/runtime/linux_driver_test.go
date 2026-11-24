package runtime

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestBuildShardArgsSkipsBuiltinModUpdates(t *testing.T) {
	d := NewLinuxDriver("/data", "/data/dst-server", nil)
	got := d.buildShardArgs("cluster_1", "Master")
	want := []string{
		"-console",
		"-persistent_storage_root", "/data",
		"-conf_dir", "clusters",
		"-cluster", "cluster_1",
		"-shard", "Master",
		"-skip_update_server_mods",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("buildShardArgs mismatch:\ngot:  %v\nwant: %v", got, want)
	}
}

func TestPrepareHookFailureAndCancellation(t *testing.T) {
	tmp := t.TempDir()
	dstDir := filepath.Join(tmp, "dst-server")
	binDir := filepath.Join(dstDir, "bin64")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatalf("mkdir bin64: %v", err)
	}
	fakeBin := filepath.Join(binDir, "dontstarve_dedicated_server_nullrenderer_x64")
	if err := os.WriteFile(fakeBin, []byte("fake"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	d := NewLinuxDriver(tmp, dstDir, nil)

	// 1. Failing Prepare must abort launch and mark shard crashed without running the binary.
	err := d.StartShard(context.Background(), "c1", "master", StartOptions{
		Arch: "64",
		Prepare: func(ctx context.Context, logFn func(level, msg string)) error {
			logFn("system", "[walrus] downloading missing mod...")
			return errors.New("workshop 1909182187 failed")
		},
	})
	if err != nil {
		t.Fatalf("StartShard with Prepare should return immediately, got %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st := d.GetShardStatus("c1", "master")
		if st.State == StateCrashed {
			if !strings.Contains(st.LastError, "1909182187") {
				t.Fatalf("expected LastError to mention failed mod, got %q", st.LastError)
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if st := d.GetShardStatus("c1", "master"); st.State != StateCrashed {
		t.Fatalf("expected StateCrashed after failed Prepare, got %+v", st)
	}

	// 2. Stopping while Prepare is in flight must cancel Prepare context and transition cleanly to stopped.
	started := make(chan struct{})
	cancelled := make(chan struct{})
	err = d.StartShard(context.Background(), "c2", "master", StartOptions{
		Arch: "64",
		Prepare: func(ctx context.Context, logFn func(level, msg string)) error {
			close(started)
			<-ctx.Done()
			close(cancelled)
			return ctx.Err()
		},
	})
	if err != nil {
		t.Fatalf("StartShard c2: %v", err)
	}
	<-started
	if err := d.StopShard(context.Background(), "c2", "master"); err != nil {
		t.Fatalf("StopShard during Prepare: %v", err)
	}
	select {
	case <-cancelled:
	case <-time.After(2 * time.Second):
		t.Fatal("expected Prepare context to be cancelled on StopShard")
	}
	if st := d.GetShardStatus("c2", "master"); st.State != StateStopped {
		t.Fatalf("expected StateStopped after cancelling Prepare, got %+v", st)
	}
}

func TestFatalStartupErrorDetection(t *testing.T) {
	d := NewLinuxDriver(t.TempDir(), t.TempDir(), nil)

	mp := &managedProc{
		clusterID: "test01",
		shard:     "master",
		state:     StateStarting,
		doneCh:    make(chan struct{}),
	}
	// Close doneCh so terminateHungProc exits immediately without attempting to signal a nil Process.
	close(mp.doneCh)

	d.checkFatalStartupLine(mp, `[00:00:07]: DoLuaFile Error: #[string "scripts/map/level.lua"]:92: Must specify the task set for a level!`)
	if mp.fatalError != "" {
		t.Fatalf("expected fatalError to stay empty before worldgen failure marker, got %q", mp.fatalError)
	}

	d.checkFatalStartupLine(mp, `[00:00:07]: WorldSim::SimThread::Main() ERROR`)
	if !strings.Contains(mp.fatalError, "world generation failed") {
		t.Fatalf("expected fatalError to mention world generation failed, got %q", mp.fatalError)
	}
	if !strings.Contains(mp.fatalError, "Must specify the task set for a level!") {
		t.Fatalf("expected fatalError to include Lua detail, got %q", mp.fatalError)
	}

	// Running shards must not be terminated by startup error checks.
	running := &managedProc{
		clusterID: "test01",
		shard:     "master",
		state:     StateRunning,
		doneCh:    make(chan struct{}),
	}
	close(running.doneCh)
	d.checkFatalStartupLine(running, `[00:00:07]: Worldgen had an error, displaying...`)
	if running.fatalError != "" {
		t.Fatalf("expected running shard to ignore startup error marker, got %q", running.fatalError)
	}
}
