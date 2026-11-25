import type {
  AuditLog,
  BackupRecord,
  ChatMessage,
  ClusterDetail,
  ClusterINI,
  ClusterModsResponse,
  ClusterSummary,
  CommandSnippet,
  FileContent,
  FileEntry,
  HostMetrics,
  ModCache,
  PlayerLists,
  PortSet,
  ProxyTestResult,
  Role,
  ScheduledTask,
  ServerINI,
  SteamStatus,
  SystemSettings,
  SystemVersionInfo,
  User,
  UserClusterPermission,
  UserWithPermissions,
  WebhookConfig,
  WebhookDelivery,
  WorkshopItem,
  WorldConfigView,
  WorldPreset,
  WorldRuntimeInfo,
} from '@/types/api'

import { ApiError, buildApiError } from '@/lib/errors'

export { ApiError, formatApiError, formatError } from '@/lib/errors'

export const AUTH_UNAUTHORIZED_EVENT = 'walrus:unauthorized'

const AUTH_EXEMPT_401_PATHS = new Set([
  '/api/v1/auth/login',
  '/api/v1/auth/me',
  '/api/v1/auth/status',
  '/api/v1/auth/setup',
])

function notifyUnauthorized(path: string) {
  if (typeof window === 'undefined') return
  if (AUTH_EXEMPT_401_PATHS.has(path)) return
  window.dispatchEvent(new CustomEvent(AUTH_UNAUTHORIZED_EVENT))
}

