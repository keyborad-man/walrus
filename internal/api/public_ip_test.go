package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestPublicIPResolverFallbackAndCache(t *testing.T) {
	var okHits atomic.Int32
	badSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
		_, _ = w.Write([]byte("error"))
	}))
	defer badSrv.Close()

	ipv6Srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("2001:db8::1\n"))
	}))
	defer ipv6Srv.Close()

	goodSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		okHits.Add(1)
		_, _ = w.Write([]byte(" 203.0.113.42 \n"))
	}))
	defer goodSrv.Close()

	resolver := newPublicIPResolver()
	resolver.endpoints = []string{badSrv.URL, ipv6Srv.URL, goodSrv.URL}

	got := resolver.Get(context.Background())
	if got != "203.0.113.42" {
		t.Fatalf("expected 203.0.113.42, got %q", got)
	}
	if okHits.Load() != 1 {
		t.Fatalf("expected 1 hit to goodSrv, got %d", okHits.Load())
	}

	// Second call must hit cache without calling the upstream again.
	got2 := resolver.Get(context.Background())
	if got2 != "203.0.113.42" {
		t.Fatalf("expected cached IP 203.0.113.42, got %q", got2)
	}
	if okHits.Load() != 1 {
		t.Fatalf("expected cached call not to hit goodSrv again, got %d hits", okHits.Load())
	}
}

func TestListClustersIncludesPasswordAndPublicIPEndpoint(t *testing.T) {
	h, cleanup := setupTestEngine(t)
	defer cleanup()

	// Setup admin user
	setupRec := doJSON(t, h, http.MethodPost, "/api/v1/auth/setup", "", map[string]any{
		"username": "admin",
		"password": "password123",
	})
	if setupRec.Code != http.StatusCreated && setupRec.Code != http.StatusOK {
		t.Fatalf("setup failed: %d %s", setupRec.Code, setupRec.Body.String())
	}
	var setupBody struct {
		Token string `json:"token"`
	}
	_ = json.Unmarshal(setupRec.Body.Bytes(), &setupBody)

	// Create cluster with password
	createRec := doJSON(t, h, http.MethodPost, "/api/v1/clusters", setupBody.Token, map[string]any{
		"cluster_id":       "pw_test",
		"cluster_name":     "Password Room",
		"cluster_password": "secret 123",
		"caves_enabled":    true,
	})
	if createRec.Code != http.StatusCreated {
		t.Fatalf("create cluster failed: %d %s", createRec.Code, createRec.Body.String())
	}

	// List clusters must expose cluster_password for building c_connect
	listRec := doJSON(t, h, http.MethodGet, "/api/v1/clusters", setupBody.Token, nil)
	if listRec.Code != http.StatusOK {
		t.Fatalf("list clusters failed: %d %s", listRec.Code, listRec.Body.String())
	}
	var listBody struct {
		Clusters []struct {
			ClusterID       string `json:"cluster_id"`
			ClusterPassword string `json:"cluster_password"`
		} `json:"clusters"`
	}
	if err := json.Unmarshal(listRec.Body.Bytes(), &listBody); err != nil {
		t.Fatalf("decode clusters: %v", err)
	}
	if len(listBody.Clusters) != 1 || listBody.Clusters[0].ClusterPassword != "secret 123" {
		t.Fatalf("expected cluster_password 'secret 123', got %+v", listBody.Clusters)
	}
}
