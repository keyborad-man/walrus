package runtime

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"walrus/internal/apperr"
	"walrus/internal/dst"
)

type mockShard struct {
	clusterID string
	shard     string
	state     ShardState
	pid       int
	startedAt time.Time
	cancel    context.CancelFunc
	fileMu    sync.Mutex
	logFile   *os.File
}

type mockWorld struct {
	day       int
	seasonIdx int
	remDays   int
	phaseIdx  int
	players   []OnlinePlayer
}

var (
	mockSeasons     = []string{"autumn", "winter", "spring", "summer"}
	mockPhases      = []string{"day", "dusk", "night"}
	mockSampleUsers = []OnlinePlayer{
		{KUID: "KU_w1ls0n01", Name: "Wilson", Prefab: "wilson", Age: 14},
		{KUID: "KU_w3ndy002", Name: "Wendy", Prefab: "wendy", Age: 11},
		{KUID: "KU_wx78003", Name: "WX-78", Prefab: "wx78", Age: 8},
		{KUID: "KU_w1gfr14", Name: "Wigfrid", Prefab: "wathgrithr", Age: 19},
	}
	announceRe  = regexp.MustCompile(`c_announce\s*\(\s*["'](.+?)["']\s*\)`)
	rollbackRe  = regexp.MustCompile(`c_rollback\s*\(\s*(\d*)\s*\)`)
	kickBanRe   = regexp.MustCompile(`TheNet:(Kick|Ban)\s*\(\s*["']([^"']+)["']\s*\)`)
	setSeasonRe = regexp.MustCompile(`ms_setseason["']\s*,\s*["'](autumn|winter|spring|summer)["']`)
)

// MockDriver simulates DST dedicated servers locally on Windows/macOS/dev machines.
type MockDriver struct {
	dataDir string
	fs      *dst.FS
	logs    *LogBus

	mu         sync.RWMutex
	shards     map[string]*mockShard
	worlds     map[string]*mockWorld
	playerHook PlayerSeenHook
	eventHook  EventHook
	pidSeq     atomic.Int32
}

func NewMockDriver(dataDir string, fs *dst.FS) *MockDriver {
	d := &MockDriver{
		dataDir: dataDir,
		fs:      fs,
		logs:    NewLogBus(),
		shards:  make(map[string]*mockShard),
		worlds:  make(map[string]*mockWorld),
	}
	d.pidSeq.Store(41000)
	return d
}

func (d *MockDriver) Mode() string { return "mock" }

func (d *MockDriver) SetPlayerSeenHook(hook PlayerSeenHook) {
	d.mu.Lock()
	d.playerHook = hook
	d.mu.Unlock()
}

func (d *MockDriver) SetEventHook(hook EventHook) {
	d.mu.Lock()
	d.eventHook = hook
	d.mu.Unlock()
}

func (d *MockDriver) emitEvent(event, title, message, clusterID string) {
	d.mu.RLock()
	hook := d.eventHook
	d.mu.RUnlock()
	if hook != nil {
		hook(event, title, message, clusterID)
	}
}

func (d *MockDriver) notifyPlayer(clusterID, kuID, name string) {
	d.mu.RLock()
	hook := d.playerHook
	d.mu.RUnlock()
	if hook != nil {
		hook(kuID, name, clusterID)
	}
}

func (d *MockDriver) key(clusterID, shard string) string {
	return clusterID + "/" + strings.ToLower(shard)
}

func (d *MockDriver) ensureWorldLocked(clusterID string) *mockWorld {
	if w, ok := d.worlds[clusterID]; ok {
		return w
	}
	w := &mockWorld{
		day:       12,
		seasonIdx: 0,
		remDays:   8,
		phaseIdx:  0,
		players: []OnlinePlayer{
			mockSampleUsers[0],
		},
	}
	d.worlds[clusterID] = w
	return w
}

func (d *MockDriver) emit(ms *mockShard, level, msg string) {
	ts := time.Now().Format("15:04:05")
	line := fmt.Sprintf("[%s]: %s", ts, msg)
	ms.fileMu.Lock()
	if ms.logFile != nil {
		_, _ = ms.logFile.WriteString(line + "\n")
	}
	ms.fileMu.Unlock()
	d.logs.Publish(ms.clusterID, ms.shard, level, line)
}

