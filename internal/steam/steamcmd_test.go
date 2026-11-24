package steam

import (
	"path/filepath"
	"testing"
	"time"
)

func TestMockServerUpdateNoDeadlockAndPersistsLogs(t *testing.T) {
	tmp := t.TempDir()
	scDir := filepath.Join(tmp, "steamcmd")
	dstDir := filepath.Join(tmp, "dst-server")

	mgr := NewManager(scDir, dstDir, "mock")
	if err := mgr.UpdateDSTServer("public", true); err != nil {
		t.Fatalf("start update: %v", err)
	}

	deadline := time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		st := mgr.Status("public")
		if st.TaskState == TaskSuccess {
			if !st.Installed || st.LocalVersion == "" {
				t.Fatalf("expected installed with version, got %+v", st)
			}
			logs := mgr.RecentLogs()
			if len(logs) < 5 {
				t.Fatalf("expected >=5 log lines, got %d", len(logs))
			}

			// Simulate process restart: new Manager should still read version + persisted logs.
			mgr2 := NewManager(scDir, dstDir, "mock")
			st2 := mgr2.Status("public")
			if !st2.Installed || st2.LocalVersion != st.LocalVersion {
				t.Fatalf("expected persisted install state after restart, got %+v", st2)
			}
			if len(mgr2.RecentLogs()) < 5 {
				t.Fatalf("expected persisted logs after restart, got %d", len(mgr2.RecentLogs()))
			}
			return
		}
		if st.TaskState == TaskFailed {
			t.Fatalf("mock update failed: %s", st.LastError)
		}
		time.Sleep(150 * time.Millisecond)
	}
	t.Fatal("timed out waiting for mock SteamCMD update (possible deadlock)")
}
