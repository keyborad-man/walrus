package steam

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"walrus/internal/apperr"
)

const (
	DSTServerAppID = "343050"
	DSTGameAppID   = "322330"
	KleiBuildsURL  = "https://s3.amazonaws.com/dstbuilds/builds.json"
)

var ansiEscapeRe = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)

type TaskState string

const (
	TaskIdle    TaskState = "idle"
	TaskRunning TaskState = "running"
	TaskSuccess TaskState = "success"
	TaskFailed  TaskState = "failed"
)

type Status struct {
	Installed      bool       `json:"installed"`
	LocalVersion   string     `json:"local_version"`
	RemoteVersion  string     `json:"remote_version"`
	UpdateNeeded   bool       `json:"update_needed"`
	Branch         string     `json:"branch"`
	TaskState      TaskState  `json:"task_state"`
	TaskKind       string     `json:"task_kind"` // "" | "server_update" | "mod_download"
	LastError      string     `json:"last_error,omitempty"`
	LastCheckedAt  *time.Time `json:"last_checked_at,omitempty"`
	LastUpdatedAt  *time.Time `json:"last_updated_at,omitempty"`
	SteamCMDReady  bool       `json:"steamcmd_ready"`
	RuntimeMode    string     `json:"runtime_mode"`
}

type LogLine struct {
	Seq       int64     `json:"seq"`
	Timestamp time.Time `json:"timestamp"`
	Message   string    `json:"message"`
}

type Manager struct {
	steamCMDDir  string
	dstServerDir string
	runtimeMode  string
	httpClient   *http.Client

	mu            sync.RWMutex
	proxyURL      string
	taskState     TaskState
	taskKind      string
	lastError     string
	remoteRelease string
	remoteBeta    string
	lastCheckedAt *time.Time
	lastUpdatedAt *time.Time
	logs          []LogLine
	subs          map[uint64]chan LogLine
	seq           atomic.Int64
	subSeq        atomic.Uint64
}

func NewManager(steamCMDDir, dstServerDir, runtimeMode string) *Manager {
	return &Manager{
		steamCMDDir:  steamCMDDir,
		dstServerDir: dstServerDir,
		runtimeMode:  runtimeMode,
		httpClient:   &http.Client{Timeout: 10 * time.Second},
		taskState:    TaskIdle,
		logs:         make([]LogLine, 0, 400),
		subs:         make(map[uint64]chan LogLine),
	}
}

// SetProxy updates the outbound network proxy used for Klei version checks and SteamCMD downloads.
// Pass an empty string to disable the panel-managed proxy.
func (m *Manager) SetProxy(proxyURL string) error {
	trimmed := strings.TrimSpace(proxyURL)
	client, err := NewHTTPClient(trimmed, 10*time.Second)
	if err != nil {
		return err
	}
	m.mu.Lock()
	m.httpClient = client
	m.proxyURL = trimmed
	m.mu.Unlock()
	return nil
}

// ProxyURL returns the currently configured proxy URL (empty if disabled).
func (m *Manager) ProxyURL() string {
	m.mu.RLock()
	p := m.proxyURL
	m.mu.RUnlock()
	return p
}

// ProxyChainsReady reports whether proxychains4 is available for tunneling SteamCMD.
func (m *Manager) ProxyChainsReady() bool {
	return ProxyChainsAvailable()
}

func (m *Manager) getHTTPClient() *http.Client {
	m.mu.RLock()
	cl := m.httpClient
	m.mu.RUnlock()
	return cl
}

