package updater

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"walrus/internal/version"
)

func TestSplitImageRefAndValidation(t *testing.T) {
	cases := []struct {
		ref      string
		wantRepo string
		wantTag  string
	}{
		{"ghcr.io/keyborad-man/walrus:latest", "ghcr.io/keyborad-man/walrus", "latest"},
		{"ghcr.io/keyborad-man/walrus:v0.2.0", "ghcr.io/keyborad-man/walrus", "v0.2.0"},
		{"ghcr.io/keyborad-man/walrus", "ghcr.io/keyborad-man/walrus", "latest"},
		{"localhost:5000/walrus:1.0", "localhost:5000/walrus", "1.0"},
		{"localhost:5000/walrus", "localhost:5000/walrus", "latest"},
		{"ghcr.io/a/b:v1@sha256:abcdef", "ghcr.io/a/b", "v1"},
	}
	for _, tc := range cases {
		repo, tag := SplitImageRef(tc.ref)
		if repo != tc.wantRepo || tag != tc.wantTag {
			t.Errorf("SplitImageRef(%q) = (%q, %q), want (%q, %q)", tc.ref, repo, tag, tc.wantRepo, tc.wantTag)
		}
	}

	if !IsValidImageRef("ghcr.io/keyborad-man/walrus:v0.2.0") {
		t.Fatal("expected valid image ref")
	}
	if IsValidImageRef("ghcr.io/a/b; rm -rf /") || IsValidImageRef("../bad") || IsValidImageRef("") {
		t.Fatal("expected invalid image refs to be rejected")
	}
}

func TestCheckUpdateGitHubReleaseAndGHCRFallback(t *testing.T) {
	// 1. GitHub Releases returns v0.2.0 while current is v0.1.0 -> update available
	gh := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/owner/walrus/releases/latest":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"tag_name":     "v0.2.0",
				"html_url":     "https://github.com/owner/walrus/releases/tag/v0.2.0",
				"body":         "Added one-click Docker upgrade",
				"published_at": "2026-11-24T08:00:00Z",
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer gh.Close()

	svc := NewService("owner/walrus", "ghcr.io/owner/walrus", "/var/run/docker.sock", "mock")
	svc.githubAPIBase = gh.URL
	svc.detectInDocker = func() bool { return false }
	svc.currentInfo = func() version.Info {
		return version.Info{Version: "v0.1.0", Commit: "abc1234", BuildTime: "2026-11-20T00:00:00Z"}
	}

	st, err := svc.CheckUpdate(context.Background())
	if err != nil {
		t.Fatalf("CheckUpdate failed: %v", err)
	}
	if !st.UpdateAvailable || st.LatestVersion != "v0.2.0" || st.Source != "github_release" {
		t.Fatalf("unexpected release check status: %+v", st)
	}
	if st.Docker.TargetImage != "ghcr.io/owner/walrus:v0.2.0" {
		t.Fatalf("expected target image ghcr.io/owner/walrus:v0.2.0, got %q", st.Docker.TargetImage)
	}

	// 2. Fallback to GHCR latest digest when GitHub Releases/Tags return 404
	gh404 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer gh404.Close()

	ghcr := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/token":
			_ = json.NewEncoder(w).Encode(map[string]string{"token": "anon-tok"})
		case "/v2/owner/walrus/manifests/latest":
			w.Header().Set("Docker-Content-Digest", "sha256:newdigest999")
			w.WriteHeader(http.StatusOK)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer ghcr.Close()

	dockerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/_ping":
			_, _ = w.Write([]byte("OK"))
		case "/containers/abcdef123456/json":
			_ = json.NewEncoder(w).Encode(ContainerInspect{
				ID:     "abcdef1234567890abcdef1234567890abcdef1234567890abcdef1234567890",
				Name:   "/walrus",
				Image:  "sha256:localimgid",
				Config: &ContainerConfig{Image: "ghcr.io/owner/walrus:latest"},
			})
		case "/images/sha256:localimgid/json":
			_ = json.NewEncoder(w).Encode(ImageInspect{
				ID:          "sha256:localimgid",
				RepoDigests: []string{"ghcr.io/owner/walrus@sha256:olddigest111"},
			})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer dockerSrv.Close()

	svc2 := NewService("owner/walrus", "ghcr.io/owner/walrus", "/var/run/docker.sock", "linux")
	svc2.githubAPIBase = gh404.URL
	svc2.ghcrBase = ghcr.URL
	svc2.dockerClient = NewDockerClientWithHTTP(dockerSrv.URL, dockerSrv.Client())
	svc2.detectInDocker = func() bool { return true }
	svc2.detectSelfID = func() string { return "abcdef123456" }
	svc2.currentInfo = func() version.Info {
		return version.Info{Version: "main-1111111"}
	}

	st2, err := svc2.CheckUpdate(context.Background())
	if err != nil {
		t.Fatalf("GHCR fallback CheckUpdate failed: %v", err)
	}
	if !st2.UpdateAvailable || st2.Source != "ghcr_latest" || st2.RemoteDigest != "sha256:newdigest999" || st2.LocalDigest != "sha256:olddigest111" {
		t.Fatalf("unexpected GHCR fallback status: %+v", st2)
	}
	if !st2.Docker.CanSelfUpgrade || st2.Docker.TargetImage != "ghcr.io/owner/walrus:latest" {
		t.Fatalf("unexpected docker capability: %+v", st2.Docker)
	}
}

