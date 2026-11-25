import { useEffect, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertTriangle,
  ArrowRight,
  CheckCircle2,
  Download,
  Loader2,
  Play,
  Plus,
  RotateCw,
  Server,
  Square,
  Upload,
} from 'lucide-react'
import { api } from '@/lib/api'
import { formatAuditAction, formatConnectCommand } from '@/lib/dst'
import { cn, getClusterHealth } from '@/lib/utils'
import { useAuth } from '@/hooks/useAuth'
import { Button, buttonVariants } from '@/components/ui/button'
import { CopyButton } from '@/components/ui/copy-button'
import { Sparkline } from '@/components/ui/sparkline'
import { DashboardSkeleton } from '@/components/ui/skeleton'
import { PageHeader } from '@/components/PageHeader'
import { DSTClockDial, ShardStateBadge } from '@/components/StatusBadge'
import { CreateClusterDialog } from '@/components/CreateClusterDialog'
import { ImportClusterDialog } from '@/components/ImportClusterDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import type { ClusterSummary, ShardStatus } from '@/types/api'

const MAX_SPARK_SAMPLES = 18

type ClusterFilter = 'all' | 'running' | 'stopped'

function formatShortTime(iso: string): string {
  if (!iso) return '—'
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return iso
  return d.toLocaleTimeString([], { hour12: false, hour: '2-digit', minute: '2-digit', second: '2-digit' })
}

