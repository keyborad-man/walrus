package runtime

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v4/process"

	"walrus/internal/apperr"
	"walrus/internal/dst"
)

type managedProc struct {
	clusterID    string
	shard        string
	opts         StartOptions
	state        ShardState
	cmd          *exec.Cmd
	stdin        io.WriteCloser
	pid          int
	startedAt    time.Time
	stopRequested bool
	restartCount int
	crashWindow  []time.Time
	lastError    string
	doneCh       chan struct{}
}

// LinuxDriver manages real Don't Starve Together dedicated server binaries via os/exec.
type LinuxDriver struct {
	dataDir      string
	dstServerDir string
	fs           *dst.FS
	logs         *LogBus

	mu         sync.RWMutex
	procs      map[string]*managedProc
	worlds     map[string]WorldRuntimeInfo
	playerHook PlayerSeenHook
	eventHook  EventHook
}

func NewLinuxDriver(dataDir, dstServerDir string, fs *dst.FS) *LinuxDriver {
	return &LinuxDriver{
		dataDir:      dataDir,
		dstServerDir: dstServerDir,
		fs:           fs,
		logs:         NewLogBus(),
		procs:        make(map[string]*managedProc),
		worlds:       make(map[string]WorldRuntimeInfo),
	}
}

func (d *LinuxDriver) Mode() string { return "linux" }

func (d *LinuxDriver) SetPlayerSeenHook(hook PlayerSeenHook) {
	d.mu.Lock()
	d.playerHook = hook
	d.mu.Unlock()
}

func (d *LinuxDriver) SetEventHook(hook EventHook) {
	d.mu.Lock()
	d.eventHook = hook
	d.mu.Unlock()
}

func (d *LinuxDriver) emitEvent(event, title, message, clusterID string) {
	d.mu.RLock()
	hook := d.eventHook
	d.mu.RUnlock()
	if hook != nil {
		hook(event, title, message, clusterID)
	}
}

func (d *LinuxDriver) key(clusterID, shard string) string {
	return clusterID + "/" + strings.ToLower(shard)
}

func (d *LinuxDriver) binaryPath(arch string) (binDir string, binPath string, err error) {
	if arch == "32" {
		binDir = filepath.Join(d.dstServerDir, "bin")
		binPath = filepath.Join(binDir, "dontstarve_dedicated_server_nullrenderer")
	} else {
		binDir = filepath.Join(d.dstServerDir, "bin64")
		binPath = filepath.Join(binDir, "dontstarve_dedicated_server_nullrenderer_x64")
		if _, statErr := os.Stat(binPath); statErr != nil {
			// Fallback to 32-bit binary if 64-bit is not installed.
			fallbackDir := filepath.Join(d.dstServerDir, "bin")
			fallbackPath := filepath.Join(fallbackDir, "dontstarve_dedicated_server_nullrenderer")
			if _, fbErr := os.Stat(fallbackPath); fbErr == nil {
				return fallbackDir, fallbackPath, nil
			}
		}
	}
	if _, statErr := os.Stat(binPath); statErr != nil {
		return "", "", apperr.Newf(apperr.CodeDSTBinaryNotFound, map[string]any{"path": binPath}, "DST server binary not found at %s (please install DST via SteamCMD first)", binPath)
	}
	return binDir, binPath, nil
}

func (d *LinuxDriver) StartShard(ctx context.Context, clusterID, shard string, opts StartOptions) error {
	shard = strings.ToLower(shard)
	if shard != "master" && shard != "caves" {
		return apperr.Newf(apperr.CodeInvalidShard, map[string]any{"shard": shard}, "invalid shard: %s", shard)
	}
	k := d.key(clusterID, shard)

	d.mu.Lock()
	if existing, ok := d.procs[k]; ok && (existing.state == StateRunning || existing.state == StateStarting) {
		d.mu.Unlock()
		return apperr.Newf(apperr.CodeShardAlreadyActive, map[string]any{"cluster_id": clusterID, "shard": shard, "state": string(existing.state)}, "shard %s/%s is already %s", clusterID, shard, existing.state)
	}
	d.mu.Unlock()

	return d.spawn(clusterID, shard, opts, 0, nil)
}

