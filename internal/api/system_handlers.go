package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"walrus/internal/apperr"
	"walrus/internal/auth"
	"walrus/internal/db"
)

func (s *Server) handleSystemMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"metrics": s.driver.HostMetrics(),
	})
}

func (s *Server) handleGetSettings(c *gin.Context) {
	st, err := s.store.GetSystemSettings(false)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"settings":     st,
		"runtime_mode": s.driver.Mode(),
		"data_dir":     s.cfg.DataDir,
	})
}

type updateSettingsRequest struct {
	PortPoolStart       *int    `json:"port_pool_start"`
	PortPoolEnd         *int    `json:"port_pool_end"`
	DefaultClusterToken *string `json:"default_cluster_token"` // nil = keep; "" = clear; value = update
	SteamAPIKey         *string `json:"steam_api_key"`
	DSTBranch           *string `json:"dst_branch"`
	DSTArch             *string `json:"dst_arch"`
	AutoUpdateMode      *string `json:"auto_update_mode"`
	BackupMaxRetain     *int    `json:"backup_max_retain"`
}

func (s *Server) handleUpdateSettings(c *gin.Context) {
	var req updateSettingsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}

	cur, err := s.store.GetSystemSettings(true)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}

	start := cur.PortPoolStart
	end := cur.PortPoolEnd
	if req.PortPoolStart != nil {
		start = *req.PortPoolStart
	}
	if req.PortPoolEnd != nil {
		end = *req.PortPoolEnd
	}
	if start < 1024 || end > 65535 || end-start < 6 {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidPortPoolRange, "invalid port pool range (must be within 1024-65535 and span at least 7 ports)")
		return
	}
	_ = s.store.SetSetting("port_pool_start", strconv.Itoa(start))
	_ = s.store.SetSetting("port_pool_end", strconv.Itoa(end))

	if req.DefaultClusterToken != nil {
		_ = s.store.SetSetting("default_cluster_token", strings.TrimSpace(*req.DefaultClusterToken))
	}
	if req.SteamAPIKey != nil {
		_ = s.store.SetSetting("steam_api_key", strings.TrimSpace(*req.SteamAPIKey))
	}
	if req.DSTBranch != nil {
		b := strings.TrimSpace(*req.DSTBranch)
		if b == "public" || b == "updatebeta" {
			_ = s.store.SetSetting("dst_branch", b)
		}
	}
	if req.DSTArch != nil {
		a := strings.TrimSpace(*req.DSTArch)
		if a == "64" || a == "32" {
			_ = s.store.SetSetting("dst_arch", a)
		}
	}
	if req.AutoUpdateMode != nil {
		m := strings.TrimSpace(*req.AutoUpdateMode)
		if m == "manual" || m == "when_empty" || m == "scheduled" {
			_ = s.store.SetSetting("auto_update_mode", m)
		}
	}
	if req.BackupMaxRetain != nil && *req.BackupMaxRetain >= 1 && *req.BackupMaxRetain <= 200 {
		_ = s.store.SetSetting("backup_max_retain", strconv.Itoa(*req.BackupMaxRetain))
	}

	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "system.settings.update", "settings", "Updated global system settings", c.ClientIP())

	updated, _ := s.store.GetSystemSettings(false)
	c.JSON(http.StatusOK, gin.H{"settings": updated})
}

func (s *Server) handleListAuditLogs(c *gin.Context) {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	action := c.Query("action")
	logs, err := s.store.ListAuditLogs(limit, action)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	if logs == nil {
		logs = []db.AuditLog{}
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}
