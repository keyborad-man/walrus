package api

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"walrus/internal/apperr"
	"walrus/internal/auth"
	"walrus/internal/db"
	"walrus/internal/dst"
	"walrus/internal/service"
)

func (s *Server) handleListClusters(c *gin.Context) {
	u := auth.CurrentUser(c)
	list, err := s.clusterSvc.ListClusters(u)
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"clusters": list})
}

func (s *Server) handlePreviewPorts(c *gin.Context) {
	ports, err := s.clusterSvc.PreviewNextPorts()
	if err != nil {
		respondError(c, http.StatusConflict, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ports": ports})
}

func (s *Server) handleListPresets(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"presets": dst.BuiltinPresets})
}

func (s *Server) handleWorldSchema(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"master": dst.WorldSchema("master"),
		"caves":  dst.WorldSchema("caves"),
	})
}

func (s *Server) handleCreateCluster(c *gin.Context) {
	var req service.CreateClusterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	detail, err := s.clusterSvc.CreateCluster(req)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cluster.create", detail.ClusterID, detail.ClusterINI.ClusterName, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"cluster": detail})
}

func (s *Server) handleGetCluster(c *gin.Context) {
	id := c.Param("id")
	detail, err := s.clusterSvc.GetCluster(id)
	if errors.Is(err, db.ErrNotFound) {
		respondCodeError(c, http.StatusNotFound, apperr.CodeClusterNotFound, "cluster not found")
		return
	}
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	u := auth.CurrentUser(c)
	effRole, _ := s.store.EffectiveClusterRole(u, id)
	c.JSON(http.StatusOK, gin.H{
		"cluster":        detail,
		"effective_role": effRole,
	})
}

func (s *Server) handleUpdateClusterConfig(c *gin.Context) {
	id := c.Param("id")
	var req service.UpdateClusterConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	detail, err := s.clusterSvc.UpdateClusterConfig(id, req)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cluster.config.update", id, "Updated cluster/port configuration", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"cluster": detail})
}

func (s *Server) handleDeleteCluster(c *gin.Context) {
	id := c.Param("id")
	if err := s.clusterSvc.DeleteCluster(c.Request.Context(), id); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	_ = s.backupSvc.RemoveClusterArchives(id)
	_ = s.cronSvc.Reload()
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cluster.delete", id, "Deleted cluster directory, backups, schedules and metadata", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleReadWorldLua(c *gin.Context) {
	id := c.Param("id")
	shard := strings.ToLower(c.Param("shard"))
	view, err := s.clusterSvc.ReadWorldConfig(id, shard)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, view)
}

type writeLuaRequest struct {
	Lua string `json:"lua" binding:"required"`
}