func (d *LinuxDriver) spawn(clusterID, shard string, opts StartOptions, restartCount int, crashWindow []time.Time) error {
	binDir, binPath, err := d.binaryPath(opts.Arch)
	if err != nil {
		return err
	}

	shardTitle := "Master"
	if shard == "caves" {
		shardTitle = "Caves"
	}

	// DST expects -persistent_storage_root <dataDir> -conf_dir clusters -cluster <clusterID> -shard <Master|Caves>
	args := []string{
		"-console",
		"-persistent_storage_root", d.dataDir,
		"-conf_dir", "clusters",
		"-cluster", clusterID,
		"-shard", shardTitle,
	}

	cmd := exec.Command(binPath, args...)
	cmd.Dir = binDir

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("open stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("open stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("open stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return apperr.Wrap(apperr.CodeShardStartFailed, err, map[string]any{"cluster_id": clusterID, "shard": shard, "detail": err.Error()}, fmt.Sprintf("start DST shard %s/%s: %v", clusterID, shard, err))
	}

	mp := &managedProc{
		clusterID:    clusterID,
		shard:        shard,
		opts:         opts,
		state:        StateStarting,
		cmd:          cmd,
		stdin:        stdin,
		pid:          cmd.Process.Pid,
		startedAt:    time.Now().UTC(),
		restartCount: restartCount,
		crashWindow:  crashWindow,
		doneCh:       make(chan struct{}),
	}

	d.mu.Lock()
	d.procs[d.key(clusterID, shard)] = mp
	d.mu.Unlock()

	d.logs.Publish(clusterID, shard, "system", fmt.Sprintf("[walrus] Launched %s shard (PID %d)", shardTitle, mp.pid))

	go d.streamOutput(mp, stdout, false)
	go d.streamOutput(mp, stderr, true)
	go d.waitLoop(mp)
	if shard == "master" {
		go d.probeLoop(mp)
	}

	return nil
}

func (d *LinuxDriver) probeLoop(mp *managedProc) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	// Wait a few seconds after launch before first probe.
	select {
	case <-mp.doneCh:
		return
	case <-time.After(8 * time.Second):
	}
	_ = d.TriggerProbe(mp.clusterID)

	for {
		select {
		case <-mp.doneCh:
			return
		case <-ticker.C:
			_ = d.TriggerProbe(mp.clusterID)
		}
	}
}

func (d *LinuxDriver) TriggerProbe(clusterID string) error {
	d.mu.RLock()
	mp, ok := d.procs[d.key(clusterID, "master")]
	running := ok && mp.state == StateRunning && mp.stdin != nil
	d.mu.RUnlock()
	if !running {
		return nil
	}
	return d.SendCommand(clusterID, "master", SilentProbeLua)
}

func (d *LinuxDriver) streamOutput(mp *managedProc, r io.Reader, isErr bool) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 512*1024)
	for sc.Scan() {
		line := sc.Text()
		if info, ok := ParseProbeLine(line); ok {
			d.mu.Lock()
			d.worlds[mp.clusterID] = info
			hook := d.playerHook
			d.mu.Unlock()
			if hook != nil {
				for _, p := range info.OnlinePlayers {
					hook(p.KUID, p.Name, mp.clusterID)
				}
			}
			continue
		}
		if chatMsg, ok := ParseChatLine(line); ok {
			if chatMsg.KUID != "" && chatMsg.Sender != "" {
				d.mu.RLock()
				hook := d.playerHook
				d.mu.RUnlock()
				if hook != nil {
					hook(chatMsg.KUID, chatMsg.Sender, mp.clusterID)
				}
			}
			if chatMsg.Kind == "join" {
				d.emitEvent("player.join", "Player Joined", chatMsg.Sender+" joined the world", mp.clusterID)
			} else if chatMsg.Kind == "leave" {
				d.emitEvent("player.leave", "Player Left", chatMsg.Sender+" left the world", mp.clusterID)
			}
		}

		level := ""
		if isErr {
			level = "error"
		}
		if strings.Contains(line, "Sim paused") || strings.Contains(line, "Shard connected") {
			d.mu.Lock()
			if mp.state == StateStarting {
				mp.state = StateRunning
			}
			d.mu.Unlock()
		}
		d.logs.Publish(mp.clusterID, mp.shard, level, line)
	}
}

