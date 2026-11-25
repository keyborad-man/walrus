import { useState } from 'react'
import { Link, useNavigate, useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertTriangle,
  Archive,
  ArrowRight,
  ChevronDown,
  Clock,
  Cpu,
  FolderOpen,
  History,
  Link2,
  Megaphone,
  MemoryStick,
  Mountain,
  Play,
  RotateCw,
  Save,
  Square,
  Terminal,
  Trash2,
  Trees,
  Users,
} from 'lucide-react'
import { api } from '@/lib/api'
import { cn, formatUptime } from '@/lib/utils'
import { formatCharacterName, formatConnectCommand } from '@/lib/dst'
import { useAuth } from '@/hooks/useAuth'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/ui/copy-button'
import { CollapsibleContent } from '@/components/ui/collapsible'
import { ClusterOverviewSkeleton } from '@/components/ui/skeleton'
import { PageHeader } from '@/components/PageHeader'
import { EmptyState } from '@/components/EmptyState'
import { DSTClockHeroDial, ShardStateBadge } from '@/components/StatusBadge'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import type { ShardStatus } from '@/types/api'

export function ClusterOverviewPage() {
  const { id = '' } = useParams<{ id: string }>()
  const { t, i18n } = useTranslation()
  const isEn = i18n.language === 'en-US'
  const { user } = useAuth()
  const qc = useQueryClient()
  const navigate = useNavigate()

  const [announceText, setAnnounceText] = useState('')
  const [rollbackDays, setRollbackDays] = useState<number | null>(null)
  const [pendingPower, setPendingPower] = useState<{
    kind: 'stop' | 'restart'
    target: 'all' | 'master' | 'caves'
  } | null>(null)
  const [busyTarget, setBusyTarget] = useState<'all' | 'master' | 'caves' | null>(null)
  const [dangerExpanded, setDangerExpanded] = useState(false)
  const [deleteOpen, setDeleteOpen] = useState(false)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['cluster', id],
    queryFn: () => api.getCluster(id),
    refetchInterval: 3000,
    enabled: !!id,
  })
  const { data: publicIpData } = useQuery({
    queryKey: ['system-public-ip'],
    queryFn: api.publicIP,
    staleTime: 10 * 60 * 1000,
    retry: false,
  })

  const cluster = data?.cluster
  const connectHost = publicIpData?.ip || window.location.hostname
  const effRole = data?.effective_role
  const canOperate = effRole === 'admin' || effRole === 'operator'
  const isAdmin = user?.role === 'admin'

  const invalidate = () => {
    void qc.invalidateQueries({ queryKey: ['cluster', id] })
    void qc.invalidateQueries({ queryKey: ['clusters'] })
  }

  const startMut = useMutation({
    mutationFn: (shard?: 'master' | 'caves') => {
      setBusyTarget(shard ?? 'all')
      return api.startCluster(id, shard)
    },
    onSuccess: (_, shard) => {
      const label =
        shard === 'master'
          ? t('common.master')
          : shard === 'caves'
            ? t('common.caves')
            : cluster?.cluster_ini.cluster_name || id
      toast.success(t('dashboard.toastStarted', { name: label }))
      invalidate()
    },
    onError: (err: Error) => toast.error(err.message),
    onSettled: () => setBusyTarget(null),
  })

  const stopMut = useMutation({
    mutationFn: (shard?: 'master' | 'caves') => {
      setBusyTarget(shard ?? 'all')
      return api.stopCluster(id, shard)
    },
    onSuccess: (_, shard) => {
      const label =
        shard === 'master'
          ? t('common.master')
          : shard === 'caves'
            ? t('common.caves')
            : cluster?.cluster_ini.cluster_name || id
      toast.success(t('dashboard.toastStopped', { name: label }))
      setPendingPower(null)
      invalidate()
    },
    onError: (err: Error) => toast.error(err.message),
    onSettled: () => setBusyTarget(null),
  })

  const restartMut = useMutation({
    mutationFn: (shard?: 'master' | 'caves') => {
      setBusyTarget(shard ?? 'all')
      return api.restartCluster(id, shard)
    },
    onSuccess: (_, shard) => {
      const label =
        shard === 'master'
          ? t('common.master')
          : shard === 'caves'
            ? t('common.caves')
            : cluster?.cluster_ini.cluster_name || id
      toast.success(t('dashboard.toastRestarted', { name: label }))
      setPendingPower(null)
      invalidate()
    },
    onError: (err: Error) => toast.error(err.message),
    onSettled: () => setBusyTarget(null),
  })

  const cmdMut = useMutation({
    mutationFn: (vars: { cmd: string; message: string }) => api.sendCommand(id, 'master', vars.cmd),
    onSuccess: (_, vars) => {
      toast.success(vars.message)
      setRollbackDays(null)
      invalidate()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: () => api.deleteCluster(id),
    onSuccess: () => {
      toast.success(t('overview.clusterDeleted'))
      setDeleteOpen(false)
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      navigate('/', { replace: true })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  if (isLoading) {
    return <ClusterOverviewSkeleton />
  }
  if (isError || !cluster) {
    return <QueryErrorCard error={error} onRetry={() => void refetch()} />
  }

  const masterRunning = cluster.master_status.state === 'running' || cluster.master_status.state === 'starting'
  const cavesRunning =
    cluster.meta.caves_enabled &&
    (cluster.caves_status?.state === 'running' || cluster.caves_status?.state === 'starting')
  const anyActive = masterRunning || cavesRunning
  const onlinePlayersCount = cluster.world_info.online_players.length
  const maxPlayers = cluster.cluster_ini.max_players || 6
  const playerRatio = Math.min(1, onlinePlayersCount / Math.max(1, maxPlayers))
  const hasWorldClock = Boolean(
    cluster.world_info.season && cluster.world_info.season !== 'unknown' && cluster.world_info.day > 0,
  )
  const showSeparateId = Boolean(
    cluster.cluster_ini.cluster_name && cluster.cluster_ini.cluster_name !== cluster.cluster_id,
  )

  const requestPower = (kind: 'stop' | 'restart', target: 'all' | 'master' | 'caves') => {
    if (onlinePlayersCount > 0) {
      setPendingPower({ kind, target })
    } else if (kind === 'stop') {
      stopMut.mutate(target === 'all' ? undefined : target)
    } else {
      restartMut.mutate(target === 'all' ? undefined : target)
    }
  }

  const handleAnnounce = (e: React.FormEvent) => {
    e.preventDefault()
    const clean = announceText
      .trim()
      .replaceAll('\\', '\\\\')
      .replaceAll('"', '\\"')
      .replace(/\r?\n/g, ' ')
    if (!clean) return
    cmdMut.mutate({ cmd: `c_announce("${clean}")`, message: t('overview.toastAnnounced') })
    setAnnounceText('')
  }

  const powerModalCopy = (() => {
    if (!pendingPower) return { title: '', desc: '', cta: '' }
    const clusterName = cluster.cluster_ini.cluster_name || cluster.cluster_id
    if (pendingPower.target === 'all') {
      return pendingPower.kind === 'restart'
        ? {
            title: t('dashboard.restartOnlineConfirmTitle'),
            desc: t('dashboard.restartOnlineConfirmDesc', { name: clusterName, count: onlinePlayersCount }),
            cta: t('overview.restartCluster'),
          }
        : {
            title: t('dashboard.stopOnlineConfirmTitle'),
            desc: t('dashboard.stopOnlineConfirmDesc', { name: clusterName, count: onlinePlayersCount }),
            cta: t('overview.stopCluster'),
          }
    }
    const shardLabel = pendingPower.target === 'caves' ? t('common.caves') : t('common.master')
    if (pendingPower.kind === 'restart') {
      return {
        title: t('overview.restartShardConfirmTitle', { shard: shardLabel }),
        desc:
          pendingPower.target === 'caves'
            ? t('overview.restartCavesConfirmDesc', { count: onlinePlayersCount })
            : t('overview.restartMasterConfirmDesc', { count: onlinePlayersCount }),
        cta: t('overview.restartShardAction', { shard: shardLabel }),
      }
    }
    return {
      title: t('overview.stopShardConfirmTitle', { shard: shardLabel }),
      desc:
        pendingPower.target === 'caves'
          ? t('overview.stopCavesConfirmDesc', { count: onlinePlayersCount })
          : t('overview.stopMasterConfirmDesc', { count: onlinePlayersCount }),
      cta: t('overview.stopShardAction', { shard: shardLabel }),
    }
  })()

  const headerSub = [
    t(`dashboard.modes.${cluster.cluster_ini.game_mode}`, cluster.cluster_ini.game_mode),
    cluster.cluster_ini.cluster_description || null,
  ]
    .filter(Boolean)
    .join(' · ')

  return (
    <div className="space-y-6">
      <PageHeader
        title={cluster.cluster_ini.cluster_name || cluster.cluster_id}
        subtitle={headerSub || undefined}
        badge={
          showSeparateId ? (
            <Badge variant="secondary" className="font-mono text-[11px]">
              {cluster.cluster_id}
            </Badge>
          ) : undefined
        }
        actions={
          canOperate ? (
            <>
              {!anyActive ? (
                <Button variant="default" disabled={busyTarget === 'all'} onClick={() => startMut.mutate(undefined)}>
                  <Play className="h-3.5 w-3.5" />
                  {t('overview.startCluster')}
                </Button>
              ) : (
                <>
                  <Button variant="warning" disabled={busyTarget === 'all'} onClick={() => requestPower('stop', 'all')}>
                    <Square className="h-3.5 w-3.5" />
                    {t('overview.stopCluster')}
                  </Button>
                  <Button
                    variant="outline"
                    disabled={busyTarget === 'all'}
                    onClick={() => requestPower('restart', 'all')}
                  >
                    <RotateCw
                      className={cn('h-3.5 w-3.5', busyTarget === 'all' && restartMut.isPending && 'animate-spin')}
                    />
                    {t('overview.restartCluster')}
                  </Button>
                </>
              )}
            </>
          ) : undefined
        }
      />

      {/* Single-Line Actionable Missing-Token Warning */}
      {!cluster.has_cluster_token && (
        <div
          role="alert"
          className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-card px-3.5 py-2.5 text-[13px] text-muted-foreground"
        >
          <div className="flex items-center gap-2.5 min-w-0">
            <span className="h-[7px] w-[7px] rounded-full bg-amber-500 shrink-0" aria-hidden="true" />
            <span className="text-foreground/90">{t('overview.tokenMissingBanner')}</span>
          </div>
          {canOperate && (
            <Link
              to={`/clusters/${encodeURIComponent(id)}/config`}
              className="inline-flex items-center gap-1 text-[13px] font-medium text-foreground hover:opacity-80 transition-opacity shrink-0"
            >
              <span>{t('overview.goConfigureToken')}</span>
              <ArrowRight className="h-3.5 w-3.5" />
            </Link>
          )}
        </div>
      )}

      {/* Unified Overview Summary Strip: World Clock + Dual-World Internal Link */}
      <section className="rounded-[10px] surface-card overflow-hidden">
        <div className="grid grid-cols-1 lg:grid-cols-12 divide-y lg:divide-y-0 lg:divide-x divide-border">
          {/* Left: World Clock */}
          <div className="lg:col-span-5 p-5 flex flex-col justify-between gap-4">
            <div className="text-xs text-muted-foreground">{t('overview.worldClockTitle')}</div>
            <div className="flex items-center gap-4">
              <DSTClockHeroDial
                day={hasWorldClock ? cluster.world_info.day : undefined}
                phase={cluster.world_info.phase}
                season={cluster.world_info.season}
                className="h-20 w-20 shrink-0"
              />
              <div className="min-w-0 space-y-1.5 flex-1">
                {hasWorldClock ? (
                  <>
                    <div className="text-lg font-semibold tracking-[-0.02em] text-foreground tabular-nums">
                      {t(`dashboard.seasons.${cluster.world_info.season}`, cluster.world_info.season)}
                      <span className="mx-1.5 text-muted-foreground font-normal">·</span>
                      {t('dashboard.day', { day: cluster.world_info.day })}
                    </div>
                    <div className="text-xs text-muted-foreground flex flex-wrap items-center gap-1.5">
                      {cluster.world_info.phase && cluster.world_info.phase !== 'unknown' && (
                        <span className="text-foreground/85">
                          {t(`dashboard.phases.${cluster.world_info.phase}`, cluster.world_info.phase)}
                        </span>
                      )}
                      {cluster.world_info.remaining_days_in_season > 0 && (
                        <>
                          <span aria-hidden="true">·</span>
                          <span className="tabular-nums">
                            {t('overview.seasonRemaining', { days: cluster.world_info.remaining_days_in_season })}
                          </span>
                        </>
                      )}
                    </div>
                  </>
                ) : (
                  <>
                    <div className="text-sm font-semibold text-foreground">{t('overview.worldStandby')}</div>
                    <p className="text-xs text-muted-foreground leading-relaxed text-pretty">
                      {t('overview.worldStandbyDesc')}
                    </p>
                  </>
                )}

                <div className="pt-2 space-y-1.5">
                  <div className="flex items-center justify-between text-xs text-muted-foreground tabular-nums">
                    <span>{t('overview.capacityLabel')}</span>
                    <span className="font-mono text-foreground">
                      {onlinePlayersCount} / {maxPlayers}
                    </span>
                  </div>
                  <div className="h-[3px] w-full rounded-full bg-secondary overflow-hidden">
                    <div
                      className="h-full w-full origin-left bg-foreground/70 [transition:transform_250ms_var(--ease-out)]"
                      style={{ transform: `scaleX(${playerRatio})` }}
                    />
                  </div>
                </div>
              </div>
            </div>
          </div>

          {/* Right: Dual-World Topology (Flat, No Card-Inside-Card) */}
          <div className="lg:col-span-7 p-5 flex flex-col justify-between gap-4">
            <div className="flex flex-wrap items-center justify-between gap-1.5">
              <span className="inline-flex items-center gap-1.5 text-xs text-muted-foreground">
                <Link2 className="h-3.5 w-3.5" />
                {t('overview.shardLinkLabel')}
              </span>
              <span className="text-[11.5px] text-muted-foreground">
                {cluster.meta.caves_enabled ? t('overview.shardLinkSynced') : t('common.singleWorld')}
              </span>
            </div>

            {cluster.meta.caves_enabled && cluster.caves_status ? (
              <div className="grid grid-cols-1 sm:grid-cols-[1fr_auto_1fr] items-center gap-3">
                <TopologyNode
                  icon={<Trees className="h-3.5 w-3.5 text-muted-foreground" />}
                  title={t('common.masterShort')}
                  port={cluster.master_server_ini.server_port}
                  connectCmd={
                    cluster.master_server_ini.server_port > 0
                      ? formatConnectCommand(
                          connectHost,
                          cluster.master_server_ini.server_port,
                          cluster.cluster_ini.cluster_password,
                        )
                      : undefined
                  }
                  status={cluster.master_status}
                />

                <div className="flex sm:flex-col items-center justify-center gap-1.5 px-1">
                  <div
                    className={cn(
                      'h-px w-8 sm:w-12',
                      masterRunning && cavesRunning
                        ? 'bg-emerald-500/60'
                        : anyActive
                          ? 'bg-amber-500/50'
                          : 'bg-border',
                    )}
                    aria-hidden="true"
                  />
                  <div className="inline-flex items-center gap-1 rounded-md border border-border bg-[var(--surface-elevated)] px-2 py-0.5 font-mono text-[11px] text-muted-foreground tabular-nums whitespace-nowrap">
                    <span>{t('overview.shardLinkInternalTag')}</span>
                    <span className="text-foreground">:{cluster.cluster_ini.master_port}</span>
                  </div>
                  <div
                    className={cn(
                      'h-px w-8 sm:hidden',
                      masterRunning && cavesRunning
                        ? 'bg-emerald-500/60'
                        : anyActive
                          ? 'bg-amber-500/50'
                          : 'bg-border',
                    )}
                    aria-hidden="true"
                  />
                </div>

                <TopologyNode
                  icon={<Mountain className="h-3.5 w-3.5 text-muted-foreground" />}
                  title={t('common.cavesShort')}
                  port={cluster.caves_server_ini?.server_port ?? 0}
                  connectCmd={
                    (cluster.caves_server_ini?.server_port ?? 0) > 0
                      ? formatConnectCommand(
                          connectHost,
                          cluster.caves_server_ini!.server_port,
                          cluster.cluster_ini.cluster_password,
                        )
                      : undefined
                  }
                  status={cluster.caves_status}
                />
              </div>
            ) : (
              <div className="grid grid-cols-1 sm:grid-cols-2 items-center gap-3">
                <TopologyNode
                  icon={<Trees className="h-3.5 w-3.5 text-muted-foreground" />}
                  title={t('common.masterShort')}
                  port={cluster.master_server_ini.server_port}
                  connectCmd={
                    cluster.master_server_ini.server_port > 0
                      ? formatConnectCommand(
                          connectHost,
                          cluster.master_server_ini.server_port,
                          cluster.cluster_ini.cluster_password,
                        )
                      : undefined
                  }
                  status={cluster.master_status}
                />
                <div className="rounded-lg border border-dashed border-border px-3.5 py-3 text-xs text-muted-foreground leading-relaxed text-pretty">
                  {t('overview.singleWorldNode')}
                </div>
              </div>
            )}
          </div>
        </div>
      </section>

      {/* Dual-World Shard Process Cards */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <ShardCard
          clusterId={id}
          title={t('common.master')}
          icon={<Trees className="h-4 w-4 text-muted-foreground" />}
          status={cluster.master_status}
          port={cluster.master_server_ini.server_port}
          connectCmd={
            cluster.master_server_ini.server_port > 0
              ? formatConnectCommand(
                  connectHost,
                  cluster.master_server_ini.server_port,
                  cluster.cluster_ini.cluster_password,
                )
              : undefined
          }
          canOperate={canOperate}
          busy={busyTarget === 'all' || busyTarget === 'master'}
          restarting={(busyTarget === 'all' || busyTarget === 'master') && restartMut.isPending}
          onStart={() => startMut.mutate('master')}
          onStop={() => requestPower('stop', 'master')}
          onRestart={() => requestPower('restart', 'master')}
        />
        {cluster.meta.caves_enabled && cluster.caves_status && (
          <ShardCard
            clusterId={id}
            title={t('common.caves')}
            icon={<Mountain className="h-4 w-4 text-muted-foreground" />}
            status={cluster.caves_status}
            port={cluster.caves_server_ini?.server_port ?? 0}
            connectCmd={
              (cluster.caves_server_ini?.server_port ?? 0) > 0
                ? formatConnectCommand(
                    connectHost,
                    cluster.caves_server_ini!.server_port,
                    cluster.cluster_ini.cluster_password,
                  )
                : undefined
            }
            canOperate={canOperate}
            busy={busyTarget === 'all' || busyTarget === 'caves'}
            restarting={(busyTarget === 'all' || busyTarget === 'caves') && restartMut.isPending}
            onStart={() => startMut.mutate('caves')}
            onStop={() => requestPower('stop', 'caves')}
            onRestart={() => requestPower('restart', 'caves')}
          />
        )}
      </div>

      {/* Quick Actions & Online Players */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <Card>
          <CardHeader className="pb-3">
            <CardTitle>{t('overview.quickActions')}</CardTitle>
          </CardHeader>
          <CardContent className="space-y-3.5">
            <div className="flex flex-wrap items-center gap-2">
              <Button
                size="sm"
                variant="outline"
                disabled={!canOperate || !masterRunning || cmdMut.isPending}
                onClick={() => cmdMut.mutate({ cmd: 'c_save()', message: t('overview.toastSaved') })}
              >
                <Save className="h-3.5 w-3.5" />
                {t('overview.saveWorld')}
              </Button>
              <span className="h-4 w-px bg-border mx-0.5 hidden sm:inline-block" aria-hidden="true" />
              <Button
                size="sm"
                variant="warning"
                disabled={!canOperate || !masterRunning || cmdMut.isPending}
                onClick={() => setRollbackDays(1)}
              >
                <History className="h-3.5 w-3.5" />
                {t('overview.rollback1')}
              </Button>
              <Button
                size="sm"
                variant="warning"
                disabled={!canOperate || !masterRunning || cmdMut.isPending}
                onClick={() => setRollbackDays(3)}
              >
                <History className="h-3.5 w-3.5" />
                {t('overview.rollback3')}
              </Button>
            </div>
            <form onSubmit={handleAnnounce} className="flex flex-col sm:flex-row gap-2">
              <Input
                placeholder={t('overview.announcePlaceholder')}
                aria-label={t('overview.announcePlaceholder')}
                value={announceText}
                onChange={(e) => setAnnounceText(e.target.value)}
                disabled={!canOperate || !masterRunning}
                className="flex-1"
              />
              <Button
                type="submit"
                className="shrink-0"
                disabled={!canOperate || !masterRunning || !announceText.trim()}
              >
                <Megaphone className="h-3.5 w-3.5" />
                {t('overview.sendAnnounce')}
              </Button>
            </form>
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-3">
            <CardTitle className="flex items-center justify-between gap-2 tabular-nums">
              <span className="flex items-center gap-2">
                <Users className="h-4 w-4 text-muted-foreground" />
                {t('overview.onlinePlayers')}
              </span>
              <span className="font-mono text-xs font-normal text-muted-foreground">
                {onlinePlayersCount} / {maxPlayers}
              </span>
            </CardTitle>
          </CardHeader>
          <CardContent>
            {onlinePlayersCount === 0 ? (
              <EmptyState compact icon={<Users className="h-4 w-4" />} title={t('overview.noPlayers')} />
            ) : (
              <div className="divide-y divide-border">
                {cluster.world_info.online_players.map((p) => (
                  <div
                    key={p.ku_id}
                    className="py-2.5 first:pt-0 last:pb-0 flex items-center justify-between gap-2 text-[13px]"
                  >
                    <div className="flex items-center gap-2.5 min-w-0">
                      <div className="h-7 w-7 shrink-0 rounded-full bg-[var(--surface-hover)] text-foreground flex items-center justify-center text-[11px] font-semibold uppercase">
                        {p.name.slice(0, 1)}
                      </div>
                      <div className="min-w-0">
                        <div className="font-medium leading-tight truncate">{p.name}</div>
                        <div className="flex items-center gap-1 text-[11.5px] font-mono text-muted-foreground">
                          <span className="truncate">{p.ku_id}</span>
                          <CopyButton value={p.ku_id} />
                        </div>
                      </div>
                    </div>
                    <span className="text-xs text-muted-foreground tabular-nums shrink-0">
                      {formatCharacterName(p.prefab, isEn)} · {t('dashboard.day', { day: p.age })}
                    </span>
                  </div>
                ))}
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      {/* Collapsible Danger Zone */}
      {isAdmin && (
        <div className="rounded-[10px] border border-border bg-card">
          <button
            type="button"
            aria-expanded={dangerExpanded}
            onClick={() => setDangerExpanded((v) => !v)}
            className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left text-xs font-medium text-muted-foreground hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-[10px] cursor-pointer transition-colors"
          >
            <span className="flex items-center gap-2">
              <Trash2 className="h-3.5 w-3.5 text-red-700 dark:text-red-400" />
              <span>{t('overview.dangerZone')}</span>
            </span>
            <ChevronDown
              className={cn('h-4 w-4 transition-transform duration-200 ease-out', dangerExpanded && 'rotate-180')}
            />
          </button>

          <CollapsibleContent open={dangerExpanded}>
            <div className="border-t border-border px-4 py-3.5 flex flex-col sm:flex-row sm:items-center justify-between gap-4">
              <p className="text-xs text-muted-foreground leading-relaxed max-w-2xl">{t('overview.dangerZoneDesc')}</p>
              <Button
                variant="dangerOutline"
                size="sm"
                className="shrink-0 self-start sm:self-auto"
                disabled={deleteMut.isPending}
                onClick={() => setDeleteOpen(true)}
              >
                <Trash2 className="h-3.5 w-3.5" />
                {t('overview.deleteCluster')}
              </Button>
            </div>
          </CollapsibleContent>
        </div>
      )}

      {/* Confirmation Dialogs */}
      <ConfirmDialog
        open={rollbackDays !== null}
        onClose={() => setRollbackDays(null)}
        onConfirm={() => {
          if (rollbackDays !== null) {
            cmdMut.mutate({
              cmd: `c_rollback(${rollbackDays})`,
              message: t('overview.toastRolledBack', { days: rollbackDays }),
            })
          }
        }}
        title={t('overview.rollbackConfirmTitle')}
        description={t('overview.rollbackConfirmDesc', { days: rollbackDays ?? 1 })}
        confirmLabel={rollbackDays === 3 ? t('overview.rollback3') : t('overview.rollback1')}
        tone="warning"
        loading={cmdMut.isPending}
      />

      <ConfirmDialog
        open={pendingPower !== null}
        onClose={() => setPendingPower(null)}
        onConfirm={() => {
          if (!pendingPower) return
          const shardArg = pendingPower.target === 'all' ? undefined : pendingPower.target
          if (pendingPower.kind === 'stop') stopMut.mutate(shardArg)
          else restartMut.mutate(shardArg)
        }}
        title={powerModalCopy.title}
        description={powerModalCopy.desc}
        confirmLabel={powerModalCopy.cta}
        tone="warning"
        loading={stopMut.isPending || restartMut.isPending}
      />

      <ConfirmDialog
        open={deleteOpen}
        onClose={() => setDeleteOpen(false)}
        onConfirm={() => deleteMut.mutate()}
        title={t('overview.deleteConfirmTitle', { id: cluster.cluster_id })}
        description={t('overview.deleteConfirm', { id: cluster.cluster_id })}
        confirmLabel={t('overview.deleteCluster')}
        tone="danger"
        confirmTextMatch={cluster.cluster_id}
        loading={deleteMut.isPending}
      />
    </div>
  )
}

function TopologyNode({
  icon,
  title,
  port,
  connectCmd,
  status,
}: {
  icon: React.ReactNode
  title: string
  port: number
  connectCmd?: string
  status: ShardStatus
}) {
  const { t } = useTranslation()
  const active = status.state === 'running' || status.state === 'starting'
  return (
    <div className="rounded-lg border border-border bg-[var(--surface-elevated)]/55 p-3 space-y-2">
      <div className="flex items-center justify-between gap-2">
        <span className="flex items-center gap-1.5 text-xs font-medium text-foreground">
          {icon}
          <span>{title}</span>
        </span>
        <ShardStateBadge state={status.state} />
      </div>
      <div className="flex items-center justify-between gap-2 text-[11.5px] font-mono tabular-nums text-muted-foreground">
        <span className="inline-flex items-center gap-0.5 text-foreground">
          :{port || '-'}
          {connectCmd && <CopyButton value={connectCmd} label={`${t('common.copyConnectCmd')}: ${connectCmd}`} />}
        </span>
        <span>{active ? `${status.cpu_percent}% · ${Math.round(status.memory_mb)} MB` : '—'}</span>
      </div>
    </div>
  )
}

function ShardCard({
  clusterId,
  title,
  icon,
  status,
  port,
  connectCmd,
  canOperate,
  busy,
  restarting,
  onStart,
  onStop,
  onRestart,
}: {
  clusterId: string
  title: string
  icon: React.ReactNode
  status: ShardStatus
  port: number
  connectCmd?: string
  canOperate: boolean
  busy: boolean
  restarting: boolean
  onStart: () => void
  onStop: () => void
  onRestart: () => void
}) {
  const { t } = useTranslation()
  const active = status.state === 'running' || status.state === 'starting'

  return (
    <Card>
      <CardHeader className="pb-3">
        <div className="flex items-center justify-between gap-3">
          <CardTitle className="flex items-center gap-2">
            {icon}
            <span>{title}</span>
          </CardTitle>
          <ShardStateBadge state={status.state} />
        </div>
      </CardHeader>
      <CardContent className="space-y-4">
        <div className="grid grid-cols-2 sm:grid-cols-4 gap-3 border-y border-border py-3 text-xs tabular-nums">
          <div>
            <div className="text-[11px] text-muted-foreground">{t('overview.port')}</div>
            <div className="mt-1 font-mono font-medium text-foreground flex items-center gap-0.5">
              <span>{port || '-'}</span>
              {connectCmd && <CopyButton value={connectCmd} label={`${t('common.copyConnectCmd')}: ${connectCmd}`} />}
            </div>
          </div>
          <div>
            <div className="text-[11px] text-muted-foreground flex items-center gap-1">
              <Clock className="h-3 w-3 shrink-0" /> {t('overview.uptime')}
            </div>
            <div className="mt-1 font-mono font-medium text-foreground">
              {active ? formatUptime(status.uptime_sec) : '-'}
            </div>
          </div>
          <div>
            <div className="text-[11px] text-muted-foreground flex items-center gap-1">
              <Cpu className="h-3 w-3 shrink-0" /> {t('overview.cpu')}
            </div>
            <div className="mt-1 font-mono font-medium text-foreground">
              {active ? `${status.cpu_percent}%` : '-'}
            </div>
          </div>
          <div>
            <div className="text-[11px] text-muted-foreground flex items-center gap-1">
              <MemoryStick className="h-3 w-3 shrink-0" /> {t('overview.memory')}
            </div>
            <div className="mt-1 font-mono font-medium text-foreground">
              {active ? `${Math.round(status.memory_mb)} MB` : '-'}
            </div>
          </div>
        </div>

        {status.last_error && (
          <div role="alert" className="rounded-lg border border-red-500/30 bg-red-500/8 p-3.5 space-y-2.5 text-xs">
            <div className="flex items-start gap-2 text-red-800 dark:text-red-300">
              <AlertTriangle className="h-4 w-4 text-red-600 dark:text-red-400 shrink-0 mt-0.5" />
              <div className="min-w-0 space-y-1">
                <div className="font-semibold">{t('overview.crashHelpTitle')}</div>
                <div className="font-mono break-words text-red-700 dark:text-red-300/90">{status.last_error}</div>
              </div>
            </div>
            <div className="flex flex-wrap gap-2">
              <Link
                to={`/clusters/${encodeURIComponent(clusterId)}/console`}
                className={buttonVariants({ size: 'sm', variant: 'outline' })}
              >
                <Terminal className="h-3.5 w-3.5" />
                {t('overview.viewConsoleLogs')}
              </Link>
              <Link
                to={`/clusters/${encodeURIComponent(clusterId)}/files`}
                className={buttonVariants({ size: 'sm', variant: 'outline' })}
              >
                <FolderOpen className="h-3.5 w-3.5" />
                {t('overview.browseCrashFiles')}
              </Link>
              <Link
                to={`/clusters/${encodeURIComponent(clusterId)}/backups`}
                className={buttonVariants({ size: 'sm', variant: 'outline' })}
              >
                <Archive className="h-3.5 w-3.5" />
                {t('overview.goBackups')}
              </Link>
            </div>
          </div>
        )}

        {canOperate && (
          <div className="flex flex-wrap items-center gap-2">
            {!active ? (
              <Button size="sm" variant="default" disabled={busy} onClick={onStart}>
                <Play className="h-3.5 w-3.5" />
                {t('dashboard.startAll')}
              </Button>
            ) : (
              <>
                <Button size="sm" variant="warning" disabled={busy} onClick={onStop}>
                  <Square className="h-3.5 w-3.5" />
                  {t('dashboard.stopAll')}
                </Button>
                <Button size="sm" variant="outline" disabled={busy} onClick={onRestart}>
                  <RotateCw className={cn('h-3.5 w-3.5', restarting && 'animate-spin')} />
                  {t('dashboard.restartAll')}
                </Button>
              </>
            )}
            {status.pid > 0 && (
              <span className="ml-auto text-xs font-mono text-muted-foreground tabular-nums">PID {status.pid}</span>
            )}
          </div>
        )}
      </CardContent>
    </Card>
  )
}
