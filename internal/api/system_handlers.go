package api

import (
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"walrus/internal/apperr"
	"walrus/internal/auth"
	"walrus/internal/db"
	"walrus/internal/steam"
	"walrus/internal/updater"
)

// ResolveEffectiveProxy determines whether the panel-managed proxy is active and returns its raw URL.
// Precedence: explicit system settings (enabled/disabled + URL) > startup default WALRUS_PROXY_URL.
func ResolveEffectiveProxy(st *db.SystemSettings, defaultProxyURL string) (bool, string) {
	def := strings.TrimSpace(defaultProxyURL)
	if st == nil {
		return def != "", def
	}
	enabled := st.ProxyEnabled
	if !st.ProxyEnabledSet {
		enabled = def != ""
	}
	if !enabled {
		return false, ""
	}
	if raw := strings.TrimSpace(st.ProxyURL); raw != "" {
		return true, raw
	}
	return def != "", def
}

func (s *Server) decoratePublicSettings(st *db.SystemSettings) {
	if st == nil {
		return
	}
	st.HasDefaultSteamAPIKey = strings.TrimSpace(s.cfg.SteamAPIKey) != ""
	defProxy := strings.TrimSpace(s.cfg.ProxyURL)
	st.HasDefaultProxyURL = defProxy != ""
	if defProxy != "" {
		st.DefaultProxyURL, _ = db.MaskProxyURL(defProxy)
	}
	if !st.ProxyEnabledSet && defProxy != "" {
		st.ProxyEnabled = true
	}
}

func (s *Server) applyActiveProxy(st *db.SystemSettings) error {
	_, effective := ResolveEffectiveProxy(st, s.cfg.ProxyURL)
	if err := s.steamMgr.SetProxy(effective); err != nil {
		return err
	}
	if err := s.modSvc.SetProxy(effective); err != nil {
		return err
	}
	return nil
}

func (s *Server) handleSystemMetrics(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"metrics": s.driver.HostMetrics(),
	})
}

func (s *Server) handleSystemPublicIP(c *gin.Context) {
	ip := ""
	if s.publicIP != nil {
		ip = s.publicIP.Get(c.Request.Context())
	}
	c.JSON(http.StatusOK, gin.H{
		"ip": ip,
	})
}

func (s *Server) handleGetSettings(c *gin.Context) {
	st, err := s.store.GetSystemSettings(false)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	s.decoratePublicSettings(st)
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
	ProxyEnabled        *bool   `json:"proxy_enabled"`
	ProxyURL            *string `json:"proxy_url"` // nil = keep; "" = clear; value = update
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

	// Validate proxy settings before persisting any mutations.
	nextProxyURL := cur.ProxyURL
	if req.ProxyURL != nil {
		trimmed := strings.TrimSpace(*req.ProxyURL)
		if trimmed != "" {
			parsed, err := steam.ParseProxyURL(trimmed)
			if err != nil {
				respondError(c, http.StatusBadRequest, err)
				return
			}
			nextProxyURL = parsed.String()
		} else {
			nextProxyURL = ""
		}
	}
	nextProxyEnabled := cur.ProxyEnabled
	nextProxyEnabledSet := cur.ProxyEnabledSet
	if req.ProxyEnabled != nil {
		nextProxyEnabled = *req.ProxyEnabled
		nextProxyEnabledSet = true
	}
	effectiveEnabled := nextProxyEnabled
	if !nextProxyEnabledSet {
		effectiveEnabled = strings.TrimSpace(s.cfg.ProxyURL) != ""
	}
	if effectiveEnabled && strings.TrimSpace(nextProxyURL) == "" && strings.TrimSpace(s.cfg.ProxyURL) == "" {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidProxyURL, "proxy URL is required when network proxy is enabled")
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
	if req.ProxyURL != nil {
		_ = s.store.SetSetting("proxy_url", nextProxyURL)
	}
	if req.ProxyEnabled != nil {
		_ = s.store.SetSetting("proxy_enabled", strconv.FormatBool(*req.ProxyEnabled))
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

	if fullSettings, err := s.store.GetSystemSettings(true); err == nil {
		_ = s.applyActiveProxy(fullSettings)
	}

	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "system.settings.update", "settings", "Updated global system settings", c.ClientIP())

	updated, _ := s.store.GetSystemSettings(false)
	s.decoratePublicSettings(updated)
	c.JSON(http.StatusOK, gin.H{
		"settings": updated,
	})
}

type testProxyRequest struct {
	ProxyURL string `json:"proxy_url"`
}

func (s *Server) handleTestProxy(c *gin.Context) {
	var req testProxyRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		respondBindError(c, err)
		return
	}

	targetProxy := strings.TrimSpace(req.ProxyURL)
	if targetProxy == "" {
		if st, err := s.store.GetSystemSettings(true); err == nil {
			if raw := strings.TrimSpace(st.ProxyURL); raw != "" {
				targetProxy = raw
			}
		}
		if targetProxy == "" {
			targetProxy = strings.TrimSpace(s.cfg.ProxyURL)
		}
	}
	if targetProxy == "" {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidProxyURL, "proxy URL is required to test connectivity")
		return
	}

	if _, err := steam.ParseProxyURL(targetProxy); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	res, err := steam.TestProxyConnectivity(c.Request.Context(), targetProxy, "")
	if err != nil {
		respondError(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": res})
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

func (s *Server) handleSystemVersion(c *gin.Context) {
	st := s.updaterSvc.Status(c.Request.Context())
	u := auth.CurrentUser(c)
	if u == nil || u.Role != db.RoleAdmin {
		st.Docker = updater.DockerCapability{}
		st.Upgrade = updater.UpgradeState{State: "idle"}
	}
	c.JSON(http.StatusOK, gin.H{"info": st})
}

func (s *Server) handleCheckSystemVersion(c *gin.Context) {
	st, err := s.updaterSvc.CheckUpdate(c.Request.Context())
	if err != nil {
		respondError(c, http.StatusBadGateway, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"info": st})
}

type systemUpgradeRequest struct {
	TargetImage string `json:"target_image"`
}

func (s *Server) handleSystemUpgrade(c *gin.Context) {
	var req systemUpgradeRequest
	if err := c.ShouldBindJSON(&req); err != nil && !errors.Is(err, io.EOF) {
		respondBindError(c, err)
		return
	}
	st, err := s.updaterSvc.StartUpgrade(c.Request.Context(), req.TargetImage)
	if err != nil {
		code, _, _ := apperr.Extract(err, "")
		switch code {
		case apperr.CodeUpgradeInProgress:
			respondError(c, http.StatusConflict, err)
		case apperr.CodeUpgradeUnavailable, apperr.CodeInvalidTargetImage:
			respondError(c, http.StatusBadRequest, err)
		default:
			respondError(c, http.StatusInternalServerError, err)
		}
		return
	}

	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "system.upgrade", "docker", "Triggered one-click Docker upgrade to "+st.Upgrade.TargetImage, c.ClientIP())
	c.JSON(http.StatusAccepted, gin.H{"info": st})
}