func (d *LinuxDriver) waitLoop(mp *managedProc) {
	err := mp.cmd.Wait()
	close(mp.doneCh)

	d.mu.Lock()
	requested := mp.stopRequested
	if requested {
		mp.state = StateStopped
		mp.pid = 0
		d.mu.Unlock()
		d.logs.Publish(mp.clusterID, mp.shard, "system", "[walrus] Shard process exited cleanly.")
		return
	}

	errMsg := "process exited unexpectedly"
	if err != nil {
		errMsg = err.Error()
	}
	mp.state = StateCrashed
	mp.lastError = errMsg
	mp.pid = 0

	// Crash circuit breaker: max 3 crashes within 5 minutes.
	now := time.Now()
	cutoff := now.Add(-5 * time.Minute)
	filtered := mp.crashWindow[:0]
	for _, ts := range mp.crashWindow {
		if ts.After(cutoff) {
			filtered = append(filtered, ts)
		}
	}
	filtered = append(filtered, now)
	mp.crashWindow = filtered

	shouldRestart := mp.opts.AutoRestart && len(filtered) <= 3
	nextRestartCount := mp.restartCount + 1
	opts := mp.opts
	d.mu.Unlock()

	d.logs.Publish(mp.clusterID, mp.shard, "error", fmt.Sprintf("[walrus] Shard exited unexpectedly: %s", errMsg))
	d.emitEvent("server.crash", "DST Shard Crashed", fmt.Sprintf("Shard %s exited unexpectedly: %s", mp.shard, errMsg), mp.clusterID)

	if !shouldRestart {
		if mp.opts.AutoRestart {
			d.logs.Publish(mp.clusterID, mp.shard, "error", "[walrus] Auto-restart circuit breaker tripped (3 crashes in 5m).")
		}
		return
	}

	backoff := time.Duration(len(filtered)*5) * time.Second
	d.logs.Publish(mp.clusterID, mp.shard, "warn", fmt.Sprintf("[walrus] Auto-restarting shard in %s (attempt %d/3)...", backoff, len(filtered)))
	time.Sleep(backoff)

	// Re-check if user stopped/restarted in the meantime.
	d.mu.RLock()
	cur := d.procs[d.key(mp.clusterID, mp.shard)]
	stillSame := cur == mp && !cur.stopRequested
	d.mu.RUnlock()
	if !stillSame {
		return
	}
	if err := d.spawn(mp.clusterID, mp.shard, opts, nextRestartCount, filtered); err != nil {
		d.logs.Publish(mp.clusterID, mp.shard, "error", fmt.Sprintf("[walrus] Auto-restart failed: %v", err))
	}
}

func (d *LinuxDriver) StopShard(ctx context.Context, clusterID, shard string) error {
	shard = strings.ToLower(shard)
	k := d.key(clusterID, shard)

	d.mu.Lock()
	mp, ok := d.procs[k]
	if !ok || mp.state == StateStopped || mp.state == StateCrashed {
		if ok {
			mp.stopRequested = true
			mp.state = StateStopped
		}
		d.mu.Unlock()
		return nil
	}
	mp.stopRequested = true
	mp.state = StateStopping
	stdin := mp.stdin
	doneCh := mp.doneCh
	cmd := mp.cmd
	d.mu.Unlock()

	d.logs.Publish(clusterID, shard, "system", "[walrus] Sending graceful shutdown: c_shutdown(true)")
	if stdin != nil {
		_, _ = io.WriteString(stdin, "c_shutdown(true)\n")
	}

	timeout := 20 * time.Second
	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-doneCh:
		return nil
	case <-ctx.Done():
	case <-timer.C:
		d.logs.Publish(clusterID, shard, "warn", "[walrus] Graceful shutdown timed out, terminating process...")
	}

	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Signal(os.Interrupt)
		select {
		case <-doneCh:
			return nil
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Kill()
			<-doneCh
		}
	}
	return nil
}

