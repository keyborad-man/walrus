package api

import (
	"bytes"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"walrus/internal/apperr"
	"walrus/internal/auth"
)

// --- SteamCMD & Version Management ---

func (s *Server) currentBranch() string {
	if st, err := s.store.GetSystemSettings(false); err == nil && st.DSTBranch != "" {
		return st.DSTBranch
	}
	return "public"
}

func (s *Server) handleSteamStatus(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status": s.steamMgr.Status(s.currentBranch()),
	})
}

func (s *Server) handleSteamCheck(c *gin.Context) {
	st, err := s.steamMgr.CheckRemoteVersion(c.Request.Context(), s.currentBranch())
	if err != nil {
		body := apperr.Body(err, apperr.CodeKleiBuildsFetchFailed)
		body["status"] = st
		c.JSON(http.StatusBadGateway, body)
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": st})
}

type steamUpdateRequest struct {
	Branch   string `json:"branch"`
	Validate *bool  `json:"validate"`
}

func (s *Server) handleSteamUpdate(c *gin.Context) {
	var req steamUpdateRequest
	_ = c.ShouldBindJSON(&req)

	branch := strings.TrimSpace(req.Branch)
	if branch == "" {
		branch = s.currentBranch()
	}
	if branch == "public" || branch == "updatebeta" {
		_ = s.store.SetSetting("dst_branch", branch)
	}
	validate := true
	if req.Validate != nil {
		validate = *req.Validate
	}

	if err := s.steamMgr.UpdateDSTServer(branch, validate); err != nil {
		respondError(c, http.StatusConflict, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "steam.update", "dst-server", "branch="+branch, c.ClientIP())
	c.JSON(http.StatusAccepted, gin.H{"status": s.steamMgr.Status(branch)})
}

// --- Visual World Config ---

type patchWorldOverridesRequest struct {
	Overrides map[string]any `json:"overrides" binding:"required"`
}

func (s *Server) handlePatchWorldOverrides(c *gin.Context) {
	id := c.Param("id")
	shard := strings.ToLower(c.Param("shard"))
	var req patchWorldOverridesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	luaOut, err := s.clusterSvc.SaveWorldOverrides(id, shard, req.Overrides)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "cluster.world.patch", id+"/"+shard, "Updated visual world overrides", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true, "lua": luaOut})
}

// --- Mod Management ---

func (s *Server) handleGetClusterMods(c *gin.Context) {
	id := c.Param("id")
	shard := c.DefaultQuery("shard", "master")
	locale := c.DefaultQuery("locale", "zh-CN")
	res, err := s.modSvc.GetClusterMods(id, shard, locale)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, res)
}

type addWorkshopModRequest struct {
	Shard        string `json:"shard"`
	Input        string `json:"input" binding:"required"`
	IsCollection bool   `json:"is_collection"`
	Locale       string `json:"locale"`
}

func (s *Server) handleAddWorkshopMod(c *gin.Context) {
	id := c.Param("id")
	var req addWorkshopModRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	shard := strings.ToLower(strings.TrimSpace(req.Shard))
	if shard == "" {
		shard = "master"
	}
	added, err := s.modSvc.AddWorkshopInput(c.Request.Context(), id, shard, req.Input, req.IsCollection, req.Locale)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "mod.add", id, strings.Join(added, ","), c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"added": added})
}

type addLocalModRequest struct {
	Shard  string `json:"shard"`
	ModID  string `json:"mod_id" binding:"required"`
	Locale string `json:"locale"`
}

func (s *Server) handleAddLocalModToCluster(c *gin.Context) {
	id := c.Param("id")
	var req addLocalModRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	shard := strings.ToLower(strings.TrimSpace(req.Shard))
	if shard == "" {
		shard = "master"
	}
	if err := s.modSvc.AddLocalModToCluster(id, shard, req.ModID, req.Locale); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "mod.add_local", id, req.ModID, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type saveClusterModRequest struct {
	Shard                string         `json:"shard"`
	Enabled              bool           `json:"enabled"`
	ConfigurationOptions map[string]any `json:"configuration_options"`
}

func (s *Server) handleSaveClusterMod(c *gin.Context) {
	id := c.Param("id")
	modID := c.Param("modId")
	var req saveClusterModRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	shard := strings.ToLower(strings.TrimSpace(req.Shard))
	if shard == "" {
		shard = "master"
	}
	if err := s.modSvc.SaveClusterMod(id, shard, modID, req.Enabled, req.ConfigurationOptions); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "mod.config.update", id+"/"+modID, "enabled="+strconv.FormatBool(req.Enabled), c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleRemoveClusterMod(c *gin.Context) {
	id := c.Param("id")
	modID := c.Param("modId")
	shard := strings.ToLower(c.DefaultQuery("shard", "master"))
	if err := s.modSvc.RemoveClusterMod(id, shard, modID); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "mod.remove", id, modID, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type saveRawModOverridesRequest struct {
	Shard string `json:"shard"`
	Lua   string `json:"lua" binding:"required"`
}

func (s *Server) handleSaveRawModOverrides(c *gin.Context) {
	id := c.Param("id")
	var req saveRawModOverridesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	shard := strings.ToLower(strings.TrimSpace(req.Shard))
	if shard == "" {
		shard = "master"
	}
	if err := s.modSvc.SaveRawModOverrides(id, shard, req.Lua); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "mod.raw.update", id+"/"+shard, "Updated raw modoverrides.lua", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type setModSyncRequest struct {
	Enabled bool `json:"enabled"`
}

func (s *Server) handleSetModSync(c *gin.Context) {
	id := c.Param("id")
	var req setModSyncRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	if err := s.modSvc.SetModSync(id, req.Enabled); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true, "mod_sync_enabled": req.Enabled})
}

func (s *Server) handleRedownloadMod(c *gin.Context) {
	modID := c.Param("modId")
	locale := c.DefaultQuery("locale", "zh-CN")
	if err := s.modSvc.RedownloadWorkshopMod(c.Request.Context(), modID, locale); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleWorkshopSearch(c *gin.Context) {
	q := c.Query("q")
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	items, total, isMock, err := s.modSvc.SearchWorkshop(c.Request.Context(), q, page)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"items":   items,
		"total":   total,
		"page":    page,
		"is_mock": isMock,
	})
}

func (s *Server) handleListModLibrary(c *gin.Context) {
	list, err := s.modSvc.ListLibraryMods()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"mods": list})
}

