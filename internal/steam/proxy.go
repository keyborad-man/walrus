package steam

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"walrus/internal/apperr"
)

// ErrProxyChainsUnsupportedScheme is returned when the configured proxy scheme
// (e.g. https://) is valid for Go HTTP clients but unsupported by proxychains4.
var ErrProxyChainsUnsupportedScheme = errors.New("proxychains4 only supports http and socks5/socks5h proxies")

const proxyTestWorkshopID = "378160973"

// ParseProxyURL validates and normalizes a proxy URL.
// Empty input returns (nil, nil) representing direct connection.
// Supported schemes: http, https, socks5, socks5h. Host and numeric port are required.
func ParseProxyURL(raw string) (*url.URL, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return nil, nil
	}
	if strings.ContainsAny(s, "\r\n\t ") {
		return nil, apperr.New(apperr.CodeInvalidProxyURL, "proxy URL must not contain whitespace or control characters")
	}
	u, err := url.Parse(s)
	if err != nil {
		return nil, apperr.WrapDetail(apperr.CodeInvalidProxyURL, err, "invalid proxy URL: %v", err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https", "socks5", "socks5h":
	default:
		return nil, apperr.Newf(
			apperr.CodeInvalidProxyURL,
			map[string]any{"scheme": u.Scheme},
			"unsupported proxy scheme %q (expected http, https, socks5, or socks5h)",
			u.Scheme,
		)
	}
	u.Scheme = scheme

	host := u.Hostname()
	portStr := u.Port()
	if host == "" || portStr == "" {
		return nil, apperr.New(apperr.CodeInvalidProxyURL, "proxy URL must include both host and port (e.g. http://127.0.0.1:7890)")
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 1 || port > 65535 {
		return nil, apperr.New(apperr.CodeInvalidProxyURL, "proxy port must be an integer between 1 and 65535")
	}
	if u.User != nil {
		user := u.User.Username()
		pass, _ := u.User.Password()
		if strings.ContainsAny(user, " \t\r\n") || strings.ContainsAny(pass, " \t\r\n") {
			return nil, apperr.New(apperr.CodeInvalidProxyURL, "proxy credentials must not contain whitespace")
		}
	}
	return u, nil
}

// MaskProxyURL hides any password in a proxy URL while keeping scheme, username, host, and port readable.
func MaskProxyURL(raw string) string {
	s := strings.TrimSpace(raw)
	if s == "" {
		return ""
	}
	u, err := url.Parse(s)
	if err != nil || u.User == nil {
		return s
	}
	if _, hasPass := u.User.Password(); !hasPass {
		return s
	}
	user := u.User.Username()
	cloned := *u
	cloned.User = nil
	// Rebuild manually to avoid url.URL escaping asterisks.
	rest := strings.TrimPrefix(cloned.String(), cloned.Scheme+"://")
	return fmt.Sprintf("%s://%s:********@%s", cloned.Scheme, url.PathEscape(user), rest)
}

// HasProxyAuth reports whether raw contains a proxy password.
func HasProxyAuth(raw string) bool {
	s := strings.TrimSpace(raw)
	if s == "" {
		return false
	}
	u, err := url.Parse(s)
	if err != nil || u.User == nil {
		return false
	}
	_, hasPass := u.User.Password()
	return hasPass
}

// NewHTTPClient creates an http.Client configured with proxyURL (if non-empty) or http.ProxyFromEnvironment.
func NewHTTPClient(proxyURL string, timeout time.Duration) (*http.Client, error) {
	base, ok := http.DefaultTransport.(*http.Transport)
	var tr *http.Transport
	if ok {
		tr = base.Clone()
	} else {
		tr = &http.Transport{}
	}

	u, err := ParseProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}
	if u != nil {
		tr.Proxy = http.ProxyURL(u)
	} else {
		tr.Proxy = http.ProxyFromEnvironment
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: tr,
	}, nil
}

// ProxyTestResult describes the result of a live proxy connectivity test.
type ProxyTestResult struct {
	OK               bool   `json:"ok"`
	LatencyMS        int64  `json:"latency_ms"`
	StatusCode       int    `json:"status_code"`
	Target           string `json:"target"`
	ProxyURL         string `json:"proxy_url"`
	ProxyChainsReady bool   `json:"proxychains_ready"`
	ProxyChainsNote  string `json:"proxychains_note,omitempty"`
}

// TestProxyConnectivity validates proxyURL and sends a lightweight Steam Workshop API request through it.
func TestProxyConnectivity(ctx context.Context, proxyURL string, endpoint string) (*ProxyTestResult, error) {
	u, err := ParseProxyURL(proxyURL)
	if err != nil {
		return nil, err
	}
	if u == nil {
		return nil, apperr.New(apperr.CodeInvalidProxyURL, "proxy URL is empty")
	}

	client, err := NewHTTPClient(u.String(), 10*time.Second)
	if err != nil {
		return nil, err
	}

	target := strings.TrimSpace(endpoint)
	if target == "" {
		target = defaultPublishedFileDetailsURL
	}

	form := url.Values{}
	form.Set("itemcount", "1")
	form.Set("publishedfileids[0]", proxyTestWorkshopID)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, target, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	start := time.Now()
	resp, err := client.Do(req)
	latency := time.Since(start).Milliseconds()
	if err != nil {
		return nil, apperr.WrapDetail(apperr.CodeProxyTestFailed, sanitizeProxyErr(err, u), "proxy connectivity test failed: %v", sanitizeProxyErr(err, u))
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64*1024))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, apperr.Newf(
			apperr.CodeProxyTestFailed,
			map[string]any{"detail": fmt.Sprintf("HTTP %d", resp.StatusCode)},
			"proxy connectivity test returned HTTP %d",
			resp.StatusCode,
		)
	}

	ready, note := detectProxyChainsSupport(u.Scheme)
	return &ProxyTestResult{
		OK:               true,
		LatencyMS:        latency,
		StatusCode:       resp.StatusCode,
		Target:           "Steam Workshop API",
		ProxyURL:         MaskProxyURL(u.String()),
		ProxyChainsReady: ready,
		ProxyChainsNote:  note,
	}, nil
}

