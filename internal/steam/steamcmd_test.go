package steam

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
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

func TestCancelSteamCMDTaskReleasesLock(t *testing.T) {
	tmp := t.TempDir()
	mgr := NewManager(filepath.Join(tmp, "steamcmd"), filepath.Join(tmp, "dst-server"), "mock")

	if mgr.CancelTask() {
		t.Fatal("expected CancelTask() to return false when idle")
	}
	if err := mgr.UpdateDSTServer("public", true); err != nil {
		t.Fatalf("start update: %v", err)
	}
	if !mgr.CancelTask() {
		t.Fatal("expected CancelTask() to return true while running")
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		st := mgr.Status("public")
		if st.TaskState == TaskFailed {
			// Lock must be released so a subsequent update can start immediately.
			if err := mgr.UpdateDSTServer("public", false); err != nil {
				t.Fatalf("expected lock released after cancel, got %v", err)
			}
			mgr.CancelTask()
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("expected cancelled task to transition to failed, got %+v", mgr.Status("public"))
}

func TestBuildServerUpdateArgs(t *testing.T) {
	mgr := NewManager("/data/steamcmd", "/data/dst-server", "linux")

	gotPublic := mgr.buildServerUpdateArgs("public", true)
	wantPublic := []string{
		"+@sSteamCmdForcePlatformType", "linux",
		"+force_install_dir", "/data/dst-server",
		"+login", "anonymous",
		"+app_info_update", "1",
		"+app_update", DSTServerAppID,
		"validate",
		"+quit",
	}
	if !reflect.DeepEqual(gotPublic, wantPublic) {
		t.Fatalf("public validate args mismatch:\ngot:  %v\nwant: %v", gotPublic, wantPublic)
	}

	gotBeta := mgr.buildServerUpdateArgs("updatebeta", false)
	wantBeta := []string{
		"+@sSteamCmdForcePlatformType", "linux",
		"+force_install_dir", "/data/dst-server",
		"+login", "anonymous",
		"+app_info_update", "1",
		"+app_update", DSTServerAppID,
		"-beta", "updatebeta",
		"+quit",
	}
	if !reflect.DeepEqual(gotBeta, wantBeta) {
		t.Fatalf("beta no-validate args mismatch:\ngot:  %v\nwant: %v", gotBeta, wantBeta)
	}
}

func TestStripANSI(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"\x1b[0mERROR! Failed to install app '343050' (Missing configuration)\r", "ERROR! Failed to install app '343050' (Missing configuration)"},
		{"Loading Steam API...\x1b[0mOK", "Loading Steam API...OK"},
		{"\x1b[1;33mConnecting...\x1b[0m", "Connecting..."},
		{"\x1b[0m", ""},
	}
	for _, tc := range cases {
		if got := stripANSI(tc.in); got != tc.want {
			t.Errorf("stripANSI(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestSetEnv(t *testing.T) {
	got := setEnv([]string{"PATH=/usr/bin", "HOME=/root", "HOME=/dup", "TZ=UTC"}, "HOME", "/data/steamcmd/home")
	want := []string{"PATH=/usr/bin", "HOME=/data/steamcmd/home", "TZ=UTC"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("setEnv replace mismatch:\ngot:  %v\nwant: %v", got, want)
	}

	gotAdd := setEnv([]string{"PATH=/usr/bin"}, "HOME", "/data/steamcmd/home")
	wantAdd := []string{"PATH=/usr/bin", "HOME=/data/steamcmd/home"}
	if !reflect.DeepEqual(gotAdd, wantAdd) {
		t.Fatalf("setEnv append mismatch:\ngot:  %v\nwant: %v", gotAdd, wantAdd)
	}
}

func TestRunWithMissingConfigRetry(t *testing.T) {
	t.Run("clears appcache and retries once on missing configuration", func(t *testing.T) {
		tmp := t.TempDir()
		mgr := NewManager(filepath.Join(tmp, "steamcmd"), filepath.Join(tmp, "dst-server"), "linux")

		appcacheFile := filepath.Join(mgr.steamCMDHomeDir(), "Steam", "appcache", "appinfo.vdf")
		if err := os.MkdirAll(filepath.Dir(appcacheFile), 0o755); err != nil {
			t.Fatalf("mkdir appcache: %v", err)
		}
		if err := os.WriteFile(appcacheFile, []byte("stale"), 0o644); err != nil {
			t.Fatalf("write appcache: %v", err)
		}

		attempts := 0
		err := mgr.runWithMissingConfigRetry(func() (bool, error) {
			attempts++
			if attempts == 1 {
				return true, errors.New("exit status 8")
			}
			if _, statErr := os.Stat(filepath.Dir(appcacheFile)); !errors.Is(statErr, os.ErrNotExist) {
				t.Errorf("expected appcache directory to be removed before retry, statErr=%v", statErr)
			}
			return false, nil
		})
		if err != nil {
			t.Fatalf("expected retry to succeed, got %v", err)
		}
		if attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", attempts)
		}

		logs := mgr.RecentLogs()
		found := false
		for _, l := range logs {
			if strings.Contains(l.Message, "Missing configuration") {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("expected retry log line, got %+v", logs)
		}
	})

	t.Run("does not retry on unrelated failure", func(t *testing.T) {
		tmp := t.TempDir()
		mgr := NewManager(filepath.Join(tmp, "steamcmd"), filepath.Join(tmp, "dst-server"), "linux")

		attempts := 0
		err := mgr.runWithMissingConfigRetry(func() (bool, error) {
			attempts++
			return false, errors.New("exit status 5")
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if attempts != 1 {
			t.Fatalf("expected 1 attempt, got %d", attempts)
		}
	})

	t.Run("retries at most once if error persists", func(t *testing.T) {
		tmp := t.TempDir()
		mgr := NewManager(filepath.Join(tmp, "steamcmd"), filepath.Join(tmp, "dst-server"), "linux")

		attempts := 0
		err := mgr.runWithMissingConfigRetry(func() (bool, error) {
			attempts++
			return true, errors.New("exit status 8")
		})
		if err == nil {
			t.Fatal("expected error, got nil")
		}
		if attempts != 2 {
			t.Fatalf("expected 2 attempts, got %d", attempts)
		}
	})
}
