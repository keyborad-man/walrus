import { describeCron } from '@/lib/utils'

export const CLUSTER_ID_RE = /^[a-zA-Z0-9_-]{2,48}$/
export const KU_ID_RE = /^KU_[a-zA-Z0-9_-]{6,32}$/

const CHARACTER_MAP: Record<string, { zh: string; en: string }> = {
  wilson: { zh: '威尔逊', en: 'Wilson' },
  willow: { zh: '薇洛', en: 'Willow' },
  wolfgang: { zh: '沃尔夫冈', en: 'Wolfgang' },
  wendy: { zh: '温蒂', en: 'Wendy' },
  wx78: { zh: 'WX-78', en: 'WX-78' },
  'wx-78': { zh: 'WX-78', en: 'WX-78' },
  wickerbottom: { zh: '薇克巴顿', en: 'Wickerbottom' },
  woodie: { zh: '伍迪', en: 'Woodie' },
  wes: { zh: '韦斯', en: 'Wes' },
  waxwell: { zh: '麦斯威尔', en: 'Maxwell' },
  maxwell: { zh: '麦斯威尔', en: 'Maxwell' },
  wathgrithr: { zh: '薇格弗德', en: 'Wigfrid' },
  wigfrid: { zh: '薇格弗德', en: 'Wigfrid' },
  webber: { zh: '韦伯', en: 'Webber' },
  winona: { zh: '薇诺娜', en: 'Winona' },
  warly: { zh: '沃利', en: 'Warly' },
  wortox: { zh: '沃拓克斯', en: 'Wortox' },
  wormwood: { zh: '沃姆伍德', en: 'Wormwood' },
  wurt: { zh: '沃特', en: 'Wurt' },
  walter: { zh: '沃尔特', en: 'Walter' },
  wanda: { zh: '旺达', en: 'Wanda' },
  wonkey: { zh: '芜猴', en: 'Wonkey' },
}

export function formatCharacterName(prefab: string, isEn: boolean): string {
  if (!prefab) return '—'
  const key = prefab.trim().toLowerCase()
  const hit = CHARACTER_MAP[key]
  if (hit) return isEn ? hit.en : hit.zh
  return prefab.charAt(0).toUpperCase() + prefab.slice(1)
}

const AUDIT_ACTION_MAP: Record<string, { zh: string; en: string }> = {
  'auth.setup': { zh: '初始化管理员', en: 'Initial Setup' },
  'auth.login': { zh: '账号登录', en: 'Sign In' },
  'auth.logout': { zh: '退出登录', en: 'Sign Out' },
  'cluster.create': { zh: '新建集群', en: 'Create Cluster' },
  'cluster.import': { zh: '导入存档 ZIP', en: 'Import Save ZIP' },
  'cluster.delete': { zh: '删除集群', en: 'Delete Cluster' },
  'cluster.start': { zh: '启动集群', en: 'Start Cluster' },
  'cluster.stop': { zh: '停止集群', en: 'Stop Cluster' },
  'cluster.restart': { zh: '重启集群', en: 'Restart Cluster' },
  'cluster.config': { zh: '修改房间配置', en: 'Update Room Config' },
  'cluster.world': { zh: '保存世界配置', en: 'Save World Config' },
  'console.command': { zh: '发送控制台指令', en: 'Console Command' },
  'mod.add': { zh: '添加模组', en: 'Add Mod' },
  'mod.save': { zh: '保存模组配置', en: 'Save Mod Config' },
  'mod.remove': { zh: '移除模组', en: 'Remove Mod' },
  'mod.sync': { zh: '切换模组同步', en: 'Toggle Mod Sync' },
  'player.kick': { zh: '踢出玩家', en: 'Kick Player' },
  'player.ban': { zh: '封禁玩家', en: 'Ban Player' },
  'player.revive': { zh: '复活玩家', en: 'Revive Player' },
  'player.lists': { zh: '保存玩家名单', en: 'Save Player Lists' },
  'backup.create': { zh: '创建存档备份', en: 'Create Backup' },
  'backup.restore': { zh: '恢复存档备份', en: 'Restore Backup' },
  'backup.delete': { zh: '删除存档备份', en: 'Delete Backup' },
  'schedule.create': { zh: '新建定时任务', en: 'Create Schedule' },
  'schedule.update': { zh: '更新定时任务', en: 'Update Schedule' },
  'schedule.delete': { zh: '删除定时任务', en: 'Delete Schedule' },
  'schedule.run': { zh: '立即执行定时任务', en: 'Run Schedule Now' },
  'file.write': { zh: '保存集群文件', en: 'Write Cluster File' },
  'file.delete': { zh: '删除集群文件', en: 'Delete Cluster File' },
  'steam.update': { zh: '更新 DST 服务端', en: 'Update DST Server' },
  'settings.update': { zh: '修改系统设置', en: 'Update System Settings' },
  'user.create': { zh: '创建用户账号', en: 'Create User' },
  'user.update': { zh: '修改用户权限', en: 'Update User' },
  'user.delete': { zh: '删除用户账号', en: 'Delete User' },
  'webhook.create': { zh: '新增通知渠道', en: 'Create Webhook' },
  'webhook.update': { zh: '修改通知渠道', en: 'Update Webhook' },
  'webhook.delete': { zh: '删除通知渠道', en: 'Delete Webhook' },
}

export function formatAuditAction(action: string, isEn: boolean): string {
  const hit = AUDIT_ACTION_MAP[action.trim().toLowerCase()]
  if (hit) return isEn ? hit.en : hit.zh
  return action
}

function escapeLuaString(value: string): string {
  return value.replace(/\\/g, '\\\\').replace(/"/g, '\\"').replace(/\r/g, '\\r').replace(/\n/g, '\\n')
}

export function formatConnectCommand(host: string, port: number, password?: string): string {
  const fallbackHost = typeof window !== 'undefined' ? window.location.hostname : ''
  const resolvedHost = escapeLuaString(host.trim() || fallbackHost || '127.0.0.1')
  if (password) {
    return `c_connect("${resolvedHost}", ${port}, "${escapeLuaString(password)}")`
  }
  return `c_connect("${resolvedHost}", ${port})`
}

const SPECIAL_CRONS = new Set(['@yearly', '@annually', '@monthly', '@weekly', '@daily', '@midnight', '@hourly'])
const CRON_FIELD_RE = /^(\*|\d+(-\d+)?(,\d+(-\d+)?)*)(\/\d+)?$/

export function validateCronExpr(expr: string, isEn: boolean): { valid: boolean; description: string | null } {
  const s = expr.trim()
  if (!s) return { valid: false, description: null }
  if (SPECIAL_CRONS.has(s) || /^@every\s+\d+[smhd]$/.test(s)) {
    return { valid: true, description: describeCron(s, isEn) }
  }
  const parts = s.split(/\s+/)
  if (parts.length !== 5) return { valid: false, description: null }
  const ok = parts.every((part) => CRON_FIELD_RE.test(part))
  if (!ok) return { valid: false, description: null }
  return { valid: true, description: describeCron(s, isEn) }
}
