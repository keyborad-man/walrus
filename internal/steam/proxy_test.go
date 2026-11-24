package steam

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestParseAndMaskProxyURL(t *testing.T) {
	validCases := []struct {
		in       string
		wantMask string
		wantAuth bool
	}{
		{"http://127.0.0.1:7890", "http://127.0.0.1:7890", false},
		{"https://proxy.example.com:8443", "https://proxy.example.com:8443", false},
		{"socks5://127.0.0.1:1080", "socks5://127.0.0.1:1080", false},
		{"socks5h://user:secret@127.0.0.1:1080", "socks5h://user:********@127.0.0.1:1080", true},
		{"http://[::1]:7890", "http://[::1]:7890", false},
	}
	for _, tc := range validCases {
		u, err := ParseProxyURL(tc.in)
		if err != nil || u == nil {
			t.Fatalf("ParseProxyURL(%q) unexpected error: %v", tc.in, err)
		}
		if got := MaskProxyURL(tc.in); got != tc.wantMask {
			t.Errorf("MaskProxyURL(%q) = %q, want %q", tc.in, got, tc.wantMask)
		}
		if got := HasProxyAuth(tc.in); got != tc.wantAuth {
			t.Errorf("HasProxyAuth(%q) = %v, want %v", tc.in, got, tc.wantAuth)
		}
	}

	if u, err := ParseProxyURL("   "); err != nil || u != nil {
		t.Fatalf("empty proxy URL should return (nil, nil), got (%v, %v)", u, err)
	}

	invalidCases := []string{
		"ftp://127.0.0.1:21",
		"127.0.0.1:7890",
		"http://127.0.0.1",
		"http://127.0.0.1:99999",
		"http://127.0.0.1:0",
		"http://127.0.0.1:7890\nhttp 1.2.3.4 80",
		"http://user pass@127.0.0.1:7890",
	}
	for _, bad := range invalidCases {
		if _, err := ParseProxyURL(bad); err == nil {
			t.Errorf("ParseProxyURL(%q) expected error, got nil", bad)
		}
	}
}

func TestBuildProxyChainsConf(t *testing.T) {
	mockLookup := func(host string) ([]net.IP, error) {
		if host == "clash.local" {
			return []net.IP{net.ParseIP("2001:db8::1"), net.ParseIP("10.0.0.5")}, nil
		}
		return nil, errors.New("no such host")
	}

	u, err := ParseProxyURL("socks5h://alice:p4ss@clash.local:7891")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	conf, err := BuildProxyChainsConf(u, mockLookup)
	if err != nil {
		t.Fatalf("BuildProxyChainsConf: %v", err)
	}
	if !strings.Contains(conf, "socks5 10.0.0.5 7891 alice p4ss\n") {
		t.Fatalf("expected resolved IPv4 socks5 entry in conf:\n%s", conf)
	}
	if !strings.Contains(conf, "proxy_dns\n") || !strings.Contains(conf, "quiet_mode\n") {
		t.Fatalf("expected proxy_dns and quiet_mode in conf:\n%s", conf)
	}

	uHTTP, _ := ParseProxyURL("http://127.0.0.1:7890")
	confHTTP, err := BuildProxyChainsConf(uHTTP, mockLookup)
	if err != nil {
		t.Fatalf("BuildProxyChainsConf http: %v", err)
	}
	if !strings.Contains(confHTTP, "http 127.0.0.1 7890\n") {
		t.Fatalf("expected http entry in conf:\n%s", confHTTP)
	}

	uHTTPS, _ := ParseProxyURL("https://127.0.0.1:8443")
	if _, err := BuildProxyChainsConf(uHTTPS, mockLookup); !errors.Is(err, ErrProxyChainsUnsupportedScheme) {
		t.Fatalf("expected ErrProxyChainsUnsupportedScheme for https://, got %v", err)
	}
}

