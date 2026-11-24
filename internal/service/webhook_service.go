package service

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"walrus/internal/apperr"
	"walrus/internal/db"
)

var AllWebhookEvents = []string{
	"server.crash",
	"server.start",
	"server.stop",
	"player.join",
	"player.leave",
	"backup.complete",
	"backup.failed",
	"steam.update_available",
	"steam.update_complete",
	"cron.failed",
}

type WebhookEvent struct {
	Event     string    `json:"event"`
	Title     string    `json:"title"`
	Message   string    `json:"message"`
	ClusterID string    `json:"cluster_id,omitempty"`
	Timestamp time.Time `json:"timestamp"`
}

type WebhookService struct {
	store      *db.Store
	httpClient *http.Client
}

func NewWebhookService(store *db.Store) *WebhookService {
	return &WebhookService{
		store:      store,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// Dispatch fans out an event asynchronously to all enabled webhooks subscribed to it.
func (s *WebhookService) Dispatch(event, title, message, clusterID string) {
	hooks, err := s.store.ListWebhooks()
	if err != nil || len(hooks) == 0 {
		return
	}
	ev := WebhookEvent{
		Event:     event,
		Title:     title,
		Message:   message,
		ClusterID: clusterID,
		Timestamp: time.Now().UTC(),
	}
	for _, h := range hooks {
		if !h.Enabled || !hookMatches(h.Events, event) {
			continue
		}
		hook := h
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			_ = s.deliver(ctx, &hook, ev)
		}()
	}
}

// SendTest synchronously sends a test event to one webhook and records the delivery result.
func (s *WebhookService) SendTest(ctx context.Context, webhookID int64) (*db.WebhookDelivery, error) {
	hook, err := s.store.GetWebhook(webhookID)
	if err != nil {
		return nil, err
	}
	ev := WebhookEvent{
		Event:     "test.ping",
		Title:     "[Walrus] Test Notification",
		Message:   "This is a test webhook notification from Walrus. Your channel is configured correctly!",
		ClusterID: "system",
		Timestamp: time.Now().UTC(),
	}
	deliv := s.deliver(ctx, hook, ev)
	return deliv, nil
}

func (s *WebhookService) deliver(ctx context.Context, hook *db.WebhookConfig, ev WebhookEvent) *db.WebhookDelivery {
	deliv := &db.WebhookDelivery{
		WebhookID: hook.ID,
		Event:     ev.Event,
	}

	targetURL, bodyBytes, headers, err := buildWebhookRequest(hook, ev)
	if err != nil {
		deliv.Error = err.Error()
		s.store.AddWebhookDelivery(deliv)
		return deliv
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, targetURL, bytes.NewReader(bodyBytes))
	if err != nil {
		deliv.Error = err.Error()
		s.store.AddWebhookDelivery(deliv)
		return deliv
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	req.Header.Set("User-Agent", "walrus-webhook/1.0")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := s.httpClient.Do(req)
	if err != nil {
		deliv.Error = err.Error()
		s.store.AddWebhookDelivery(deliv)
		return deliv
	}
	defer resp.Body.Close()

	deliv.StatusCode = resp.StatusCode
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		deliv.Success = true
	} else {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		deliv.Error = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	s.store.AddWebhookDelivery(deliv)
	return deliv
}

func buildWebhookRequest(hook *db.WebhookConfig, ev WebhookEvent) (string, []byte, map[string]string, error) {
	headers := map[string]string{}
	targetURL := strings.TrimSpace(hook.URL)
	if targetURL == "" {
		return "", nil, nil, apperr.New(apperr.CodeWebhookURLEmpty, "webhook URL is empty")
	}

	fullText := ev.Message
	if ev.ClusterID != "" {
		fullText = fmt.Sprintf("[%s] %s", ev.ClusterID, ev.Message)
	}

	var payload any
	switch strings.ToLower(hook.Template) {
	case "discord":
		color := 0xf59e0b // amber
		if strings.Contains(ev.Event, "crash") || strings.Contains(ev.Event, "failed") {
			color = 0xef4444 // red
		} else if strings.Contains(ev.Event, "start") || strings.Contains(ev.Event, "complete") || strings.Contains(ev.Event, "join") {
			color = 0x10b981 // emerald
		}
		payload = map[string]any{
			"username": "Walrus",
			"embeds": []map[string]any{
				{
					"title":       ev.Title,
					"description": fullText,
					"color":       color,
					"timestamp":   ev.Timestamp.Format(time.RFC3339),
					"footer":      map[string]string{"text": "Event: " + ev.Event},
				},
			},
		}

	case "feishu":
		m := map[string]any{
			"msg_type": "text",
			"content": map[string]string{
				"text": fmt.Sprintf("%s\n%s", ev.Title, fullText),
			},
		}
		if hook.Secret != "" {
			ts := strconv.FormatInt(time.Now().Unix(), 10)
			strToSign := ts + "\n" + hook.Secret
			mac := hmac.New(sha256.New, []byte(strToSign))
			m["timestamp"] = ts
			m["sign"] = base64.StdEncoding.EncodeToString(mac.Sum(nil))
		}
		payload = m

	case "dingtalk":
		if hook.Secret != "" {
			ts := strconv.FormatInt(time.Now().UnixMilli(), 10)
			strToSign := ts + "\n" + hook.Secret
			mac := hmac.New(sha256.New, []byte(hook.Secret))
			_, _ = mac.Write([]byte(strToSign))
			sign := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
			sep := "?"
			if strings.Contains(targetURL, "?") {
				sep = "&"
			}
			targetURL = fmt.Sprintf("%s%stimestamp=%s&sign=%s", targetURL, sep, ts, sign)
		}
		payload = map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"title": ev.Title,
				"text":  fmt.Sprintf("### %s\n\n%s", ev.Title, fullText),
			},
		}

	case "wecom":
		payload = map[string]any{
			"msgtype": "markdown",
			"markdown": map[string]string{
				"content": fmt.Sprintf("**%s**\n> %s", ev.Title, fullText),
			},
		}

	case "telegram":
		// Expect URL like https://api.telegram.org/bot<TOKEN>/sendMessage and Secret = chat_id (or chat_id in URL query)
		m := map[string]any{
			"text": fmt.Sprintf("%s\n%s", ev.Title, fullText),
		}
		if hook.Secret != "" {
			m["chat_id"] = hook.Secret
		}
		payload = m

	case "bark":
		payload = map[string]any{
			"title": ev.Title,
			"body":  fullText,
			"group": "walrus",
		}

	default: // generic
		payload = ev
	}

	b, err := json.Marshal(payload)
	if err != nil {
		return "", nil, nil, err
	}
	if strings.ToLower(hook.Template) == "generic" && hook.Secret != "" {
		mac := hmac.New(sha256.New, []byte(hook.Secret))
		_, _ = mac.Write(b)
		headers["X-Walrus-Signature"] = "sha256=" + hex.EncodeToString(mac.Sum(nil))
	}
	return targetURL, b, headers, nil
}

func hookMatches(subscribed []string, event string) bool {
	if len(subscribed) == 0 {
		return true
	}
	for _, s := range subscribed {
		if s == "*" || s == event {
			return true
		}
	}
	return false
}
