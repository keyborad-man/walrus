package ws

import (
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"walrus/internal/apperr"
	"walrus/internal/auth"
	"walrus/internal/db"
	"walrus/internal/steam"
)

type SteamHandler struct {
	store    *db.Store
	steamMgr *steam.Manager
	upgrader *websocket.Upgrader
}

func NewSteamHandler(store *db.Store, steamMgr *steam.Manager, devMode bool) *SteamHandler {
	return &SteamHandler{store: store, steamMgr: steamMgr, upgrader: newUpgrader(devMode)}
}

func (h *SteamHandler) ServeSteamLogs(c *gin.Context) {
	u := auth.CurrentUser(c)
	if u == nil || u.Role != db.RoleAdmin {
		c.JSON(http.StatusForbidden, apperr.CodeBody(apperr.CodeAdminRoleRequired, "admin role required", nil))
		return
	}

	conn, err := h.upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var mu sync.Mutex
	writeJSON := func(v any) error {
		mu.Lock()
		defer mu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteJSON(v)
	}

	branch := "public"
	if st, err := h.store.GetSystemSettings(false); err == nil && st.DSTBranch != "" {
		branch = st.DSTBranch
	}

	ch, unsub := h.steamMgr.Subscribe()
	defer unsub()

	if err := writeJSON(gin.H{
		"type":   "history",
		"logs":   h.steamMgr.RecentLogs(),
		"status": h.steamMgr.Status(branch),
	}); err != nil {
		return
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		conn.SetReadLimit(4096)
		_ = conn.SetReadDeadline(time.Now().Add(65 * time.Second))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(65 * time.Second))
			return nil
		})
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				return
			}
		}
	}()

	statusTick := time.NewTicker(2 * time.Second)
	pingTick := time.NewTicker(25 * time.Second)
	defer statusTick.Stop()
	defer pingTick.Stop()

	for {
		select {
		case <-done:
			return
		case line, ok := <-ch:
			if !ok {
				return
			}
			if err := writeJSON(gin.H{"type": "log", "entry": line}); err != nil {
				return
			}
		case <-statusTick.C:
			if st, err := h.store.GetSystemSettings(false); err == nil && st.DSTBranch != "" {
				branch = st.DSTBranch
			}
			if err := writeJSON(gin.H{"type": "status", "status": h.steamMgr.Status(branch)}); err != nil {
				return
			}
		case <-pingTick.C:
			mu.Lock()
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			err := conn.WriteMessage(websocket.PingMessage, nil)
			mu.Unlock()
			if err != nil {
				return
			}
		}
	}
}