async function safeFetch(input: RequestInfo | URL, init?: RequestInit): Promise<Response> {
  try {
    return await fetch(input, init)
  } catch (err) {
    const raw = err instanceof Error ? err.message : 'Network error'
    throw new ApiError(raw, 0, { code: 'NETWORK_ERROR', rawMessage: raw })
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await safeFetch(path, {
    credentials: 'include',
    headers: {
      'Content-Type': 'application/json',
      ...(init?.headers ?? {}),
    },
    ...init,
  })
  const text = await res.text()
  let data: unknown = {}
  if (text) {
    try {
      data = JSON.parse(text)
    } catch {
      data = { error: text }
    }
  }
  if (!res.ok) {
    if (res.status === 401) {
      notifyUnauthorized(path)
    }
    throw buildApiError(data, res.status)
  }
  return data as T
}

export const api = {
  // Auth
  authStatus: () => request<{ initialized: boolean; runtime_mode: string }>('/api/v1/auth/status'),
  setup: (body: { username: string; password: string; default_cluster_token?: string; locale?: string }) =>
    request<{ user: User; token: string }>('/api/v1/auth/setup', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  login: (body: { username: string; password: string }) =>
    request<{ user: User; token: string }>('/api/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  logout: () => request<{ ok: boolean }>('/api/v1/auth/logout', { method: 'POST' }),
  me: () => request<{ user: User; permissions: UserClusterPermission[] }>('/api/v1/auth/me'),

  // System
  healthz: () => request<{ status: string; runtime: string; version: string }>('/healthz'),
  metrics: () => request<{ metrics: HostMetrics }>('/api/v1/system/metrics'),
  publicIP: () => request<{ ip: string }>('/api/v1/system/public-ip'),
  systemVersion: () => request<{ info: SystemVersionInfo }>('/api/v1/system/version'),
  checkSystemVersion: () =>
    request<{ info: SystemVersionInfo }>('/api/v1/system/version/check', { method: 'POST' }),
  triggerSystemUpgrade: (target_image?: string) =>
    request<{ info: SystemVersionInfo }>('/api/v1/system/upgrade', {
      method: 'POST',
      body: JSON.stringify(target_image ? { target_image } : {}),
    }),
  presets: () => request<{ presets: WorldPreset[] }>('/api/v1/presets'),
  getSettings: () =>
    request<{ settings: SystemSettings; runtime_mode: string; data_dir: string }>(
      '/api/v1/system/settings',
    ),
  updateSettings: (body: Partial<SystemSettings>) =>
    request<{ settings: SystemSettings }>('/api/v1/system/settings', {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  testProxy: (proxy_url?: string) =>
    request<{ result: ProxyTestResult }>('/api/v1/system/proxy/test', {
      method: 'POST',
      body: JSON.stringify(proxy_url ? { proxy_url } : {}),
    }),
  auditLogs: (limit = 100, action = '') =>
    request<{ logs: AuditLog[] }>(`/api/v1/audit-logs?limit=${limit}&action=${encodeURIComponent(action)}`),

  // Users
  listUsers: () => request<{ users: UserWithPermissions[] }>('/api/v1/users'),
  createUser: (body: {
    username: string
    password: string
    role: Role
    locale?: string
    permissions?: UserClusterPermission[]
  }) =>
    request<{ user: User }>('/api/v1/users', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  updateUser: (
    uid: number,
    body: {
      role: Role
      disabled: boolean
      password?: string
      locale?: string
      permissions?: UserClusterPermission[]
    },
  ) =>
    request<{ user: User }>(`/api/v1/users/${uid}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  deleteUser: (uid: number) => request<{ ok: boolean }>(`/api/v1/users/${uid}`, { method: 'DELETE' }),

  // Clusters
  listClusters: () => request<{ clusters: ClusterSummary[] }>('/api/v1/clusters'),
  previewNextPorts: () => request<{ ports: PortSet }>('/api/v1/clusters/next-ports'),
  createCluster: (body: {
    cluster_id: string
    cluster_name: string
    cluster_description?: string
    cluster_password?: string
    game_mode: string
    max_players: number
    pvp: boolean
    pause_when_empty: boolean
    cluster_intention: string
    caves_enabled: boolean
    master_preset: string
    cluster_token?: string
    auto_restart: boolean
  }) =>
    request<{ cluster: ClusterDetail }>('/api/v1/clusters', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  getCluster: (id: string) =>
    request<{ cluster: ClusterDetail; effective_role: Role }>(`/api/v1/clusters/${encodeURIComponent(id)}`),
  updateClusterConfig: (
    id: string,
    body: {
      cluster_ini?: ClusterINI
      master_server_ini?: ServerINI
      caves_server_ini?: ServerINI
      caves_enabled?: boolean
      auto_restart?: boolean
      mod_sync_enabled?: boolean
      cluster_token?: string
    },
  ) =>
    request<{ cluster: ClusterDetail }>(`/api/v1/clusters/${encodeURIComponent(id)}/config`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  deleteCluster: (id: string) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}`, { method: 'DELETE' }),

  // World Lua & Visual Overrides
  readWorldLua: (id: string, shard: 'master' | 'caves') =>
    request<WorldConfigView>(`/api/v1/clusters/${encodeURIComponent(id)}/world/${shard}`),
  writeWorldLua: (id: string, shard: 'master' | 'caves', lua: string) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/world/${shard}`, {
      method: 'PUT',
      body: JSON.stringify({ lua }),
    }),
  patchWorldOverrides: (id: string, shard: 'master' | 'caves', overrides: Record<string, unknown>) =>
    request<{ ok: boolean; lua: string }>(`/api/v1/clusters/${encodeURIComponent(id)}/world/${shard}/overrides`, {
      method: 'PUT',
      body: JSON.stringify({ overrides }),
    }),

  // SteamCMD
  steamStatus: () => request<{ status: SteamStatus }>('/api/v1/steam/status'),
  steamCheck: () => request<{ status: SteamStatus }>('/api/v1/steam/check', { method: 'POST' }),
  steamUpdate: (body: { branch?: string; validate?: boolean }) =>
    request<{ status: SteamStatus }>('/api/v1/steam/update', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  steamCancel: () => request<{ cancelled: boolean; status: SteamStatus }>('/api/v1/steam/cancel', { method: 'POST' }),
  wsSteamLogsUrl: () => {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${proto}//${window.location.host}/api/v1/ws/steam/logs`
  },

  // Mods
  getClusterMods: (id: string, shard: 'master' | 'caves', locale: string) =>
    request<ClusterModsResponse>(
      `/api/v1/clusters/${encodeURIComponent(id)}/mods?shard=${shard}&locale=${encodeURIComponent(locale)}`,
    ),
  addWorkshopMod: (
    id: string,
    body: { shard: 'master' | 'caves'; input: string; is_collection: boolean; locale: string },
  ) =>
    request<{ added: string[] }>(`/api/v1/clusters/${encodeURIComponent(id)}/mods/workshop`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  addLocalModToCluster: (id: string, body: { shard: 'master' | 'caves'; mod_id: string; locale: string }) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/mods/local`, {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  saveClusterMod: (
    id: string,
    modId: string,
    body: { shard: 'master' | 'caves'; enabled: boolean; configuration_options: Record<string, unknown> },
  ) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/mods/${encodeURIComponent(modId)}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  removeClusterMod: (id: string, modId: string, shard: 'master' | 'caves') =>
    request<{ ok: boolean }>(
      `/api/v1/clusters/${encodeURIComponent(id)}/mods/${encodeURIComponent(modId)}?shard=${shard}`,
      { method: 'DELETE' },
    ),
  saveRawModOverrides: (id: string, shard: 'master' | 'caves', lua: string) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/mods/raw`, {
      method: 'PUT',
      body: JSON.stringify({ shard, lua }),
    }),
  setModSync: (id: string, enabled: boolean) =>
    request<{ ok: boolean; mod_sync_enabled: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/mods/sync`, {
      method: 'PUT',
      body: JSON.stringify({ enabled }),
    }),
  redownloadMod: (modId: string, locale: string) =>
    request<{ ok: boolean }>(`/api/v1/mods/${encodeURIComponent(modId)}/download?locale=${encodeURIComponent(locale)}`, {
      method: 'POST',
    }),
  searchWorkshop: (q: string, page = 1) =>
    request<{ items: WorkshopItem[]; total: number; page: number; is_mock: boolean }>(
      `/api/v1/mods/workshop/search?q=${encodeURIComponent(q)}&page=${page}`,
    ),
  listModLibrary: () => request<{ mods: ModCache[] }>('/api/v1/mods/library'),
  uploadLocalMod: async (file: File, locale: string): Promise<{ mod: ModCache }> => {
    const fd = new FormData()
    fd.append('file', file)
    fd.append('locale', locale)
    const res = await safeFetch('/api/v1/mods/local/upload', {
      method: 'POST',
      credentials: 'include',
      body: fd,
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      if (res.status === 401) notifyUnauthorized('/api/v1/mods/local/upload')
      throw buildApiError(data, res.status)
    }
    return data as { mod: ModCache }
  },
  deleteLocalMod: (modId: string) =>
    request<{ ok: boolean }>(`/api/v1/mods/local/${encodeURIComponent(modId)}`, { method: 'DELETE' }),

  // Players, Lists, Chat & GM Snippets
  getPlayers: (id: string) =>
    request<{ world_info: WorldRuntimeInfo; lists: PlayerLists }>(`/api/v1/clusters/${encodeURIComponent(id)}/players`),
  savePlayerLists: (id: string, body: { adminlist: string[]; whitelist: string[]; blocklist: string[] }) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/players/lists`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  playerAction: (id: string, action: 'kick' | 'ban' | 'revive', ku_id: string) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/players/action`, {
      method: 'POST',
      body: JSON.stringify({ action, ku_id }),
    }),
  triggerProbe: (id: string) =>
    request<{ world_info: WorldRuntimeInfo }>(`/api/v1/clusters/${encodeURIComponent(id)}/probe`, {
      method: 'POST',
    }),
  getChat: (id: string, limit = 150) =>
    request<{ messages: ChatMessage[] }>(`/api/v1/clusters/${encodeURIComponent(id)}/chat?limit=${limit}`),
  listSnippets: () => request<{ snippets: CommandSnippet[] }>('/api/v1/snippets'),
  createSnippet: (body: { name: string; shard: string; command: string; category?: string }) =>
    request<{ snippet: CommandSnippet }>('/api/v1/snippets', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  deleteSnippet: (sid: number) => request<{ ok: boolean }>(`/api/v1/snippets/${sid}`, { method: 'DELETE' }),

  // Phase 3: Import Cluster ZIP
  importClusterZip: async (body: {
    file: File
    cluster_id: string
    cluster_name?: string
    cluster_token?: string
  }): Promise<{ cluster: ClusterDetail }> => {
    const fd = new FormData()
    fd.append('file', body.file)
    fd.append('cluster_id', body.cluster_id)
    if (body.cluster_name) fd.append('cluster_name', body.cluster_name)
    if (body.cluster_token) fd.append('cluster_token', body.cluster_token)
    const res = await safeFetch('/api/v1/clusters/import', {
      method: 'POST',
      credentials: 'include',
      body: fd,
    })
    const data = await res.json().catch(() => ({}))
    if (!res.ok) {
      if (res.status === 401) notifyUnauthorized('/api/v1/clusters/import')
      throw buildApiError(data, res.status)
    }
    return data as { cluster: ClusterDetail }
  },

  // Phase 3: Sandboxed Cluster Files
  listFiles: (id: string, path = '') =>
    request<{ path: string; entries: FileEntry[] }>(
      `/api/v1/clusters/${encodeURIComponent(id)}/files?path=${encodeURIComponent(path)}`,
    ),
  readFile: (id: string, path: string) =>
    request<FileContent>(
      `/api/v1/clusters/${encodeURIComponent(id)}/files/content?path=${encodeURIComponent(path)}`,
    ),
  writeFile: (id: string, path: string, content: string) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/files/content`, {
      method: 'PUT',
      body: JSON.stringify({ path, content }),
    }),
  deleteFile: (id: string, path: string) =>
    request<{ ok: boolean }>(
      `/api/v1/clusters/${encodeURIComponent(id)}/files?path=${encodeURIComponent(path)}`,
      { method: 'DELETE' },
    ),
  fileDownloadUrl: (id: string, path: string) =>
    `/api/v1/clusters/${encodeURIComponent(id)}/files/download?path=${encodeURIComponent(path)}`,

  // Phase 3: Cron Schedules
  listClusterSchedules: (id: string) =>
    request<{ tasks: ScheduledTask[] }>(`/api/v1/clusters/${encodeURIComponent(id)}/schedules`),
  listAllSchedules: () => request<{ tasks: ScheduledTask[] }>('/api/v1/schedules'),
  createSchedule: (body: {
    cluster_id: string
    name: string
    task_type: string
    cron_expr: string
    params: Record<string, unknown>
    enabled: boolean
  }) =>
    request<{ task: ScheduledTask }>('/api/v1/schedules', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  updateSchedule: (
    tid: number,
    body: {
      cluster_id: string
      name: string
      task_type: string
      cron_expr: string
      params: Record<string, unknown>
      enabled: boolean
    },
  ) =>
    request<{ task: ScheduledTask }>(`/api/v1/schedules/${tid}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  deleteSchedule: (tid: number) => request<{ ok: boolean }>(`/api/v1/schedules/${tid}`, { method: 'DELETE' }),
  runScheduleNow: (tid: number) => request<{ ok: boolean }>(`/api/v1/schedules/${tid}/run`, { method: 'POST' }),

  // Phase 3: Webhooks
  listWebhooks: () =>
    request<{ webhooks: WebhookConfig[]; deliveries: WebhookDelivery[]; all_events: string[] }>('/api/v1/webhooks'),
  createWebhook: (body: {
    name: string
    template: string
    url: string
    secret: string
    events: string[]
    enabled: boolean
  }) =>
    request<{ webhook: WebhookConfig }>('/api/v1/webhooks', {
      method: 'POST',
      body: JSON.stringify(body),
    }),
  updateWebhook: (
    wid: number,
    body: {
      name: string
      template: string
      url: string
      secret: string
      events: string[]
      enabled: boolean
    },
  ) =>
    request<{ webhook: WebhookConfig }>(`/api/v1/webhooks/${wid}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    }),
  deleteWebhook: (wid: number) => request<{ ok: boolean }>(`/api/v1/webhooks/${wid}`, { method: 'DELETE' }),
  testWebhook: (wid: number) =>
    request<{ delivery: WebhookDelivery }>(`/api/v1/webhooks/${wid}/test`, { method: 'POST' }),

  // Lifecycle & Console
  startCluster: (id: string, shard?: 'master' | 'caves') =>
    request<{ cluster: ClusterDetail }>(
      `/api/v1/clusters/${encodeURIComponent(id)}/start${shard ? `?shard=${shard}` : ''}`,
      { method: 'POST' },
    ),
  stopCluster: (id: string, shard?: 'master' | 'caves') =>
    request<{ cluster: ClusterDetail }>(
      `/api/v1/clusters/${encodeURIComponent(id)}/stop${shard ? `?shard=${shard}` : ''}`,
      { method: 'POST' },
    ),
  restartCluster: (id: string, shard?: 'master' | 'caves') =>
    request<{ cluster: ClusterDetail }>(
      `/api/v1/clusters/${encodeURIComponent(id)}/restart${shard ? `?shard=${shard}` : ''}`,
      { method: 'POST' },
    ),
  sendCommand: (id: string, shard: 'master' | 'caves', command: string) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/command`, {
      method: 'POST',
      body: JSON.stringify({ shard, command }),
    }),

  // Backups
  listBackups: (id: string) =>
    request<{ backups: BackupRecord[] }>(`/api/v1/clusters/${encodeURIComponent(id)}/backups`),
  createBackup: (id: string, note: string) =>
    request<{ backup: BackupRecord }>(`/api/v1/clusters/${encodeURIComponent(id)}/backups`, {
      method: 'POST',
      body: JSON.stringify({ note }),
    }),
  restoreBackup: (id: string, bid: number) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/backups/${bid}/restore`, {
      method: 'POST',
    }),
  deleteBackup: (id: string, bid: number) =>
    request<{ ok: boolean }>(`/api/v1/clusters/${encodeURIComponent(id)}/backups/${bid}`, {
      method: 'DELETE',
    }),
  backupDownloadUrl: (id: string, bid: number) =>
    `/api/v1/clusters/${encodeURIComponent(id)}/backups/${bid}/download`,

  wsLogsUrl: (id: string, shard: 'master' | 'caves') => {
    const proto = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
    return `${proto}//${window.location.host}/api/v1/ws/clusters/${encodeURIComponent(id)}/logs?shard=${shard}`
  },
}
