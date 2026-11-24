package db

import "time"

type Role string

const (
	RoleAdmin    Role = "admin"
	RoleOperator Role = "operator"
	RoleViewer   Role = "viewer"
)

func (r Role) Rank() int {
	switch r {
	case RoleAdmin:
		return 3
	case RoleOperator:
		return 2
	case RoleViewer:
		return 1
	default:
		return 0
	}
}

func (r Role) Valid() bool {
	return r == RoleAdmin || r == RoleOperator || r == RoleViewer
}

type User struct {
	ID           int64     `json:"id"`
	Username     string    `json:"username"`
	PasswordHash string    `json:"-"`
	Role         Role      `json:"role"`
	Disabled     bool      `json:"disabled"`
	Locale       string    `json:"locale"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

type UserClusterPermission struct {
	UserID    int64  `json:"user_id"`
	ClusterID string `json:"cluster_id"`
	Role      Role   `json:"role"`
}

type ClusterMeta struct {
	ClusterID      string    `json:"cluster_id"`
	DisplayName    string    `json:"display_name"`
	Description    string    `json:"description"`
	CavesEnabled   bool      `json:"caves_enabled"`
	ModSyncEnabled bool      `json:"mod_sync_enabled"`
	AutoRestart    bool      `json:"auto_restart"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type BackupRecord struct {
	ID        int64     `json:"id"`
	ClusterID string    `json:"cluster_id"`
	Filename  string    `json:"filename"`
	SizeBytes int64     `json:"size_bytes"`
	Trigger   string    `json:"trigger"` // manual, scheduled, pre_restore, pre_update
	Note      string    `json:"note"`
	CreatedBy string    `json:"created_by"`
	CreatedAt time.Time `json:"created_at"`
}

type AuditLog struct {
	ID        int64     `json:"id"`
	Actor     string    `json:"actor"`
	Action    string    `json:"action"`
	Target    string    `json:"target"`
	Detail    string    `json:"detail"`
	IP        string    `json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}

type SystemSettings struct {
	PortPoolStart       int    `json:"port_pool_start"`
	PortPoolEnd         int    `json:"port_pool_end"`
	DefaultClusterToken string `json:"default_cluster_token"`
	HasDefaultToken     bool   `json:"has_default_token"`
	SteamAPIKey           string `json:"steam_api_key"`
	HasSteamAPIKey        bool   `json:"has_steam_api_key"`
	HasDefaultSteamAPIKey bool   `json:"has_default_steam_api_key"`
	DSTBranch           string `json:"dst_branch"`       // public | updatebeta
	DSTArch             string `json:"dst_arch"`         // 64 | 32
	AutoUpdateMode      string `json:"auto_update_mode"` // manual | when_empty | scheduled
	BackupMaxRetain     int    `json:"backup_max_retain"`
}

type ModCache struct {
	ModID        string    `json:"mod_id"` // e.g. "workshop-378160973" or local folder name
	WorkshopID   string    `json:"workshop_id"`
	IsLocal      bool      `json:"is_local"`
	Name         string    `json:"name"`
	Author       string    `json:"author"`
	Version      string    `json:"version"`
	Description  string    `json:"description"`
	PreviewURL   string    `json:"preview_url"`
	ClientOnly   bool      `json:"client_only"`
	ServerTags   string    `json:"server_tags"`
	TimeUpdated  int64     `json:"time_updated"`
	LastSyncedAt time.Time `json:"last_synced_at"`
}

type PlayerHistory struct {
	KUID          string    `json:"ku_id"`
	LastName      string    `json:"last_name"`
	LastClusterID string    `json:"last_cluster_id"`
	LastSeenAt    time.Time `json:"last_seen_at"`
}

type CommandSnippet struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Shard     string    `json:"shard"`
	Command   string    `json:"command"`
	Category  string    `json:"category"`
	CreatedAt time.Time `json:"created_at"`
}

type ScheduledTask struct {
	ID         int64      `json:"id"`
	ClusterID  string     `json:"cluster_id"` // specific cluster_id or "*"
	Name       string     `json:"name"`
	TaskType   string     `json:"task_type"` // backup | restart | announce | update
	CronExpr   string     `json:"cron_expr"` // 5-field standard cron
	ParamsJSON string     `json:"params_json"`
	Enabled    bool       `json:"enabled"`
	LastRunAt  *time.Time `json:"last_run_at,omitempty"`
	LastStatus string     `json:"last_status"`
	CreatedAt  time.Time  `json:"created_at"`
}

type WebhookConfig struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	Template  string    `json:"template"` // generic | discord | feishu | dingtalk | wecom | telegram | bark
	URL       string    `json:"url"`
	Secret    string    `json:"secret"`
	Events    []string  `json:"events"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
}

type WebhookDelivery struct {
	ID         int64     `json:"id"`
	WebhookID  int64     `json:"webhook_id"`
	Event      string    `json:"event"`
	Success    bool      `json:"success"`
	StatusCode int       `json:"status_code"`
	Error      string    `json:"error,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}
