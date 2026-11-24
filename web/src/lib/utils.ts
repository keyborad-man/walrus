import { type ClassValue, clsx } from 'clsx'
import { twMerge } from 'tailwind-merge'
import type { ShardState, ShardStatus } from '@/types/api'

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs))
}

export function formatBytes(bytes: number): string {
  if (!bytes || bytes <= 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / Math.pow(1024, i)).toFixed(i === 0 ? 0 : 1)} ${units[i]}`
}

export function formatUptime(seconds: number): string {
  if (!seconds || seconds <= 0) return '0s'
  const d = Math.floor(seconds / 86400)
  const h = Math.floor((seconds % 86400) / 3600)
  const m = Math.floor((seconds % 3600) / 60)
  const s = seconds % 60
  if (d > 0) return `${d}d ${h}h ${m}m`
  if (h > 0) return `${h}h ${m}m`
  if (m > 0) return `${m}m ${s}s`
  return `${s}s`
}

export function formatDateTime(iso: string): string {
  if (!iso) return '-'
  const dt = new Date(iso)
  if (Number.isNaN(dt.getTime())) return iso
  return dt.toLocaleString()
}

export type CompositeClusterHealth = 'running' | 'degraded' | 'starting' | 'stopping' | 'crashed' | 'stopped'

export function getClusterHealth(
  masterStatus?: ShardStatus,
  cavesEnabled?: boolean,
  cavesStatus?: ShardStatus,
): CompositeClusterHealth {
  const m: ShardState = masterStatus?.state ?? 'stopped'
  if (!cavesEnabled) return m
  const c: ShardState = cavesStatus?.state ?? 'stopped'
  if (m === 'crashed' || c === 'crashed') return 'crashed'
  if (m === 'starting' || c === 'starting') return 'starting'
  if (m === 'stopping' || c === 'stopping') return 'stopping'
  if (m === 'running' && c === 'running') return 'running'
  if (m === 'running' || c === 'running') return 'degraded'
  return 'stopped'
}

export function describeCron(expr: string, isEn: boolean): string | null {
  const s = expr.trim()
  if (!s) return null
  if (s === '@daily' || s === '@midnight') return isEn ? 'Runs every day at 00:00' : '每天 00:00 执行'
  if (s === '@hourly') return isEn ? 'Runs at the start of every hour' : '每小时整点执行'
  if (s === '@weekly') return isEn ? 'Runs every Sunday at 00:00' : '每周日 00:00 执行'
  if (s === '@monthly') return isEn ? 'Runs on the 1st of every month at 00:00' : '每月 1 日 00:00 执行'

  const parts = s.split(/\s+/)
  if (parts.length !== 5) return null
  const [min, hour, dom, mon, dow] = parts
  const pad = (v: string) => v.padStart(2, '0')
  const isNum = (v: string) => /^\d+$/.test(v)

  if (min.startsWith('*/') && hour === '*' && dom === '*' && mon === '*' && dow === '*') {
    const n = min.slice(2)
    return isEn ? `Runs every ${n} minutes` : `每 ${n} 分钟执行一次`
  }
  if (isNum(min) && hour === '*' && dom === '*' && mon === '*' && dow === '*') {
    return isEn ? `Runs every hour at :${pad(min)}` : `每小时第 ${pad(min)} 分执行`
  }
  if (isNum(min) && hour.startsWith('*/') && dom === '*' && mon === '*' && dow === '*') {
    const n = hour.slice(2)
    return isEn ? `Runs every ${n} hours at :${pad(min)}` : `每 ${n} 小时整点（:${pad(min)}）执行`
  }
  if (isNum(min) && isNum(hour) && dom === '*' && mon === '*' && dow === '*') {
    return isEn ? `Runs daily at ${pad(hour)}:${pad(min)}` : `每天 ${pad(hour)}:${pad(min)} 执行`
  }
  if (isNum(min) && isNum(hour) && dom === '*' && mon === '*' && isNum(dow)) {
    const daysEn = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday', 'Sunday']
    const daysZh = ['周日', '周一', '周二', '周三', '周四', '周五', '周六', '周日']
    const idx = Number(dow)
    if (idx >= 0 && idx <= 7) {
      return isEn
        ? `Runs every ${daysEn[idx]} at ${pad(hour)}:${pad(min)}`
        : `每${daysZh[idx]} ${pad(hour)}:${pad(min)} 执行`
    }
  }
  return null
}