export function DashboardPage() {
  const { t, i18n } = useTranslation()
  const isEn = i18n.language === 'en-US'
  const { user } = useAuth()
  const qc = useQueryClient()
  const navigate = useNavigate()

  const [createOpen, setCreateOpen] = useState(false)
  const [importOpen, setImportOpen] = useState(false)
  const [clusterFilter, setClusterFilter] = useState<ClusterFilter>('all')
  const [busyClusterId, setBusyClusterId] = useState<string | null>(null)
  const [batchBusy, setBatchBusy] = useState(false)
  const [stopAllConfirmOpen, setStopAllConfirmOpen] = useState(false)
  const [pendingAction, setPendingAction] = useState<{ kind: 'stop' | 'restart'; cluster: ClusterSummary } | null>(null)
  const [history, setHistory] = useState<{ cpu: number[]; mem: number[]; disk: number[]; players: number[] }>({
    cpu: [],
    mem: [],
    disk: [],
    players: [],
  })

  const { data: metricsData, dataUpdatedAt: metricsUpdatedAt } = useQuery({
    queryKey: ['system-metrics'],
    queryFn: api.metrics,
    refetchInterval: 4000,
  })
  const { data: publicIpData } = useQuery({
    queryKey: ['system-public-ip'],
    queryFn: api.publicIP,
    staleTime: 10 * 60 * 1000,
    retry: false,
  })
  const {
    data: clustersData,
    isLoading,
    isError: clustersErrorFlag,
    error: clustersError,
    refetch: refetchClusters,
  } = useQuery({
    queryKey: ['clusters'],
    queryFn: api.listClusters,
    refetchInterval: 4000,
  })
  const { data: steamData } = useQuery({
    queryKey: ['steam-status'],
    queryFn: api.steamStatus,
    refetchInterval: 30000,
  })
  const isAdmin = user?.role === 'admin'
  const { data: auditData } = useQuery({
    queryKey: ['dashboard-audit-recent'],
    queryFn: () => api.auditLogs(5, ''),
    enabled: isAdmin,
    refetchInterval: 15000,
  })

  const metrics = metricsData?.metrics
  const connectHost = publicIpData?.ip || window.location.hostname
  const clusters = clustersData?.clusters ?? []
  const steamStatus = steamData?.status
  const recentLogs = auditData?.logs ?? []

  const onlineClusters = clusters.filter(
    (c) => c.master_status.state === 'running' || c.caves_status?.state === 'running',
  )
  const totalOnlinePlayers = clusters.reduce((sum, c) => sum + (c.world_info?.online_players?.length ?? 0), 0)
  const operableClusters = clusters.filter((c) => c.effective_role === 'admin' || c.effective_role === 'operator')

  useEffect(() => {
    if (!metrics || !metricsUpdatedAt) return
    setHistory((prev) => ({
      cpu: [...prev.cpu.slice(-(MAX_SPARK_SAMPLES - 1)), metrics.cpu_percent],
      mem: [...prev.mem.slice(-(MAX_SPARK_SAMPLES - 1)), metrics.memory_percent],
      disk: [...prev.disk.slice(-(MAX_SPARK_SAMPLES - 1)), metrics.disk_percent],
      players: [...prev.players.slice(-(MAX_SPARK_SAMPLES - 1)), totalOnlinePlayers],
    }))
  }, [metrics, metricsUpdatedAt, totalOnlinePlayers])

  const filteredClusters = clusters.filter((c) => {
    const running = c.master_status.state === 'running' || c.caves_status?.state === 'running'
    if (clusterFilter === 'running') return running
    if (clusterFilter === 'stopped') return !running
    return true
  })

  const startMut = useMutation({
    mutationFn: (id: string) => {
      setBusyClusterId(id)
      return api.startCluster(id)
    },
    onSuccess: (_, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.success(t('dashboard.toastStarted', { name: target?.cluster_name || id }))
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      void qc.invalidateQueries({ queryKey: ['dashboard-audit-recent'] })
    },
    onError: (err: Error) => toast.error(err.message),
    onSettled: () => setBusyClusterId(null),
  })

  const stopMut = useMutation({
    mutationFn: (id: string) => {
      setBusyClusterId(id)
      return api.stopCluster(id)
    },
    onSuccess: (_, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.success(t('dashboard.toastStopped', { name: target?.cluster_name || id }))
      setPendingAction(null)
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      void qc.invalidateQueries({ queryKey: ['dashboard-audit-recent'] })
    },
    onError: (err: Error) => toast.error(err.message),
    onSettled: () => setBusyClusterId(null),
  })

  const restartMut = useMutation({
    mutationFn: (id: string) => {
      setBusyClusterId(id)
      return api.restartCluster(id)
    },
    onSuccess: (_, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.success(t('dashboard.toastRestarted', { name: target?.cluster_name || id }))
      setPendingAction(null)
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      void qc.invalidateQueries({ queryKey: ['dashboard-audit-recent'] })
    },
    onError: (err: Error) => toast.error(err.message),
    onSettled: () => setBusyClusterId(null),
  })

  const requestClusterAction = (kind: 'stop' | 'restart', c: ClusterSummary) => {
    const players = c.world_info?.online_players?.length ?? 0
    if (players > 0) {
      setPendingAction({ kind, cluster: c })
    } else if (kind === 'stop') {
      stopMut.mutate(c.cluster_id)
    } else {
      restartMut.mutate(c.cluster_id)
    }
  }

  const runBatchStart = async () => {
    const stopped = operableClusters.filter((c) => c.master_status.state !== 'running')
    if (stopped.length === 0) return
    setBatchBusy(true)
    try {
      await Promise.allSettled(stopped.map((c) => api.startCluster(c.cluster_id)))
      toast.success(t('common.actionSuccess'))
      void qc.invalidateQueries({ queryKey: ['clusters'] })
    } finally {
      setBatchBusy(false)
    }
  }

  const runBatchStop = async () => {
    const running = operableClusters.filter(
      (c) => c.master_status.state === 'running' || c.caves_status?.state === 'running',
    )
    if (running.length === 0) return
    setBatchBusy(true)
    try {
      await Promise.allSettled(running.map((c) => api.stopCluster(c.cluster_id)))
      toast.success(t('common.actionSuccess'))
      setStopAllConfirmOpen(false)
      void qc.invalidateQueries({ queryKey: ['clusters'] })
    } finally {
      setBatchBusy(false)
    }
  }

  const showHeaderCreate = isAdmin && clusters.length > 0
  const serverInstalled = steamStatus ? steamStatus.installed : true

  const dynamicSubtitle = [
    metrics ? `${metrics.os}/${metrics.arch}` : null,
    t('dashboard.subtitleSummary', { total: clusters.length, running: onlineClusters.length }),
  ]
    .filter(Boolean)
    .join(' · ')

  const diskPercent = metrics?.disk_percent ?? 0
  const diskSub =
    diskPercent >= 85
      ? t('dashboard.diskCriticalSub')
      : diskPercent >= 70
        ? t('dashboard.diskElevatedSub')
        : t('dashboard.diskNormalSub')

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('dashboard.title')}
        subtitle={dynamicSubtitle || t('dashboard.subtitle')}
        actions={
          <>
            {operableClusters.length > 1 && (
              <>
                <Button size="sm" variant="outline" disabled={batchBusy} onClick={() => void runBatchStart()}>
                  <Play className="h-3.5 w-3.5" />
                  {t('dashboard.startAllClusters')}
                </Button>
                {onlineClusters.length > 0 && (
                  <Button
                    size="sm"
                    variant="outline"
                    disabled={batchBusy}
                    onClick={() => setStopAllConfirmOpen(true)}
                  >
                    <Square className="h-3.5 w-3.5" />
                    {t('dashboard.stopAllClusters')}
                  </Button>
                )}
              </>
            )}
            {showHeaderCreate && (
              <>
                <Button variant="outline" onClick={() => setImportOpen(true)}>
                  <Upload className="h-3.5 w-3.5" />
                  {t('importCluster.btn')}
                </Button>
                <Button onClick={() => setCreateOpen(true)}>
                  <Plus className="h-3.5 w-3.5" />
                  {t('dashboard.createCluster')}
                </Button>
              </>
            )}
          </>
        }
      />

      {/* Compact Single-Line Server Missing Notice */}
      {isAdmin && steamStatus && !steamStatus.installed && clusters.length > 0 && (
        <div
          role="status"
          className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-card px-3.5 py-2.5 text-[13px] text-muted-foreground"
        >
          <div className="flex items-center gap-2.5 min-w-0">
            <span className="h-[7px] w-[7px] rounded-full bg-amber-500 shrink-0" aria-hidden="true" />
            <span className="text-foreground/90">{t('dashboard.steamNotInstalledBanner')}</span>
          </div>
          <Link
            to="/steam"
            className="inline-flex items-center gap-1 text-[13px] font-medium text-foreground hover:opacity-80 transition-opacity shrink-0"
          >
            <span>{t('dashboard.goSteamSetup')}</span>
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
        </div>
      )}

      {/* Unified 4-Cell Precision Host Telemetry Strip */}
      <section aria-label={t('dashboard.liveTelemetry')} className="rounded-[10px] surface-card overflow-hidden">
        <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 divide-y sm:divide-y-0 lg:divide-x divide-border">
          <TelemetryCell
            label={t('dashboard.hostCpu')}
            value={metrics ? String(metrics.cpu_percent) : '—'}
            unit={metrics ? '%' : undefined}
            desc={t('dashboard.hostCpuSub')}
            percent={metrics?.cpu_percent ?? 0}
            samples={history.cpu}
          />
          <TelemetryCell
            label={t('dashboard.hostMem')}
            meta={
              metrics
                ? `${(metrics.memory_used_mb / 1024).toFixed(1)} / ${(metrics.memory_total_mb / 1024).toFixed(1)} GB`
                : undefined
            }
            value={metrics ? String(metrics.memory_percent) : '—'}
            unit={metrics ? '%' : undefined}
            desc={t('dashboard.hostMemSub')}
            percent={metrics?.memory_percent ?? 0}
            samples={history.mem}
            className="sm:border-l lg:border-l-0 border-border"
          />
          <TelemetryCell
            label={t('dashboard.hostDisk')}
            meta={metrics ? `${metrics.disk_used_gb} / ${metrics.disk_total_gb} GB` : undefined}
            value={metrics ? String(metrics.disk_percent) : '—'}
            unit={metrics ? '%' : undefined}
            desc={diskSub}
            percent={diskPercent}
            samples={history.disk}
            className="sm:border-t lg:border-t-0 border-border"
          />
          <TelemetryCell
            label={t('dashboard.onlinePlayersCard')}
            value={String(totalOnlinePlayers)}
            unit={t('dashboard.onlinePlayersUnit') || undefined}
            desc={t('dashboard.runningClustersSub', { running: onlineClusters.length, total: clusters.length })}
            percent={0}
            samples={history.players.length > 0 ? history.players : [totalOnlinePlayers]}
            className="sm:border-t sm:border-l lg:border-t-0 lg:border-l-0 border-border"
          />
        </div>
      </section>

      {/* Clusters Section */}
      <section className="space-y-3">
        {clusters.length > 0 && (
          <div className="flex flex-wrap items-center justify-between gap-2.5">
            <div className="flex items-center gap-2">
              <h2 className="text-[15px] font-semibold tracking-[-0.01em] text-foreground">
                {t('dashboard.clustersHeading')}
              </h2>
              <span className="inline-flex items-center justify-center rounded-full border border-[var(--border-strong)] px-2 py-0 text-[11.5px] leading-[18px] font-mono tabular-nums text-muted-foreground">
                {clusters.length}
              </span>
            </div>
            <div
              role="group"
              aria-label={t('dashboard.clustersHeading')}
              className="inline-flex items-center rounded-lg border border-border bg-card p-0.5 text-xs"
            >
              {(
                [
                  { key: 'all', label: t('dashboard.filterAll') },
                  { key: 'running', label: t('dashboard.filterRunning') },
                  { key: 'stopped', label: t('dashboard.filterStopped') },
                ] as const
              ).map((item) => (
                <button
                  key={item.key}
                  type="button"
                  onClick={() => setClusterFilter(item.key)}
                  className={cn(
                    'rounded-[6px] px-2.5 py-1 transition-colors cursor-pointer',
                    clusterFilter === item.key
                      ? 'bg-[var(--surface-hover)] text-foreground font-medium'
                      : 'text-muted-foreground hover:text-foreground',
                  )}
                >
                  {item.label}
                </button>
              ))}
            </div>
          </div>
        )}

        {isLoading ? (
          <DashboardSkeleton />
        ) : clustersErrorFlag ? (
          <QueryErrorCard error={clustersError} onRetry={() => void refetchClusters()} />
        ) : clusters.length === 0 ? (
          <div className="rounded-[10px] border border-dashed border-border bg-card py-11 px-6 flex flex-col items-center justify-center text-center gap-5">
            <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-[var(--surface-elevated)] text-foreground">
              <Server className="h-5 w-5" />
            </div>
            <div className="space-y-1 max-w-lg">
              <h2 className="text-base font-semibold tracking-[-0.015em]">{t('dashboard.emptyTitle')}</h2>
              <p className="text-xs text-muted-foreground leading-relaxed text-pretty">{t('dashboard.emptyDesc')}</p>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-3 gap-2.5 w-full max-w-2xl text-left">
              <div
                className={cn(
                  'rounded-lg border px-3.5 py-3 text-xs space-y-1',
                  serverInstalled ? 'border-emerald-600/30 bg-emerald-500/5' : 'border-[var(--border-strong)] bg-[var(--surface-elevated)]',
                )}
              >
                <div className="flex items-center justify-between gap-1 text-[11px] font-medium">
                  <span className="font-mono text-muted-foreground">01</span>
                  {serverInstalled ? (
                    <span className="inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-400">
                      <CheckCircle2 className="h-3 w-3" />
                      {t('dashboard.stepInstalled')}
                    </span>
                  ) : (
                    <span className="text-amber-700 dark:text-amber-400">{t('dashboard.stepPending')}</span>
                  )}
                </div>
                <div className="font-medium text-foreground">{t('dashboard.quickStartStep1')}</div>
              </div>
              <div
                className={cn(
                  'rounded-lg border px-3.5 py-3 text-xs space-y-1',
                  serverInstalled ? 'border-[var(--border-strong)] bg-[var(--surface-elevated)]' : 'border-border bg-background/50 text-muted-foreground',
                )}
              >
                <div className="flex items-center justify-between gap-1 text-[11px] font-medium">
                  <span className="font-mono text-muted-foreground">02</span>
                  {serverInstalled && <span className="text-foreground">{t('dashboard.stepPending')}</span>}
                </div>
                <div className="font-medium text-foreground">{t('dashboard.quickStartStep2')}</div>
              </div>
              <div className="rounded-lg border border-border bg-background/50 px-3.5 py-3 text-xs space-y-1 text-muted-foreground">
                <div className="font-mono text-[11px]">03</div>
                <div className="font-medium text-foreground">{t('dashboard.quickStartStep3')}</div>
              </div>
            </div>
            {isAdmin && (
              <div className="flex flex-wrap items-center justify-center gap-2 pt-1">
                {!serverInstalled ? (
                  <>
                    <Link to="/steam" className={buttonVariants({ variant: 'default' })}>
                      <Download className="h-3.5 w-3.5" />
                      {t('dashboard.goSteamSetup')}
                    </Link>
                    <Button variant="outline" onClick={() => setCreateOpen(true)}>
                      <Plus className="h-3.5 w-3.5" />
                      {t('dashboard.createCluster')}
                    </Button>
                    <Button variant="outline" onClick={() => setImportOpen(true)}>
                      <Upload className="h-3.5 w-3.5" />
                      {t('importCluster.btn')}
                    </Button>
                  </>
                ) : (
                  <>
                    <Button onClick={() => setCreateOpen(true)}>
                      <Plus className="h-3.5 w-3.5" />
                      {t('dashboard.createCluster')}
                    </Button>
                    <Button variant="outline" onClick={() => setImportOpen(true)}>
                      <Upload className="h-3.5 w-3.5" />
                      {t('importCluster.btn')}
                    </Button>
                    <Link to="/steam" className={buttonVariants({ variant: 'ghost' })}>
                      <Download className="h-3.5 w-3.5" />
                      {t('dashboard.goSteamSetup')}
                    </Link>
                  </>
                )}
              </div>
            )}
          </div>
        ) : (
          <div className="rounded-[10px] surface-card overflow-hidden">
            {/* Desktop Table Header */}
            <div className="hidden lg:grid lg:grid-cols-[minmax(170px,1.25fr)_94px_minmax(150px,1.1fr)_100px_minmax(240px,1.85fr)_184px] items-center gap-5 px-4 py-2.5 border-b border-border text-xs text-muted-foreground">
              <span>{t('dashboard.colCluster')}</span>
              <span>{t('dashboard.colStatus')}</span>
              <span>{t('dashboard.colWorld')}</span>
              <span>{t('dashboard.colPlayers')}</span>
              <span>{t('dashboard.colShards')}</span>
              <span />
            </div>

            {filteredClusters.length === 0 ? (
              <div className="py-10 px-4 text-center text-xs text-muted-foreground">{t('dashboard.filterEmpty')}</div>
            ) : (
              <div className="divide-y divide-border">
                {filteredClusters.map((c) => {
                  const compositeHealth = getClusterHealth(c.master_status, c.caves_enabled, c.caves_status)
                  const isAnyActive =
                    c.master_status.state === 'running' ||
                    c.master_status.state === 'starting' ||
                    (c.caves_enabled &&
                      (c.caves_status?.state === 'running' || c.caves_status?.state === 'starting'))
                  const canOperate = c.effective_role === 'admin' || c.effective_role === 'operator'
                  const players = c.world_info?.online_players?.length ?? 0
                  const playerRatio = c.max_players > 0 ? Math.min(1, players / c.max_players) : 0
                  const isRowBusy = busyClusterId === c.cluster_id || batchBusy
                  const isRowRestarting = busyClusterId === c.cluster_id && restartMut.isPending
                  const hasWorldClock = Boolean(
                    c.world_info?.season && c.world_info.season !== 'unknown' && c.world_info.day > 0,
                  )
                  const phaseLabel =
                    c.world_info?.phase && c.world_info.phase !== 'unknown'
                      ? t(`dashboard.phases.${c.world_info.phase}`, '')
                      : ''
                  const overviewHref = `/clusters/${encodeURIComponent(c.cluster_id)}/overview`

                  return (
                    <article
                      key={c.cluster_id}
                      className="grid grid-cols-1 lg:grid-cols-[minmax(170px,1.25fr)_94px_minmax(150px,1.1fr)_100px_minmax(240px,1.85fr)_184px] items-start lg:items-center gap-3.5 lg:gap-5 px-4 py-3.5 hover:bg-[var(--surface-hover)]/35 transition-colors"
                    >
                      {/* Col 1: Name + ID + Missing Token */}
                      <div className="min-w-0">
                        <div className="flex items-center justify-between gap-2 lg:block">
                          <Link
                            to={overviewHref}
                            className="text-[14px] font-semibold tracking-[-0.01em] text-foreground hover:underline truncate block"
                          >
                            {c.cluster_name || c.cluster_id}
                          </Link>
                          <div className="lg:hidden shrink-0">
                            <ShardStateBadge state={compositeHealth} />
                          </div>
                        </div>
                        <div className="mt-0.5 flex flex-wrap items-center gap-x-2 gap-y-0.5 text-xs">
                          <code className="text-[11.5px] text-muted-foreground">{c.cluster_id}</code>
                          <span className="text-muted-foreground/60">·</span>
                          <span className="text-muted-foreground">
                            {t(`dashboard.modes.${c.game_mode}`, c.game_mode)}
                          </span>
                          {!c.has_token && (
                            <Link
                              to={`/clusters/${encodeURIComponent(c.cluster_id)}/config`}
                              title={t('dashboard.tokenMissingAction')}
                              className="inline-flex items-center gap-1 text-[11.5px] font-medium text-amber-700 dark:text-amber-400 hover:underline"
                            >
                              <AlertTriangle className="h-3 w-3 shrink-0" />
                              <span>{t('dashboard.tokenMissing')}</span>
                            </Link>
                          )}
                        </div>
                      </div>

                      {/* Col 2: Status (desktop) */}
                      <div className="hidden lg:flex items-center">
                        <ShardStateBadge state={compositeHealth} />
                      </div>

                      {/* Col 3: World Clock */}
                      <div className="flex items-center gap-2.5 min-w-0">
                        <DSTClockDial phase={c.world_info?.phase} className="h-6 w-6 shrink-0" />
                        <div className="min-w-0 tabular-nums">
                          {hasWorldClock ? (
                            <>
                              <div className="text-[13px] text-foreground leading-snug truncate">
                                {t(`dashboard.seasons.${c.world_info.season}`, c.world_info.season)} ·{' '}
                                {t('dashboard.day', { day: c.world_info.day })}
                              </div>
                              <div className="text-[11.5px] text-muted-foreground truncate">
                                {[
                                  phaseLabel,
                                  c.world_info.remaining_days_in_season > 0
                                    ? t('overview.seasonRemaining', { days: c.world_info.remaining_days_in_season })
                                    : null,
                                ]
                                  .filter(Boolean)
                                  .join(' · ') || '—'}
                              </div>
                            </>
                          ) : (
                            <>
                              <div className="text-[13px] text-muted-foreground leading-snug">
                                {t('dashboard.seasons.unknown')}
                              </div>
                              <div className="text-[11.5px] text-muted-foreground/75">—</div>
                            </>
                          )}
                        </div>
                      </div>

                      {/* Col 4: Players */}
                      <div className="w-full max-w-[140px] lg:max-w-none tabular-nums">
                        <div className="text-[13px] font-medium text-foreground">
                          {players} <span className="text-muted-foreground font-normal">/ {c.max_players}</span>
                        </div>
                        <div className="mt-1.5 h-[3px] w-full rounded-full bg-secondary overflow-hidden">
                          <div
                            className="h-full w-full origin-left bg-foreground/65 [transition:transform_220ms_var(--ease-out)]"
                            style={{ transform: `scaleX(${playerRatio})` }}
                          />
                        </div>
                      </div>

                      {/* Col 5: Shards */}
                      <div className="grid grid-cols-2 gap-3 min-w-0">
                        <ShardCell
                          label={t('common.masterShort')}
                          port={c.master_port}
                          connectHost={connectHost}
                          password={c.cluster_password}
                          status={c.master_status}
                        />
                        {c.caves_enabled && c.caves_status ? (
                          <ShardCell
                            label={t('common.cavesShort')}
                            port={c.caves_port}
                            connectHost={connectHost}
                            password={c.cluster_password}
                            status={c.caves_status}
                          />
                        ) : (
                          <div className="text-xs text-muted-foreground self-center">{t('common.singleWorld')}</div>
                        )}
                      </div>

                      {/* Col 6: Actions */}
                      <div className="flex items-center justify-end gap-1 pt-1 lg:pt-0 border-t border-border/60 lg:border-t-0">
                        {canOperate && (
                          <>
                            {!isAnyActive ? (
                              <Button
                                size="sm"
                                variant="ghost"
                                disabled={isRowBusy}
                                onClick={() => startMut.mutate(c.cluster_id)}
                              >
                                <Play className="h-3.5 w-3.5" />
                                {t('dashboard.startAll')}
                              </Button>
                            ) : (
                              <>
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  disabled={isRowBusy}
                                  onClick={() => requestClusterAction('stop', c)}
                                >
                                  <Square className="h-3 w-3" />
                                  {t('dashboard.stopAll')}
                                </Button>
                                <Button
                                  size="sm"
                                  variant="ghost"
                                  disabled={isRowBusy}
                                  onClick={() => requestClusterAction('restart', c)}
                                >
                                  <RotateCw className={cn('h-3.5 w-3.5', isRowRestarting && 'animate-spin')} />
                                  {t('dashboard.restartAll')}
                                </Button>
                              </>
                            )}
                          </>
                        )}
                        <Link
                          to={overviewHref}
                          className={cn(buttonVariants({ size: 'sm', variant: 'outline' }), 'group ml-0.5')}
                        >
                          <span>{t('dashboard.openCluster')}</span>
                          <ArrowRight className="h-3.5 w-3.5 transition-transform duration-150 motion-safe:group-hover:translate-x-0.5" />
                        </Link>
                      </div>
                    </article>
                  )
                })}
              </div>
            )}
          </div>
        )}
      </section>

      {/* Bottom Split: Recent Activity + DST Server Summary (Admin only) */}
      {isAdmin && (
        <section className="grid grid-cols-1 lg:grid-cols-[1.6fr_1fr] gap-4">
          <div className="rounded-[10px] surface-card overflow-hidden flex flex-col">
            <div className="flex items-center justify-between gap-2 px-4 py-3 border-b border-border">
              <h3 className="text-[13.5px] font-semibold text-foreground">{t('dashboard.recentActivity')}</h3>
              <Link to="/audit" className="text-xs text-muted-foreground hover:text-foreground transition-colors">
                {t('dashboard.viewAuditLog')}
              </Link>
            </div>
            {recentLogs.length === 0 ? (
              <div className="px-4 py-8 text-center text-xs text-muted-foreground">
                {t('dashboard.noRecentActivity')}
              </div>
            ) : (
              <div className="divide-y divide-border">
                {recentLogs.map((log) => (
                  <div
                    key={log.id}
                    className="grid grid-cols-[68px_1fr_auto] items-center gap-3 px-4 py-2.5 text-[13px]"
                  >
                    <time className="font-mono text-xs text-muted-foreground tabular-nums">
                      {formatShortTime(log.created_at)}
                    </time>
                    <div className="min-w-0 truncate text-foreground">
                      <span>{formatAuditAction(log.action, isEn)}</span>
                      {log.target && (
                        <code className="ml-1.5 text-[12px] text-muted-foreground">{log.target}</code>
                      )}
                    </div>
                    <span className="text-xs text-muted-foreground truncate max-w-[96px]">{log.actor}</span>
                  </div>
                ))}
              </div>
            )}
          </div>

          <div className="rounded-[10px] surface-card overflow-hidden flex flex-col justify-between">
            <div>
              <div className="px-4 py-3 border-b border-border">
                <h3 className="text-[13.5px] font-semibold text-foreground">{t('dashboard.serverCardTitle')}</h3>
              </div>
              <dl className="grid grid-cols-[auto_1fr] gap-x-4 gap-y-2.5 px-4 py-3.5 text-[13px]">
                <dt className="text-muted-foreground">{t('steam.localVersion')}</dt>
                <dd
                  className={cn(
                    'text-right font-mono tabular-nums',
                    steamStatus && !steamStatus.installed
                      ? 'text-amber-700 dark:text-amber-400 font-sans font-medium'
                      : 'text-foreground',
                  )}
                >
                  {steamStatus
                    ? steamStatus.installed
                      ? steamStatus.local_version || '—'
                      : t('steam.notInstalled')
                    : '—'}
                </dd>
                <dt className="text-muted-foreground">{t('steam.remoteVersion')}</dt>
                <dd className="text-right font-mono tabular-nums text-foreground">
                  {steamStatus?.remote_version || '—'}
                </dd>
                <dt className="text-muted-foreground">{t('settings.branch')}</dt>
                <dd className="text-right font-mono text-foreground">{steamStatus?.branch || 'public'}</dd>
              </dl>
            </div>
            <div className="px-4 pb-4 pt-1">
              <Link to="/steam" className={cn(buttonVariants({ variant: 'outline' }), 'w-full')}>
                <Download className="h-3.5 w-3.5" />
                {steamStatus && (!steamStatus.installed || steamStatus.update_needed)
                  ? t('dashboard.goSteamSetup')
                  : t('dashboard.serverManageBtn')}
              </Link>
            </div>
          </div>
        </section>
      )}

      <CreateClusterDialog
        open={createOpen}
        onClose={() => setCreateOpen(false)}
        onCreated={(cid) => navigate(`/clusters/${encodeURIComponent(cid)}/overview`)}
      />
      <ImportClusterDialog
        open={importOpen}
        onClose={() => setImportOpen(false)}
        onImported={(cid) => navigate(`/clusters/${encodeURIComponent(cid)}/overview`)}
      />

      <ConfirmDialog
        open={!!pendingAction}
        onClose={() => setPendingAction(null)}
        onConfirm={() => {
          if (!pendingAction) return
          if (pendingAction.kind === 'stop') stopMut.mutate(pendingAction.cluster.cluster_id)
          else restartMut.mutate(pendingAction.cluster.cluster_id)
        }}
        title={
          pendingAction?.kind === 'restart'
            ? t('dashboard.restartOnlineConfirmTitle')
            : t('dashboard.stopOnlineConfirmTitle')
        }
        description={
          pendingAction?.kind === 'restart'
            ? t('dashboard.restartOnlineConfirmDesc', {
                name: pendingAction.cluster.cluster_name || pendingAction.cluster.cluster_id,
                count: pendingAction.cluster.world_info?.online_players?.length ?? 0,
              })
            : t('dashboard.stopOnlineConfirmDesc', {
                name: pendingAction?.cluster.cluster_name || pendingAction?.cluster.cluster_id || '',
                count: pendingAction?.cluster.world_info?.online_players?.length ?? 0,
              })
        }
        confirmLabel={pendingAction?.kind === 'restart' ? t('dashboard.restartAll') : t('dashboard.stopAll')}
        tone="warning"
        loading={stopMut.isPending || restartMut.isPending}
      />

      <ConfirmDialog
        open={stopAllConfirmOpen}
        onClose={() => setStopAllConfirmOpen(false)}
        onConfirm={() => void runBatchStop()}
        title={t('dashboard.stopAllConfirmTitle')}
        description={t('dashboard.stopAllConfirmDesc', { count: onlineClusters.length })}
        confirmLabel={t('dashboard.stopAllClusters')}
        tone="warning"
        loading={batchBusy}
      />
    </div>
  )
}

