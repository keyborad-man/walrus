import React, { useEffect, useMemo, useRef, useState } from 'react'
import { createPortal } from 'react-dom'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Activity,
  Archive,
  Bell,
  CornerDownLeft,
  Download,
  FolderOpen,
  Globe,
  LayoutDashboard,
  Moon,
  Package,
  Play,
  RotateCw,
  ScrollText,
  Search,
  Server,
  Settings,
  Sliders,
  Square,
  Sun,
  Terminal,
  UserCheck,
  Users,
} from 'lucide-react'
import { api } from '@/lib/api'
import { cn, getClusterHealth } from '@/lib/utils'
import { useAuth } from '@/hooks/useAuth'
import { useTheme } from '@/hooks/useTheme'
import { setLanguage } from '@/i18n'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import type { ClusterSummary } from '@/types/api'

interface CommandItem {
  id: string
  group: string
  label: string
  subtitle?: string
  keywords: string
  icon: React.ReactNode
  badge?: React.ReactNode
  onSelect: () => void
}

interface CommandPaletteProps {
  open: boolean
  onClose: () => void
  clusters: ClusterSummary[]
  activeClusterId?: string
  currentSubPage: string
  updateNeeded: boolean
  onNavigate: (to: string) => void
}

export function CommandPalette({
  open,
  onClose,
  clusters,
  activeClusterId,
  currentSubPage,
  updateNeeded,
  onNavigate,
}: CommandPaletteProps) {
  const { t, i18n } = useTranslation()
  const { user } = useAuth()
  const { theme, toggle: toggleTheme } = useTheme()
  const qc = useQueryClient()

  const [query, setQuery] = useState('')
  const [selectedIdx, setSelectedIdx] = useState(0)
  const [mounted, setMounted] = useState(open)
  const [entered, setEntered] = useState(false)
  const [pendingPower, setPendingPower] = useState<{ kind: 'stop' | 'restart'; cluster: ClusterSummary } | null>(null)

  const inputRef = useRef<HTMLInputElement | null>(null)
  const listRef = useRef<HTMLDivElement | null>(null)
  const onCloseRef = useRef(onClose)
  onCloseRef.current = onClose

  const startMut = useMutation({
    mutationFn: (id: string) => api.startCluster(id),
    onSuccess: (_, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.success(t('dashboard.toastStarted', { name: target?.cluster_name || id }))
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      void qc.invalidateQueries({ queryKey: ['cluster', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const stopMut = useMutation({
    mutationFn: (id: string) => api.stopCluster(id),
    onSuccess: (_, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.success(t('dashboard.toastStopped', { name: target?.cluster_name || id }))
      setPendingPower(null)
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      void qc.invalidateQueries({ queryKey: ['cluster', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const restartMut = useMutation({
    mutationFn: (id: string) => api.restartCluster(id),
    onSuccess: (_, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.success(t('dashboard.toastRestarted', { name: target?.cluster_name || id }))
      setPendingPower(null)
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      void qc.invalidateQueries({ queryKey: ['cluster', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  useEffect(() => {
    if (open) {
      setMounted(true)
      setQuery('')
      setSelectedIdx(0)
      let raf2 = 0
      const raf1 = window.requestAnimationFrame(() => {
        raf2 = window.requestAnimationFrame(() => {
          setEntered(true)
          inputRef.current?.focus()
        })
      })
      return () => {
        window.cancelAnimationFrame(raf1)
        window.cancelAnimationFrame(raf2)
      }
    }
    setEntered(false)
    const timer = window.setTimeout(() => setMounted(false), 180)
    return () => window.clearTimeout(timer)
  }, [open])

  const activeCluster = clusters.find((c) => c.cluster_id === activeClusterId)

  const items = useMemo<CommandItem[]>(() => {
    const list: CommandItem[] = []
    const go = (path: string) => {
      onCloseRef.current()
      onNavigate(path)
    }

    const subPages = [
      { key: 'overview', label: t('nav.overview'), icon: <Activity className="h-4 w-4" />, kw: 'overview status shard 概览 状态' },
      { key: 'console', label: t('nav.console'), icon: <Terminal className="h-4 w-4" />, kw: 'console logs gm lua 控制台 日志 指令' },
      { key: 'players', label: t('nav.players'), icon: <UserCheck className="h-4 w-4" />, kw: 'players chat kick ban adminlist 玩家 聊天 白名单' },
      { key: 'config', label: t('nav.config'), icon: <Sliders className="h-4 w-4" />, kw: 'config world settings ports token 房间 世界 端口 配置' },
      { key: 'mods', label: t('nav.mods'), icon: <Package className="h-4 w-4" />, kw: 'mods workshop lua 模组 创意工坊' },
      { key: 'backups', label: t('nav.backups'), icon: <Archive className="h-4 w-4" />, kw: 'backups restore cron schedule 备份 恢复 定时任务' },
      { key: 'files', label: t('nav.files'), icon: <FolderOpen className="h-4 w-4" />, kw: 'files browser log ini 文件 目录' },
    ]

    // 1. Cluster subpages: show active cluster (or the sole cluster when on Dashboard) by default,
    // and index all clusters' subpages when searching so typing "控制台" / "mods" works from anywhere!
    const targetClusters = activeCluster
      ? [activeCluster]
      : query.trim().length > 0
        ? clusters
        : clusters.slice(0, 1)

    for (const target of targetClusters) {
      const base = `/clusters/${encodeURIComponent(target.cluster_id)}`
      const name = target.cluster_name || target.cluster_id
      const groupName = t('command.groupCurrentCluster', { name })
      for (const sp of subPages) {
        list.push({
          id: `sub-${target.cluster_id}-${sp.key}`,
          group: groupName,
          label: sp.label,
          subtitle: name,
          keywords: `${sp.label} ${sp.kw} ${target.cluster_id} ${target.cluster_name}`,
          icon: sp.icon,
          onSelect: () => go(`${base}/${sp.key}`),
        })
      }
    }

    // 2. Clusters switch
    const clusterGroup = t('command.groupClusters')
    for (const c of clusters) {
      const health = getClusterHealth(c.master_status, c.caves_enabled, c.caves_status)
      const players = c.world_info?.online_players?.length ?? 0
      const targetSub = activeClusterId ? currentSubPage : 'overview'
      const dotCls =
        health === 'running'
          ? 'bg-emerald-500'
          : health === 'crashed'
            ? 'bg-red-500'
            : health === 'degraded' || health === 'starting' || health === 'stopping'
              ? 'bg-amber-500'
              : 'bg-muted-foreground/50'
      list.push({
        id: `cluster-${c.cluster_id}`,
        group: clusterGroup,
        label: c.cluster_name || c.cluster_id,
        subtitle: `${c.cluster_id} · :${c.master_port}`,
        keywords: `cluster switch 集群 切换 ${c.cluster_id} ${c.cluster_name} ${c.description ?? ''}`,
        icon: <Server className="h-4 w-4" />,
        badge: (
          <span className="inline-flex items-center gap-1.5 text-[11px] font-mono tabular-nums text-muted-foreground">
            <span className={cn('h-1.5 w-1.5 rounded-full', dotCls)} />
            {players}/{c.max_players}
          </span>
        ),
        onSelect: () => go(`/clusters/${encodeURIComponent(c.cluster_id)}/${targetSub}`),
      })
    }

    // 3. Power actions for operable clusters
    const powerGroup = t('command.groupPower')
    for (const c of clusters) {
      const canOperate = c.effective_role === 'admin' || c.effective_role === 'operator'
      if (!canOperate) continue
      const name = c.cluster_name || c.cluster_id
      const isAnyActive =
        c.master_status.state === 'running' ||
        c.master_status.state === 'starting' ||
        (c.caves_enabled && (c.caves_status?.state === 'running' || c.caves_status?.state === 'starting'))
      const players = c.world_info?.online_players?.length ?? 0

      if (!isAnyActive) {
        list.push({
          id: `power-start-${c.cluster_id}`,
          group: powerGroup,
          label: t('command.startCluster', { name }),
          subtitle: c.cluster_id,
          keywords: `start launch boot 开服 启动 ${name} ${c.cluster_id}`,
          icon: <Play className="h-4 w-4 text-emerald-600 dark:text-emerald-400" />,
          onSelect: () => {
            onCloseRef.current()
            startMut.mutate(c.cluster_id)
          },
        })
      } else {
        list.push({
          id: `power-restart-${c.cluster_id}`,
          group: powerGroup,
          label: t('command.restartCluster', { name }),
          subtitle: c.cluster_id,
          keywords: `restart reboot 重启 ${name} ${c.cluster_id}`,
          icon: <RotateCw className="h-4 w-4 text-amber-600 dark:text-amber-400" />,
          onSelect: () => {
            onCloseRef.current()
            if (players > 0) setPendingPower({ kind: 'restart', cluster: c })
            else restartMut.mutate(c.cluster_id)
          },
        })
        list.push({
          id: `power-stop-${c.cluster_id}`,
          group: powerGroup,
          label: t('command.stopCluster', { name }),
          subtitle: c.cluster_id,
          keywords: `stop shutdown 停服 停止 ${name} ${c.cluster_id}`,
          icon: <Square className="h-4 w-4 text-amber-600 dark:text-amber-400" />,
          onSelect: () => {
            onCloseRef.current()
            if (players > 0) setPendingPower({ kind: 'stop', cluster: c })
            else stopMut.mutate(c.cluster_id)
          },
        })
      }
    }

    // 4. Navigation & System
    const sysGroup = t('command.groupSystem')
    list.push({
      id: 'nav-dashboard',
      group: sysGroup,
      label: t('nav.dashboard'),
      keywords: 'dashboard home overview 全局 总览 首页',
      icon: <LayoutDashboard className="h-4 w-4" />,
      onSelect: () => go('/'),
    })
    if (user?.role === 'admin') {
      list.push(
        {
          id: 'nav-steam',
          group: sysGroup,
          label: t('nav.steam'),
          keywords: 'steam steamcmd update install server 服务端 更新 安装',
          icon: <Download className="h-4 w-4" />,
          badge: updateNeeded ? (
            <span className="rounded-full border border-amber-500/40 bg-amber-500/15 px-1.5 py-0.5 text-[10px] font-semibold text-amber-700 dark:text-amber-300">
              {t('nav.steamUpdateBadge')}
            </span>
          ) : undefined,
          onSelect: () => go('/steam'),
        },
        {
          id: 'nav-webhooks',
          group: sysGroup,
          label: t('nav.webhooks'),
          keywords: 'webhooks alerts discord feishu dingtalk bark 通知 告警',
          icon: <Bell className="h-4 w-4" />,
          onSelect: () => go('/webhooks'),
        },
        {
          id: 'nav-users',
          group: sysGroup,
          label: t('nav.users'),
          keywords: 'users roles rbac permissions 用户 权限 角色',
          icon: <Users className="h-4 w-4" />,
          onSelect: () => go('/users'),
        },
        {
          id: 'nav-settings',
          group: sysGroup,
          label: t('nav.settings'),
          keywords: 'settings ports token steam key 设置 端口 令牌',
          icon: <Settings className="h-4 w-4" />,
          onSelect: () => go('/settings'),
        },
        {
          id: 'nav-audit',
          group: sysGroup,
          label: t('nav.audit'),
          keywords: 'audit logs history 审计 操作记录',
          icon: <ScrollText className="h-4 w-4" />,
          onSelect: () => go('/audit'),
        },
      )
    }

    // 5. Preferences
    const prefGroup = t('command.groupPrefs')
    list.push(
      {
        id: 'pref-theme',
        group: prefGroup,
        label: theme === 'dark' ? t('command.switchThemeLight') : t('command.switchThemeDark'),
        keywords: 'theme dark light mode appearance 主题 深色 浅色 暗黑 明亮',
        icon: theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />,
        onSelect: () => {
          toggleTheme()
          onCloseRef.current()
        },
      },
      {
        id: 'pref-lang',
        group: prefGroup,
        label: t('command.switchLang'),
        keywords: 'language locale chinese english 语言 中文 英文',
        icon: <Globe className="h-4 w-4" />,
        onSelect: () => {
          const next = i18n.language === 'en-US' ? 'zh-CN' : 'en-US'
          void setLanguage(next)
          onCloseRef.current()
        },
      },
    )

    return list
  }, [activeCluster, activeClusterId, clusters, currentSubPage, i18n.language, onNavigate, query, restartMut, startMut, stopMut, t, theme, toggleTheme, updateNeeded, user?.role])

  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase()
    if (!q) return items
    const parts = q.split(/\s+/).filter(Boolean)
    return items.filter((item) => {
      const hay = `${item.label} ${item.subtitle ?? ''} ${item.group} ${item.keywords}`.toLowerCase()
      return parts.every((p) => hay.includes(p))
    })
  }, [items, query])

  const grouped = useMemo(() => {
    const map = new Map<string, CommandItem[]>()
    for (const item of filtered) {
      const arr = map.get(item.group) ?? []
      arr.push(item)
      map.set(item.group, arr)
    }
    return Array.from(map.entries())
  }, [filtered])

  useEffect(() => {
    setSelectedIdx(0)
  }, [query])

  useEffect(() => {
    if (!open || !mounted) return
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onCloseRef.current()
      } else if (e.key === 'ArrowDown') {
        e.preventDefault()
        setSelectedIdx((i) => (filtered.length > 0 ? (i + 1) % filtered.length : 0))
      } else if (e.key === 'ArrowUp') {
        e.preventDefault()
        setSelectedIdx((i) => (filtered.length > 0 ? (i - 1 + filtered.length) % filtered.length : 0))
      } else if (e.key === 'Enter') {
        const target = filtered[selectedIdx]
        if (target) {
          e.preventDefault()
          target.onSelect()
        }
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [open, mounted, filtered, selectedIdx])

  useEffect(() => {
    const activeEl = listRef.current?.querySelector<HTMLElement>('[data-cmd-active="true"]')
    activeEl?.scrollIntoView({ block: 'nearest' })
  }, [selectedIdx])

  let flatIndex = -1

  return (
    <>
      {mounted &&
        typeof document !== 'undefined' &&
        createPortal(
          <div
            className={cn(
              'fixed inset-0 z-[60] flex items-start justify-center px-3 pt-[10vh] sm:pt-[14vh]',
              !entered && 'pointer-events-none',
            )}
          >
          <div
            className={cn(
              'fixed inset-0 bg-black/68 backdrop-blur-xs transition-opacity duration-180',
              entered ? 'opacity-100' : 'opacity-0',
            )}
            onClick={onClose}
            aria-hidden="true"
          />

          <div
            role="dialog"
            aria-modal="true"
            aria-label={t('command.title')}
            className={cn(
              'relative z-10 w-full max-w-xl overflow-hidden rounded-xl border border-[var(--border-strong)] bg-popover text-popover-foreground shadow-[0_24px_60px_-14px_rgba(0,0,0,0.55)] [transition:opacity_180ms_var(--ease-out),scale_180ms_var(--ease-out),translate_180ms_var(--ease-out)] motion-reduce:[transition:opacity_160ms_ease]',
              entered ? 'opacity-100 scale-100 translate-y-0' : 'opacity-0 motion-safe:scale-[0.98] motion-safe:-translate-y-1.5',
            )}
          >
            {/* Search Input Header */}
            <div className="flex items-center gap-3 border-b border-border px-4 py-3">
              <Search className="h-4 w-4 text-muted-foreground shrink-0" aria-hidden="true" />
              <input
                ref={inputRef}
                type="text"
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder={t('command.placeholder')}
                aria-label={t('command.title')}
                className="flex-1 bg-transparent text-sm text-foreground placeholder:text-muted-foreground/80 focus:outline-none"
              />
              <kbd className="hidden sm:inline-flex items-center rounded border border-border bg-secondary/70 px-1.5 py-0.5 font-mono text-[10.5px] text-muted-foreground">
                ESC
              </kbd>
            </div>

            {/* Results */}
            <div ref={listRef} className="max-h-[55vh] overflow-y-auto p-2 space-y-3">
              {grouped.length === 0 ? (
                <div className="py-10 text-center text-xs text-muted-foreground">{t('command.empty')}</div>
              ) : (
                grouped.map(([groupName, groupItems]) => (
                  <div key={groupName} className="space-y-0.5">
                    <div className="px-2.5 pt-1 pb-0.5 text-[11.5px] font-medium text-muted-foreground">{groupName}</div>
                    {groupItems.map((item) => {
                      flatIndex += 1
                      const idx = flatIndex
                      const isActive = idx === selectedIdx
                      return (
                        <button
                          key={item.id}
                          type="button"
                          data-cmd-active={isActive ? 'true' : undefined}
                          onMouseEnter={() => setSelectedIdx(idx)}
                          onClick={item.onSelect}
                          className={cn(
                            'flex w-full items-center justify-between gap-3 rounded-lg px-3 py-2 text-left text-[13px] transition-colors cursor-pointer',
                            isActive
                              ? 'bg-[var(--surface-hover)] text-foreground'
                              : 'text-muted-foreground hover:text-foreground',
                          )}
                        >
                          <span className="flex items-center gap-2.5 min-w-0">
                            <span className={cn('shrink-0', isActive ? 'text-foreground' : 'text-muted-foreground')}>
                              {item.icon}
                            </span>
                            <span className="truncate font-medium text-foreground">{item.label}</span>
                            {item.subtitle && (
                              <span className="hidden sm:inline truncate text-xs font-mono text-muted-foreground">
                                {item.subtitle}
                              </span>
                            )}
                          </span>
                          <span className="flex items-center gap-2 shrink-0">
                            {item.badge}
                            {isActive && <CornerDownLeft className="h-3.5 w-3.5 text-muted-foreground" aria-hidden="true" />}
                          </span>
                        </button>
                      )
                    })}
                  </div>
                ))
              )}
            </div>

            {/* Footer Key Hints */}
            <div className="flex items-center justify-between gap-3 border-t border-border bg-[var(--surface-elevated)]/65 px-4 py-2 text-[11px] text-muted-foreground">
              <div className="flex items-center gap-3">
                <span className="inline-flex items-center gap-1">
                  <kbd className="rounded border border-border bg-card px-1 py-0.5 font-mono text-[10px]">↑↓</kbd>
                  {t('command.hintNavigate')}
                </span>
                <span className="inline-flex items-center gap-1">
                  <kbd className="rounded border border-border bg-card px-1 py-0.5 font-mono text-[10px]">↵</kbd>
                  {t('command.hintConfirm')}
                </span>
              </div>
              <span className="inline-flex items-center gap-1">
                <kbd className="rounded border border-border bg-card px-1 py-0.5 font-mono text-[10px]">ESC</kbd>
                {t('command.hintClose')}
              </span>
            </div>
          </div>
        </div>,
        document.body,
      )}

      <ConfirmDialog
        open={!!pendingPower}
        onClose={() => setPendingPower(null)}
        onConfirm={() => {
          if (!pendingPower) return
          if (pendingPower.kind === 'stop') stopMut.mutate(pendingPower.cluster.cluster_id)
          else restartMut.mutate(pendingPower.cluster.cluster_id)
        }}
        title={
          pendingPower?.kind === 'restart'
            ? t('dashboard.restartOnlineConfirmTitle')
            : t('dashboard.stopOnlineConfirmTitle')
        }
        description={
          pendingPower?.kind === 'restart'
            ? t('dashboard.restartOnlineConfirmDesc', {
                name: pendingPower.cluster.cluster_name || pendingPower.cluster.cluster_id,
                count: pendingPower.cluster.world_info?.online_players?.length ?? 0,
              })
            : t('dashboard.stopOnlineConfirmDesc', {
                name: pendingPower?.cluster.cluster_name || pendingPower?.cluster.cluster_id || '',
                count: pendingPower?.cluster.world_info?.online_players?.length ?? 0,
              })
        }
        confirmLabel={pendingPower?.kind === 'restart' ? t('dashboard.restartAll') : t('dashboard.stopAll')}
        tone="warning"
        loading={stopMut.isPending || restartMut.isPending}
      />
    </>
  )
}