func TestExecuteContainerSwapAndRollback(t *testing.T) {
	var (
		mu      sync.Mutex
		actions []string
		failNew bool
	)

	dockerSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		actions = append(actions, r.Method+" "+r.URL.Path)
		shouldFail := failNew
		mu.Unlock()

		switch {
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/stop"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/rename"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && r.URL.Path == "/containers/create":
			_ = json.NewEncoder(w).Encode(map[string]string{"Id": "new-container-id"})
		case r.Method == http.MethodPost && r.URL.Path == "/containers/new-container-id/start":
			if shouldFail {
				w.WriteHeader(http.StatusInternalServerError)
				_ = json.NewEncoder(w).Encode(map[string]string{"message": "port already allocated"})
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodPost && strings.HasSuffix(r.URL.Path, "/start"):
			w.WriteHeader(http.StatusNoContent)
		case r.Method == http.MethodDelete:
			w.WriteHeader(http.StatusNoContent)
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer dockerSrv.Close()

	client := NewDockerClientWithHTTP(dockerSrv.URL, dockerSrv.Client())
	spec := UpgradeSpec{
		OldContainerID: "old-id",
		ContainerName:  "walrus",
		TargetImage:    "ghcr.io/owner/walrus:v0.2.0",
		DockerSock:     "/var/run/docker.sock",
		CreatePayload:  map[string]any{"Image": "ghcr.io/owner/walrus:v0.2.0"},
	}

	// Happy path
	if err := ExecuteContainerSwap(context.Background(), client, spec, 1*time.Millisecond); err != nil {
		t.Fatalf("ExecuteContainerSwap happy path failed: %v", err)
	}
	wantHappy := []string{
		"POST /containers/old-id/stop",
		"POST /containers/old-id/rename",
		"POST /containers/create",
		"POST /containers/new-container-id/start",
		"DELETE /containers/old-id",
	}
	if len(actions) != len(wantHappy) {
		t.Fatalf("unexpected happy path actions: got %v, want %v", actions, wantHappy)
	}
	for i := range wantHappy {
		if actions[i] != wantHappy[i] {
			t.Fatalf("action[%d] = %q, want %q", i, actions[i], wantHappy[i])
		}
	}

	// Failure path -> must rollback by removing new container, renaming old back, and restarting old
	mu.Lock()
	actions = nil
	failNew = true
	mu.Unlock()

	if err := ExecuteContainerSwap(context.Background(), client, spec, 1*time.Millisecond); err == nil {
		t.Fatal("expected error when starting new container fails")
	}
	joined := strings.Join(actions, " | ")
	if !strings.Contains(joined, "DELETE /containers/new-container-id") ||
		!strings.Contains(joined, "POST /containers/old-id/start") {
		t.Fatalf("expected rollback actions to remove new container and restart old container, got: %s", joined)
	}
}