func (d *LinuxDriver) SendCommand(clusterID, shard, command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	k := d.key(clusterID, shard)
	d.mu.RLock()
	mp, ok := d.procs[k]
	if !ok || (mp.state != StateRunning && mp.state != StateStarting) || mp.stdin == nil {
		d.mu.RUnlock()
		normShard := strings.ToLower(shard)
		return apperr.Newf(apperr.CodeShardNotRunning, map[string]any{"cluster_id": clusterID, "shard": normShard}, "shard %s/%s is not running", clusterID, normShard)
	}
	stdin := mp.stdin
	d.mu.RUnlock()

	if !strings.Contains(command, ProbePrefix) {
		d.logs.Publish(clusterID, shard, "system", "[Console Input] > "+command)
	}
	_, err := io.WriteString(stdin, command+"\n")
	return err
}

func (d *LinuxDriver) GetShardStatus(clusterID, shard string) ShardStatus {
	shard = strings.ToLower(shard)
	k := d.key(clusterID, shard)
	d.mu.RLock()
	mp, ok := d.procs[k]
	if !ok {
		d.mu.RUnlock()
		return ShardStatus{Shard: shard, State: StateStopped}
	}
	st := ShardStatus{
		Shard:        shard,
		State:        mp.state,
		PID:          mp.pid,
		RestartCount: mp.restartCount,
		LastError:    mp.lastError,
	}
	if mp.state == StateRunning || mp.state == StateStarting || mp.state == StateStopping {
		started := mp.startedAt
		st.StartedAt = &started
		st.UptimeSec = int64(time.Since(started).Seconds())
	}
	pid := mp.pid
	d.mu.RUnlock()

	if pid > 0 {
		if p, err := process.NewProcess(int32(pid)); err == nil {
			if cpuPct, err := p.CPUPercent(); err == nil {
				st.CPUPercent = round1(cpuPct)
			}
			if memInfo, err := p.MemoryInfo(); err == nil && memInfo != nil {
				st.MemoryMB = round1(float64(memInfo.RSS) / (1024 * 1024))
			}
		}
	}
	return st
}

func (d *LinuxDriver) GetWorldInfo(clusterID string) WorldRuntimeInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()
	mp := d.procs[d.key(clusterID, "master")]
	if mp == nil || mp.state != StateRunning {
		return WorldRuntimeInfo{
			Season:        "unknown",
			Phase:         "unknown",
			OnlinePlayers: []OnlinePlayer{},
		}
	}
	if w, ok := d.worlds[clusterID]; ok {
		if w.OnlinePlayers == nil {
			w.OnlinePlayers = []OnlinePlayer{}
		}
		return w
	}
	return WorldRuntimeInfo{
		Season:        "unknown",
		Phase:         "unknown",
		OnlinePlayers: []OnlinePlayer{},
	}
}

func (d *LinuxDriver) ReadRecentChat(clusterID string, limit int) ([]ChatMessage, error) {
	dir, err := d.fs.ShardDir(clusterID, "master")
	if err != nil {
		return nil, err
	}
	return ReadChatLogFile(filepath.Join(dir, "server_chat_log.txt"), filepath.Join(dir, "server_log.txt"), limit)
}

func (d *LinuxDriver) ReadRecentLogs(clusterID, shard string, limit int) ([]LogEntry, error) {
	if entries := d.logs.Recent(clusterID, shard, limit); len(entries) > 0 {
		return entries, nil
	}
	dir, err := d.fs.ShardDir(clusterID, shard)
	if err != nil {
		return nil, err
	}
	return ReadTailFromFile(filepath.Join(dir, "server_log.txt"), shard, limit)
}

func (d *LinuxDriver) SubscribeLogs(clusterID, shard string) (<-chan LogEntry, func()) {
	return d.logs.Subscribe(clusterID, shard)
}

func (d *LinuxDriver) HostMetrics() HostMetrics {
	return CollectHostMetrics(d.dataDir, d.Mode())
}

func (d *LinuxDriver) ShutdownAll(ctx context.Context) {
	d.mu.RLock()
	targets := make([][2]string, 0, len(d.procs))
	for _, mp := range d.procs {
		if mp.state == StateRunning || mp.state == StateStarting {
			targets = append(targets, [2]string{mp.clusterID, mp.shard})
		}
	}
	d.mu.RUnlock()

	var wg sync.WaitGroup
	for _, t := range targets {
		wg.Add(1)
		go func(cid, sh string) {
			defer wg.Done()
			_ = d.StopShard(ctx, cid, sh)
		}(t[0], t[1])
	}
	wg.Wait()
}
