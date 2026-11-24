package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"walrus/internal/api"
	"walrus/internal/auth"
	"walrus/internal/config"
	"walrus/internal/db"
	"walrus/internal/dst"
	"walrus/internal/runtime"
	"walrus/internal/service"
	"walrus/internal/steam"
	"walrus/web"
)

func main() {
	cfg := config.Load()

	logLevel := slog.LevelInfo
	if cfg.DevMode {
		logLevel = slog.LevelDebug
	}
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: logLevel}))
	slog.SetDefault(logger)

	if err := cfg.EnsureDirs(); err != nil {
		slog.Error("failed to initialize data directories", "dir", cfg.DataDir, "err", err)
		os.Exit(1)
	}

	store, err := db.Open(cfg.DBPath())
	if err != nil {
		slog.Error("failed to open sqlite database", "path", cfg.DBPath(), "err", err)
		os.Exit(1)
	}
	defer store.Close()

	jwtSecret, err := store.EnsureJWTSecret(cfg.JWTSecret)
	if err != nil {
		slog.Error("failed to resolve jwt secret", "err", err)
		os.Exit(1)
	}

	dstFS := dst.NewFS(cfg.ClustersDir())

	var driver runtime.Driver
	if cfg.RuntimeMode == "linux" {
		driver = runtime.NewLinuxDriver(cfg.DataDir, cfg.DSTServerDir(), dstFS)
	} else {
		driver = runtime.NewMockDriver(cfg.DataDir, dstFS)
	}
	driver.SetPlayerSeenHook(func(kuID, name, clusterID string) {
		_ = store.UpsertPlayerHistory(kuID, name, clusterID)
	})

	steamMgr := steam.NewManager(cfg.SteamCMDDir(), cfg.DSTServerDir(), driver.Mode())
	workshopClient := steam.NewWorkshopClient(driver.Mode())

	authSvc := auth.NewService(store, jwtSecret, cfg.CookieSecure)
	webhookSvc := service.NewWebhookService(store)
	driver.SetEventHook(func(event, title, message, clusterID string) {
		webhookSvc.Dispatch(event, title, message, clusterID)
	})

	clusterSvc := service.NewClusterService(store, dstFS, driver)
	backupSvc := service.NewBackupService(cfg.BackupsDir(), store, dstFS, driver)
	modSvc := service.NewModService(cfg.DSTServerDir(), cfg.LocalModsDir(), store, dstFS, steamMgr, workshopClient)
	fileSvc := service.NewFileService(dstFS)
	cronSvc := service.NewCronService(store, dstFS, driver, clusterSvc, backupSvc, steamMgr, webhookSvc)

	backupSvc.SetOnImportedHook(func() {
		_ = modSvc.SyncDedicatedServerModsSetup()
	})

	if err := cronSvc.Start(); err != nil {
		slog.Warn("cron scheduler init warning", "err", err)
	}

	// Background initial Klei version check + 6-hour polling loop
	go func() {
		checkOnce := func() {
			branch := "public"
			if st, err := store.GetSystemSettings(false); err == nil && st.DSTBranch != "" {
				branch = st.DSTBranch
			}
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			st, err := steamMgr.CheckRemoteVersion(ctx, branch)
			if err == nil && st.UpdateNeeded && st.Installed {
				webhookSvc.Dispatch("steam.update_available", "DST Update Available", "Local "+st.LocalVersion+" -> Remote "+st.RemoteVersion, "*")
			}
		}
		checkOnce()
		ticker := time.NewTicker(6 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			checkOnce()
		}
	}()

	srv := api.NewServer(cfg, store, authSvc, driver, clusterSvc, backupSvc, modSvc, fileSvc, cronSvc, webhookSvc, steamMgr, web.DistFS())
	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Engine(),
		ReadHeaderTimeout: 15 * time.Second,
	}

	slog.Info("walrus starting",
		"addr", cfg.Addr,
		"data_dir", cfg.DataDir,
		"runtime", driver.Mode(),
		"dev", cfg.DevMode,
	)

	go func() {
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server failed", "err", err)
			os.Exit(1)
		}
	}()

	// Wait for termination signal and gracefully save+stop all running shards.
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit
	slog.Info("received shutdown signal, gracefully saving and stopping running shards...", "signal", sig.String())

	ctx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
	defer cancel()

	cronSvc.Stop()
	driver.ShutdownAll(ctx)
	if err := httpSrv.Shutdown(ctx); err != nil {
		slog.Warn("http server shutdown warning", "err", err)
	}
	slog.Info("walrus exited cleanly")
}
