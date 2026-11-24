export type Role = 'admin' | 'operator' | 'viewer'

export type ShardState = 'stopped' | 'starting' | 'running' | 'stopping' | 'crashed'

export interface User {
  id: number
  username: string
  role: Role
  disabled: boolean
  locale: string
  created_at: string
  updated_at: string
}

export interface UserClusterPermission {
  user_id?: number
  cluster_id: string
  role: Role
}

export interface UserWithPermissions extends User {
  permissions: UserClusterPermission[]
}

export interface ShardStatus {
  shard: 'master' | 'caves' | string
  state: ShardState
  pid: number
  started_at?: string
  uptime_sec: number
  cpu_percent: number
  memory_mb: number
  restart_count: number
  last_error?: string
}

export interface OnlinePlayer {
  ku_id: string
  name: string
  prefab: string
  age: number
}

export interface WorldRuntimeInfo {
  day: number
  season: 'autumn' | 'winter' | 'spring' | 'summer' | 'unknown' | string
  remaining_days_in_season: number
  phase: 'day' | 'dusk' | 'night' | 'unknown' | string
  online_players: OnlinePlayer[]
}

export interface ClusterSummary {
  cluster_id: string
  cluster_name: string
  description: string
  game_mode: string
  max_players: number
  caves_enabled: boolean
  auto_restart: boolean
  master_port: number
  caves_port?: number
  master_status: ShardStatus
  caves_status?: ShardStatus
  world_info: WorldRuntimeInfo
  effective_role: Role
  has_token: boolean
}

export interface ClusterINI {
  game_mode: string
  max_players: number
  pvp: boolean
  pause_when_empty: boolean
  cluster_name: string
  cluster_description: string
  cluster_password: string
  cluster_intention: string
  cluster_language: string
  lan_only_cluster: boolean
  offline_cluster: boolean
  autosaver_enabled: boolean
  tick_rate: number
  whitelist_slots: number
  console_enabled: boolean
  max_snapshots: number
  shard_enabled: boolean
  bind_ip: string
  master_ip: string
  master_port: number
  cluster_key: string
}

export interface ServerINI {
  server_port: number
  is_master: boolean
  name: string
  id: string
  encode_user_path: boolean
  master_server_port: number
  authentication_port: number
}

export interface ClusterMeta {
  cluster_id: string
  display_name: string
  description: string
  caves_enabled: boolean
  mod_sync_enabled: boolean
  auto_restart: boolean
  created_at: string
  updated_at: string
}

export interface ClusterDetail {
  cluster_id: string
  meta: ClusterMeta
  cluster_ini: ClusterINI
  master_server_ini: ServerINI
  caves_server_ini?: ServerINI
  masked_cluster_token: string
  has_cluster_token: boolean
  using_default_token: boolean
  master_status: ShardStatus
  caves_status?: ShardStatus
  world_info: WorldRuntimeInfo
}

export interface PortSet {
  master_server_port: number
  caves_server_port: number
  master_auth_port: number
  caves_auth_port: number
  master_steam_port: number
  caves_steam_port: number
  cluster_master_port: number
}

export interface WorldPreset {
  id: string
  shard: 'master' | 'caves'
  name_zh: string
  name_en: string
  description: string
  lua: string
}

export interface LogEntry {
  seq: number
  timestamp: string
  shard: string
  level: 'info' | 'warn' | 'error' | 'chat' | 'system' | string
  message: string
}

export interface HostMetrics {
  cpu_percent: number
  memory_used_mb: number
  memory_total_mb: number
  memory_percent: number
  disk_used_gb: number
  disk_total_gb: number
  disk_percent: number
  os: string
  arch: string
  runtime_mode: string
}

export interface BackupRecord {
  id: number
  cluster_id: string
  filename: string
  size_bytes: number
  trigger: string
  note: string
  created_by: string
  created_at: string
}

export interface SystemSettings {
  port_pool_start: number
  port_pool_end: number
  default_cluster_token: string
  has_default_token: boolean
  steam_api_key: string
  has_steam_api_key: boolean
  has_default_steam_api_key?: boolean
  dst_branch: string
  dst_arch: string
  auto_update_mode: string
  backup_max_retain: number
}

