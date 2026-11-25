package api

import (
	"context"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

var defaultPublicIPEndpoints = []string{
	"https://4.ipw.cn",
	"https://api.ipify.org",
	"https://ipv4.icanhazip.com",
	"https://ifconfig.me/ip",
}

const (
	publicIPPositiveTTL = 30 * time.Minute
	publicIPNegativeTTL = 2 * time.Minute
	publicIPTimeout     = 3 * time.Second
)

type publicIPResolver struct {
	mu        sync.Mutex
	endpoints []string
	client    *http.Client
	cachedIP  string
	expiresAt time.Time
	inFlight  chan struct{}
}

func newPublicIPResolver() *publicIPResolver {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	// Always query directly so an outbound HTTP proxy does not mask the game server's own IP.
	transport.Proxy = nil
	return &publicIPResolver{
		endpoints: defaultPublicIPEndpoints,
		client: &http.Client{
			Transport: transport,
			Timeout:   publicIPTimeout,
		},
	}
}

func (r *publicIPResolver) Get(ctx context.Context) string {
	for {
		r.mu.Lock()
		if time.Now().Before(r.expiresAt) {
			ip := r.cachedIP
			r.mu.Unlock()
			return ip
		}
		if r.inFlight != nil {
			wait := r.inFlight
			r.mu.Unlock()
			select {
			case <-wait:
				continue
			case <-ctx.Done():
				return ""
			}
		}
		done := make(chan struct{})
		r.inFlight = done
		endpoints := append([]string(nil), r.endpoints...)
		client := r.client
		r.mu.Unlock()

		ip := fetchPublicIPv4(ctx, client, endpoints)
		ttl := publicIPNegativeTTL
		if ip != "" {
			ttl = publicIPPositiveTTL
		}

		r.mu.Lock()
		if ip != "" || ctx.Err() == nil {
			r.cachedIP = ip
			r.expiresAt = time.Now().Add(ttl)
		}
		r.inFlight = nil
		close(done)
		r.mu.Unlock()
		return ip
	}
}

func fetchPublicIPv4(parent context.Context, client *http.Client, endpoints []string) string {
	if len(endpoints) == 0 {
		return ""
	}
	ctx, cancel := context.WithTimeout(parent, publicIPTimeout)
	defer cancel()

	resultCh := make(chan string, len(endpoints))
	var wg sync.WaitGroup
	for _, ep := range endpoints {
		wg.Add(1)
		go func(url string) {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
			if err != nil {
				return
			}
			req.Header.Set("User-Agent", "walrus-dst-panel")
			resp, err := client.Do(req)
			if err != nil {
				return
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				return
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 128))
			if err != nil {
				return
			}
			if ip := parseIPv4(string(body)); ip != "" {
				select {
				case resultCh <- ip:
				default:
				}
			}
		}(ep)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	for ip := range resultCh {
		if ip != "" {
			cancel()
			return ip
		}
	}
	return ""
}

func parseIPv4(raw string) string {
	ip := net.ParseIP(strings.TrimSpace(raw))
	if ip == nil {
		return ""
	}
	v4 := ip.To4()
	if v4 == nil {
		return ""
	}
	return v4.String()
}