func sanitizeProxyErr(err error, u *url.URL) error {
	if err == nil || u == nil || u.User == nil {
		return err
	}
	pass, hasPass := u.User.Password()
	if !hasPass || pass == "" {
		return err
	}
	msg := strings.ReplaceAll(err.Error(), pass, "********")
	if enc := url.QueryEscape(pass); enc != pass {
		msg = strings.ReplaceAll(msg, enc, "********")
	}
	return errors.New(msg)
}

// BuildProxyChainsConf generates a temporary proxychains4 configuration file body for u.
// Because proxychains4 requires literal IP addresses in [ProxyList], hostnames are resolved via lookupIP.
func BuildProxyChainsConf(u *url.URL, lookupIP func(string) ([]net.IP, error)) (string, error) {
	if u == nil {
		return "", apperr.New(apperr.CodeInvalidProxyURL, "proxy URL is empty")
	}
	var pcType string
	switch strings.ToLower(u.Scheme) {
	case "http":
		pcType = "http"
	case "socks5", "socks5h":
		pcType = "socks5"
	case "https":
		return "", ErrProxyChainsUnsupportedScheme
	default:
		return "", apperr.Newf(apperr.CodeInvalidProxyURL, map[string]any{"scheme": u.Scheme}, "unsupported proxy scheme %q", u.Scheme)
	}

	host := u.Hostname()
	port := u.Port()
	ipStr := host
	if net.ParseIP(host) == nil {
		if lookupIP == nil {
			lookupIP = net.LookupIP
		}
		ips, err := lookupIP(host)
		if err != nil || len(ips) == 0 {
			return "", fmt.Errorf("resolve proxy host %q: %w", host, err)
		}
		var chosen net.IP
		for _, ip := range ips {
			if v4 := ip.To4(); v4 != nil {
				chosen = v4
				break
			}
		}
		if chosen == nil {
			chosen = ips[0]
		}
		ipStr = chosen.String()
	}

	proxyLine := fmt.Sprintf("%s %s %s", pcType, ipStr, port)
	if u.User != nil {
		user := u.User.Username()
		pass, hasPass := u.User.Password()
		if user != "" && hasPass {
			proxyLine = fmt.Sprintf("%s %s %s %s %s", pcType, ipStr, port, user, pass)
		}
	}

	var sb strings.Builder
	sb.WriteString("strict_chain\n")
	sb.WriteString("quiet_mode\n")
	sb.WriteString("proxy_dns\n")
	sb.WriteString("remote_dns_subnet 224\n")
	sb.WriteString("tcp_read_time_out 15000\n")
	sb.WriteString("tcp_connect_time_out 8000\n")
	sb.WriteString("localnet 127.0.0.0/255.0.0.0\n")
	sb.WriteString("localnet ::1/128\n")
	sb.WriteString("\n[ProxyList]\n")
	sb.WriteString(proxyLine)
	sb.WriteString("\n")
	return sb.String(), nil
}

// applyProxyEnv sets standard HTTP(S)/ALL proxy environment variables on env.
func applyProxyEnv(env []string, proxyURL string) []string {
	if strings.TrimSpace(proxyURL) == "" {
		return env
	}
	for _, k := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		env = setEnv(env, k, proxyURL)
	}
	return env
}

// multiarchProxyChainsLib returns the bare soname (e.g. "libproxychains4.so") when both
// 32-bit and 64-bit libraries are installed on the host so glibc ld.so loads the right
// ELF class for both 64-bit bash (steamcmd.sh) and 32-bit steamcmd.
func multiarchProxyChainsLib() string {
	candidates := []struct {
		soname string
		lib64  string
		lib32  string
	}{
		{
			soname: "libproxychains4.so",
			lib64:  "/usr/lib/x86_64-linux-gnu/libproxychains4.so",
			lib32:  "/usr/lib/i386-linux-gnu/libproxychains4.so",
		},
		{
			soname: "libproxychains.so.4",
			lib64:  "/usr/lib/x86_64-linux-gnu/libproxychains.so.4",
			lib32:  "/usr/lib/i386-linux-gnu/libproxychains.so.4",
		},
	}
	for _, c := range candidates {
		if _, err32 := os.Stat(c.lib32); err32 == nil {
			if _, err64 := os.Stat(c.lib64); err64 == nil {
				return c.soname
			}
		}
	}
	return ""
}

func findProxyChainsBinary() string {
	for _, name := range []string{"proxychains4", "proxychains"} {
		if p, err := exec.LookPath(name); err == nil {
			return p
		}
	}
	return ""
}

// ProxyChainsAvailable reports whether proxychains4 (multiarch preload or CLI binary) is available.
func ProxyChainsAvailable() bool {
	return multiarchProxyChainsLib() != "" || findProxyChainsBinary() != ""
}

func detectProxyChainsSupport(scheme string) (bool, string) {
	if strings.EqualFold(scheme, "https") {
		return false, "https_scheme_unsupported_by_proxychains"
	}
	if multiarchProxyChainsLib() != "" {
		return true, ""
	}
	if findProxyChainsBinary() != "" {
		return true, ""
	}
	return false, "proxychains_not_installed"
}
