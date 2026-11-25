package api

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"walrus/internal/apperr"
	"walrus/internal/auth"
	"walrus/internal/db"
	"walrus/internal/service"
)

// --- Cluster ZIP Save Import ---

func (s *Server) handleImportClusterZip(c *gin.Context) {
	clusterID := strings.TrimSpace(c.PostForm("cluster_id"))
	clusterName := strings.TrimSpace(c.PostForm("cluster_name"))
	tokenOverride := strings.TrimSpace(c.PostForm("cluster_token"))

	if clusterID == "" {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeClusterIDRequired, "cluster_id is required")
		return
	}

	fh, err := c.FormFile("file")
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeMissingZipFile, "missing zip file field 'file'")
		return
	}
	if fh.Size > 512*1024*1024 {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeSaveArchiveTooLarge, "save archive exceeds 512MB limit")
		return
	}
	f, err := fh.Open()
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	if err := s.backupSvc.ImportClusterZip(clusterID, clusterName, tokenOverride, bytes.NewReader(data), int64(len(data))); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}

	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cluster.import", clusterID, "Imported save archive "+fh.Filename, c.ClientIP())

	detail, _ := s.clusterSvc.GetCluster(clusterID)
	c.JSON(http.StatusCreated, gin.H{"cluster": detail})
}

// --- Sandboxed Cluster File Browser ---

func (s *Server) handleListFiles(c *gin.Context) {
	id := c.Param("id")
	rel := c.Query("path")
	entries, cleanPath, err := s.fileSvc.ListDir(id, rel)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"path":    cleanPath,
		"entries": entries,
	})
}

func (s *Server) handleReadFile(c *gin.Context) {
	id := c.Param("id")
	rel := c.Query("path")
	res, err := s.fileSvc.ReadFile(id, rel)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

type writeFileRequest struct {
	Path    string `json:"path" binding:"required"`
	Content string `json:"content"`
}

func (s *Server) handleWriteFile(c *gin.Context) {
	id := c.Param("id")
	var req writeFileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	if err := s.fileSvc.WriteFile(id, req.Path, req.Content); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "file.write", id+"/"+req.Path, "Edited cluster file", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleDownloadFile(c *gin.Context) {
	id := c.Param("id")
	rel := c.Query("path")
	abs, name, err := s.fileSvc.DownloadPath(id, rel)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.FileAttachment(abs, name)
}

func (s *Server) handleDeleteFile(c *gin.Context) {
	id := c.Param("id")
	rel := c.Query("path")
	if err := s.fileSvc.DeleteFile(id, rel); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "file.delete", id+"/"+rel, "Deleted file", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Cron Scheduled Tasks ---

func (s *Server) handleListClusterSchedules(c *gin.Context) {
	id := c.Param("id")
	list, err := s.store.ListScheduledTasks(id)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": list})
}

func (s *Server) handleListAllSchedules(c *gin.Context) {
	list, err := s.store.ListScheduledTasks("")
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"tasks": list})
}

type schedulePayload struct {
	ClusterID string         `json:"cluster_id" binding:"required"`
	Name      string         `json:"name" binding:"required"`
	TaskType  string         `json:"task_type" binding:"required"` // backup | restart | announce | update | mod_update
	CronExpr  string         `json:"cron_expr" binding:"required"`
	Params    map[string]any `json:"params"`
	Enabled   bool           `json:"enabled"`
}

func isValidScheduleTaskType(taskType string) bool {
	switch taskType {
	case "backup", "restart", "announce", "update", "mod_update":
		return true
	default:
		return false
	}
}

func (s *Server) authorizeScheduleCluster(c *gin.Context, clusterID string) bool {
	u := auth.CurrentUser(c)
	if u == nil {
		return false
	}
	if u.Role == db.RoleAdmin {
		return true
	}
	if clusterID == "*" {
		return false
	}
	eff, err := s.store.EffectiveClusterRole(u, clusterID)
	return err == nil && eff.Rank() >= db.RoleOperator.Rank()
}

func (s *Server) handleCreateSchedule(c *gin.Context) {
	var req schedulePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	if !isValidScheduleTaskType(req.TaskType) {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidTaskType, "invalid task_type")
		return
	}
	if req.TaskType == "update" {
		if auth.CurrentUser(c).Role != db.RoleAdmin {
			respondCodeError(c, http.StatusForbidden, apperr.CodeAdminRequiredForUpdate, "admin role required for server update tasks")
			return
		}
		req.ClusterID = "*"
	}
	if !s.authorizeScheduleCluster(c, req.ClusterID) {
		respondCodeError(c, http.StatusForbidden, apperr.CodeInsufficientClusterPerm, "insufficient permission for target cluster")
		return
	}
	if err := s.cronSvc.ValidateExpr(req.CronExpr); err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidCronExpr, "invalid cron expression: "+err.Error(), map[string]any{"detail": err.Error()})
		return
	}

	paramsBytes, _ := json.Marshal(req.Params)
	task, err := s.store.CreateScheduledTask(&db.ScheduledTask{
		ClusterID:  req.ClusterID,
		Name:       req.Name,
		TaskType:   req.TaskType,
		CronExpr:   req.CronExpr,
		ParamsJSON: string(paramsBytes),
		Enabled:    req.Enabled,
	})
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	_ = s.cronSvc.Reload()
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cron.create", req.ClusterID, req.Name+" ("+req.CronExpr+")", c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"task": task})
}

