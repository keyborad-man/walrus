package steam

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSearchWorkshopCuratedWithoutAPIKeyInLinuxMode(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		if err := r.ParseForm(); err != nil {
			t.Errorf("parse form: %v", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		var details []map[string]any
		for k, vals := range r.PostForm {
			if len(k) > 16 && k[:16] == "publishedfileids" && len(vals) > 0 {
				id := vals[0]
				details = append(details, map[string]any{
					"publishedfileid": id,
					"result":          1,
					"title":           "Steam Title " + id,
					"description":     "[h1]BBCode[/h1]",
					"preview_url":     "https://steam.example/preview/" + id + ".jpg",
					"time_updated":    1700000000,
					"subscriptions":   999000,
					"tags":            []map[string]string{{"tag": "all_clients_require_mod"}},
				})
			}
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"response": map[string]any{
				"publishedfiledetails": details,
			},
		})
	}))
	defer srv.Close()

	client := NewWorkshopClient("linux")
	client.detailsEndpoint = srv.URL

	// 1. Empty query in linux mode without API key returns enriched curated catalog.
	items, total, isCurated, err := client.SearchWorkshop(context.Background(), "", "", 1)
	if err != nil {
		t.Fatalf("expected curated catalog without error, got %v", err)
	}
	if !isCurated {
		t.Fatalf("expected isCurated=true")
	}
	if total != len(curatedPopularMods) || len(items) != len(curatedPopularMods) {
		t.Fatalf("expected %d curated items, got len=%d total=%d", len(curatedPopularMods), len(items), total)
	}
	if items[0].PreviewURL != "https://steam.example/preview/378160973.jpg" {
		t.Fatalf("expected enriched preview URL, got %q", items[0].PreviewURL)
	}
	if items[0].Title != curatedPopularMods[0].Title {
		t.Fatalf("expected curated bilingual title preserved, got %q", items[0].Title)
	}
	if hits.Load() != 1 {
		t.Fatalf("expected 1 Steam details request, got %d", hits.Load())
	}

	// 2. Keyword filter hits cache without extra Steam HTTP call.
	filtered, total2, isCurated2, err := client.SearchWorkshop(context.Background(), "", "血条", 1)
	if err != nil || !isCurated2 {
		t.Fatalf("unexpected filter result: err=%v isCurated=%v", err, isCurated2)
	}
	if total2 != 2 || len(filtered) != 2 {
		t.Fatalf("expected 2 healthbar mods matching '血条', got %d", len(filtered))
	}
	if hits.Load() != 1 {
		t.Fatalf("expected cached curated catalog (still 1 hit), got %d", hits.Load())
	}

	// 3. Direct Workshop ID or Steam URL lookup works without API key.
	direct, total3, isCurated3, err := client.SearchWorkshop(
		context.Background(),
		"",
		"https://steamcommunity.com/sharedfiles/filedetails/?id=1392778117",
		1,
	)
	if err != nil || !isCurated3 {
		t.Fatalf("unexpected direct URL lookup: err=%v isCurated=%v", err, isCurated3)
	}
	if total3 != 1 || len(direct) != 1 || direct[0].WorkshopID != "1392778117" {
		t.Fatalf("expected direct lookup for 1392778117, got %+v", direct)
	}
}

func TestSearchWorkshopCuratedFallbackWhenSteamOffline(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer srv.Close()

	client := NewWorkshopClient("linux")
	client.detailsEndpoint = srv.URL

	items, total, isCurated, err := client.SearchWorkshop(context.Background(), "", "", 1)
	if err != nil {
		t.Fatalf("expected offline fallback without error, got %v", err)
	}
	if !isCurated || total != len(curatedPopularMods) || len(items) != len(curatedPopularMods) {
		t.Fatalf("unexpected fallback result: len=%d total=%d isCurated=%v", len(items), total, isCurated)
	}
}

func TestSearchWorkshopDefaultKeyOverrideAndGracefulFallback(t *testing.T) {
	var gotKey, gotQueryType string
	querySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.URL.Query().Get("key")
		gotQueryType = r.URL.Query().Get("query_type")
		if gotKey == "revoked-default-key" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		if gotKey == "bad-user-key" {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"response": map[string]any{
				"total": 1,
				"publishedfiledetails": []map[string]any{
					{
						"publishedfileid":   "378160973",
						"title":             "Global Positions",
						"short_description": "Live Steam search result",
						"preview_url":       "https://steam.example/gp.jpg",
						"subscriptions":     8000000,
					},
				},
			},
		})
	}))
	defer querySrv.Close()

	detailsSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
	}))
	defer detailsSrv.Close()

	client := NewWorkshopClient("linux")
	client.queryEndpoint = querySrv.URL
	client.detailsEndpoint = detailsSrv.URL
	client.SetDefaultAPIKey("default-valid-key")

	// 1. Empty user key falls back to default key; empty query uses query_type=9 (popular).
	items, total, isCurated, err := client.SearchWorkshop(context.Background(), "", "", 1)
	if err != nil || isCurated || total != 1 || len(items) != 1 {
		t.Fatalf("expected live popular search via default key, got len=%d total=%d isCurated=%v err=%v", len(items), total, isCurated, err)
	}
	if gotKey != "default-valid-key" || gotQueryType != "9" {
		t.Fatalf("expected key=default-valid-key query_type=9, got key=%q query_type=%q", gotKey, gotQueryType)
	}

	// 2. User custom key overrides default key; text query uses query_type=12.
	items, _, isCurated, err = client.SearchWorkshop(context.Background(), "custom-user-key", "map", 1)
	if err != nil || isCurated || len(items) != 1 {
		t.Fatalf("expected custom user key search, got len=%d isCurated=%v err=%v", len(items), isCurated, err)
	}
	if gotKey != "custom-user-key" || gotQueryType != "12" {
		t.Fatalf("expected key=custom-user-key query_type=12, got key=%q query_type=%q", gotKey, gotQueryType)
	}

	// 3. Invalid custom user key returns explicit error (does not silently swallow).
	if _, _, _, err := client.SearchWorkshop(context.Background(), "bad-user-key", "map", 1); err == nil {
		t.Fatal("expected error for rejected user API key, got nil")
	}

	// 4. Rejected default key degrades gracefully to curated catalog instead of failing.
	client.SetDefaultAPIKey("revoked-default-key")
	items, total, isCurated, err = client.SearchWorkshop(context.Background(), "", "", 1)
	if err != nil || !isCurated || total != len(curatedPopularMods) || len(items) != len(curatedPopularMods) {
		t.Fatalf("expected graceful fallback to curated catalog on revoked default key, got len=%d total=%d isCurated=%v err=%v", len(items), total, isCurated, err)
	}
}
