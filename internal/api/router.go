package api

import (
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/gin-gonic/gin"

	"walrus/internal/apperr"
	"walrus/internal/auth"
	"walrus/internal/config"
	"walrus/internal/db"
	"walrus/internal/runtime"
	"walrus/internal/service"
	"walrus/internal/steam"
	"walrus/internal/updater"
	"walrus/internal/version"
	"walrus/internal/ws"
)

type Server struct {
	cfg        *config.Config
	store      *db.Store
	authSvc    *auth.Service
	driver     runtime.Driver
	clusterSvc *service.ClusterService
	backupSvc  *service.BackupService
	modSvc     *service.ModService
	fileSvc    *service.FileService
	cronSvc    *service.CronService
	webhookSvc *service.WebhookService
	steamMgr   *steam.Manager
	updaterSvc *updater.Service
	wsHandler  *ws.ConsoleHandler
	steamWS    *ws.SteamHandler
	staticFS   fs.FS
}

func NewServer(
	cfg *config.Config,
	store *db.Store,
	authSvc *auth.Service,
	driver runtime.Driver,
	clusterSvc *service.ClusterService,
	backupSvc *service.BackupService,
	modSvc *service.ModService,
	fileSvc *service.FileService,
	cronSvc *service.CronService,
	webhookSvc *service.WebhookService,
	steamMgr *steam.Manager,
	updaterSvc *updater.Service,
	staticFS fs.FS,
) *Server {
	if updaterSvc == nil {
		updaterSvc = updater.NewService(cfg.GitHubRepo, cfg.DockerImage, cfg.DockerSock, driver.Mode())
	}
	return &Server{
		cfg:        cfg,
		store:      store,
		authSvc:    authSvc,
		driver:     driver,
		clusterSvc: clusterSvc,
		backupSvc:  backupSvc,
		modSvc:     modSvc,
		fileSvc:    fileSvc,
		cronSvc:    cronSvc,
		webhookSvc: webhookSvc,
		steamMgr:   steamMgr,
		updaterSvc: updaterSvc,
		wsHandler:  ws.NewConsoleHandler(store, driver),
		steamWS:    ws.NewSteamHandler(store, steamMgr),
		staticFS:   staticFS,
	}
}