function TelemetryCell({
  label,
  meta,
  value,
  unit,
  desc,
  percent,
  samples,
  className,
}: {
  label: string
  meta?: string
  value: string
  unit?: string
  desc?: string
  percent: number
  samples: number[]
  className?: string
}) {
  const { t } = useTranslation()
  const isCritical = percent >= 85
  const isElevated = percent >= 70 && percent < 85
  const sparkTone = isCritical ? 'red' : isElevated ? 'amber' : 'neutral'

  return (
    <div className={cn('px-4.5 py-4 flex flex-col justify-between gap-2.5', className)}>
      <div className="flex items-center justify-between gap-2 text-xs text-muted-foreground">
        <span className="truncate">{label}</span>
        {meta && <code className="text-[11.5px] tabular-nums text-muted-foreground shrink-0">{meta}</code>}
      </div>

      <div>
        <div
          className={cn(
            'text-[26px] font-semibold tracking-[-0.025em] leading-none tabular-nums',
            isCritical
              ? 'text-red-700 dark:text-red-400'
              : isElevated
                ? 'text-amber-700 dark:text-amber-400'
                : 'text-foreground',
          )}
          title={
            isCritical
              ? t('dashboard.highUsageAlert')
              : isElevated
                ? t('dashboard.elevatedUsageAlert')
                : undefined
          }
        >
          {value}
          {unit && <span className="ml-0.5 text-[14px] font-normal text-muted-foreground tracking-normal">{unit}</span>}
        </div>
        {desc && <div className="mt-1 text-xs text-muted-foreground truncate">{desc}</div>}
      </div>

      <div className="pt-0.5">
        <Sparkline values={samples.length > 0 ? samples : [percent]} tone={sparkTone} height={30} />
      </div>
    </div>
  )
}

