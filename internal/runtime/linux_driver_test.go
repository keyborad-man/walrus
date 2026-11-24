package runtime

import (
	"strings"
	"testing"
)

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
