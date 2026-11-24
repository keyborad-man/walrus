package steam

import (
	"archive/zip"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"walrus/internal/apperr"
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

func TestBuildWorkshopDownloadArgsAndMockBatch(t *testing.T) {
	got := buildWorkshopDownloadArgs("/data/steamcmd/workshop_cache", []string{"1699194522", "1909182187"})
	want := []string{
		"+force_install_dir", "/data/steamcmd/workshop_cache",
		"+login", "anonymous",
		"+workshop_download_item", DSTGameAppID, "1699194522",
		"+workshop_download_item", DSTGameAppID, "1909182187",
		"+quit",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("workshop batch args mismatch:\ngot:  %v\nwant: %v", got, want)
	}

	tmp := t.TempDir()
	dstDir := filepath.Join(tmp, "dst-server")
	mgr := NewManager(filepath.Join(tmp, "steamcmd"), dstDir, "mock")

	var progress []string
	if err := mgr.DownloadWorkshopMods(t.Context(), []string{"378160973", "1909182187", "378160973", " "}, func(msg string) {
		progress = append(progress, msg)
	}); err != nil {
		t.Fatalf("DownloadWorkshopMods mock: %v", err)
	}
	if len(progress) < 2 {
		t.Fatalf("expected progress callbacks, got %v", progress)
	}
	for _, wid := range []string{"378160973", "1909182187"} {
		if _, err := os.Stat(filepath.Join(dstDir, "mods", "workshop-"+wid, "modinfo.lua")); err != nil {
			t.Fatalf("expected modinfo.lua for %s: %v", wid, err)
		}
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

func writeTestZip(t *testing.T, path string, files map[string]string) {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, content := range files {
		w, err := zw.Create(name)
		if err != nil {
			t.Fatalf("create zip entry %s: %v", name, err)
		}
		if _, err := w.Write([]byte(content)); err != nil {
			t.Fatalf("write zip entry %s: %v", name, err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir zip parent: %v", err)
	}
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write zip file: %v", err)
	}
}

func TestInstallWorkshopModDir(t *testing.T) {
	t.Run("extracts legacy bin archive with windows backslash paths", func(t *testing.T) {
		tmp := t.TempDir()
		srcDir := filepath.Join(tmp, "staging", "2007975851")
		destDir := filepath.Join(tmp, "mods", "workshop-2007975851")

		writeTestZip(t, filepath.Join(srcDir, "2007975851_legacy.bin"), map[string]string{
			"modinfo.lua":           `name = "[DST] Legion" version = "6.1.1"`,
			`anim\agronssword.zip`:  "anim-bytes",
			`scripts\prefabs\a.lua`: "return {}",
		})

		archive, err := installWorkshopModDir(srcDir, destDir)
		if err != nil {
			t.Fatalf("installWorkshopModDir legacy: %v", err)
		}
		if filepath.Base(archive) != "2007975851_legacy.bin" {
			t.Fatalf("expected archive 2007975851_legacy.bin, got %q", archive)
		}
		if _, err := os.Stat(filepath.Join(destDir, "modinfo.lua")); err != nil {
			t.Fatalf("expected extracted modinfo.lua: %v", err)
		}
		if got, err := os.ReadFile(filepath.Join(destDir, "anim", "agronssword.zip")); err != nil || string(got) != "anim-bytes" {
			t.Fatalf("expected normalized anim/agronssword.zip, got %q err=%v", string(got), err)
		}
		if _, err := os.Stat(filepath.Join(destDir, "2007975851_legacy.bin")); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("expected raw legacy .bin not to be copied into destDir, err=%v", err)
		}
	})

	t.Run("extracts legacy archive with top-level subfolder", func(t *testing.T) {
		tmp := t.TempDir()
		srcDir := filepath.Join(tmp, "staging", "999999")
		destDir := filepath.Join(tmp, "mods", "workshop-999999")

		writeTestZip(t, filepath.Join(srcDir, "999999_legacy.bin"), map[string]string{
			"my_mod/modinfo.lua": `name = "Wrapped Mod"`,
			"my_mod/modmain.lua": `-- main`,
		})

		if _, err := installWorkshopModDir(srcDir, destDir); err != nil {
			t.Fatalf("installWorkshopModDir wrapped: %v", err)
		}
		if _, err := os.Stat(filepath.Join(destDir, "modinfo.lua")); err != nil {
			t.Fatalf("expected unwrapped modinfo.lua at destDir root: %v", err)
		}
	})

	t.Run("copies standard unpacked UGC mod directly", func(t *testing.T) {
		tmp := t.TempDir()
		srcDir := filepath.Join(tmp, "staging", "1991746508")
		destDir := filepath.Join(tmp, "mods", "workshop-1991746508")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatalf("mkdir src: %v", err)
		}
		if err := os.WriteFile(filepath.Join(srcDir, "modinfo.lua"), []byte(`name = "Myth"`), 0o644); err != nil {
			t.Fatalf("write modinfo: %v", err)
		}

		archive, err := installWorkshopModDir(srcDir, destDir)
		if err != nil {
			t.Fatalf("installWorkshopModDir unpacked: %v", err)
		}
		if archive != "" {
			t.Fatalf("expected empty archive path for unpacked UGC mod, got %q", archive)
		}
		if _, err := os.Stat(filepath.Join(destDir, "modinfo.lua")); err != nil {
			t.Fatalf("expected copied modinfo.lua: %v", err)
		}
	})

	t.Run("errors when directory has neither modinfo nor archive", func(t *testing.T) {
		tmp := t.TempDir()
		srcDir := filepath.Join(tmp, "staging", "empty")
		destDir := filepath.Join(tmp, "mods", "workshop-empty")
		if err := os.MkdirAll(srcDir, 0o755); err != nil {
			t.Fatalf("mkdir empty: %v", err)
		}
		_, err := installWorkshopModDir(srcDir, destDir)
		code, _, _ := apperr.Extract(err, "")
		if code != apperr.CodeWorkshopDownloadMissing {
			t.Fatalf("expected %s, got code=%q err=%v", apperr.CodeWorkshopDownloadMissing, code, err)
		}
	})

	t.Run("rejects archive without modinfo and zip-slip entries", func(t *testing.T) {
		tmp := t.TempDir()
		srcNoInfo := filepath.Join(tmp, "staging", "noinfo")
		writeTestZip(t, filepath.Join(srcNoInfo, "noinfo_legacy.bin"), map[string]string{
			"readme.txt": "no modinfo here",
		})
		_, err := installWorkshopModDir(srcNoInfo, filepath.Join(tmp, "mods", "workshop-noinfo"))
		code, _, _ := apperr.Extract(err, "")
		if code != apperr.CodeModZipMissingModInfo {
			t.Fatalf("expected %s, got code=%q err=%v", apperr.CodeModZipMissingModInfo, code, err)
		}

		srcSlip := filepath.Join(tmp, "staging", "slip")
		writeTestZip(t, filepath.Join(srcSlip, "slip_legacy.bin"), map[string]string{
			"../escape.lua": "evil",
		})
		_, err = installWorkshopModDir(srcSlip, filepath.Join(tmp, "mods", "workshop-slip"))
		code, _, _ = apperr.Extract(err, "")
		if code != apperr.CodeIllegalZipPath {
			t.Fatalf("expected %s, got code=%q err=%v", apperr.CodeIllegalZipPath, code, err)
		}
	})
}