func (m *Manager) publish(msg string) {
	line := LogLine{
		Seq:       m.seq.Add(1),
		Timestamp: time.Now().UTC(),
		Message:   msg,
	}
	m.mu.Lock()
	if len(m.logs) >= 400 {
		copy(m.logs, m.logs[1:])
		m.logs[len(m.logs)-1] = line
	} else {
		m.logs = append(m.logs, line)
	}
	for _, ch := range m.subs {
		select {
		case ch <- line:
		default:
		}
	}
	m.mu.Unlock()

	// Persist log line to disk so history survives page & server restarts.
	_ = os.MkdirAll(m.steamCMDDir, 0o755)
	if f, err := os.OpenFile(filepath.Join(m.steamCMDDir, "last_run.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644); err == nil {
		_, _ = f.WriteString(msg + "\n")
		_ = f.Close()
	}
}

func (m *Manager) RecentLogs() []LogLine {
	m.mu.RLock()
	if len(m.logs) > 0 {
		out := make([]LogLine, len(m.logs))
		copy(out, m.logs)
		m.mu.RUnlock()
		return out
	}
	m.mu.RUnlock()

	// Fallback to persisted last_run.log if in-memory buffer is empty (e.g., after server restart).
	b, err := os.ReadFile(filepath.Join(m.steamCMDDir, "last_run.log"))
	if err != nil || len(b) == 0 {
		return []LogLine{}
	}
	rawLines := strings.Split(strings.TrimRight(string(b), "\r\n"), "\n")
	if len(rawLines) > 400 {
		rawLines = rawLines[len(rawLines)-400:]
	}
	now := time.Now().UTC()
	out := make([]LogLine, 0, len(rawLines))
	for i, text := range rawLines {
		out = append(out, LogLine{
			Seq:       int64(i + 1),
			Timestamp: now,
			Message:   strings.TrimRight(text, "\r"),
		})
	}
	return out
}

func (m *Manager) Subscribe() (<-chan LogLine, func()) {
	ch := make(chan LogLine, 128)
	id := m.subSeq.Add(1)
	m.mu.Lock()
	m.subs[id] = ch
	m.mu.Unlock()
	return ch, func() {
		m.mu.Lock()
		if _, ok := m.subs[id]; ok {
			delete(m.subs, id)
			close(ch)
		}
		m.mu.Unlock()
	}
}

func (m *Manager) LocalVersion() string {
	b, err := os.ReadFile(filepath.Join(m.dstServerDir, "version.txt"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

func (m *Manager) IsInstalled() bool {
	if m.LocalVersion() != "" {
		return true
	}
	for _, p := range []string{
		filepath.Join(m.dstServerDir, "bin64", "dontstarve_dedicated_server_nullrenderer_x64"),
		filepath.Join(m.dstServerDir, "bin", "dontstarve_dedicated_server_nullrenderer"),
	} {
		if _, err := os.Stat(p); err == nil {
			return true
		}
	}
	return false
}

func (m *Manager) SteamCMDPath() string {
	return filepath.Join(m.steamCMDDir, "steamcmd.sh")
}

func (m *Manager) steamCMDHomeDir() string {
	return filepath.Join(m.steamCMDDir, "home")
}

func (m *Manager) Status(branch string) Status {
	if branch == "" {
		branch = "public"
	}
	m.mu.RLock()
	defer m.mu.RUnlock()

	localVer := m.LocalVersion()
	remoteVer := m.remoteRelease
	if branch == "updatebeta" && m.remoteBeta != "" {
		remoteVer = m.remoteBeta
	}
	installed := m.IsInstalled()
	updateNeeded := false
	if remoteVer != "" && (localVer == "" || localVer != remoteVer) {
		updateNeeded = true
	}
	_, scErr := os.Stat(m.SteamCMDPath())

	return Status{
		Installed:     installed,
		LocalVersion:  localVer,
		RemoteVersion: remoteVer,
		UpdateNeeded:  updateNeeded,
		Branch:        branch,
		TaskState:     m.taskState,
		TaskKind:      m.taskKind,
		LastError:     m.lastError,
		LastCheckedAt: m.lastCheckedAt,
		LastUpdatedAt: m.lastUpdatedAt,
		SteamCMDReady: scErr == nil || m.runtimeMode == "mock",
		RuntimeMode:   m.runtimeMode,
	}
}

// CheckRemoteVersion queries Klei's official builds.json endpoint.
func (m *Manager) CheckRemoteVersion(ctx context.Context, branch string) (Status, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, KleiBuildsURL, nil)
	if err != nil {
		return m.Status(branch), err
	}
	resp, err := m.getHTTPClient().Do(req)
	if err != nil {
		if m.runtimeMode == "mock" {
			now := time.Now().UTC()
			m.mu.Lock()
			if m.remoteRelease == "" {
				m.remoteRelease = "654320"
				m.remoteBeta = "654510"
			}
			m.lastCheckedAt = &now
			m.mu.Unlock()
			return m.Status(branch), nil
		}
		return m.Status(branch), apperr.WrapDetail(apperr.CodeKleiBuildsFetchFailed, err, "fetch Klei builds.json: %v", err)
	}
	defer resp.Body.Close()

	var payload struct {
		Release []string `json:"release"`
		Beta    []string `json:"beta"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return m.Status(branch), apperr.WrapDetail(apperr.CodeKleiBuildsFetchFailed, err, "decode builds.json: %v", err)
	}

	now := time.Now().UTC()
	m.mu.Lock()
	if len(payload.Release) > 0 {
		m.remoteRelease = maxNumericString(payload.Release)
	}
	if len(payload.Beta) > 0 {
		m.remoteBeta = maxNumericString(payload.Beta)
	}
	m.lastCheckedAt = &now
	m.mu.Unlock()

	return m.Status(branch), nil
}

func maxNumericString(items []string) string {
	best := items[0]
	bestNum, _ := strconv.Atoi(best)
	for _, it := range items[1:] {
		if n, err := strconv.Atoi(it); err == nil && n > bestNum {
			bestNum = n
			best = it
		}
	}
	return best
}

// UpdateDSTServer runs SteamCMD asynchronously to install or update the DST dedicated server.
func (m *Manager) UpdateDSTServer(branch string, validate bool) error {
	if branch == "" {
		branch = "public"
	}
	m.mu.Lock()
	if m.taskState == TaskRunning {
		m.mu.Unlock()
		return apperr.New(apperr.CodeSteamCMDBusy, "a SteamCMD task is already running")
	}
	m.taskState = TaskRunning
	m.taskKind = "server_update"
	m.lastError = ""
	m.logs = m.logs[:0]
	m.mu.Unlock()
	_ = os.Remove(filepath.Join(m.steamCMDDir, "last_run.log"))

	go m.runServerUpdate(branch, validate)
	return nil
}

func (m *Manager) runServerUpdate(branch string, validate bool) {
	if m.runtimeMode == "mock" {
		m.runMockServerUpdate(branch)
		return
	}

	scPath := m.SteamCMDPath()
	if _, err := os.Stat(scPath); err != nil {
		m.finishTask(apperr.Newf(apperr.CodeSteamCMDNotFound, map[string]any{"path": scPath}, "steamcmd.sh not found at %s", scPath))
		return
	}
	if err := os.MkdirAll(m.dstServerDir, 0o755); err != nil {
		m.finishTask(err)
		return
	}

	args := m.buildServerUpdateArgs(branch, validate)

	m.publish(fmt.Sprintf("[walrus] Running SteamCMD update (app %s, branch=%s)...", DSTServerAppID, branch))
	err := m.runWithMissingConfigRetry(func() (bool, error) {
		return m.execSteamCMD(args)
	})
	m.finishTask(err)
}

func (m *Manager) buildServerUpdateArgs(branch string, validate bool) []string {
	args := []string{
		"+@sSteamCmdForcePlatformType", "linux",
		"+force_install_dir", m.dstServerDir,
		"+login", "anonymous",
		"+app_info_update", "1",
		"+app_update", DSTServerAppID,
	}
	if branch != "" && branch != "public" {
		args = append(args, "-beta", branch)
	}
	if validate {
		args = append(args, "validate")
	}
	return append(args, "+quit")
}

func (m *Manager) runMockServerUpdate(branch string) {
	m.mu.RLock()
	targetVer := m.remoteRelease
	if branch == "updatebeta" && m.remoteBeta != "" {
		targetVer = m.remoteBeta
	}
	m.mu.RUnlock()
	if targetVer == "" {
		targetVer = "654320"
	}

	steps := []string{
		fmt.Sprintf("[walrus] [Mock SteamCMD] Initializing install/update into %s (branch=%s)", m.dstServerDir, branch),
		"Redirecting stderr to '/data/steamcmd/logs/stderr.txt'",
		"Logging in user 'anonymous' to Steam Public...OK",
		"Waiting for client config...OK",
		"Waiting for user info...OK",
		" Update state (0x3) reconfiguring, progress: 0.00 (0 / 0)",
		" Update state (0x61) downloading, progress: 24.50 (412 MB / 1682 MB)",
		" Update state (0x61) downloading, progress: 58.10 (977 MB / 1682 MB)",
		" Update state (0x61) downloading, progress: 92.40 (1554 MB / 1682 MB)",
		" Update state (0x5) verifying install, progress: 100.00 (1682 MB / 1682 MB)",
		fmt.Sprintf("Success! App '%s' fully installed (build %s).", DSTServerAppID, targetVer),
	}

	for _, line := range steps {
		m.publish(line)
		time.Sleep(350 * time.Millisecond)
	}

	_ = os.MkdirAll(filepath.Join(m.dstServerDir, "mods"), 0o755)
	_ = os.WriteFile(filepath.Join(m.dstServerDir, "version.txt"), []byte(targetVer+"\n"), 0o644)
	m.finishTask(nil)
}

// DownloadWorkshopMod downloads a single Workshop item into DST's mods/workshop-<id> folder.
func (m *Manager) DownloadWorkshopMod(ctx context.Context, workshopID string) (string, error) {
	destDir := filepath.Join(m.dstServerDir, "mods", "workshop-"+workshopID)
	if err := os.MkdirAll(filepath.Dir(destDir), 0o755); err != nil {
		return "", err
	}

	if m.runtimeMode == "mock" {
		if p := m.ProxyURL(); p != "" {
			m.publish(fmt.Sprintf("[walrus] [Mock SteamCMD] Using network proxy: %s", MaskProxyURL(p)))
		}
		m.publish(fmt.Sprintf("[walrus] [Mock SteamCMD] Downloading Workshop item %s...", workshopID))
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return "", err
		}
		modInfoPath := filepath.Join(destDir, "modinfo.lua")
		if _, err := os.Stat(modInfoPath); errors.Is(err, os.ErrNotExist) {
			_ = os.WriteFile(modInfoPath, []byte(mockModInfoLua(workshopID)), 0o644)
		}
		m.publish(fmt.Sprintf("[walrus] [Mock SteamCMD] Workshop item %s ready at %s", workshopID, destDir))
		return destDir, nil
	}

	m.mu.Lock()
	if m.taskState == TaskRunning {
		m.mu.Unlock()
		return "", apperr.New(apperr.CodeSteamCMDBusy, "another SteamCMD operation is currently running")
	}
	m.taskState = TaskRunning
	m.taskKind = "mod_download"
	m.lastError = ""
	m.mu.Unlock()

	staging := filepath.Join(m.steamCMDDir, "workshop_cache")
	args := []string{
		"+force_install_dir", staging,
		"+login", "anonymous",
		"+workshop_download_item", DSTGameAppID, workshopID,
		"+quit",
	}
	m.publish(fmt.Sprintf("[walrus] Downloading Workshop mod %s via SteamCMD...", workshopID))
	err := m.runWithMissingConfigRetry(func() (bool, error) {
		return m.execSteamCMD(args)
	})
	if err != nil {
		m.finishTask(err)
		return "", err
	}

	downloadedPath := filepath.Join(staging, "steamapps", "workshop", "content", DSTGameAppID, workshopID)
	if _, statErr := os.Stat(downloadedPath); statErr != nil {
		err = apperr.Newf(apperr.CodeWorkshopDownloadMissing, map[string]any{"path": downloadedPath}, "workshop files not found after download at %s", downloadedPath)
		m.finishTask(err)
		return "", err
	}

	_ = os.RemoveAll(destDir)
	if err := copyDir(downloadedPath, destDir); err != nil {
		m.finishTask(err)
		return "", err
	}
	m.finishTask(nil)
	return destDir, nil
}

func (m *Manager) runWithMissingConfigRetry(run func() (bool, error)) error {
	missingConfig, err := run()
	if err != nil && missingConfig {
		m.publish("[walrus] SteamCMD reported 'Missing configuration'; clearing appcache and retrying once...")
		_ = os.RemoveAll(filepath.Join(m.steamCMDHomeDir(), "Steam", "appcache"))
		_, err = run()
	}
	return err
}

func (m *Manager) execSteamCMD(args []string) (bool, error) {
	homeDir := m.steamCMDHomeDir()
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		return false, err
	}

	cmd, cleanup, err := m.prepareSteamCMDCommand(args, homeDir, m.ProxyURL())
	if err != nil {
		return false, err
	}
	if cleanup != nil {
		defer cleanup()
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return false, err
	}
	if err := cmd.Start(); err != nil {
		return false, err
	}

	var missingConfig atomic.Bool
	var wg sync.WaitGroup
	pipe := func(r io.Reader) {
		defer wg.Done()
		sc := bufio.NewScanner(r)
		sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
		for sc.Scan() {
			raw := sc.Text()
			clean := stripANSI(raw)
			if raw != "" && strings.TrimSpace(clean) == "" {
				continue
			}
			if strings.Contains(strings.ToLower(clean), "missing configuration") {
				missingConfig.Store(true)
			}
			m.publish(clean)
		}
	}
	wg.Add(2)
	go pipe(stdout)
	go pipe(stderr)
	wg.Wait()
	return missingConfig.Load(), cmd.Wait()
}

func (m *Manager) prepareSteamCMDCommand(args []string, homeDir, proxyURL string) (*exec.Cmd, func(), error) {
	env := setEnv(os.Environ(), "HOME", homeDir)
	proxyURL = strings.TrimSpace(proxyURL)

	if proxyURL == "" {
		cmd := exec.Command(m.SteamCMDPath(), args...)
		cmd.Dir = m.steamCMDDir
		cmd.Env = env
		return cmd, nil, nil
	}

	m.publish(fmt.Sprintf("[walrus] Using network proxy: %s", MaskProxyURL(proxyURL)))
	env = applyProxyEnv(env, proxyURL)

	u, err := ParseProxyURL(proxyURL)
	if err != nil || u == nil {
		cmd := exec.Command(m.SteamCMDPath(), args...)
		cmd.Dir = m.steamCMDDir
		cmd.Env = env
		return cmd, nil, nil
	}

	confBody, confErr := BuildProxyChainsConf(u, nil)
	if confErr != nil {
		if errors.Is(confErr, ErrProxyChainsUnsupportedScheme) {
			m.publish("[walrus] Notice: proxychains4 does not support https:// proxy scheme; SteamCMD will only receive proxy environment variables (use http:// or socks5:// for full SteamCMD tunneling).")
		} else {
			m.publish(fmt.Sprintf("[walrus] Warning: failed to build proxychains4 config (%v); falling back to proxy environment variables.", confErr))
		}
		cmd := exec.Command(m.SteamCMDPath(), args...)
		cmd.Dir = m.steamCMDDir
		cmd.Env = env
		return cmd, nil, nil
	}

	multiarchLib := multiarchProxyChainsLib()
	pcBin := ""
	if multiarchLib == "" {
		pcBin = findProxyChainsBinary()
	}
	if multiarchLib == "" && pcBin == "" {
		m.publish("[walrus] Notice: proxychains4 not found on host; SteamCMD may ignore HTTP_PROXY environment variables (install proxychains4 or use the official Docker image).")
		cmd := exec.Command(m.SteamCMDPath(), args...)
		cmd.Dir = m.steamCMDDir
		cmd.Env = env
		return cmd, nil, nil
	}

	_ = os.MkdirAll(m.steamCMDDir, 0o755)
	tmpFile, err := os.CreateTemp(m.steamCMDDir, ".proxychains-*.conf")
	if err != nil {
		return nil, nil, fmt.Errorf("create temp proxychains config: %w", err)
	}
	confPath := tmpFile.Name()
	cleanup := func() { _ = os.Remove(confPath) }
	if err := tmpFile.Chmod(0o600); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return nil, nil, err
	}
	if _, err := tmpFile.WriteString(confBody); err != nil {
		_ = tmpFile.Close()
		cleanup()
		return nil, nil, err
	}
	if err := tmpFile.Close(); err != nil {
		cleanup()
		return nil, nil, err
	}

	if multiarchLib != "" {
		env = setEnv(env, "PROXYCHAINS_CONF_FILE", confPath)
		env = setEnv(env, "PROXYCHAINS_QUIET_MODE", "1")
		preload := multiarchLib
		if existing := strings.TrimSpace(os.Getenv("LD_PRELOAD")); existing != "" {
			preload = multiarchLib + " " + existing
		}
		env = setEnv(env, "LD_PRELOAD", preload)
		cmd := exec.Command(m.SteamCMDPath(), args...)
		cmd.Dir = m.steamCMDDir
		cmd.Env = env
		return cmd, cleanup, nil
	}

	pcArgs := append([]string{"-q", "-f", confPath, m.SteamCMDPath()}, args...)
	cmd := exec.Command(pcBin, pcArgs...)
	cmd.Dir = m.steamCMDDir
	cmd.Env = env
	return cmd, cleanup, nil
}

func stripANSI(s string) string {
	return strings.TrimRight(ansiEscapeRe.ReplaceAllString(s, ""), "\r")
}

func setEnv(env []string, key, val string) []string {
	prefix := key + "="
	out := make([]string, 0, len(env)+1)
	replaced := false
	for _, entry := range env {
		if strings.HasPrefix(entry, prefix) {
			if !replaced {
				out = append(out, prefix+val)
				replaced = true
			}
			continue
		}
		out = append(out, entry)
	}
	if !replaced {
		out = append(out, prefix+val)
	}
	return out
}

func (m *Manager) finishTask(err error) {
	now := time.Now().UTC()
	var msg string
	m.mu.Lock()
	if err != nil {
		m.taskState = TaskFailed
		m.lastError = err.Error()
		msg = "[walrus] SteamCMD task failed: " + err.Error()
	} else {
		m.taskState = TaskSuccess
		m.lastError = ""
		m.lastUpdatedAt = &now
		msg = "[walrus] SteamCMD task completed successfully."
	}
	m.mu.Unlock()
	m.publish(msg)
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()
		out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, info.Mode()&0o644|0o600)
		if err != nil {
			return err
		}
		defer out.Close()
		_, err = io.Copy(out, in)
		return err
	})
}

func mockModInfoLua(workshopID string) string {
	switch workshopID {
	case "378160973":
		return `local is_zh = (locale == "zh" or locale == "zhr")
name = is_zh and "全球定位 (Global Positions)" or "Global Positions"
description = is_zh and "与服务器内所有队友共享小地图位置与信号火。" or "Share map positions and signal fires with teammates."
author = "rezecib"
version = "1.7.4"
api_version = 10
dst_compatible = true
all_clients_require_mod = true
client_only_mod = false
configuration_options = {
  {
    name = "SHOWPLAYERS",
    label = is_zh and "显示玩家图标" or "Show Players",
    hover = is_zh and "在小地图上显示其他玩家位置" or "Show other players on the map",
    options = {
      { description = is_zh and "总是" or "Always", data = "always" },
      { description = is_zh and "默认(仅指南针)" or "Default", data = "default" },
      { description = is_zh and "从不" or "Never", data = "never" },
    },
    default = "default",
  },
  {
    name = "SHAREMINIMAPPROGRESS",
    label = is_zh and "共享地图探索进度" or "Share Map Progress",
    options = {
      { description = is_zh and "开启" or "Enabled", data = true },
      { description = is_zh and "关闭" or "Disabled", data = false },
    },
    default = true,
  },
  {
    name = "FIREOPTIONS",
    label = is_zh and "营火信号标记" or "Campfire Pings",
    options = {
      { description = is_zh and "显示" or "Show", data = 1 },
      { description = is_zh and "仅木炭火" or "Charcoal Only", data = 2 },
      { description = is_zh and "隐藏" or "Hidden", data = 3 },
    },
    default = 2,
  },
}
`
	case "1207269058":
		return `local is_zh = (locale == "zh" or locale == "zhr")
name = is_zh and "简易血条 DST (Simple Health Bar)" or "Simple Health Bar DST"
description = is_zh and "在生物头顶显示美观的血条与伤害数字。" or "Displays clean health bars and damage numbers above creatures."
author = "DYX"
version = "2.25"
api_version = 10
dst_compatible = true
all_clients_require_mod = true
client_only_mod = false
configuration_options = {
  {
    name = "hbstyle",
    label = is_zh and "血条样式" or "Bar Style",
    options = {
      { description = is_zh and "心形" or "Heart", data = "heart" },
      { description = is_zh and "简洁条" or "Simple", data = "simple" },
      { description = is_zh and "经典" or "Standard", data = "standard" },
    },
    default = "heart",
  },
  {
    name = "ddon",
    label = is_zh and "伤害数字飘字" or "Damage Numbers",
    options = {
      { description = is_zh and "开启" or "On", data = true },
      { description = is_zh and "关闭" or "Off", data = false },
    },
    default = true,
  },
}
`
	case "375850599":
		return `local is_zh = (locale == "zh" or locale == "zhr")
name = is_zh and "五格装备栏 (Extra Equip Slots)" or "Extra Equip Slots"
description = is_zh and "为背包和护符增加独立的装备槽位。" or "Adds separate equip slots for backpacks and amulets."
author = "xVars"
version = "1.7.0"
api_version = 10
dst_compatible = true
all_clients_require_mod = true
client_only_mod = false
configuration_options = {
  {
    name = "compass_state",
    label = is_zh and "指南针槽位" or "Compass Slot",
    options = {
      { description = is_zh and "开启" or "On", data = 1 },
      { description = is_zh and "关闭" or "Off", data = 0 },
    },
    default = 1,
  },
}
`
	default:
		return fmt.Sprintf(`local is_zh = (locale == "zh" or locale == "zhr")
name = is_zh and "工坊模组 #%s" or "Workshop Mod #%s"
description = is_zh and "通过 Steam 创意工坊安装的模组（Mock 示例配置）" or "Installed from Steam Workshop (Mock sample modinfo)"
author = "Workshop Author"
version = "1.0.0"
api_version = 10
dst_compatible = true
all_clients_require_mod = true
client_only_mod = false
configuration_options = {
  {
    name = "enabled_feature",
    label = is_zh and "启用增强特性" or "Enhanced Feature",
    options = {
      { description = is_zh and "开启" or "Enabled", data = true },
      { description = is_zh and "关闭" or "Disabled", data = false },
    },
    default = true,
  },
  {
    name = "multiplier",
    label = is_zh and "倍率调节" or "Multiplier",
    options = {
      { description = "0.5x", data = 0.5 },
      { description = "1x", data = 1 },
      { description = "2x", data = 2 },
    },
    default = 1,
  },
}
`, workshopID, workshopID)
	}
}
