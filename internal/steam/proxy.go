package steam

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"walrus/internal/apperr"
)

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
	OK         bool   `json:"ok"`
	LatencyMS  int64  `json:"latency_ms"`
	StatusCode int    `json:"status_code"`
	Target     string `json:"target"`
	ProxyURL   string `json:"proxy_url"`
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

	return &ProxyTestResult{
		OK:         true,
		LatencyMS:  latency,
		StatusCode: resp.StatusCode,
		Target:     "Steam Workshop API",
		ProxyURL:   MaskProxyURL(u.String()),
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
