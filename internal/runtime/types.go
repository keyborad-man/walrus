package runtime

import (
	"context"
	"time"
)

type ShardState string

const (
	StateStopped  ShardState = "stopped"
	StateStarting ShardState = "starting"
	StateRunning  ShardState = "running"
	StateStopping ShardState = "stopping"
	StateCrashed  ShardState = "crashed"
)

type ShardStatus struct {
	Shard        string     `json:"shard"`
	State        ShardState `json:"state"`
	PID          int        `json:"pid"`
	StartedAt    *time.Time `json:"started_at,omitempty"`
	UptimeSec    int64      `json:"uptime_sec"`
	CPUPercent   float64    `json:"cpu_percent"`
	MemoryMB     float64    `json:"memory_mb"`
	RestartCount int        `json:"restart_count"`
	LastError    string     `json:"last_error,omitempty"`
}

type OnlinePlayer struct {
	KUID   string `json:"ku_id"`
	Name   string `json:"name"`
	Prefab string `json:"prefab"`
	Age    int    `json:"age"`
}

type WorldRuntimeInfo struct {
	Day                   int            `json:"day"`
	Season                string         `json:"season"` // autumn | winter | spring | summer | unknown
	RemainingDaysInSeason int            `json:"remaining_days_in_season"`
	Phase                 string         `json:"phase"` // day | dusk | night | unknown
	OnlinePlayers         []OnlinePlayer `json:"online_players"`
}

type LogEntry struct {
	Seq       int64     `json:"seq"`
	Timestamp time.Time `json:"timestamp"`
	Shard     string    `json:"shard"`
	Level     string    `json:"level"` // info | warn | error | chat | system
	Message   string    `json:"message"`
}

type ChatMessage struct {
	Timestamp string `json:"timestamp"`
	Kind      string `json:"kind"` // say | announcement | join | leave | death | system
	KUID      string `json:"ku_id,omitempty"`
	Sender    string `json:"sender,omitempty"`
	Message   string `json:"message"`
}

type PlayerSeenHook func(kuID, name, clusterID string)

type EventHook func(event, title, message, clusterID string)

type HostMetrics struct {
	CPUPercent    float64 `json:"cpu_percent"`
	MemoryUsedMB  float64 `json:"memory_used_mb"`
	MemoryTotalMB float64 `json:"memory_total_mb"`
	MemoryPercent float64 `json:"memory_percent"`
	DiskUsedGB    float64 `json:"disk_used_gb"`
	DiskTotalGB   float64 `json:"disk_total_gb"`
	DiskPercent   float64 `json:"disk_percent"`
	OS            string  `json:"os"`
	Arch          string  `json:"arch"`
	RuntimeMode   string  `json:"runtime_mode"`
}

type StartOptions struct {
	AutoRestart bool
	Arch        string // "64" or "32"
}

type Driver interface {
	Mode() string
	SetPlayerSeenHook(hook PlayerSeenHook)
	SetEventHook(hook EventHook)
	StartShard(ctx context.Context, clusterID, shard string, opts StartOptions) error
	StopShard(ctx context.Context, clusterID, shard string) error
	SendCommand(clusterID, shard, command string) error
	TriggerProbe(clusterID string) error
	GetShardStatus(clusterID, shard string) ShardStatus
	GetWorldInfo(clusterID string) WorldRuntimeInfo
	ReadRecentLogs(clusterID, shard string, limit int) ([]LogEntry, error)
	ReadRecentChat(clusterID string, limit int) ([]ChatMessage, error)
	SubscribeLogs(clusterID, shard string) (<-chan LogEntry, func())
	HostMetrics() HostMetrics
	ShutdownAll(ctx context.Context)
}
