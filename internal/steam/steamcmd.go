package steam

import (
	"archive/zip"
	"bufio"
	"bytes"
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
	DSTServerAppID             = "343050"
	DSTGameAppID               = "322330"
	KleiBuildsURL              = "https://s3.amazonaws.com/dstbuilds/builds.json"
	defaultServerUpdateTimeout = 30 * time.Minute
	defaultModDownloadTimeout  = 5 * time.Minute
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
	cancelTask    context.CancelFunc
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

// SetProxy updates the outbound network proxy used for Klei version check HTTP requests.
// SteamCMD itself always connects directly so Linux epoll sockets are not disrupted by LD_PRELOAD wrappers.
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

// CancelTask terminates any currently running SteamCMD task so the global lock is released promptly.
func (m *Manager) CancelTask() bool {
	m.mu.Lock()
	if m.taskState != TaskRunning || m.cancelTask == nil {
		m.mu.Unlock()
		return false
	}
	cancel := m.cancelTask
	m.mu.Unlock()

	m.publish("[walrus] Cancelling active SteamCMD task...")
	cancel()
	return true
}

// UpdateDSTServer runs SteamCMD asynchronously to install or update the DST dedicated server.
func (m *Manager) UpdateDSTServer(branch string, validate bool) error {
	if branch == "" {
		branch = "public"
	}
	taskCtx, cancel := context.WithTimeout(context.Background(), defaultServerUpdateTimeout)
	m.mu.Lock()
	if m.taskState == TaskRunning {
		m.mu.Unlock()
		cancel()
		return apperr.New(apperr.CodeSteamCMDBusy, "a SteamCMD task is already running")
	}
	m.taskState = TaskRunning
	m.taskKind = "server_update"
	m.lastError = ""
	m.logs = m.logs[:0]
	m.cancelTask = cancel
	m.mu.Unlock()
	_ = os.Remove(filepath.Join(m.steamCMDDir, "last_run.log"))

	go func() {
		defer cancel()
		m.runServerUpdate(taskCtx, branch, validate)
	}()
	return nil
}

func (m *Manager) runServerUpdate(ctx context.Context, branch string, validate bool) {
	if m.runtimeMode == "mock" {
		m.runMockServerUpdate(ctx, branch)
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
		return m.execSteamCMD(ctx, args)
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

func (m *Manager) runMockServerUpdate(ctx context.Context, branch string) {
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
		select {
		case <-ctx.Done():
			m.finishTask(ctx.Err())
			return
		default:
		}
		m.publish(line)
		timer := time.NewTimer(350 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			m.finishTask(ctx.Err())
			return
		case <-timer.C:
		}
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

	taskCtx, cancel := context.WithTimeout(ctx, defaultModDownloadTimeout)
	m.mu.Lock()
	if m.taskState == TaskRunning {
		m.mu.Unlock()
		cancel()
		return "", apperr.New(apperr.CodeSteamCMDBusy, "another SteamCMD operation is currently running")
	}
	m.taskState = TaskRunning
	m.taskKind = "mod_download"
	m.lastError = ""
	m.cancelTask = cancel
	m.mu.Unlock()
	defer cancel()

	staging := filepath.Join(m.steamCMDDir, "workshop_cache")
	args := buildWorkshopDownloadArgs(staging, []string{workshopID})
	m.publish(fmt.Sprintf("[walrus] Downloading Workshop mod %s via SteamCMD...", workshopID))
	err := m.runWithMissingConfigRetry(func() (bool, error) {
		return m.execSteamCMD(taskCtx, args)
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

	extractedArchive, err := installWorkshopModDir(downloadedPath, destDir)
	if err != nil {
		m.finishTask(err)
		return "", err
	}
	if extractedArchive != "" {
		m.publish(fmt.Sprintf("[walrus] Extracted legacy Workshop archive %s for mod %s", filepath.Base(extractedArchive), workshopID))
	}
	m.finishTask(nil)
	return destDir, nil
}

// DownloadWorkshopMods downloads multiple Workshop items in a single SteamCMD session
// and installs each into DST's mods/workshop-<id> folder.
func (m *Manager) DownloadWorkshopMods(ctx context.Context, workshopIDs []string, onProgress func(msg string)) error {
	cleanIDs := make([]string, 0, len(workshopIDs))
	seen := make(map[string]struct{}, len(workshopIDs))
	for _, raw := range workshopIDs {
		wid := strings.TrimSpace(raw)
		if wid == "" {
			continue
		}
		if _, dup := seen[wid]; dup {
			continue
		}
		seen[wid] = struct{}{}
		cleanIDs = append(cleanIDs, wid)
	}
	if len(cleanIDs) == 0 {
		return nil
	}

	notify := func(msg string) {
		m.publish(msg)
		if onProgress != nil {
			onProgress(msg)
		}
	}

	modsRoot := filepath.Join(m.dstServerDir, "mods")
	if err := os.MkdirAll(modsRoot, 0o755); err != nil {
		return err
	}

	if m.runtimeMode == "mock" {
		notify(fmt.Sprintf("[walrus] [Mock SteamCMD] Downloading %d Workshop mod(s): %s", len(cleanIDs), strings.Join(cleanIDs, ", ")))
		for _, wid := range cleanIDs {
			if err := ctx.Err(); err != nil {
				return err
			}
			destDir := filepath.Join(modsRoot, "workshop-"+wid)
			if err := os.MkdirAll(destDir, 0o755); err != nil {
				return err
			}
			modInfoPath := filepath.Join(destDir, "modinfo.lua")
			if _, err := os.Stat(modInfoPath); errors.Is(err, os.ErrNotExist) {
				if err := os.WriteFile(modInfoPath, []byte(mockModInfoLua(wid)), 0o644); err != nil {
					return err
				}
			}
		}
		notify(fmt.Sprintf("[walrus] [Mock SteamCMD] %d Workshop mod(s) ready.", len(cleanIDs)))
		return nil
	}

	timeout := time.Duration(len(cleanIDs)) * 2 * time.Minute
	if timeout < defaultModDownloadTimeout {
		timeout = defaultModDownloadTimeout
	}
	if timeout > defaultServerUpdateTimeout {
		timeout = defaultServerUpdateTimeout
	}
	taskCtx, cancel := context.WithTimeout(ctx, timeout)

	m.mu.Lock()
	if m.taskState == TaskRunning {
		m.mu.Unlock()
		cancel()
		return apperr.New(apperr.CodeSteamCMDBusy, "another SteamCMD operation is currently running")
	}
	m.taskState = TaskRunning
	m.taskKind = "mod_download"
	m.lastError = ""
	m.cancelTask = cancel
	m.mu.Unlock()
	defer cancel()

	staging := filepath.Join(m.steamCMDDir, "workshop_cache")
	args := buildWorkshopDownloadArgs(staging, cleanIDs)
	notify(fmt.Sprintf("[walrus] Downloading %d Workshop mod(s) via SteamCMD: %s", len(cleanIDs), strings.Join(cleanIDs, ", ")))

	cmdErr := m.runWithMissingConfigRetry(func() (bool, error) {
		return m.execSteamCMD(taskCtx, args)
	})
	if ctxErr := taskCtx.Err(); ctxErr != nil {
		err := fmt.Errorf("workshop batch download aborted: %w", ctxErr)
		if cmdErr != nil {
			err = cmdErr
		}
		m.finishTask(err)
		return err
	}

	// Install every mod that landed in staging so partial successes are preserved on disk.
	var failedIDs []string
	var firstInstallErr error
	installedCount := 0
	for _, wid := range cleanIDs {
		downloadedPath := filepath.Join(staging, "steamapps", "workshop", "content", DSTGameAppID, wid)
		destDir := filepath.Join(modsRoot, "workshop-"+wid)
		if _, statErr := os.Stat(downloadedPath); statErr != nil {
			failedIDs = append(failedIDs, wid)
			continue
		}
		extractedArchive, installErr := installWorkshopModDir(downloadedPath, destDir)
		if installErr != nil {
			failedIDs = append(failedIDs, wid)
			if firstInstallErr == nil {
				firstInstallErr = installErr
			}
			continue
		}
		installedCount++
		if extractedArchive != "" {
			notify(fmt.Sprintf("[walrus] Extracted legacy Workshop archive %s for mod %s", filepath.Base(extractedArchive), wid))
		}
	}

	if len(failedIDs) > 0 {
		detail := fmt.Sprintf("failed to download/install %d Workshop mod(s): %s", len(failedIDs), strings.Join(failedIDs, ", "))
		if firstInstallErr != nil {
			detail += fmt.Sprintf(" (%v)", firstInstallErr)
		} else if cmdErr != nil {
			detail += fmt.Sprintf(" (%v)", cmdErr)
		}
		err := apperr.Newf(apperr.CodeWorkshopDownloadMissing, map[string]any{"workshop_ids": strings.Join(failedIDs, ", ")}, "%s", detail)
		m.finishTask(err)
		return err
	}

	notify(fmt.Sprintf("[walrus] Installed %d Workshop mod(s) into %s", installedCount, modsRoot))
	m.finishTask(nil)
	return nil
}

func buildWorkshopDownloadArgs(staging string, workshopIDs []string) []string {
	args := make([]string, 0, 5+len(workshopIDs)*3)
	args = append(args,
		"+force_install_dir", staging,
		"+login", "anonymous",
	)
	for _, wid := range workshopIDs {
		args = append(args, "+workshop_download_item", DSTGameAppID, wid)
	}
	return append(args, "+quit")
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

func (m *Manager) execSteamCMD(ctx context.Context, args []string) (bool, error) {
	homeDir := m.steamCMDHomeDir()
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		return false, err
	}

	cmd := m.prepareSteamCMDCommand(ctx, args, homeDir)

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
	waitErr := cmd.Wait()
	if ctxErr := ctx.Err(); ctxErr != nil {
		if errors.Is(ctxErr, context.DeadlineExceeded) {
			return false, fmt.Errorf("SteamCMD timed out: %w", ctxErr)
		}
		return false, fmt.Errorf("SteamCMD cancelled: %w", ctxErr)
	}
	return missingConfig.Load(), waitErr
}

func (m *Manager) prepareSteamCMDCommand(ctx context.Context, args []string, homeDir string) *exec.Cmd {
	env := setEnv(os.Environ(), "HOME", homeDir)
	cmd := exec.CommandContext(ctx, m.SteamCMDPath(), args...)
	cmd.Dir = m.steamCMDDir
	cmd.Env = env
	configureCmdProcessGroup(cmd)
	return cmd
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
	m.cancelTask = nil
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

// installWorkshopModDir installs a SteamCMD-downloaded Workshop item into destDir.
// Modern UGC items already contain modinfo.lua directly; legacy Workshop items are
// downloaded by SteamCMD as a single ZIP archive (e.g. "<id>_legacy.bin") which must
// be extracted with normalized path separators so Linux servers and modinfo.lua parsing work.
func installWorkshopModDir(downloadedPath, destDir string) (extractedArchive string, err error) {
	fi, err := os.Stat(downloadedPath)
	if err != nil {
		return "", apperr.Newf(apperr.CodeWorkshopDownloadMissing, map[string]any{"path": downloadedPath}, "workshop files not found after download at %s", downloadedPath)
	}

	var modRoot string
	if !fi.IsDir() {
		if !isZipFile(downloadedPath) {
			return "", apperr.Newf(apperr.CodeWorkshopDownloadMissing, map[string]any{"path": downloadedPath}, "workshop download is neither a mod directory nor a ZIP archive: %s", downloadedPath)
		}
		tmpDir, tmpErr := os.MkdirTemp("", "walrus-workshop-extract-*")
		if tmpErr != nil {
			return "", tmpErr
		}
		defer os.RemoveAll(tmpDir)

		if err := extractZipArchive(downloadedPath, tmpDir); err != nil {
			return "", err
		}
		modRoot = findModRootDir(tmpDir)
		if modRoot == "" {
			return "", apperr.New(apperr.CodeModZipMissingModInfo, "workshop archive does not contain modinfo.lua at root or top-level folder")
		}
		extractedArchive = downloadedPath
	} else if root := findModRootDir(downloadedPath); root != "" {
		modRoot = root
	} else if archivePath := findLegacyWorkshopArchive(downloadedPath); archivePath != "" {
		tmpDir, tmpErr := os.MkdirTemp("", "walrus-workshop-extract-*")
		if tmpErr != nil {
			return "", tmpErr
		}
		defer os.RemoveAll(tmpDir)

		if err := extractZipArchive(archivePath, tmpDir); err != nil {
			return "", err
		}
		modRoot = findModRootDir(tmpDir)
		if modRoot == "" {
			return "", apperr.New(apperr.CodeModZipMissingModInfo, "workshop archive does not contain modinfo.lua at root or top-level folder")
		}
		extractedArchive = archivePath
	} else {
		return "", apperr.Newf(apperr.CodeWorkshopDownloadMissing, map[string]any{"path": downloadedPath}, "modinfo.lua not found in downloaded workshop directory %s", downloadedPath)
	}

	parentDir := filepath.Dir(destDir)
	if err := os.MkdirAll(parentDir, 0o755); err != nil {
		return "", err
	}
	stageDest, err := os.MkdirTemp(parentDir, ".workshop-stage-*")
	if err != nil {
		return "", err
	}
	stageCleanup := true
	defer func() {
		if stageCleanup {
			_ = os.RemoveAll(stageDest)
		}
	}()

	if err := copyDir(modRoot, stageDest); err != nil {
		return "", err
	}
	_ = os.Chmod(stageDest, 0o755)
	if err := os.RemoveAll(destDir); err != nil {
		return "", err
	}
	if err := os.Rename(stageDest, destDir); err != nil {
		return "", err
	}
	stageCleanup = false
	return extractedArchive, nil
}

func findModRootDir(dir string) string {
	if fi, err := os.Stat(filepath.Join(dir, "modinfo.lua")); err == nil && !fi.IsDir() {
		return dir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		cand := filepath.Join(dir, e.Name())
		if fi, statErr := os.Stat(filepath.Join(cand, "modinfo.lua")); statErr == nil && !fi.IsDir() {
			return cand
		}
	}
	return ""
}

func findLegacyWorkshopArchive(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var legacyBin, binOrZip, otherZip string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		full := filepath.Join(dir, e.Name())
		if !isZipFile(full) {
			continue
		}
		lower := strings.ToLower(e.Name())
		switch {
		case strings.HasSuffix(lower, "_legacy.bin"):
			if legacyBin == "" {
				legacyBin = full
			}
		case strings.HasSuffix(lower, ".bin") || strings.HasSuffix(lower, ".zip"):
			if binOrZip == "" {
				binOrZip = full
			}
		default:
			if otherZip == "" {
				otherZip = full
			}
		}
	}
	if legacyBin != "" {
		return legacyBin
	}
	if binOrZip != "" {
		return binOrZip
	}
	return otherZip
}

func isZipFile(path string) bool {
	f, err := os.Open(path)
	if err != nil {
		return false
	}
	defer f.Close()
	var hdr [4]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return false
	}
	return bytes.Equal(hdr[:], []byte{'P', 'K', 0x03, 0x04}) || bytes.Equal(hdr[:], []byte{'P', 'K', 0x05, 0x06})
}

func extractZipArchive(archivePath, destDir string) error {
	zr, err := zip.OpenReader(archivePath)
	if err != nil {
		return apperr.WrapDetail(apperr.CodeInvalidZipArchive, err, "invalid workshop archive: %v", err)
	}
	defer zr.Close()

	cleanRoot := filepath.Clean(destDir)
	cleanPrefix := cleanRoot + string(os.PathSeparator)

	for _, f := range zr.File {
		// Legacy DST workshop archives created on Windows often store '\' separators.
		normName := strings.ReplaceAll(f.Name, "\\", "/")
		if strings.HasPrefix(normName, "/") {
			return apperr.Newf(apperr.CodeIllegalZipPath, map[string]any{"path": f.Name}, "illegal path in workshop archive: %s", f.Name)
		}
		target := filepath.Clean(filepath.Join(cleanRoot, filepath.FromSlash(normName)))
		if target != cleanRoot && !strings.HasPrefix(target+string(os.PathSeparator), cleanPrefix) {
			return apperr.Newf(apperr.CodeIllegalZipPath, map[string]any{"path": f.Name}, "illegal path in workshop archive: %s", f.Name)
		}
		if f.FileInfo().IsDir() || strings.HasSuffix(normName, "/") {
			if err := os.MkdirAll(target, 0o755); err != nil {
				return err
			}
			continue
		}
		if err := extractZipEntry(f, target); err != nil {
			return err
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, target string) error {
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.OpenFile(target, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, rc)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
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