function ShardCell({
  label,
  port,
  connectHost,
  password,
  status,
}: {
  label: string
  port?: number
  connectHost: string
  password?: string
  status: ShardStatus
}) {
  const { t } = useTranslation()
  const state = status.state
  const active = state === 'running' || state === 'starting'
  const connectCmd = port ? formatConnectCommand(connectHost, port, password) : ''

  const dotCls =
    state === 'running'
      ? 'bg-emerald-600 dark:bg-emerald-500'
      : state === 'crashed'
        ? 'bg-red-600 dark:bg-red-500'
        : state === 'starting' || state === 'stopping'
          ? 'bg-amber-500'
          : 'bg-muted-foreground/45'

  return (
    <div className="min-w-0">
      <div className="flex items-center gap-1.5 text-[13px] whitespace-nowrap">
        {state === 'starting' || state === 'stopping' ? (
          <Loader2 className="h-3 w-3 animate-spin text-amber-500 shrink-0" aria-hidden="true" />
        ) : (
          <span className={cn('h-[7px] w-[7px] rounded-full shrink-0', dotCls)} aria-hidden="true" />
        )}
        <span className="text-foreground">{label}</span>
        {port ? (
          <span className="inline-flex items-center gap-0.5 font-mono text-[11.5px] text-muted-foreground tabular-nums">
            :{port}
            <CopyButton value={connectCmd} label={`${t('common.copyConnectCmd')}: ${connectCmd}`} />
          </span>
        ) : null}
      </div>
      <div className="mt-0.5 pl-3.5 text-[11.5px] text-muted-foreground tabular-nums truncate">
        {active ? `${status.cpu_percent}% · ${Math.round(status.memory_mb)} MB` : t(`common.${state}`)}
      </div>
    </div>
  )
}