func (s *Server) handleUpdateSchedule(c *gin.Context) {
	tid, err := strconv.ParseInt(c.Param("tid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidTaskID, "invalid task id")
		return
	}
	existing, err := s.store.GetScheduledTask(tid)
	if err != nil {
		respondCodeError(c, http.StatusNotFound, apperr.CodeTaskNotFound, "scheduled task not found")
		return
	}
	var req schedulePayload
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	if !isValidScheduleTaskType(req.TaskType) {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidTaskType, "invalid task_type")
		return
	}
	if req.TaskType == "update" || existing.TaskType == "update" {
		if auth.CurrentUser(c).Role != db.RoleAdmin {
			respondCodeError(c, http.StatusForbidden, apperr.CodeAdminRequiredForUpdate, "admin role required for server update tasks")
			return
		}
		if req.TaskType == "update" {
			req.ClusterID = "*"
		}
	}
	if !s.authorizeScheduleCluster(c, existing.ClusterID) || !s.authorizeScheduleCluster(c, req.ClusterID) {
		respondCodeError(c, http.StatusForbidden, apperr.CodeInsufficientPermission, "insufficient permission")
		return
	}
	if err := s.cronSvc.ValidateExpr(req.CronExpr); err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidCronExpr, "invalid cron expression: "+err.Error(), map[string]any{"detail": err.Error()})
		return
	}
	paramsBytes, _ := json.Marshal(req.Params)
	updated, err := s.store.UpdateScheduledTask(&db.ScheduledTask{
		ID:         tid,
		ClusterID:  req.ClusterID,
		Name:       req.Name,
		TaskType:   req.TaskType,
		CronExpr:   req.CronExpr,
		ParamsJSON: string(paramsBytes),
		Enabled:    req.Enabled,
	})
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	_ = s.cronSvc.Reload()
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cron.update", req.ClusterID, req.Name, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"task": updated})
}