func (s *Server) Engine() *gin.Engine {
	if !s.cfg.DevMode {
		gin.SetMode(gin.ReleaseMode)
	}
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	if s.cfg.DevMode {
		r.Use(devCORS())
	}

	r.GET("/healthz", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{
			"status":  "ok",
			"runtime": s.driver.Mode(),
			"version": version.Current().Version,
		})
	})

	v1 := r.Group("/api/v1")
	{
		// Public auth endpoints
		v1.GET("/auth/status", s.handleAuthStatus)
		v1.POST("/auth/setup", s.handleSetup)
		v1.POST("/auth/login", s.handleLogin)
		v1.POST("/auth/logout", s.handleLogout)

		// Authenticated routes
		authed := v1.Group("")
		authed.Use(s.authSvc.RequireAuth())
		{
			authed.GET("/auth/me", s.handleMe)
			authed.GET("/system/metrics", s.handleSystemMetrics)
			authed.GET("/system/version", s.handleSystemVersion)
			authed.GET("/presets", s.handleListPresets)
			authed.GET("/steam/status", s.handleSteamStatus)

			// Workshop & Mod Library (read/search available to all authenticated users; upload/delete admin)
			authed.GET("/mods/workshop/search", s.handleWorkshopSearch)
			authed.GET("/mods/library", s.handleListModLibrary)
			authed.POST("/mods/:modId/download", s.handleRedownloadMod)

			// GM Command Snippets
			authed.GET("/snippets", s.handleListSnippets)
			authed.POST("/snippets", s.handleCreateSnippet)
			authed.DELETE("/snippets/:sid", s.handleDeleteSnippet)

			// Clusters
			authed.GET("/clusters", s.handleListClusters)
			authed.GET("/clusters/next-ports", s.authSvc.RequireAdmin(), s.handlePreviewPorts)
			authed.POST("/clusters", s.authSvc.RequireAdmin(), s.handleCreateCluster)
			authed.POST("/clusters/import", s.authSvc.RequireAdmin(), s.handleImportClusterZip)

			// Scheduled Tasks (per-cluster authorization inside handlers)
			authed.POST("/schedules", s.handleCreateSchedule)
			authed.PUT("/schedules/:tid", s.handleUpdateSchedule)
			authed.DELETE("/schedules/:tid", s.handleDeleteSchedule)
			authed.POST("/schedules/:tid/run", s.handleRunScheduleNow)

			// Per-cluster routes with RBAC
			clusterView := authed.Group("/clusters/:id")
			clusterView.Use(s.authSvc.RequireClusterRole(db.RoleViewer))
			{
				clusterView.GET("", s.handleGetCluster)
				clusterView.GET("/world/:shard", s.handleReadWorldLua)
				clusterView.GET("/logs", s.handleRecentLogs)
				clusterView.GET("/backups", s.handleListBackups)
				clusterView.GET("/backups/:bid/download", s.handleDownloadBackup)
				clusterView.GET("/mods", s.handleGetClusterMods)
				clusterView.GET("/players", s.handleGetPlayers)
				clusterView.GET("/chat", s.handleGetChat)
				clusterView.GET("/schedules", s.handleListClusterSchedules)
				clusterView.GET("/files", s.handleListFiles)
				clusterView.GET("/files/content", s.handleReadFile)
				clusterView.GET("/files/download", s.handleDownloadFile)
			}

			clusterOp := authed.Group("/clusters/:id")
			clusterOp.Use(s.authSvc.RequireClusterRole(db.RoleOperator))
			{
				clusterOp.PUT("/config", s.handleUpdateClusterConfig)
				clusterOp.PUT("/world/:shard", s.handleWriteWorldLua)
				clusterOp.PUT("/world/:shard/overrides", s.handlePatchWorldOverrides)
				clusterOp.POST("/start", s.handleStartCluster)
				clusterOp.POST("/stop", s.handleStopCluster)
				clusterOp.POST("/restart", s.handleRestartCluster)
				clusterOp.POST("/command", s.handleSendCommand)
				clusterOp.POST("/probe", s.handleTriggerProbe)
				clusterOp.POST("/backups", s.handleCreateBackup)
				clusterOp.POST("/backups/:bid/restore", s.handleRestoreBackup)
				clusterOp.DELETE("/backups/:bid", s.handleDeleteBackup)
				// Files
				clusterOp.PUT("/files/content", s.handleWriteFile)
				clusterOp.DELETE("/files", s.handleDeleteFile)
				// Mods per cluster
				clusterOp.POST("/mods/workshop", s.handleAddWorkshopMod)
				clusterOp.POST("/mods/local", s.handleAddLocalModToCluster)
				clusterOp.PUT("/mods/raw", s.handleSaveRawModOverrides)
				clusterOp.PUT("/mods/sync", s.handleSetModSync)
				clusterOp.PUT("/mods/:modId", s.handleSaveClusterMod)
				clusterOp.DELETE("/mods/:modId", s.handleRemoveClusterMod)
				// Players
				clusterOp.PUT("/players/lists", s.handleSavePlayerLists)
				clusterOp.POST("/players/action", s.handlePlayerAction)
			}

			clusterAdmin := authed.Group("/clusters/:id")
			clusterAdmin.Use(s.authSvc.RequireAdmin())
			{
				clusterAdmin.DELETE("", s.handleDeleteCluster)
			}

			// WebSocket streams
			authed.GET("/ws/clusters/:id/logs", s.wsHandler.ServeClusterLogs)
			authed.GET("/ws/steam/logs", s.steamWS.ServeSteamLogs)

			// Admin-only system routes
			admin := authed.Group("")
			admin.Use(s.authSvc.RequireAdmin())
			{
				admin.POST("/steam/check", s.handleSteamCheck)
				admin.POST("/steam/update", s.handleSteamUpdate)
				admin.POST("/mods/local/upload", s.handleUploadLocalMod)
				admin.DELETE("/mods/local/:modId", s.handleDeleteLocalMod)
				admin.GET("/schedules", s.handleListAllSchedules)
				admin.GET("/webhooks", s.handleListWebhooks)
				admin.POST("/webhooks", s.handleCreateWebhook)
				admin.PUT("/webhooks/:wid", s.handleUpdateWebhook)
				admin.DELETE("/webhooks/:wid", s.handleDeleteWebhook)
				admin.POST("/webhooks/:wid/test", s.handleTestWebhook)
				admin.GET("/users", s.handleListUsers)
				admin.POST("/users", s.handleCreateUser)
				admin.PUT("/users/:uid", s.handleUpdateUser)
				admin.DELETE("/users/:uid", s.handleDeleteUser)
				admin.GET("/system/settings", s.handleGetSettings)
				admin.PUT("/system/settings", s.handleUpdateSettings)
				admin.POST("/system/version/check", s.handleCheckSystemVersion)
				admin.POST("/system/upgrade", s.handleSystemUpgrade)
				admin.GET("/audit-logs", s.handleListAuditLogs)
			}
		}
	}

	s.mountFrontend(r)
	return r
}

func (s *Server) mountFrontend(r *gin.Engine) {
	var fsys fs.FS
	if s.cfg.StaticDir != "" {
		fsys = os.DirFS(s.cfg.StaticDir)
	} else {
		fsys = s.staticFS
	}

	r.NoRoute(func(c *gin.Context) {
		reqPath := c.Request.URL.Path
		if strings.HasPrefix(reqPath, "/api/") {
			respondCodeError(c, http.StatusNotFound, apperr.CodeAPIRouteNotFound, "api route not found")
			return
		}
		if fsys == nil {
			c.String(http.StatusNotFound, "Frontend build not embedded. Run `npm run build` in web/ or use Vite dev server.")
			return
		}

		clean := strings.TrimPrefix(path.Clean(reqPath), "/")
		if clean != "" && clean != "." && clean != "index.html" && !strings.HasSuffix(clean, "/index.html") {
			if f, err := fsys.Open(clean); err == nil {
				fi, statErr := f.Stat()
				_ = f.Close()
				if statErr == nil && !fi.IsDir() {
					c.FileFromFS(clean, http.FS(fsys))
					return
				}
			}
		}

		// Serve index.html directly (avoids net/http.FileServer 301 redirect loop on /index.html)
		indexBytes, err := fs.ReadFile(fsys, "index.html")
		if err != nil {
			c.String(http.StatusNotFound, "Frontend index.html missing")
			return
		}
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexBytes)
	})
}

func devCORS() gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		}
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	}
}
