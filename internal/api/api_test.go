package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"walrus/internal/apperr"
	"walrus/internal/auth"
	"walrus/internal/config"
	"walrus/internal/db"
	"walrus/internal/dst"
	"walrus/internal/runtime"
	"walrus/internal/service"
	"walrus/internal/steam"
	"walrus/web"
)

func setupTestEngine(t *testing.T) (http.Handler, func()) {
	t.Helper()
	tmp := t.TempDir()
	cfg := &config.Config{
		Addr:        ":0",
		DataDir:     tmp,
		RuntimeMode: "mock",
	}
	if err := cfg.EnsureDirs(); err != nil {
		t.Fatalf("ensure dirs: %v", err)
	}
	store, err := db.Open(cfg.DBPath())
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	fs := dst.NewFS(cfg.ClustersDir())
	driver := runtime.NewMockDriver(cfg.DataDir, fs)
	driver.SetPlayerSeenHook(func(kuID, name, clusterID string) {
		_ = store.UpsertPlayerHistory(kuID, name, clusterID)
	})
	steamMgr := steam.NewManager(cfg.SteamCMDDir(), cfg.DSTServerDir(), driver.Mode())
	workshop := steam.NewWorkshopClient(driver.Mode())
	authSvc := auth.NewService(store, "test-secret-key-123456", false)
	webhookSvc := service.NewWebhookService(store)
	clusterSvc := service.NewClusterService(store, fs, driver)
	backupSvc := service.NewBackupService(cfg.BackupsDir(), store, fs, driver)
	modSvc := service.NewModService(cfg.DSTServerDir(), cfg.LocalModsDir(), store, fs, steamMgr, workshop)
	fileSvc := service.NewFileService(fs)
	cronSvc := service.NewCronService(store, fs, driver, clusterSvc, backupSvc, steamMgr, webhookSvc)
	_ = cronSvc.Start()

	srv := NewServer(cfg, store, authSvc, driver, clusterSvc, backupSvc, modSvc, fileSvc, cronSvc, webhookSvc, steamMgr, web.DistFS())
	return srv.Engine(), func() {
		cronSvc.Stop()
		driver.ShutdownAll(context.Background())
		_ = store.Close()
	}
}