func (s *Server) handleUploadLocalMod(c *gin.Context) {
	fh, err := c.FormFile("file")
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeMissingZipFile, "missing zip file field 'file'")
		return
	}
	if fh.Size > 150*1024*1024 {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeModArchiveTooLarge, "mod archive exceeds 150MB limit")
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
	locale := c.DefaultPostForm("locale", "zh-CN")
	rec, err := s.modSvc.UploadLocalModZip(fh.Filename, bytes.NewReader(data), int64(len(data)), locale)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "mod.local.upload", rec.ModID, rec.Name, c.ClientIP())
	c.JSON(http.StatusCreated, gin.H{"mod": rec})
}

func (s *Server) handleDeleteLocalMod(c *gin.Context) {
	modID := c.Param("modId")
	if err := s.modSvc.DeleteLocalMod(modID); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "mod.local.delete", modID, "Deleted local mod", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

// --- Players, Lists, Chat & GM Snippets ---

func (s *Server) handleGetPlayers(c *gin.Context) {
	id := c.Param("id")
	lists, err := s.clusterSvc.GetPlayerLists(id)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"world_info": s.driver.GetWorldInfo(id),
		"lists":      lists,
	})
}

type savePlayerListsRequest struct {
	AdminList []string `json:"adminlist"`
	WhiteList []string `json:"whitelist"`
	BlockList []string `json:"blocklist"`
}

func (s *Server) handleSavePlayerLists(c *gin.Context) {
	id := c.Param("id")
	var req savePlayerListsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	if err := s.clusterSvc.SavePlayerLists(id, req.AdminList, req.WhiteList, req.BlockList); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "players.lists.update", id, "Updated admin/white/block lists", c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

type playerActionRequest struct {
	Action string `json:"action" binding:"required"` // kick | ban | revive
	KUID   string `json:"ku_id" binding:"required"`
}

func (s *Server) handlePlayerAction(c *gin.Context) {
	id := c.Param("id")
	var req playerActionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	if err := s.clusterSvc.PlayerAction(id, req.Action, req.KUID); err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	u := auth.CurrentUser(c)
	s.store.AddAuditLog(u.Username, "players."+req.Action, id, req.KUID, c.ClientIP())
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (s *Server) handleTriggerProbe(c *gin.Context) {
	id := c.Param("id")
	_ = s.clusterSvc.TriggerWorldProbe(id)
	c.JSON(http.StatusOK, gin.H{"world_info": s.driver.GetWorldInfo(id)})
}

func (s *Server) handleGetChat(c *gin.Context) {
	id := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "150"))
	msgs, err := s.clusterSvc.ReadRecentChat(id, limit)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"messages": msgs})
}

func (s *Server) handleListSnippets(c *gin.Context) {
	list, err := s.store.ListCommandSnippets()
	if err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"snippets": list})
}

type createSnippetRequest struct {
	Name     string `json:"name" binding:"required"`
	Shard    string `json:"shard"`
	Command  string `json:"command" binding:"required"`
	Category string `json:"category"`
}

func (s *Server) handleCreateSnippet(c *gin.Context) {
	var req createSnippetRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		respondBindError(c, err)
		return
	}
	item, err := s.store.CreateCommandSnippet(req.Name, req.Shard, req.Command, req.Category)
	if err != nil {
		respondError(c, http.StatusBadRequest, err)
		return
	}
	c.JSON(http.StatusCreated, gin.H{"snippet": item})
}

func (s *Server) handleDeleteSnippet(c *gin.Context) {
	sid, err := strconv.ParseInt(c.Param("sid"), 10, 64)
	if err != nil {
		respondCodeError(c, http.StatusBadRequest, apperr.CodeInvalidSnippetID, "invalid snippet id")
		return
	}
	if err := s.store.DeleteCommandSnippet(sid); err != nil {
		respondError(c, http.StatusInternalServerError, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"ok": true})
}