func (d *MockDriver) StartShard(ctx context.Context, clusterID, shard string, opts StartOptions) error {
	shard = strings.ToLower(shard)
	k := d.key(clusterID, shard)

	shardDir, err := d.fs.ShardDir(clusterID, shard)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(shardDir, 0o755); err != nil {
		return err
	}

	d.mu.Lock()
	if existing, ok := d.shards[k]; ok && (existing.state == StateRunning || existing.state == StateStarting) {
		d.mu.Unlock()
		normShard := strings.ToLower(shard)
		return apperr.Newf(apperr.CodeShardAlreadyActive, map[string]any{"cluster_id": clusterID, "shard": normShard, "state": string(existing.state)}, "shard %s/%s is already %s", clusterID, normShard, existing.state)
	}

	logPath := filepath.Join(shardDir, "server_log.txt")
	lf, _ := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)

	runCtx, cancel := context.WithCancel(context.Background())
	ms := &mockShard{
		clusterID: clusterID,
		shard:     shard,
		state:     StateStarting,
		pid:       int(d.pidSeq.Add(1)),
		startedAt: time.Now().UTC(),
		cancel:    cancel,
		logFile:   lf,
	}
	d.shards[k] = ms
	w := d.ensureWorldLocked(clusterID)
	d.mu.Unlock()

	go d.runMockLoop(runCtx, ms, w)
	return nil
}

func (d *MockDriver) runMockLoop(ctx context.Context, ms *mockShard, w *mockWorld) {
	shardTitle := "Master"
	if ms.shard == "caves" {
		shardTitle = "Caves"
	}

	d.emit(ms, "system", fmt.Sprintf("[walrus] Starting %s shard (Mock Runtime, PID %d)...", shardTitle, ms.pid))
	d.emit(ms, "info", fmt.Sprintf("Don't Starve Together Dedicated Server (Mock Build 642810) - Shard: %s", shardTitle))
	d.emit(ms, "info", fmt.Sprintf("Loading cluster=%s shard=%s leveldataoverride.lua...", ms.clusterID, shardTitle))

	select {
	case <-time.After(600 * time.Millisecond):
	case <-ctx.Done():
		return
	}
	d.emit(ms, "info", "Generating/Loading world snapshot and validating Klei cluster_token...")

	select {
	case <-time.After(600 * time.Millisecond):
	case <-ctx.Done():
		return
	}

	d.mu.Lock()
	if ms.state == StateStarting {
		ms.state = StateRunning
	}
	d.mu.Unlock()

	if ms.shard == "master" {
		d.emit(ms, "info", "Registering server with Klei Lobby...")
		d.emit(ms, "info", "Sim paused (World ready for connections)")
		d.emit(ms, "chat", "[Join Announcement] Wilson")
		d.emit(ms, "chat", "[Say] (KU_w1ls0n01) Wilson: Base camp is near the Florid Postern!")
		d.notifyPlayer(ms.clusterID, "KU_w1ls0n01", "Wilson")
	} else {
		d.emit(ms, "info", "Connected to Master shard successfully")
		d.emit(ms, "info", "Sim paused (Caves ready)")
	}

	ticker := time.NewTicker(12 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if ms.shard != "master" {
				continue
			}
			d.mu.Lock()
			if ms.state != StateRunning {
				d.mu.Unlock()
				return
			}
			w.phaseIdx = (w.phaseIdx + 1) % len(mockPhases)
			if w.phaseIdx == 0 {
				w.day++
				w.remDays--
				if w.remDays <= 0 {
					w.seasonIdx = (w.seasonIdx + 1) % len(mockSeasons)
					w.remDays = 15
				}
				dayNum := w.day
				season := mockSeasons[w.seasonIdx]
				d.mu.Unlock()
				d.emit(ms, "info", fmt.Sprintf("World advanced to Day %d (%s). Auto-saving snapshot...", dayNum, season))
				continue
			}

			// Occasional friendly mock event
			if rand.Intn(4) == 0 && len(w.players) < 3 {
				cand := mockSampleUsers[len(w.players)%len(mockSampleUsers)]
				w.players = append(w.players, cand)
				d.mu.Unlock()
				d.notifyPlayer(ms.clusterID, cand.KUID, cand.Name)
				d.emitEvent("player.join", "Player Joined", cand.Name+" joined the server", ms.clusterID)
				d.emit(ms, "chat", fmt.Sprintf("[Join Announcement] %s", cand.Name))
				d.emit(ms, "chat", fmt.Sprintf("[Say] (%s) %s: Anyone need gears or silk?", cand.KUID, cand.Name))
				continue
			}
			d.mu.Unlock()
		}
	}
}