func TestWorkshopAndSteamManagerRouteThroughProxy(t *testing.T) {
	var proxyHits atomic.Int32
	proxySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		proxyHits.Add(1)
		// Act as a mock HTTP proxy directly answering the proxied HTTP target.
		switch {
		case strings.Contains(r.URL.String(), "details"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"response": map[string]any{
					"publishedfiledetails": []map[string]any{
						{
							"publishedfileid": "378160973",
							"result":          1,
							"title":           "Proxied Global Positions",
						},
					},
				},
			})
		case strings.Contains(r.URL.String(), "query"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"response": map[string]any{
					"total": 1,
					"publishedfiledetails": []map[string]any{
						{
							"publishedfileid":   "378160973",
							"title":             "Proxied Search Result",
							"short_description": "via proxy",
						},
					},
				},
			})
		default:
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer proxySrv.Close()

	// 1. WorkshopClient routes both GetPublishedFileDetails and SearchWorkshop through configured proxy.
	client := NewWorkshopClient("linux")
	client.detailsEndpoint = "http://steam-api.invalid/details"
	client.queryEndpoint = "http://steam-api.invalid/query"
	if err := client.SetProxy(proxySrv.URL); err != nil {
		t.Fatalf("SetProxy: %v", err)
	}

	items, err := client.GetPublishedFileDetails(context.Background(), []string{"378160973"})
	if err != nil || len(items) != 1 || items[0].Title != "Proxied Global Positions" {
		t.Fatalf("expected proxied workshop details, got items=%+v err=%v", items, err)
	}

	searchItems, total, isMock, err := client.SearchWorkshop(context.Background(), "test-key", "global", 1)
	if err != nil || isMock || total != 1 || len(searchItems) != 1 || searchItems[0].Title != "Proxied Search Result" {
		t.Fatalf("expected proxied workshop search, got items=%+v total=%d isMock=%v err=%v", searchItems, total, isMock, err)
	}

	// 2. TestProxyConnectivity works through the proxy.
	res, err := TestProxyConnectivity(context.Background(), proxySrv.URL, "http://steam-api.invalid/details")
	if err != nil || !res.OK || res.StatusCode != http.StatusOK {
		t.Fatalf("expected proxy connectivity test OK, got res=%+v err=%v", res, err)
	}

	if proxyHits.Load() != 3 {
		t.Fatalf("expected 3 requests routed via proxy, got %d", proxyHits.Load())
	}

	// 3. SteamCMD Manager sets proxy env and masks password in published logs.
	tmp := t.TempDir()
	mgr := NewManager(filepath.Join(tmp, "steamcmd"), filepath.Join(tmp, "dst-server"), "linux")
	secretProxy := "http://bob:SuperSecretPass@127.0.0.1:7890"
	if err := mgr.SetProxy(secretProxy); err != nil {
		t.Fatalf("mgr.SetProxy: %v", err)
	}
	cmd, cleanup, err := mgr.prepareSteamCMDCommand([]string{"+quit"}, mgr.steamCMDHomeDir(), mgr.ProxyURL())
	if err != nil {
		t.Fatalf("prepareSteamCMDCommand: %v", err)
	}
	if cleanup != nil {
		defer cleanup()
	}

	hasHTTPProxy := false
	hasAllProxy := false
	for _, e := range cmd.Env {
		if e == "HTTP_PROXY="+secretProxy {
			hasHTTPProxy = true
		}
		if e == "ALL_PROXY="+secretProxy {
			hasAllProxy = true
		}
	}
	if !hasHTTPProxy || !hasAllProxy {
		t.Fatalf("expected HTTP_PROXY and ALL_PROXY injected into SteamCMD env")
	}

	for _, line := range mgr.RecentLogs() {
		if strings.Contains(line.Message, "SuperSecretPass") {
			t.Fatalf("proxy password leaked into SteamCMD logs: %s", line.Message)
		}
	}
}