func (s *Server) handleWriteWorldLua(c *gin.Context) {
	id := c.Param("id")
	shard := strings.ToLower(c.Param("shard"))
	var req writeLuaRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	if err := s.clusterSvc.WriteWorldLua(id, shard, req.Lua); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cluster.world.update", id+"/"+shard, "Updated leveldataoverride.lua", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Lifecycle & Console ---

func (s *Server) handleStartCluster(c *gin.Context) {
	id := c.Param("id")
	shard := strings.ToLower(c.Query("shard"))
	var err error
	if shard == "master" || shard == "caves" {
		err = s.clusterSvc.StartShard(c.Request.Context(), id, shard)
	} else {
		err = s.clusterSvc.StartCluster(c.Request.Context(), id)
	}
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	target := id
	if shard != "" {
		target += "/" + shard
	}
	s.store.AddAuditLog(u.Username, "cluster.start", target, "Started server", c.ClientIP())
	if s.webhookSvc != nil {
		s.webhookSvc.Dispatch("server.start", "Cluster Started", "Started "+target+" by "+u.Username, id)
	}
	detail, _ := s.clusterSvc.GetCluster(id)
	c.JSON(http.StatusOK, gin.H{"cluster": detail})
}

func (s *Server) handleStopCluster(c *gin.Context) {
	id := c.Param("id")
	shard := strings.ToLower(c.Query("shard"))
	var err error
	if shard == "master" || shard == "caves" {
		err = s.clusterSvc.StopShard(c.Request.Context(), id, shard)
	} else {
		err = s.clusterSvc.StopCluster(c.Request.Context(), id)
	}
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	target := id
	if shard != "" {
		target += "/" + shard
	}
	s.store.AddAuditLog(u.Username, "cluster.stop", target, "Stopped server", c.ClientIP())
	if s.webhookSvc != nil {
		s.webhookSvc.Dispatch("server.stop", "Cluster Stopped", "Stopped "+target+" by "+u.Username, id)
	}
	detail, _ := s.clusterSvc.GetCluster(id)
	c.JSON(http.StatusOK, gin.H{"cluster": detail})
}

func (s *Server) handleRestartCluster(c *gin.Context) {
	id := c.Param("id")
	shard := strings.ToLower(c.Query("shard"))
	var err error
	if shard == "master" || shard == "caves" {
		_ = s.clusterSvc.StopShard(c.Request.Context(), id, shard)
		err = s.clusterSvc.StartShard(c.Request.Context(), id, shard)
	} else {
		err = s.clusterSvc.RestartCluster(c.Request.Context(), id)
	}
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cluster.restart", id, "Restarted server", c.ClientIP())
	detail, _ := s.clusterSvc.GetCluster(id)
	c.JSON(http.StatusOK, gin.H{"cluster": detail})
}

type sendCommandRequest struct {
	Shard   string `json:"shard"`
	Command string `json:"command" binding:"required"`
}

func (s *Server) handleSendCommand(c *gin.Context) {
	id := c.Param("id")
	var req sendCommandRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	shard := strings.ToLower(strings.TrimSpace(req.Shard))
	if shard == "" {
		shard = "master"
	}
	if err := s.clusterSvc.SendCommand(id, shard, req.Command); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "console.command", id+"/"+shard, req.Command, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleRecentLogs(c *gin.Context) {
	id := c.Param("id")
	shard := strings.ToLower(c.DefaultQuery("shard", "master"))
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	entries, err := s.driver.ReadRecentLogs(id, shard, limit)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"shard": shard, "entries": entries})
}

// --- Backups ---

func (s *Server) handleListBackups(c *gin.Context) {
	id := c.Param("id")
	list, err := s.backupSvc.ListBackups(id)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"backups": list})
}

type createBackupRequest struct {
	Note string `json:"note"`
}

func (s *Server) handleCreateBackup(c *gin.Context) {
	id := c.Param("id")
	var req createBackupRequest
	_ = c.ShouldBindJSON(&req)

	u := auth.CurrentUser(c)
	rec, err := s.backupSvc.CreateBackup(c.Request.Context(), id, "manual", req.Note, u.Username)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	s.store.AddAuditLog(u.Username, "backup.create", id, rec.Filename, c.ClientIP())
	if s.webhookSvc != nil {
		s.webhookSvc.Dispatch("backup.complete", "Manual Backup Completed", rec.Filename+" created by "+u.Username, id)
	}
	c.JSON(http.StatusCreated, gin.H{"backup": rec})
}

func (s *Server) handleDownloadBackup(c *gin.Context) {
	id := c.Param("id")
	bid, err := strconv.ParseInt(c.Param("bid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidBackupID, "invalid backup id")
		return
	}
	path, rec, err := s.backupSvc.BackupFilePath(id, bid)
	if err != nil {
		respondError(c, http.StatusNotFound, err)
		return
	}
	c.FileAttachment(path, rec.Filename)
}

func (s *Server) handleRestoreBackup(c *gin.Context) {
	id := c.Param("id")
	bid, err := strconv.ParseInt(c.Param("bid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidBackupID, "invalid backup id")
		return
	}
	u := auth.CurrentUser(c)
	if err := s.backupSvc.RestoreBackup(c.Request.Context(), id, bid, u.Username); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	s.store.AddAuditLog(u.Username, "backup.restore", id, "Restored backup #"+strconv.FormatInt(bid, 10), c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleDeleteBackup(c *gin.Context) {
	id := c.Param("id")
	bid, err := strconv.ParseInt(c.Param("bid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidBackupID, "invalid backup id")
		return
	}
	if err := s.backupSvc.DeleteBackup(id, bid); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "backup.delete", id, "Deleted backup #"+strconv.FormatInt(bid, 10), c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