func (d *MockDriver) StopShard(ctx context.Context, clusterID, shard string) error {
	k := d.key(clusterID, shard)
	d.mu.Lock()
	ms, ok := d.shards[k]
	if !ok || ms.state == StateStopped {
		d.mu.Unlock()
		return nil
	}
	ms.state = StateStopping
	d.mu.Unlock()

	d.emit(ms, "system", "[walrus] Sending graceful shutdown: c_shutdown(true)")
	d.emit(ms, "info", "Saving world snapshot before exit...")
	time.Sleep(250 * time.Millisecond)
	d.emit(ms, "info", "Shutting down cleanly.")

	d.mu.Lock()
	ms.cancel()
	ms.fileMu.Lock()
	if ms.logFile != nil {
		_ = ms.logFile.Close()
		ms.logFile = nil
	}
	ms.fileMu.Unlock()
	ms.state = StateStopped
	ms.pid = 0
	d.mu.Unlock()
	return nil
}

func (d *MockDriver) SendCommand(clusterID, shard, command string) error {
	command = strings.TrimSpace(command)
	if command == "" {
		return nil
	}
	k := d.key(clusterID, shard)
	d.mu.RLock()
	ms, ok := d.shards[k]
	if !ok || (ms.state != StateRunning && ms.state != StateStarting) {
		d.mu.RUnlock()
		normShard := strings.ToLower(shard)
		return apperr.Newf(apperr.CodeShardNotRunning, map[string]any{"cluster_id": clusterID, "shard": normShard}, "shard %s/%s is not running", clusterID, normShard)
	}
	w := d.worlds[clusterID]
	d.mu.RUnlock()

	d.emit(ms, "system", fmt.Sprintf("[Console Input] > %s", command))

	switch {
	case strings.HasPrefix(command, "c_save"):
		d.emit(ms, "info", "Manual save triggered. World snapshot saved successfully.")
	case strings.HasPrefix(command, "c_announce"):
		if m := announceRe.FindStringSubmatch(command); len(m) == 2 {
			d.emit(ms, "chat", fmt.Sprintf("[Announcement] %s", m[1]))
		} else {
			d.emit(ms, "chat", "[Announcement] "+command)
		}
	case strings.HasPrefix(command, "c_rollback"):
		days := 1
		if m := rollbackRe.FindStringSubmatch(command); len(m) == 2 && m[1] != "" {
			if n, err := strconv.Atoi(m[1]); err == nil && n > 0 {
				days = n
			}
		}
		d.mu.Lock()
		if w != nil {
			w.day -= days
			if w.day < 1 {
				w.day = 1
			}
		}
		d.mu.Unlock()
		d.emit(ms, "warn", fmt.Sprintf("Rolling back world by %d snapshot(s)... Reload complete.", days))
	case strings.HasPrefix(command, "c_listallplayers"):
		d.mu.RLock()
		if w != nil && len(w.players) > 0 {
			for i, p := range w.players {
				d.emit(ms, "info", fmt.Sprintf("[%d] (%s) %s <%s>", i+1, p.KUID, p.Name, p.Prefab))
			}
		} else {
			d.emit(ms, "info", "No players currently online.")
		}
		d.mu.RUnlock()
	case strings.HasPrefix(command, "c_shutdown"):
		go func() { _ = d.StopShard(context.Background(), clusterID, shard) }()
	case kickBanRe.MatchString(command):
		m := kickBanRe.FindStringSubmatch(command)
		action, targetID := m[1], m[2]
		var kickedName string
		d.mu.Lock()
		if w != nil {
			kept := w.players[:0]
			for _, p := range w.players {
				if p.KUID == targetID {
					kickedName = p.Name
					continue
				}
				kept = append(kept, p)
			}
			w.players = kept
		}
		d.mu.Unlock()
		if kickedName == "" {
			kickedName = targetID
		}
		d.emit(ms, "warn", fmt.Sprintf("Executed TheNet:%s(%s)", action, targetID))
		d.emit(ms, "chat", fmt.Sprintf("[Leave Announcement] %s", kickedName))
	case setSeasonRe.MatchString(command):
		m := setSeasonRe.FindStringSubmatch(command)
		targetSeason := m[1]
		d.mu.Lock()
		if w != nil {
			for i, sn := range mockSeasons {
				if sn == targetSeason {
					w.seasonIdx = i
					w.remDays = 15
					break
				}
			}
		}
		d.mu.Unlock()
		d.emit(ms, "info", fmt.Sprintf("Season changed to %s.", targetSeason))
	case strings.Contains(command, "ms_nextcycle"):
		d.mu.Lock()
		if w != nil {
			w.day++
		}
		d.mu.Unlock()
		d.emit(ms, "info", "Skipped to next day cycle.")
	default:
		d.emit(ms, "info", fmt.Sprintf("Remote Lua executed on %s: %s", shard, command))
	}
	return nil
}

