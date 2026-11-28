package config

import (
	"flag"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Config holds the runtime configuration for Walrus.
type Config struct {
	Addr          string // HTTP listen address, e.g. ":8080"
	DataDir       string // Unified persistent data root, e.g. "/data" or "./data"
	RuntimeMode   string // "auto", "linux", or "mock"
	JWTSecret     string // Optional override for JWT signing secret
	SteamAPIKey   string // Optional default Steam Web API Key (overridden by System Settings)
	ProxyURL      string // Optional default outbound proxy URL for Mod/Steam (overridden by System Settings)
	DevMode       bool   // Enables verbose logs and CORS for Vite dev server
	StaticDir     string // Optional external static dir override (empty => embedded)
	CookieSecure  bool   // Set Secure flag on auth cookies (enable behind HTTPS)
	DockerSock    string // Path to Docker Unix socket for self-upgrade (default: /var/run/docker.sock)
	GitHubRepo    string // Optional GitHub repo override for version check (owner/repo)
	DockerImage   string // Optional Docker image repo override (e.g. ghcr.io/owner/repo)
	ShowVersion   bool   // Print version info and exit
	UpgradeHelper bool   // Internal mode: run as temporary Docker self-upgrade helper
}

// Load parses configuration from environment variables and CLI flags.
func Load() *Config {
	cfg := &Config{
		Addr:         envOr("WALRUS_ADDR", ":8080"),
		DataDir:      envOr("WALRUS_DATA_DIR", "./data"),
		RuntimeMode:  strings.ToLower(envOr("WALRUS_RUNTIME", "auto")),
		JWTSecret:    os.Getenv("WALRUS_JWT_SECRET"),
		SteamAPIKey:  envOr("WALRUS_STEAM_API_KEY", ""),
		ProxyURL:     strings.TrimSpace(os.Getenv("WALRUS_PROXY_URL")),
		DevMode:      envBool("WALRUS_DEV", false),
		StaticDir:    os.Getenv("WALRUS_STATIC_DIR"),
		CookieSecure: envBool("WALRUS_COOKIE_SECURE", false),
		DockerSock:   envOr("WALRUS_DOCKER_SOCK", "/var/run/docker.sock"),
		GitHubRepo:   os.Getenv("WALRUS_GITHUB_REPO"),
		DockerImage:  os.Getenv("WALRUS_DOCKER_IMAGE"),
	}

	flag.StringVar(&cfg.Addr, "addr", cfg.Addr, "HTTP listen address (env: WALRUS_ADDR)")
	flag.StringVar(&cfg.DataDir, "data-dir", cfg.DataDir, "Persistent data root directory (env: WALRUS_DATA_DIR)")
	flag.StringVar(&cfg.RuntimeMode, "runtime", cfg.RuntimeMode, "Runtime driver: auto, linux, mock (env: WALRUS_RUNTIME)")
	flag.StringVar(&cfg.SteamAPIKey, "steam-api-key", cfg.SteamAPIKey, "Default Steam Web API Key for Workshop search (env: WALRUS_STEAM_API_KEY)")
	flag.StringVar(&cfg.ProxyURL, "proxy-url", cfg.ProxyURL, "Default network proxy URL for Mod queries & SteamCMD downloads (env: WALRUS_PROXY_URL)")
	flag.BoolVar(&cfg.DevMode, "dev", cfg.DevMode, "Enable dev mode with CORS (env: WALRUS_DEV)")
	flag.StringVar(&cfg.StaticDir, "static-dir", cfg.StaticDir, "Serve frontend from directory instead of embedded assets")
	flag.BoolVar(&cfg.CookieSecure, "cookie-secure", cfg.CookieSecure, "Set Secure flag on auth cookies")
	flag.StringVar(&cfg.DockerSock, "docker-sock", cfg.DockerSock, "Docker Unix socket path for one-click upgrade (env: WALRUS_DOCKER_SOCK)")
	flag.BoolVar(&cfg.ShowVersion, "version", false, "Print version information and exit")
	flag.BoolVar(&cfg.UpgradeHelper, "self-upgrade-helper", false, "Internal: execute container replacement helper")
	flag.Parse()

	absData, err := filepath.Abs(cfg.DataDir)
	if err == nil {
		cfg.DataDir = absData
	}

	cfg.RuntimeMode = strings.ToLower(strings.TrimSpace(cfg.RuntimeMode))
	if cfg.RuntimeMode == "" || cfg.RuntimeMode == "auto" {
		if runtime.GOOS == "linux" {
			cfg.RuntimeMode = "linux"
		} else {
			cfg.RuntimeMode = "mock"
		}
	}

	return cfg
}

func (c *Config) DBDir() string        { return filepath.Join(c.DataDir, "db") }
func (c *Config) DBPath() string       { return filepath.Join(c.DBDir(), "walrus.db") }
func (c *Config) ClustersDir() string  { return filepath.Join(c.DataDir, "clusters") }
func (c *Config) BackupsDir() string   { return filepath.Join(c.DataDir, "backups") }
func (c *Config) SteamCMDDir() string  { return filepath.Join(c.DataDir, "steamcmd") }
func (c *Config) DSTServerDir() string { return filepath.Join(c.DataDir, "dst-server") }
func (c *Config) LocalModsDir() string { return filepath.Join(c.DataDir, "local-mods") }

// EnsureDirs creates all required subdirectories under DataDir.
func (c *Config) EnsureDirs() error {
	dirs := []string{
		c.DBDir(),
		c.ClustersDir(),
		c.BackupsDir(),
		c.SteamCMDDir(),
		c.DSTServerDir(),
		c.LocalModsDir(),
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return nil
}

func envOr(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv(key)))
	switch v {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	default:
		return fallback
	}
}