func (s *Server) handleDeleteSchedule(c *gin.Context) {
	tid, err := strconv.ParseInt(c.Param("tid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidTaskID, "invalid task id")
		return
	}
	existing, err := s.store.GetScheduledTask(tid)
	if err != nil {
		respondCodeError(c, http.StatusNotFound, apperr.CodeTaskNotFound, "scheduled task not found")
		return
	}
	if !s.authorizeScheduleCluster(c, existing.ClusterID) {
		respondCodeError(c, http.StatusForbidden, apperr.CodeInsufficientPermission, "insufficient permission")
		return
	}
	if err := s.store.DeleteScheduledTask(tid); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	_ = s.cronSvc.Reload()
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cron.delete", existing.ClusterID, existing.Name, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleRunScheduleNow(c *gin.Context) {
	tid, err := strconv.ParseInt(c.Param("tid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidTaskID, "invalid task id")
		return
	}
	existing, err := s.store.GetScheduledTask(tid)
	if err != nil {
		respondCodeError(c, http.StatusNotFound, apperr.CodeTaskNotFound, "scheduled task not found")
		return
	}
	if !s.authorizeScheduleCluster(c, existing.ClusterID) {
		respondCodeError(c, http.StatusForbidden, apperr.CodeInsufficientPermission, "insufficient permission")
		return
	}
	if err := s.cronSvc.RunNow(tid); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cron.run_now", existing.ClusterID, existing.Name, c.ClientIP())
	c.JSON(http.StatusAccepted, gin.H{"ok": true})
}

// --- Webhooks ---

func (s *Server) handleListWebhooks(c *gin.Context) {
	hooks, err := s.store.ListWebhooks()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	// Mask secrets in response
	masked := make([]db.WebhookConfig, len(hooks))
	for i, h := range hooks {
		h.Secret = db.MaskSecret(h.Secret)
		masked[i] = h
	}
	deliveries, _ := s.store.ListWebhookDeliveries(50)
	c.JSON(http.StatusOK, gin.H{
		"webhooks":   masked,
		"deliveries": deliveries,
		"all_events": service.AllWebhookEvents,
	})
}

type webhookPayload struct {
	Name     string   `json:"name" binding:"required"`
	Template string   `json:"template" binding:"required"`
	URL      string   `json:"url" binding:"required"`
	Secret   string   `json:"secret"`
	Events   []string `json:"events"`
	Enabled  bool     `json:"enabled"`
}

func (s *Server) handleCreateWebhook(c *gin.Context) {
	var req webhookPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	w, err := s.store.CreateWebhook(&db.WebhookConfig{
		Name:     req.Name,
		Template: req.Template,
		URL:      req.URL,
		Secret:   req.Secret,
		Events:   req.Events,
		Enabled:  req.Enabled,
	})
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "webhook.create", w.Name, w.Template, c.ClientIP())
	w.Secret = db.MaskSecret(w.Secret)
	c.JSON(http.StatusCreated, gin.H{"webhook": w})
}

func (s *Server) handleUpdateWebhook(c *gin.Context) {
	wid, err := strconv.ParseInt(c.Param("wid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidWebhookID, "invalid webhook id")
		return
	}
	existing, err := s.store.GetWebhook(wid)
	if err != nil {
		respondCodeError(c, http.StatusNotFound, apperr.CodeWebhookNotFound, "webhook not found")
		return
	}
	var req webhookPayload
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	secret := existing.Secret
	if strings.TrimSpace(req.Secret) != "" {
		secret = strings.TrimSpace(req.Secret)
	}
	w, err := s.store.UpdateWebhook(&db.WebhookConfig{
		ID:       wid,
		Name:     req.Name,
		Template: req.Template,
		URL:      req.URL,
		Secret:   secret,
		Events:   req.Events,
		Enabled:  req.Enabled,
	})
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "webhook.update", w.Name, w.Template, c.ClientIP())
	w.Secret = db.MaskSecret(w.Secret)
	c.JSON(http.StatusOK, gin.H{"webhook": w})
}

func (s *Server) handleDeleteWebhook(c *gin.Context) {
	wid, err := strconv.ParseInt(c.Param("wid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidWebhookID, "invalid webhook id")
		return
	}
	if err := s.store.DeleteWebhook(wid); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "webhook.delete", strconv.FormatInt(wid, 10), "Deleted webhook", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleTestWebhook(c *gin.Context) {
	wid, err := strconv.ParseInt(c.Param("wid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidWebhookID, "invalid webhook id")
		return
	}
	deliv, err := s.webhookSvc.SendTest(c.Request.Context(), wid)
	if err != nil {
		respondCodeError(c, http.StatusNotFound, apperr.CodeWebhookNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"delivery": deliv})
}