func (d *MockDriver) TriggerProbe(clusterID string) error {
	return nil
}

func (d *MockDriver) ReadRecentChat(clusterID string, limit int) ([]ChatMessage, error) {
	dir, err := d.fs.ShardDir(clusterID, "master")
	if err != nil {
		return nil, err
	}
	return ReadChatLogFile(filepath.Join(dir, "server_chat_log.txt"), filepath.Join(dir, "server_log.txt"), limit)
}

func (d *MockDriver) GetShardStatus(clusterID, shard string) ShardStatus {
	shard = strings.ToLower(shard)
	k := d.key(clusterID, shard)
	d.mu.RLock()
	defer d.mu.RUnlock()

	ms, ok := d.shards[k]
	if !ok || ms.state == StateStopped {
		return ShardStatus{Shard: shard, State: StateStopped}
	}
	started := ms.startedAt
	uptime := int64(time.Since(started).Seconds())
	cpu := 2.4
	mem := 286.5
	if shard == "caves" {
		cpu = 1.6
		mem = 214.2
	}
	return ShardStatus{
		Shard:      shard,
		State:      ms.state,
		PID:        ms.pid,
		StartedAt:  &started,
		UptimeSec:  uptime,
		CPUPercent: cpu,
		MemoryMB:   mem,
	}
}

func (d *MockDriver) GetWorldInfo(clusterID string) WorldRuntimeInfo {
	d.mu.RLock()
	defer d.mu.RUnlock()

	master := d.shards[d.key(clusterID, "master")]
	if master == nil || master.state != StateRunning {
		return WorldRuntimeInfo{
			Season:        "unknown",
			Phase:         "unknown",
			OnlinePlayers: []OnlinePlayer{},
		}
	}
	w := d.ensureWorldLocked(clusterID)
	players := make([]OnlinePlayer, len(w.players))
	copy(players, w.players)
	return WorldRuntimeInfo{
		Day:                   w.day,
		Season:                mockSeasons[w.seasonIdx],
		RemainingDaysInSeason: w.remDays,
		Phase:                 mockPhases[w.phaseIdx],
		OnlinePlayers:         players,
	}
}

func (d *MockDriver) ReadRecentLogs(clusterID, shard string, limit int) ([]LogEntry, error) {
	if entries := d.logs.Recent(clusterID, shard, limit); len(entries) > 0 {
		return entries, nil
	}
	dir, err := d.fs.ShardDir(clusterID, shard)
	if err != nil {
		return nil, err
	}
	return ReadTailFromFile(filepath.Join(dir, "server_log.txt"), shard, limit)
}

func (d *MockDriver) SubscribeLogs(clusterID, shard string) (<-chan LogEntry, func()) {
	return d.logs.Subscribe(clusterID, shard)
}

func (d *MockDriver) HostMetrics() HostMetrics {
	return CollectHostMetrics(d.dataDir, d.Mode())
}

func (d *MockDriver) ShutdownAll(ctx context.Context) {
	d.mu.RLock()
	keys := make([][2]string, 0, len(d.shards))
	for _, ms := range d.shards {
		if ms.state == StateRunning || ms.state == StateStarting {
			keys = append(keys, [2]string{ms.clusterID, ms.shard})
		}
	}
	d.mu.RUnlock()
	for _, item := range keys {
		_ = d.StopShard(ctx, item[0], item[1])
	}
}
