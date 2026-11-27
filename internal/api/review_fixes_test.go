package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gorilla/websocket"
)

func setupAdmin(t *testing.T, h http.Handler) string {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/api/v1/auth/setup", "", map[string]any{
		"username": "admin",
		"password": "password123",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup failed: %d %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return resp.Token
}

func createUserAndLogin(t *testing.T, h http.Handler, adminTok, name, role string, perms []map[string]any) string {
	t.Helper()
	body := map[string]any{"username": name, "password": "password123", "role": role}
	if perms != nil {
		body["permissions"] = perms
	}
	rec := doJSON(t, h, http.MethodPost, "/api/v1/users", adminTok, body)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create user %s failed: %d %s", name, rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{"username": name, "password": "password123"})
	if rec.Code != http.StatusOK {
		t.Fatalf("login %s failed: %d", name, rec.Code)
	}
	var resp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &resp)
	return resp.Token
}

func createTestCluster(t *testing.T, h http.Handler, adminTok, id string) {
	t.Helper()
	rec := doJSON(t, h, http.MethodPost, "/api/v1/clusters", adminTok, map[string]any{
		"cluster_id": id, "cluster_name": id, "game_mode": "endless", "max_players": 6, "caves_enabled": false,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create cluster %s failed: %d %s", id, rec.Code, rec.Body.String())
	}
}

func TestSetupCannotRunTwice(t *testing.T) {
	h, cleanup := setupTestEngine(t)
	defer cleanup()
	setupAdmin(t, h)
	rec := doJSON(t, h, http.MethodPost, "/api/v1/auth/setup", "", map[string]any{
		"username": "admin2",
		"password": "password123",
	})
	if rec.Code != http.StatusConflict {
		t.Fatalf("second setup should be 409, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestDeletingSoleGrantedClusterKeepsUserRestricted(t *testing.T) {
	h, cleanup := setupTestEngine(t)
	defer cleanup()
	adminTok := setupAdmin(t, h)
	createTestCluster(t, h, adminTok, "alpha")
	createTestCluster(t, h, adminTok, "beta")

	bobTok := createUserAndLogin(t, h, adminTok, "bob", "operator",
		[]map[string]any{{"cluster_id": "alpha", "role": "viewer"}})

	if rec := doJSON(t, h, http.MethodGet, "/api/v1/clusters/beta", bobTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("bob should not see beta before delete, got %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodDelete, "/api/v1/clusters/alpha", adminTok, nil); rec.Code != http.StatusOK {
		t.Fatalf("delete alpha failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodGet, "/api/v1/clusters/beta", bobTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("bob must stay restricted after alpha is deleted, got %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/api/v1/clusters/beta/start", bobTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("bob must not start beta after alpha is deleted, got %d", rec.Code)
	}
}

func TestViewerCannotManageSnippetsOrDownloadMods(t *testing.T) {
	h, cleanup := setupTestEngine(t)
	defer cleanup()
	adminTok := setupAdmin(t, h)
	viewerTok := createUserAndLogin(t, h, adminTok, "guest", "viewer", nil)

	snippet := map[string]any{"name": "save", "shard": "master", "command": "c_save()"}
	if rec := doJSON(t, h, http.MethodPost, "/api/v1/snippets", viewerTok, snippet); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer create snippet should be 403, got %d", rec.Code)
	}
	rec := doJSON(t, h, http.MethodPost, "/api/v1/snippets", adminTok, snippet)
	if rec.Code != http.StatusCreated {
		t.Fatalf("admin create snippet failed: %d %s", rec.Code, rec.Body.String())
	}
	if rec := doJSON(t, h, http.MethodDelete, "/api/v1/snippets/1", viewerTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer delete snippet should be 403, got %d", rec.Code)
	}
	if rec := doJSON(t, h, http.MethodPost, "/api/v1/mods/workshop-123/download", viewerTok, nil); rec.Code != http.StatusForbidden {
		t.Fatalf("viewer mod download should be 403, got %d", rec.Code)
	}
	// Scoped operator grant on any cluster is enough for shared resources.
	createTestCluster(t, h, adminTok, "gamma")
	scopedTok := createUserAndLogin(t, h, adminTok, "scoped", "viewer",
		[]map[string]any{{"cluster_id": "gamma", "role": "operator"}})
	if rec := doJSON(t, h, http.MethodPost, "/api/v1/snippets", scopedTok, snippet); rec.Code != http.StatusCreated {
		t.Fatalf("scoped operator create snippet should succeed, got %d %s", rec.Code, rec.Body.String())
	}
	// Non-numeric workshop IDs are rejected.
	if rec := doJSON(t, h, http.MethodPost, "/api/v1/mods/workshop-..%2Fx/download", adminTok, nil); rec.Code == http.StatusOK {
		t.Fatalf("non-numeric workshop id should be rejected, got %d", rec.Code)
	}
}

func TestConsoleWSRejectsCrossOrigin(t *testing.T) {
	h, cleanup := setupTestEngine(t)
	defer cleanup()
	adminTok := setupAdmin(t, h)
	createTestCluster(t, h, adminTok, "delta")

	ts := httptest.NewServer(h)
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/ws/clusters/delta/logs?shard=master&token=" + adminTok

	hdr := http.Header{"Origin": []string{"http://evil.example"}}
	if conn, _, err := websocket.DefaultDialer.Dial(wsURL, hdr); err == nil {
		conn.Close()
		t.Fatal("cross-origin websocket handshake should be rejected")
	}
	hdr = http.Header{"Origin": []string{ts.URL}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, hdr)
	if err != nil {
		t.Fatalf("same-origin websocket handshake should succeed: %v", err)
	}
	conn.Close()
}