func doJSON(t *testing.T, h http.Handler, method, url, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			t.Fatalf("encode body: %v", err)
		}
	}
	req := httptest.NewRequest(method, url, &buf)
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestEndToEndFlowAndRBAC(t *testing.T) {
	h, cleanup := setupTestEngine(t)
	defer cleanup()

	// 1. Initial status must be uninitialized
	rec := doJSON(t, h, http.MethodGet, "/api/v1/auth/status", "", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var statusResp map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &statusResp)
	if statusResp["initialized"] != false {
		t.Fatalf("expected initialized=false, got %v", statusResp["initialized"])
	}

	// 2. Setup initial admin
	rec = doJSON(t, h, http.MethodPost, "/api/v1/auth/setup", "", map[string]any{
		"username":              "admin",
		"password":              "secret123",
		"default_cluster_token": "pds-g^KU_demo_token_1234",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on setup, got %d: %s", rec.Code, rec.Body.String())
	}
	var setupResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &setupResp)
	adminTok := setupResp.Token
	if adminTok == "" {
		t.Fatal("expected jwt token from setup")
	}

	// 3. Create cluster
	rec = doJSON(t, h, http.MethodPost, "/api/v1/clusters", adminTok, map[string]any{
		"cluster_id":    "friends_s1",
		"cluster_name":  "Friends Endless",
		"game_mode":     "endless",
		"max_players":   8,
		"caves_enabled": true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create cluster, got %d: %s", rec.Code, rec.Body.String())
	}

	// 4. Create viewer user & verify RBAC blocks write operations
	rec = doJSON(t, h, http.MethodPost, "/api/v1/users", adminTok, map[string]any{
		"username": "guest",
		"password": "password123",
		"role":     "viewer",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("expected 201 on create viewer, got %d: %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "guest",
		"password": "password123",
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on viewer login, got %d", rec.Code)
	}
	var loginResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &loginResp)
	viewerTok := loginResp.Token

	// Viewer CAN view cluster
	rec = doJSON(t, h, http.MethodGet, "/api/v1/clusters/friends_s1", viewerTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("viewer should be able to view cluster, got %d", rec.Code)
	}
	// Viewer CANNOT start cluster
	rec = doJSON(t, h, http.MethodPost, "/api/v1/clusters/friends_s1/start", viewerTok, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer should get 403 on start, got %d", rec.Code)
	}
	// Viewer CANNOT view system settings
	rec = doJSON(t, h, http.MethodGet, "/api/v1/system/settings", viewerTok, nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("viewer should get 403 on system settings, got %d", rec.Code)
	}

	// 5. Admin starts cluster, creates backup, checks SPA fallback & API 404
	rec = doJSON(t, h, http.MethodPost, "/api/v1/clusters/friends_s1/start", adminTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin start failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/clusters/friends_s1/backups", adminTok, map[string]any{
		"note": "initial backup",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("backup failed: %d %s", rec.Code, rec.Body.String())
	}

	// Root path & SPA fallback should return 200 index.html (never 301 redirect loop)
	for _, p := range []string{"/", "/index.html", "/clusters/friends_s1/overview"} {
		req := httptest.NewRequest(http.MethodGet, p, nil)
		spaRec := httptest.NewRecorder()
		h.ServeHTTP(spaRec, req)
		if spaRec.Code != http.StatusOK || !bytes.Contains(spaRec.Body.Bytes(), []byte("<!doctype html>")) {
			t.Fatalf("expected 200 SPA index.html for %s, got status=%d", p, spaRec.Code)
		}
	}

	// Unknown API path must return 404 JSON
	api404 := doJSON(t, h, http.MethodGet, "/api/v1/unknown", adminTok, nil)
	if api404.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown API route, got %d", api404.Code)
	}

	// Cleanly verify default token masking in settings
	rec = doJSON(t, h, http.MethodGet, "/api/v1/system/settings", adminTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 on settings, got %d", rec.Code)
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("pds-g^KU_demo_token_1234")) {
		t.Fatal("raw default cluster token leaked in settings response!")
	}

	// 6. Phase 2: Visual world overrides lossless patch
	rec = doJSON(t, h, http.MethodPut, "/api/v1/clusters/friends_s1/world/master/overrides", adminTok, map[string]any{
		"overrides": map[string]any{
			"world_size": "huge",
			"autumn":     "longseason",
		},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("patch world overrides failed: %d %s", rec.Code, rec.Body.String())
	}

	// 7. Phase 2: Add Workshop Mod & inspect parsed modinfo options in zh-CN
	rec = doJSON(t, h, http.MethodPost, "/api/v1/clusters/friends_s1/mods/workshop", adminTok, map[string]any{
		"shard":  "master",
		"input":  "378160973",
		"locale": "zh-CN",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("add workshop mod failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/api/v1/clusters/friends_s1/mods?shard=master&locale=zh-CN", adminTok, nil)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("SHOWPLAYERS")) {
		t.Fatalf("expected parsed modinfo options, got %d %s", rec.Code, rec.Body.String())
	}

	// 8. Phase 2: Player lists, chat, snippets & SteamCMD status
	rec = doJSON(t, h, http.MethodPut, "/api/v1/clusters/friends_s1/players/lists", adminTok, map[string]any{
		"adminlist": []string{"KU_w1ls0n01"},
		"whitelist": []string{},
		"blocklist": []string{},
	})
	if rec.Code != http.StatusOK {
		t.Fatalf("save player lists failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/api/v1/clusters/friends_s1/players", adminTok, nil)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("KU_w1ls0n01")) {
		t.Fatalf("expected KU_w1ls0n01 in player lists, got %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/api/v1/steam/status", adminTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("steam status failed: %d %s", rec.Code, rec.Body.String())
	}

	// 9. Phase 3: Sandboxed File Browser & Path Traversal Protection
	rec = doJSON(t, h, http.MethodGet, "/api/v1/clusters/friends_s1/files", adminTok, nil)
	if rec.Code != http.StatusOK || !bytes.Contains(rec.Body.Bytes(), []byte("cluster.ini")) {
		t.Fatalf("list files failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodGet, "/api/v1/clusters/friends_s1/files/content?path=../../db/walrus.db", adminTok, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected path traversal blocked with 400, got %d", rec.Code)
	}
	rec = doJSON(t, h, http.MethodDelete, "/api/v1/clusters/friends_s1/files?path=cluster.ini", adminTok, nil)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected protected cluster.ini deletion blocked with 400, got %d", rec.Code)
	}

	// 10. Phase 3: Cron Schedule CRUD + Validation + RunNow
	rec = doJSON(t, h, http.MethodPost, "/api/v1/schedules", adminTok, map[string]any{
		"cluster_id": "friends_s1",
		"name":       "Nightly Backup",
		"task_type":  "backup",
		"cron_expr":  "0 4 * * *",
		"params":     map[string]any{"note": "nightly"},
		"enabled":    true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create schedule failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/schedules", adminTok, map[string]any{
		"cluster_id": "friends_s1",
		"name":       "Bad Cron",
		"task_type":  "backup",
		"cron_expr":  "not-a-cron",
		"enabled":    true,
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected invalid cron 400, got %d", rec.Code)
	}

	// 11. Phase 3: Webhook Delivery via real httptest server
	var receivedPayload []byte
	whServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedPayload, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusOK)
	}))
	defer whServer.Close()

	rec = doJSON(t, h, http.MethodPost, "/api/v1/webhooks", adminTok, map[string]any{
		"name":     "Discord Test",
		"template": "discord",
		"url":      whServer.URL,
		"events":   []string{"server.crash", "backup.complete"},
		"enabled":  true,
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create webhook failed: %d %s", rec.Code, rec.Body.String())
	}
	var whResp struct {
		Webhook struct {
			ID int64 `json:"id"`
		} `json:"webhook"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &whResp)
	rec = doJSON(t, h, http.MethodPost, "/api/v1/webhooks/1/test", adminTok, nil)
	if rec.Code != http.StatusOK || !bytes.Contains(receivedPayload, []byte("Walrus")) {
		t.Fatalf("webhook test delivery failed: code=%d payload=%s", rec.Code, string(receivedPayload))
	}

	// 12. Phase 3: Import local client save ZIP with nested Cluster_1/ wrapper & port remapping
	var zipBuf bytes.Buffer
	zw := zip.NewWriter(&zipBuf)
	fw, _ := zw.Create("Cluster_1/cluster.ini")
	_, _ = fw.Write([]byte("[GAMEPLAY]\ngame_mode = survival\nmax_players = 4\n[NETWORK]\ncluster_name = My Local Save\n[SHARD]\nmaster_port = 10990\n"))
	fw2, _ := zw.Create("Cluster_1/Master/server.ini")
	_, _ = fw2.Write([]byte("[NETWORK]\nserver_port = 10990\n[SHARD]\nis_master = true\n"))
	fw3, _ := zw.Create("Cluster_1/Caves/server.ini")
	_, _ = fw3.Write([]byte("[NETWORK]\nserver_port = 10991\n[SHARD]\nis_master = false\nname = Caves\n"))
	_ = zw.Close()

	var mpBuf bytes.Buffer
	mw := multipart.NewWriter(&mpBuf)
	_ = mw.WriteField("cluster_id", "imported_local")
	part, _ := mw.CreateFormFile("file", "Cluster_1.zip")
	_, _ = part.Write(zipBuf.Bytes())
	_ = mw.Close()

	impReq := httptest.NewRequest(http.MethodPost, "/api/v1/clusters/import", &mpBuf)
	impReq.Header.Set("Content-Type", mw.FormDataContentType())
	impReq.Header.Set("Authorization", "Bearer "+adminTok)
	impRec := httptest.NewRecorder()
	h.ServeHTTP(impRec, impReq)
	if impRec.Code != http.StatusCreated || !bytes.Contains(impRec.Body.Bytes(), []byte("My Local Save")) {
		t.Fatalf("cluster zip import failed: %d %s", impRec.Code, impRec.Body.String())
	}
	// Verify ports were remapped away from friends_s1 (which took 10990..10996)
	if bytes.Contains(impRec.Body.Bytes(), []byte(`"server_port":10990`)) {
		t.Fatalf("expected imported cluster ports to be remapped away from 10990: %s", impRec.Body.String())
	}

	_ = time.Now()
	_ = filepath.Clean(".")
}

func TestStructuredErrorCodesAndConsoleWS(t *testing.T) {
	h, cleanup := setupTestEngine(t)
	defer cleanup()

	// Setup admin
	rec := doJSON(t, h, http.MethodPost, "/api/v1/auth/setup", "", map[string]any{
		"username": "admin",
		"password": "password123",
	})
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup failed: %d %s", rec.Code, rec.Body.String())
	}
	var setupResp struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &setupResp)
	adminTok := setupResp.Token

	// 1. Invalid login credentials returns INVALID_CREDENTIALS
	rec = doJSON(t, h, http.MethodPost, "/api/v1/auth/login", "", map[string]any{
		"username": "admin",
		"password": "wrongpassword",
	})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
	var errResp struct {
		Code   string         `json:"code"`
		Error  string         `json:"error"`
		Params map[string]any `json:"params"`
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
	if errResp.Code != apperr.CodeInvalidCredentials || errResp.Error == "" {
		t.Fatalf("unexpected login error payload: %s", rec.Body.String())
	}

	// 2. Create cluster test01 and verify duplicate returns CLUSTER_ALREADY_EXISTS + params
	createPayload := map[string]any{
		"cluster_id":    "test01",
		"cluster_name":  "Test World",
		"game_mode":     "endless",
		"max_players":   6,
		"caves_enabled": false,
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/clusters", adminTok, createPayload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create cluster failed: %d %s", rec.Code, rec.Body.String())
	}
	rec = doJSON(t, h, http.MethodPost, "/api/v1/clusters", adminTok, createPayload)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on duplicate cluster, got %d", rec.Code)
	}
	errResp = struct {
		Code   string         `json:"code"`
		Error  string         `json:"error"`
		Params map[string]any `json:"params"`
	}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
	if errResp.Code != apperr.CodeClusterAlreadyExists || errResp.Params["cluster_id"] != "test01" {
		t.Fatalf("unexpected duplicate cluster error payload: %s", rec.Body.String())
	}

	// 3. HTTP command on stopped shard returns SHARD_NOT_RUNNING + cluster_id + shard
	rec = doJSON(t, h, http.MethodPost, "/api/v1/clusters/test01/command", adminTok, map[string]any{
		"shard":   "master",
		"command": "c_save()",
	})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 on stopped shard command, got %d", rec.Code)
	}
	errResp = struct {
		Code   string         `json:"code"`
		Error  string         `json:"error"`
		Params map[string]any `json:"params"`
	}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
	if errResp.Code != apperr.CodeShardNotRunning || errResp.Params["cluster_id"] != "test01" || errResp.Params["shard"] != "master" {
		t.Fatalf("unexpected shard not running payload: %s", rec.Body.String())
	}

	// 4. Unknown API route returns API_ROUTE_NOT_FOUND
	rec = doJSON(t, h, http.MethodGet, "/api/v1/does-not-exist", adminTok, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	errResp = struct {
		Code   string         `json:"code"`
		Error  string         `json:"error"`
		Params map[string]any `json:"params"`
	}{}
	_ = json.Unmarshal(rec.Body.Bytes(), &errResp)
	if errResp.Code != apperr.CodeAPIRouteNotFound {
		t.Fatalf("unexpected 404 payload: %s", rec.Body.String())
	}

	// 5. WebSocket console: stopped shard returns structured error with req_id; running shard returns command_ack
	ts := httptest.NewServer(h)
	defer ts.Close()
	wsURL := "ws" + strings.TrimPrefix(ts.URL, "http") + "/api/v1/ws/clusters/test01/logs?shard=master&token=" + adminTok
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial failed: %v", err)
	}
	defer conn.Close()

	// Drain initial history & status frames
	for i := 0; i < 2; i++ {
		var m map[string]any
		if err := conn.ReadJSON(&m); err != nil {
			t.Fatalf("read initial ws frame %d: %v", i, err)
		}
	}

	// Send command while stopped
	if err := conn.WriteJSON(map[string]any{"type": "command", "command": "c_save()", "req_id": "cmd-1"}); err != nil {
		t.Fatalf("ws write failed: %v", err)
	}
	var wsErr map[string]any
	if err := conn.ReadJSON(&wsErr); err != nil {
		t.Fatalf("ws read error frame failed: %v", err)
	}
	if wsErr["type"] != "error" || wsErr["req_id"] != "cmd-1" || wsErr["code"] != apperr.CodeShardNotRunning {
		t.Fatalf("unexpected ws error frame: %+v", wsErr)
	}
	wsParams, _ := wsErr["params"].(map[string]any)
	if wsParams["cluster_id"] != "test01" || wsParams["shard"] != "master" {
		t.Fatalf("unexpected ws error params: %+v", wsErr)
	}

	// Start cluster and send command -> expect command_ack with req_id
	rec = doJSON(t, h, http.MethodPost, "/api/v1/clusters/test01/start", adminTok, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("start cluster failed: %d %s", rec.Code, rec.Body.String())
	}
	if err := conn.WriteJSON(map[string]any{"type": "command", "command": "c_save()", "req_id": "cmd-2"}); err != nil {
		t.Fatalf("ws write command while running failed: %v", err)
	}
	gotAck := false
	_ = conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	for i := 0; i < 15; i++ {
		var frame map[string]any
		if err := conn.ReadJSON(&frame); err != nil {
			t.Fatalf("ws read waiting for command_ack: %v", err)
		}
		if frame["type"] == "command_ack" && frame["req_id"] == "cmd-2" {
			gotAck = true
			break
		}
	}
	if !gotAck {
		t.Fatal("did not receive command_ack for cmd-2")
	}
}
