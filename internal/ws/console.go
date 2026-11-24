package ws

import (
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"

	"walrus/internal/apperr"
	"walrus/internal/auth"
	"walrus/internal/db"
	"walrus/internal/runtime"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,
	WriteBufferSize: 4096,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

type ConsoleHandler struct {
	store  *db.Store
	driver runtime.Driver
}

func NewConsoleHandler(store *db.Store, driver runtime.Driver) *ConsoleHandler {
	return &ConsoleHandler{store: store, driver: driver}
}

type clientMessage struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	ReqID   string `json:"req_id,omitempty"`
}

func (h *ConsoleHandler) ServeClusterLogs(c *gin.Context) {
	u := auth.CurrentUser(c)
	clusterID := c.Param("id")
	shard := strings.ToLower(c.DefaultQuery("shard", "master"))
	if shard != "master" && shard != "caves" {
		c.JSON(http.StatusBadRequest, apperr.CodeBody(apperr.CodeInvalidShard, "invalid shard", map[string]any{"shard": shard}))
		return
	}

	effRole, err := h.store.EffectiveClusterRole(u, clusterID)
	if err != nil || effRole.Rank() < db.RoleViewer.Rank() {
		c.JSON(http.StatusForbidden, apperr.CodeBody(apperr.CodeForbidden, "forbidden", nil))
		return
	}

	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var writeMu sync.Mutex
	writeJSON := func(v any) error {
		writeMu.Lock()
		defer writeMu.Unlock()
		_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		return conn.WriteJSON(v)
	}

	// Send initial scrollback and status.
	recent, _ := h.driver.ReadRecentLogs(clusterID, shard, 200)
	if recent == nil {
		recent = []runtime.LogEntry{}
	}
	if err := writeJSON(gin.H{
		"type":    "history",
		"shard":   shard,
		"entries": recent,
	}); err != nil {
		return
	}
	if err := writeJSON(gin.H{
		"type":   "status",
		"shard":  shard,
		"status": h.driver.GetShardStatus(clusterID, shard),
	}); err != nil {
		return
	}

	logCh, unsub := h.driver.SubscribeLogs(clusterID, shard)
	defer unsub()

	done := make(chan struct{})
	// Reader loop (handles client commands + detects close).
	go func() {
		defer close(done)
		conn.SetReadLimit(16 * 1024)
		_ = conn.SetReadDeadline(time.Now().Add(65 * time.Second))
		conn.SetPongHandler(func(string) error {
			_ = conn.SetReadDeadline(time.Now().Add(65 * time.Second))
			return nil
		})
		for {
			var msg clientMessage
			if err := conn.ReadJSON(&msg); err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(65 * time.Second))
			if msg.Type == "command" && strings.TrimSpace(msg.Command) != "" {
				if effRole.Rank() < db.RoleOperator.Rank() {
					payload := apperr.CodeBody(apperr.CodeConsoleOperatorRequired, "operator permission required to send commands", nil)
					payload["type"] = "error"
					if msg.ReqID != "" {
						payload["req_id"] = msg.ReqID
					}
					_ = writeJSON(payload)
					continue
				}
				if err := h.driver.SendCommand(clusterID, shard, msg.Command); err != nil {
					payload := apperr.Body(err, apperr.CodeInternalError)
					payload["type"] = "error"
					if msg.ReqID != "" {
						payload["req_id"] = msg.ReqID
					}
					_ = writeJSON(payload)
				} else {
					h.store.AddAuditLog(u.Username, "console.command", clusterID+"/"+shard, msg.Command, c.ClientIP())
					ack := gin.H{"type": "command_ack"}
					if msg.ReqID != "" {
						ack["req_id"] = msg.ReqID
					}
					_ = writeJSON(ack)
				}
			}
		}
	}()

	statusTick := time.NewTicker(3 * time.Second)
	pingTick := time.NewTicker(25 * time.Second)
	defer statusTick.Stop()
	defer pingTick.Stop()

	for {
		select {
		case <-done:
			return
		case entry, ok := <-logCh:
			if !ok {
				return
			}
			if err := writeJSON(gin.H{
				"type":  "log",
				"shard": shard,
				"entry": entry,
			}); err != nil {
				return
			}
		case <-statusTick.C:
			if err := writeJSON(gin.H{
				"type":   "status",
				"shard":  shard,
				"status": h.driver.GetShardStatus(clusterID, shard),
			}); err != nil {
				return
			}
		case <-pingTick.C:
			writeMu.Lock()
			_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
			err := conn.WriteMessage(websocket.PingMessage, nil)
			writeMu.Unlock()
			if err != nil {
				return
			}
		}
	}
}