export interface AuditLog {
  id: number
  actor: string
  action: string
  target: string
  detail: string
  ip: string
  created_at: string
}

// --- Phase 2 Types ---

export interface SteamStatus {
  installed: boolean
  local_version: string
  remote_version: string
  update_needed: boolean
  branch: string
  task_state: 'idle' | 'running' | 'success' | 'failed'
  task_kind: string
  last_error?: string
  last_checked_at?: string
  last_updated_at?: string
  steamcmd_ready: boolean
  runtime_mode: string
}

export interface SteamLogLine {
  seq: number
  timestamp: string
  message: string
}

export interface WorldOptionChoice {
  value: string
  label_zh: string
  label_en: string
}

export interface WorldOptionField {
  key: string
  label_zh: string
  label_en: string
  default: string
  order?: number
  choices: WorldOptionChoice[]
}

export type WorldOptionSection = 'settings' | 'worldgen'

export interface WorldOptionGroup {
  id: string
  section: WorldOptionSection
  name_zh: string
  name_en: string
  options: WorldOptionField[]
}

export interface ParsedLevelData {
  id: string
  settings_id?: string
  worldgen_id?: string
  name: string
  location: string
  playstyle: string
  overrides: Record<string, unknown>
}

export interface WorldConfigView {
  shard: string
  lua: string
  parsed?: ParsedLevelData
  schema: WorldOptionGroup[]
}

export interface ModOptionChoice {
  description: string
  data: unknown
  hover?: string
}

export interface ModConfigOption {
  name: string
  label: string
  hover?: string
  default: unknown
  options: ModOptionChoice[]
}

export interface ClusterModItem {
  mod_id: string
  workshop_id?: string
  is_local: boolean
  enabled: boolean
  downloaded: boolean
  name: string
  author: string
  version: string
  description: string
  preview_url?: string
  client_only: boolean
  all_clients_require_mod: boolean
  schema_options: ModConfigOption[]
  configuration_options: Record<string, unknown>
}

export interface ClusterModsResponse {
  shard: string
  mod_sync_enabled: boolean
  raw_lua: string
  mods: ClusterModItem[]
}

export interface WorkshopItem {
  workshop_id: string
  mod_id: string
  title: string
  description: string
  preview_url: string
  time_updated: number
  subscriptions: number
  file_size: number
  tags: string[]
  is_client_only: boolean
}

export interface ModCache {
  mod_id: string
  workshop_id: string
  is_local: boolean
  name: string
  author: string
  version: string
  description: string
  preview_url: string
  client_only: boolean
  server_tags: string
  time_updated: number
  last_synced_at: string
}

export interface PlayerHistory {
  ku_id: string
  last_name: string
  last_cluster_id: string
  last_seen_at: string
}

export interface PlayerLists {
  adminlist: string[]
  whitelist: string[]
  blocklist: string[]
  history: PlayerHistory[]
}

export interface ChatMessage {
  timestamp: string
  kind: 'say' | 'announcement' | 'join' | 'leave' | 'death' | 'system' | string
  ku_id?: string
  sender?: string
  message: string
}

export interface CommandSnippet {
  id: number
  name: string
  shard: string
  command: string
  category: string
  created_at: string
}

// --- Phase 3 Types ---

export interface FileEntry {
  name: string
  rel_path: string
  is_dir: boolean
  size_bytes: number
  mod_time: string
  editable: boolean
  protected: boolean
}

export interface FileContent {
  rel_path: string
  size_bytes: number
  editable: boolean
  truncated: boolean
  is_binary: boolean
  content: string
}

export interface ScheduledTask {
  id: number
  cluster_id: string
  name: string
  task_type: 'backup' | 'restart' | 'announce' | 'update' | string
  cron_expr: string
  params_json: string
  enabled: boolean
  last_run_at?: string
  last_status: string
  created_at: string
}

export interface WebhookConfig {
  id: number
  name: string
  template: 'generic' | 'discord' | 'feishu' | 'dingtalk' | 'wecom' | 'telegram' | 'bark' | string
  url: string
  secret: string
  events: string[]
  enabled: boolean
  created_at: string
}

export interface WebhookDelivery {
  id: number
  webhook_id: number
  event: string
  success: boolean
  status_code: number
  error?: string
  created_at: string
}
