import { type ReactNode, useEffect, useRef, useState } from 'react'
import { Link, useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import type { TFunction } from 'i18next'
import { toast } from 'sonner'
import {
  AlertTriangle,
  ArrowRight,
  Terminal,
  CheckCircle2,
  ChevronDown,
  CircleAlert,
  Copy,
  Download,
  Eye,
  EyeOff,
  Flame,
  Flower2,
  Loader2,
  Play,
  Plus,
  RotateCw,
  Server,
  Snowflake,
  Square,
  Sun,
  Upload,
} from 'lucide-react'
import { api } from '@/lib/api'
import { formatCharacterName, formatConnectCommand } from '@/lib/dst'
import { cn, type CompositeClusterHealth, formatUptime, getClusterHealth } from '@/lib/utils'
import { useAuth } from '@/hooks/useAuth'
import { Button, buttonVariants } from '@/components/ui/button'
import { CopyButton, copyTextWithFallback } from '@/components/ui/copy-button'
import { CollapsibleContent } from '@/components/ui/collapsible'
import { DashboardSkeleton } from '@/components/ui/skeleton'
import { PageHeader } from '@/components/PageHeader'
import { DSTClockDial, ShardStateBadge } from '@/components/StatusBadge'
import { ImportClusterDialog } from '@/components/ImportClusterDialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import type { ClusterSummary, HostMetrics, ShardStatus, SteamStatus } from '@/types/api'

type ClusterFilter = 'all' | 'running' | 'stopped'

/** Filtering only earns its place once the lobby no longer fits at a glance. */
const FILTER_MIN_CLUSTERS = 4
const MAX_PLAYER_CHIPS = 5

const isClusterActive = (c: ClusterSummary) =>
  c.master_status.state === 'running' ||
  c.master_status.state === 'starting' ||
  (c.caves_enabled && (c.caves_status?.state === 'running' || c.caves_status?.state === 'starting'))

const clusterName = (c: ClusterSummary) => c.cluster_name || c.cluster_id

const isCrashed = (c: ClusterSummary) =>
  getClusterHealth(c.master_status, c.caves_enabled, c.caves_status) === 'crashed'

/** First non-empty line of a shard's last error, for a one-line summary. */
const crashErrors = (c: ClusterSummary) =>
  [c.master_status, c.caves_enabled ? c.caves_status : undefined]
    .filter((s): s is ShardStatus => Boolean(s && s.state === 'crashed' && s.last_error))
    .map((s) => s.last_error as string)

const DETAILS_STORAGE_KEY = 'walrus-dashboard-details-open'

function readOpenDetails(): string[] {
  try {
    const raw = localStorage.getItem(DETAILS_STORAGE_KEY)
    const parsed: unknown = raw ? JSON.parse(raw) : []
    return Array.isArray(parsed) ? parsed.filter((x): x is string => typeof x === 'string') : []
  } catch {
    return []
  }
}

function writeOpenDetails(id: string, open: boolean) {
  try {
    const next = new Set(readOpenDetails())
    if (open) next.add(id)
    else next.delete(id)
    localStorage.setItem(DETAILS_STORAGE_KEY, JSON.stringify([...next]))
  } catch {
    // Storage can be unavailable (private mode); the toggle still works for this visit.
  }
}

const crashedShardNames = (c: ClusterSummary, t: TFunction) =>
  [
    c.master_status.state === 'crashed' ? t('common.masterShort') : null,
    c.caves_enabled && c.caves_status?.state === 'crashed' ? t('common.cavesShort') : null,
  ].filter((x): x is string => Boolean(x))

type CrashDiagnosisKind = { kind: 'mod'; modId: string } | { kind: 'oom' } | null

/** Recognise the crash causes an owner can act on without reading Lua. */
function diagnoseCrash(errors: string[]): CrashDiagnosisKind {
  const text = errors.join('\n')
  const mod = text.match(/workshop-(\d+)/)
  if (mod) return { kind: 'mod', modId: mod[1] }
  if (/out of memory|cannot allocate memory|bad_alloc/i.test(text)) return { kind: 'oom' }
  return null
}

/** "3 minutes ago" within a day, a short date and time beyond that. */
function formatWhen(iso: string | undefined, locale: string): string {
  if (!iso) return ''
  const d = new Date(iso)
  if (Number.isNaN(d.getTime())) return ''
  const diffSec = Math.round((d.getTime() - Date.now()) / 1000)
  const rtf = new Intl.RelativeTimeFormat(locale, { numeric: 'auto' })
  if (Math.abs(diffSec) < 60) return rtf.format(diffSec, 'second')
  if (Math.abs(diffSec) < 3600) return rtf.format(Math.round(diffSec / 60), 'minute')
  if (Math.abs(diffSec) < 86400) return rtf.format(Math.round(diffSec / 3600), 'hour')
  return d.toLocaleString(locale, { month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit', hour12: false })
}

/** Most recent crash time across the cluster's shards. */
const lastCrashAt = (c: ClusterSummary) =>
  [c.master_status.last_crash_at, c.caves_enabled ? c.caves_status?.last_crash_at : undefined]
    .filter((x): x is string => Boolean(x))
    .sort()
    .pop()

const totalRestarts = (c: ClusterSummary) =>
  (c.master_status.restart_count ?? 0) + (c.caves_enabled ? (c.caves_status?.restart_count ?? 0) : 0)

export function DashboardPage() {
  const { t } = useTranslation()
  const { user } = useAuth()
  const qc = useQueryClient()
  const navigate = useNavigate()

  const [importOpen, setImportOpen] = useState(false)
  const [clusterFilter, setClusterFilter] = useState<ClusterFilter>('all')
  const [busyClusterId, setBusyClusterId] = useState<string | null>(null)
  const [batchBusy, setBatchBusy] = useState(false)
  const [stopAllConfirmOpen, setStopAllConfirmOpen] = useState(false)
  // Worlds started from this page stay in their row until they are actually up, so they don't jump away.
  const [startedHere, setStartedHere] = useState<Set<string>>(() => new Set())
  const [pendingAction, setPendingAction] = useState<{ kind: 'stop' | 'restart'; cluster: ClusterSummary } | null>(null)

  const { data: metricsData } = useQuery({
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
    dataUpdatedAt: clustersUpdatedAt,
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
  const metrics = metricsData?.metrics
  const connectHost = publicIpData?.ip || window.location.hostname
  const clusters = clustersData?.clusters ?? []
  const steamStatus = steamData?.status
  const serverInstalled = steamStatus ? steamStatus.installed : true
  // Mock runtime fakes the DST binary and token, so nothing blocks a start there.
  const enforceStartPrereqs = steamStatus ? steamStatus.runtime_mode !== 'mock' : false

  const onlineClusters = clusters.filter(
    (c) => c.master_status.state === 'running' || c.caves_status?.state === 'running',
  )
  const totalOnlinePlayers = clusters.reduce((sum, c) => sum + (c.world_info?.online_players?.length ?? 0), 0)
  const operableClusters = clusters.filter((c) => c.effective_role === 'admin' || c.effective_role === 'operator')

  const startBlockReason = (c: ClusterSummary): string | null => {
    if (!enforceStartPrereqs) return null
    if (!serverInstalled) return t('dashboard.startBlockedNoServer')
    if (!c.has_token) return t('dashboard.startBlockedNoToken')
    return null
  }

  // Crashed worlds are left out: they should be looked at, not restarted in bulk.
  const stoppedOperable = operableClusters.filter((c) => !isClusterActive(c) && !isCrashed(c))
  const batchStartTargets = stoppedOperable.filter((c) => !startBlockReason(c))
  const batchStopTargets = operableClusters.filter(
    (c) => c.master_status.state === 'running' || c.caves_status?.state === 'running',
  )
  const playersOnStop = batchStopTargets
    .map((c) => ({ name: clusterName(c), count: c.world_info?.online_players?.length ?? 0 }))
    .filter((x) => x.count > 0)
  const playersOnStopTotal = playersOnStop.reduce((sum, x) => sum + x.count, 0)

  const filteredClusters = clusters.filter((c) => {
    const running = c.master_status.state === 'running' || c.caves_status?.state === 'running'
    if (clusterFilter === 'running') return running
    if (clusterFilter === 'stopped') return !running
    return true
  })

  const afterClusterChange = () => void qc.invalidateQueries({ queryKey: ['clusters'] })

  const startMut = useMutation({
    mutationFn: (id: string) => {
      setBusyClusterId(id)
      return api.startCluster(id)
    },
    onSuccess: (_, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.success(t('dashboard.toastStarting', { name: target ? clusterName(target) : id }))
      setStartedHere((prev) => new Set(prev).add(id))
      afterClusterChange()
    },
    onError: (err: Error, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.error(t('dashboard.toastStartFailed', { name: target ? clusterName(target) : id }), {
        description: err.message,
      })
    },
    onSettled: () => setBusyClusterId(null),
  })

  const stopMut = useMutation({
    mutationFn: (id: string) => {
      setBusyClusterId(id)
      return api.stopCluster(id)
    },
    onSuccess: (_, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.success(t('dashboard.toastStopped', { name: target ? clusterName(target) : id }))
      setPendingAction(null)
      afterClusterChange()
    },
    onError: (err: Error, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.error(t('dashboard.toastStopFailed', { name: target ? clusterName(target) : id }), {
        description: err.message,
      })
    },
    onSettled: () => setBusyClusterId(null),
  })

  const restartMut = useMutation({
    mutationFn: (id: string) => {
      setBusyClusterId(id)
      return api.restartCluster(id)
    },
    onSuccess: (_, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.success(t('dashboard.toastRestarted', { name: target ? clusterName(target) : id }))
      setPendingAction(null)
      afterClusterChange()
    },
    onError: (err: Error, id) => {
      const target = clusters.find((c) => c.cluster_id === id)
      toast.error(t('dashboard.toastRestartFailed', { name: target ? clusterName(target) : id }), {
        description: err.message,
      })
    },
    onSettled: () => setBusyClusterId(null),
  })

  // Restart only what crashed, so players in the other shard stay in the game.
  const startCrashedShards = async (c: ClusterSummary) => {
    const shards = (['master', 'caves'] as const).filter((sh) =>
      sh === 'master' ? c.master_status.state === 'crashed' : c.caves_enabled && c.caves_status?.state === 'crashed',
    )
    setBusyClusterId(c.cluster_id)
    try {
      for (const sh of shards) await api.startCluster(c.cluster_id, sh)
      toast.success(t('dashboard.toastShardRestarting', { name: clusterName(c), shards: crashedShardNames(c, t).join('、') }))
    } catch (err) {
      toast.error(t('dashboard.toastStartFailed', { name: clusterName(c) }), {
        description: err instanceof Error ? err.message : String(err),
      })
    } finally {
      setBusyClusterId(null)
      afterClusterChange()
    }
  }

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

  /** Runs a batch and reports exactly what happened, per cluster, instead of a blanket "success". */
  const runBatch = async (kind: 'start' | 'stop', targets: ClusterSummary[]) => {
    if (targets.length === 0) return
    setBatchBusy(true)
    try {
      const results = await Promise.allSettled(
        targets.map((c) => (kind === 'start' ? api.startCluster(c.cluster_id) : api.stopCluster(c.cluster_id))),
      )
      const failures = results.flatMap((r, i) =>
        r.status === 'rejected'
          ? [`${clusterName(targets[i])}：${r.reason instanceof Error ? r.reason.message : String(r.reason)}`]
          : [],
      )
      const ok = targets.length - failures.length
      const doneKey = kind === 'start' ? 'dashboard.batchStartDone' : 'dashboard.batchStopDone'
      if (failures.length === 0) {
        toast.success(t(doneKey, { count: ok }))
      } else if (ok === 0) {
        toast.error(t(kind === 'start' ? 'dashboard.batchStartFailed' : 'dashboard.batchStopFailed'), {
          description: failures.join('\n'),
        })
      } else {
        toast.warning(t('dashboard.batchPartial', { ok, total: targets.length }), {
          description: failures.join('\n'),
        })
      }
      if (kind === 'stop') setStopAllConfirmOpen(false)
      afterClusterChange()
    } finally {
      setBatchBusy(false)
    }
  }

  // has_token is false only when there is no cluster token AND no global default, so the
  // global default in Settings is the one fix that covers every world: explain it once, here.
  const tokenMissing = clusters.filter((c) => !c.has_token)

  const showHeaderCreate = isAdmin && clusters.length > 0
  const showFilter = clusters.length >= FILTER_MIN_CLUSTERS
  // A batch button for a single world just duplicates that world's own button.
  const showBatchStart = stoppedOperable.length >= 2
  const showBatchStop = batchStopTargets.length >= 2

  // A crash is the moment the owner actually opens the panel. A fully-down crashed world leads the page;
  // a partly-running one keeps a single card that carries the crash inside it, so it never appears twice.
  const stillComingUp = (c: ClusterSummary) => startedHere.has(c.cluster_id) && c.master_status.state === 'starting'
  const crashedClusters = filteredClusters.filter((c) => isCrashed(c) && !isClusterActive(c))
  const awakeClusters = filteredClusters.filter((c) => isClusterActive(c) && !stillComingUp(c))
  const asleepClusters = filteredClusters.filter((c) => (!isClusterActive(c) && !isCrashed(c)) || stillComingUp(c))
  useEffect(() => {
    // Forget a world once it has settled (running, stopped or crashed).
    setStartedHere((prev) => {
      const next = new Set([...prev].filter((id) => clusters.find((c) => c.cluster_id === id)?.master_status.state === 'starting'))
      return next.size === prev.size ? prev : next
    })
  }, [clusters])

  // Announce state changes that happen between polls, for people who cannot watch the badges.
  const prevHealth = useRef<Record<string, string>>({})
  const [liveMessage, setLiveMessage] = useState('')
  useEffect(() => {
    const messages: string[] = []
    for (const c of clusters) {
      const h = getClusterHealth(c.master_status, c.caves_enabled, c.caves_status)
      const before = prevHealth.current[c.cluster_id]
      if (before && before !== h) {
        if (h === 'crashed') messages.push(t('dashboard.liveCrashed', { name: clusterName(c) }))
        else if (h === 'running') messages.push(t('dashboard.liveRunning', { name: clusterName(c) }))
        else if (h === 'stopped') messages.push(t('dashboard.liveStopped', { name: clusterName(c) }))
      }
      prevHealth.current[c.cluster_id] = h
    }
    if (messages.length > 0) setLiveMessage(messages.join('；'))
  }, [clusters, t])

  const subtitle =
    clusters.length > 0
      ? t('dashboard.subtitleSummary', {
          total: clusters.length,
          running: onlineClusters.length,
          players: totalOnlinePlayers,
        })
      : t('dashboard.subtitle')

  return (
    <div className="space-y-6">
      <PageHeader
        title={t('dashboard.title')}
        subtitle={subtitle}
        actions={
          showHeaderCreate ? (
            <>
              <Button variant="outline" onClick={() => setImportOpen(true)}>
                <Upload className="h-3.5 w-3.5" />
                {t('importCluster.btn')}
              </Button>
              <Button variant="outline" onClick={() => navigate('/clusters/new')}>
                <Plus className="h-3.5 w-3.5" />
                {t('dashboard.createCluster')}
              </Button>
            </>
          ) : undefined
        }
      />

      <SetupNotices
        items={[
          ...(isAdmin && steamStatus && !steamStatus.installed && clusters.length > 0
            ? [
                {
                  key: 'server',
                  title: t('dashboard.serverMissingTitle'),
                  desc: t('dashboard.serverMissingDesc'),
                  href: '/steam',
                  action: t('dashboard.goSteamSetup'),
                },
              ]
            : []),
          ...(tokenMissing.length > 0
            ? [
                {
                  key: 'token',
                  title:
                    tokenMissing.length === 1
                      ? t('dashboard.tokenMissingOneTitle', { name: clusterName(tokenMissing[0]) })
                      : t('dashboard.tokensMissingTitle', { count: tokenMissing.length }),
                  desc: (
                    <>
                      {t('dashboard.tokenMissingWhy')}
                      <br />
                      {t('dashboard.tokenWherePrefix')}
                      <a
                        href="https://accounts.klei.com/account/game/servers?game=DontStarveTogether"
                        target="_blank"
                        rel="noreferrer"
                        className="font-medium text-foreground underline underline-offset-2 hover:opacity-80"
                      >
                        accounts.klei.com
                      </a>
                      {t('dashboard.tokenWhereSuffix')}
                      {isAdmin ? t('dashboard.tokensMissingFix') : t('dashboard.tokenAskOwner')}
                    </>
                  ),
                  href: isAdmin ? '/settings' : undefined,
                  action: t('dashboard.goSettingsToken'),
                },
              ]
            : []),
        ]}
      />

      <section aria-labelledby="dashboard-worlds-heading" className="space-y-3">
        {clusters.length > 0 && (
          <div className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2.5">
            <div className="flex items-center gap-2">
              <h2
                id="dashboard-worlds-heading"
                className="text-[15px] font-semibold tracking-[-0.01em] text-foreground"
              >
                {t('dashboard.clustersHeading')}
              </h2>
            </div>
            <div className="flex flex-wrap items-center gap-2">
              {asleepClusters.length > 0 && awakeClusters.length > 0 && (
                // On narrow screens the not-running list sits below tall cards; give it a shortcut.
                <a
                  href="#dashboard-asleep"
                  className="inline-flex min-h-8 items-center gap-1 rounded-md px-2 text-[13px] text-muted-foreground hover:text-foreground underline underline-offset-2 xl:hidden"
                >
                  {t('dashboard.asleepJump', { count: asleepClusters.length })}
                  <ChevronDown className="h-3.5 w-3.5" aria-hidden="true" />
                </a>
              )}
              {showFilter && (
                <div
                  role="group"
                  aria-label={t('dashboard.filterLabel')}
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
                      aria-pressed={clusterFilter === item.key}
                      onClick={() => setClusterFilter(item.key)}
                      className={cn(
                        'min-h-8 rounded-[6px] px-3 transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                        clusterFilter === item.key
                          ? 'bg-[var(--surface-hover)] text-foreground font-medium'
                          : 'text-muted-foreground hover:text-foreground',
                      )}
                    >
                      {item.label}
                    </button>
                  ))}
                </div>
              )}
              {showBatchStart && (
                <Button
                  size="sm"
                  variant="ghost"
                  disabled={batchBusy || batchStartTargets.length === 0}
                  title={batchStartTargets.length === 0 ? (startBlockReason(stoppedOperable[0]) ?? undefined) : undefined}
                  onClick={() => void runBatch('start', batchStartTargets)}
                >
                  <Play className="h-3.5 w-3.5" />
                  {t('dashboard.startAllClusters')}
                </Button>
              )}
              {showBatchStop && (
                <Button size="sm" variant="ghost" disabled={batchBusy} onClick={() => setStopAllConfirmOpen(true)}>
                  <Square className="h-3 w-3" />
                  {t('dashboard.stopAllClusters')}
                </Button>
              )}
            </div>
          </div>
        )}

        {clustersErrorFlag && clustersData && (
          <p
            role="status"
            className="flex items-center gap-2 rounded-lg border border-amber-500/30 bg-amber-500/[0.06] px-3 py-2 text-[13px] text-amber-800 dark:text-amber-300"
          >
            <AlertTriangle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
            {t('dashboard.staleData', { seconds: Math.max(1, Math.round((Date.now() - clustersUpdatedAt) / 1000)) })}
          </p>
        )}
        {isLoading ? (
          <DashboardSkeleton />
        ) : clustersErrorFlag && !clustersData ? (
          <QueryErrorCard error={clustersError} onRetry={() => void refetchClusters()} />
        ) : clusters.length === 0 ? (
          <EmptyLobby
            isAdmin={isAdmin}
            serverInstalled={serverInstalled}
            onCreate={() => navigate('/clusters/new')}
            onImport={() => setImportOpen(true)}
          />
        ) : filteredClusters.length === 0 ? (
          <div className="rounded-[10px] border border-dashed border-border py-10 px-4 text-center text-[13px] text-muted-foreground">
            {t('dashboard.filterEmpty')}
          </div>
        ) : (
          <div className="flex flex-col gap-4">
            {crashedClusters.length > 0 && (
              <div>
              <h3 className="sr-only">{t('dashboard.crashedHeading')}</h3>
              <ul className="space-y-2">
                {crashedClusters.map((c) => (
                  <CrashedWorldRow
                    key={c.cluster_id}
                    cluster={c}
                    busy={busyClusterId === c.cluster_id || batchBusy}
                    startBlockReason={startBlockReason(c)}
                    onRecover={() => startMut.mutate(c.cluster_id)}
                  />
                ))}
              </ul>
              </div>
            )}
            {awakeClusters.length > 0 && (
              <div className="grid gap-4 grid-cols-[repeat(auto-fit,minmax(min(100%,21rem),1fr))]">
                {awakeClusters.map((c) => (
                  <WorldCard
                    key={c.cluster_id}
                    cluster={c}
                    connectHost={connectHost}
                    hostIsGuess={!publicIpData?.ip}
                    isAdmin={isAdmin}
                    busy={busyClusterId === c.cluster_id || batchBusy}
                    restarting={busyClusterId === c.cluster_id && restartMut.isPending}
                    onStop={() => requestClusterAction('stop', c)}
                    onRestart={() => requestClusterAction('restart', c)}
                    onStartCrashedShards={() => startCrashedShards(c)}
                  />
                ))}
              </div>
            )}
            {asleepClusters.length > 0 && (
              <div id="dashboard-asleep" className="scroll-mt-20 space-y-2">
                {awakeClusters.length > 0 && (
                  <h3 className="text-[13px] font-medium text-muted-foreground">
                    {t('dashboard.asleepHeading', { count: asleepClusters.length })}
                  </h3>
                )}
                <ul className="rounded-xl surface-card divide-y divide-border">
                  {asleepClusters.map((c) => (
                    <SleepingWorldRow
                      key={c.cluster_id}
                      cluster={c}
                      busy={busyClusterId === c.cluster_id || batchBusy}
                      startBlockReason={startBlockReason(c)}
                      onStart={() => startMut.mutate(c.cluster_id)}
                    />
                  ))}
                </ul>
              </div>
            )}
          </div>
        )}
      </section>

      <p className="sr-only" aria-live="polite">
        {liveMessage}
      </p>

      <HostStrip metrics={metrics} steamStatus={steamStatus} isAdmin={isAdmin} />

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
        description={t(
          pendingAction?.kind === 'restart' ? 'dashboard.restartOnlineConfirmDesc' : 'dashboard.stopOnlineConfirmDesc',
          {
            name: pendingAction ? clusterName(pendingAction.cluster) : '',
            count: pendingAction?.cluster.world_info?.online_players?.length ?? 0,
          },
        )}
        confirmLabel={pendingAction?.kind === 'restart' ? t('dashboard.restartAll') : t('dashboard.stopAll')}
        tone="warning"
        loading={stopMut.isPending || restartMut.isPending}
      />

      <ConfirmDialog
        open={stopAllConfirmOpen}
        onClose={() => setStopAllConfirmOpen(false)}
        onConfirm={() => void runBatch('stop', batchStopTargets)}
        title={t('dashboard.stopAllConfirmTitle', { count: batchStopTargets.length })}
        description={
          playersOnStopTotal > 0
            ? t('dashboard.stopAllConfirmDescPlayers', {
                count: playersOnStopTotal,
                detail: playersOnStop.map((x) => t('dashboard.playersInWorld', { name: x.name, count: x.count })).join('、'),
              })
            : t('dashboard.stopAllConfirmDescEmpty')
        }
        confirmLabel={t('dashboard.stopAllClusters')}
        tone="warning"
        loading={batchBusy}
      />
    </div>
  )
}

interface SetupNotice {
  key: string
  title: string
  desc: ReactNode
  href?: string
  action: string
}

/** Blockers before friends can play: one item reads as a banner, several as a short checklist. */
function SetupNotices({ items }: { items: SetupNotice[] }) {
  const { t } = useTranslation()
  if (items.length === 0) return null

  return (
    <div className="rounded-[10px] border border-amber-500/35 bg-amber-500/[0.07]">
      {items.length > 1 && (
        <p className="flex items-center gap-2 px-4 pt-3.5 text-[14px] font-semibold text-foreground">
          <AlertTriangle className="h-4 w-4 shrink-0 text-amber-700 dark:text-amber-400" aria-hidden="true" />
          {t('dashboard.setupRemaining', { count: items.length })}
        </p>
      )}
      <ul className={cn(items.length > 1 && 'divide-y divide-amber-500/20')}>
        {items.map((item) => (
          <li
            key={item.key}
            className="flex flex-col gap-3 px-4 py-3.5 sm:flex-row sm:items-center sm:justify-between"
          >
            <div className="flex items-start gap-3 min-w-0">
              {items.length === 1 ? (
                <AlertTriangle className="mt-0.5 h-4 w-4 shrink-0 text-amber-700 dark:text-amber-400" aria-hidden="true" />
              ) : (
                <span className="mt-[7px] ml-[5px] mr-[3px] h-1.5 w-1.5 shrink-0 rounded-full bg-amber-500" aria-hidden="true" />
              )}
              <div className="min-w-0 space-y-0.5">
                <p className={cn('text-foreground', items.length === 1 ? 'text-[14px] font-semibold' : 'text-[13px] font-medium')}>
                  {item.title}
                </p>
                <p className="text-[13px] text-muted-foreground text-pretty">{item.desc}</p>
              </div>
            </div>
            {item.href && (
              <Link
                to={item.href}
                className={cn(buttonVariants({ variant: 'warning', size: 'sm' }), 'shrink-0 self-start sm:self-auto')}
              >
                {item.action}
                <ArrowRight className="h-3.5 w-3.5" />
              </Link>
            )}
          </li>
        ))}
      </ul>
    </div>
  )
}

const SEASON_ICON: Record<string, typeof Flame> = {
  autumn: Flame,
  winter: Snowflake,
  spring: Flower2,
  summer: Sun,
}

function WorldCard({
  cluster: c,
  connectHost,
  hostIsGuess,
  isAdmin,
  busy,
  restarting,
  onStop,
  onRestart,
  onStartCrashedShards,
}: {
  cluster: ClusterSummary
  connectHost: string
  hostIsGuess: boolean
  isAdmin: boolean
  busy: boolean
  restarting: boolean
  onStop: () => void
  onRestart: () => void
  onStartCrashedShards: () => void
}) {
  const { t, i18n } = useTranslation()
  const isEn = i18n.language === 'en-US'
  const [detailsOpen, setDetailsOpen] = useState(() => readOpenDetails().includes(c.cluster_id))
  const toggleDetails = () =>
    setDetailsOpen((v) => {
      writeOpenDetails(c.cluster_id, !v)
      return !v
    })

  const name = clusterName(c)
  const health = getClusterHealth(c.master_status, c.caves_enabled, c.caves_status)
  const restarts = totalRestarts(c)
  const lit = health === 'running' && c.world_info?.day > 0 && c.has_token
  // Forest down means nobody can be in the world, whatever the caves report.
  const masterCrashed = c.master_status.state === 'crashed'
  const running = c.master_status.state === 'running'
  const canOperate = c.effective_role === 'admin' || c.effective_role === 'operator'
  const players = c.world_info?.online_players ?? []
  const overviewHref = `/clusters/${encodeURIComponent(c.cluster_id)}/overview`
  const detailsId = `world-details-${c.cluster_id}`
  const forWorld = (action: string) => t('dashboard.ariaFor', { action, name })

  return (
    <article
      aria-label={name}
      className={cn(
        '@container flex flex-col rounded-xl surface-card surface-interactive overflow-hidden',
        lit && 'border-[color-mix(in_srgb,var(--ember)_55%,var(--border))]',
      )}
    >
      <header className="flex items-start justify-between gap-3 px-5 pt-4">
        <div className="min-w-0">
          <h3 className="min-w-0">
            <Link
              to={overviewHref}
              className="block truncate text-[16px] font-semibold tracking-[-0.015em] text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
            >
              {name}
            </Link>
          </h3>
          <p className="mt-0.5 text-[13px] text-muted-foreground">
            {t(`dashboard.modes.${c.game_mode}`, c.game_mode)}
            <span aria-hidden="true" className="mx-1.5 opacity-50">
              ·
            </span>
            {c.caves_enabled ? t('dashboard.worldsForestCaves') : t('dashboard.worldsForestOnly')}
          </p>
          <Readiness cluster={c} health={health} />
        </div>
        <div className="flex shrink-0 flex-col items-end gap-1.5 pt-0.5">
          {/* A partial crash is serious but not total: say so with amber, not the red of a dead world. */}
          <ShardStateBadge state={health === 'crashed' ? 'degraded' : health} />
          {restarts > 0 && health !== 'crashed' && (
            <Link
              to={`/clusters/${encodeURIComponent(c.cluster_id)}/console`}
              aria-label={t('dashboard.crashRestartsAria', { name })}
              className="inline-flex items-center gap-1 rounded-md bg-amber-500/10 px-1.5 py-0.5 text-[12px] font-medium text-amber-800 underline decoration-dotted underline-offset-2 hover:decoration-solid dark:text-amber-300"
            >
              <CircleAlert className="h-3 w-3 shrink-0" aria-hidden="true" />
              {lastCrashAt(c)
                ? t('dashboard.crashRestartsAt', { count: restarts, time: formatWhen(lastCrashAt(c), i18n.language) })
                : t('dashboard.crashRestarts', { count: restarts })}
            </Link>
          )}
        </div>
      </header>

      {health === 'crashed' && (
        // One shard down while the other runs: an amber section of this card, not a second card.
        <div className="mx-5 mt-4 space-y-2.5 rounded-lg bg-amber-500/[0.08] px-3.5 py-3">
          <p className="flex items-center gap-1.5 text-[13px] font-semibold text-foreground">
            <CircleAlert className="h-4 w-4 shrink-0 text-amber-700 dark:text-amber-400" aria-hidden="true" />
            {t('dashboard.shardCrashed', { shards: crashedShardNames(c, t).join('、') })}
          </p>
          <CrashDiagnosis cluster={c} />
          <CrashActions
            cluster={c}
            busy={busy}
            blockReason={null}
            recoverLabel={t('dashboard.restartShard', { shard: crashedShardNames(c, t).join('、') })}
            onRecover={onStartCrashedShards}
          />
        </div>
      )}

      {masterCrashed ? (
        <div className="flex-1" />
      ) : (
        <div className="grid flex-1 content-start items-start gap-x-6 gap-y-4 px-5 py-4 @md:grid-cols-2 @3xl:grid-cols-[minmax(0,2fr)_minmax(0,1.15fr)]">
          <div className="flex min-w-0 flex-col gap-4 @3xl:grid @3xl:grid-cols-2 @3xl:gap-6">
            <WorldClock cluster={c} running={running} />
            {running && <PlayerList players={players} max={c.max_players} isEn={isEn} />}
          </div>
          {running && (
            <JoinInfo cluster={c} connectHost={connectHost} hostIsGuess={hostIsGuess} isAdmin={isAdmin} />
          )}
        </div>
      )}

      <div className="border-t border-border">
        <button
          type="button"
          aria-expanded={detailsOpen}
          aria-controls={detailsId}
          aria-label={forWorld(t('dashboard.techDetails'))}
          onClick={toggleDetails}
          className="flex w-full min-h-10 items-center justify-between gap-2 px-5 text-[13px] text-muted-foreground hover:text-foreground transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-ring"
        >
          <span>{t('dashboard.techDetails')}</span>
          <ChevronDown
            className={cn(
              'h-3.5 w-3.5 [transition:transform_200ms_var(--ease-out)] motion-reduce:transition-none',
              detailsOpen && 'rotate-180',
            )}
            aria-hidden="true"
          />
        </button>
        <CollapsibleContent open={detailsOpen}>
          <div id={detailsId} className="space-y-2 px-5 pb-4">
            <ShardDetail label={t('common.masterShort')} port={c.master_port} status={c.master_status} />
            {c.caves_enabled && c.caves_status && (
              <ShardDetail label={t('common.cavesShort')} port={c.caves_port} status={c.caves_status} />
            )}
            <p className="pt-1 text-[12px] text-muted-foreground">
              {t('dashboard.clusterDir')} <code className="text-foreground/85">{c.cluster_id}</code>
              {c.auto_restart && (
                <>
                  <span aria-hidden="true" className="mx-1.5 opacity-50">
                    ·
                  </span>
                  {t('dashboard.autoRestartOn')}
                </>
              )}
            </p>
          </div>
        </CollapsibleContent>
      </div>

      <footer className="flex flex-wrap items-center justify-end gap-x-2 gap-y-2 border-t border-border px-5 py-3">
        {canOperate && (
          <>
            <Button size="sm" variant="ghost" disabled={busy} onClick={onStop} aria-label={forWorld(t('dashboard.stopAll'))}>
              <Square className="h-3 w-3" />
              {t('dashboard.stopAll')}
            </Button>
            <Button size="sm" variant="ghost" disabled={busy} onClick={onRestart} aria-label={forWorld(t('dashboard.restartAll'))}>
              <RotateCw className={cn('h-3.5 w-3.5', restarting && 'animate-spin')} />
              {t('dashboard.restartAll')}
            </Button>
          </>
        )}
        <Link
          to={overviewHref}
          aria-label={forWorld(t('dashboard.openCluster'))}
          className={cn(buttonVariants({ size: 'sm', variant: 'outline' }), 'group')}
        >
          <span>{t('dashboard.openCluster')}</span>
          <ArrowRight className="h-3.5 w-3.5 transition-transform duration-150 motion-safe:group-hover:translate-x-0.5" />
        </Link>
      </footer>
    </article>
  )
}

/** One line that answers the question the owner actually has: can my friends get in right now? */
function Readiness({ cluster: c, health }: { cluster: ClusterSummary; health: CompositeClusterHealth }) {
  const { t } = useTranslation()
  const masterUp = c.master_status.state === 'running'

  let tone: 'ok' | 'wait' | 'warn' = 'wait'
  let text = t('dashboard.readyStarting')
  if (!c.has_token) {
    tone = 'warn'
    text = t('dashboard.readyNoToken')
  } else if (health === 'running') {
    tone = c.world_info?.day > 0 ? 'ok' : 'wait'
    text = c.world_info?.day > 0 ? t('dashboard.readyYes') : t('dashboard.readyWaking')
  } else if (health === 'crashed') {
    tone = 'warn'
    text = masterUp ? t('dashboard.readyCavesCrashed') : t('dashboard.readyForestCrashed')
  } else if (health === 'degraded') {
    tone = 'warn'
    text = masterUp ? t('dashboard.readyForestOnly') : t('dashboard.readyNo')
  } else if (health === 'stopping') {
    text = t('dashboard.readyStopping')
  }

  const Icon = tone === 'ok' ? CheckCircle2 : tone === 'warn' ? AlertTriangle : Loader2
  return (
    <p
      className={cn(
        'mt-2.5 inline-flex w-fit items-center gap-1.5 rounded-md px-2 py-1 text-[14px] font-semibold',
        tone === 'ok' && 'bg-emerald-500/12 text-emerald-700 dark:text-emerald-400',
        tone === 'warn' && 'bg-amber-500/12 text-amber-800 dark:text-amber-300',
        tone === 'wait' && 'bg-[var(--surface-elevated)] text-muted-foreground',
      )}
    >
      <Icon className={cn('h-3.5 w-3.5 shrink-0', tone === 'wait' && 'animate-spin')} aria-hidden="true" />
      {text}
    </p>
  )
}

/**
 * What happened, when, and what to do about it, in plain words. The raw trace stays one
 * click away for whoever can read it, so the summary line never gets truncated mid-path.
 */
function CrashDiagnosis({ cluster: c }: { cluster: ClusterSummary }) {
  const { t, i18n } = useTranslation()
  const errors = crashErrors(c)
  const diagnosis = diagnoseCrash(errors)
  const { data: libraryData } = useQuery({
    queryKey: ['mod-library'],
    queryFn: api.listModLibrary,
    enabled: diagnosis?.kind === 'mod',
    staleTime: 5 * 60 * 1000,
  })
  const restarts = totalRestarts(c)
  const pending =
    (c.master_status.state === 'crashed' && c.master_status.auto_restart_pending) ||
    (c.caves_enabled && c.caves_status?.state === 'crashed' && c.caves_status.auto_restart_pending)
  const when = formatWhen(lastCrashAt(c), i18n.language)
  const modName =
    diagnosis?.kind === 'mod'
      ? (libraryData?.mods.find((m) => m.workshop_id === diagnosis.modId || m.mod_id === `workshop-${diagnosis.modId}`)?.name ??
        `workshop-${diagnosis.modId}`)
      : ''

  const status = [
    when ? t('dashboard.crashedAt', { time: when }) : '',
    pending
      ? t('dashboard.autoRestartPending')
      : restarts > 0 && c.auto_restart
        ? t('dashboard.crashedAutoRestarted', { count: restarts })
        : '',
  ]
    .filter(Boolean)
    .join(' · ')

  return (
    <div className="min-w-0 space-y-1">
      {status && (
        <p className="flex items-center gap-1.5 text-[13px] text-muted-foreground">
          {pending && <Loader2 className="h-3.5 w-3.5 shrink-0 animate-spin" aria-hidden="true" />}
          {status}
        </p>
      )}
      <p className="text-[13px] text-foreground text-pretty">
        {diagnosis?.kind === 'mod'
          ? t('dashboard.diagnoseMod', { name: modName })
          : diagnosis?.kind === 'oom'
            ? t('dashboard.diagnoseOom')
            : t('dashboard.crashedHint')}
      </p>
      {errors.length > 0 && (
        <details className="group">
          <summary className="inline-flex cursor-pointer list-none items-center gap-1 rounded-sm text-[12px] text-muted-foreground underline underline-offset-2 hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring [&::-webkit-details-marker]:hidden">
            <span className="group-open:hidden">{t('dashboard.showFullError')}</span>
            <span className="hidden group-open:inline">{t('dashboard.hideFullError')}</span>
          </summary>
          <pre className="mt-1.5 max-h-48 overflow-auto whitespace-pre-wrap break-words rounded-md bg-[var(--surface-terminal)] px-3 py-2 font-mono text-[12px] leading-relaxed text-[var(--foreground)]">
            {errors.join('\n\n')}
          </pre>
        </details>
      )}
    </div>
  )
}

/** The action that fixes the cause, when the cause is known; the logs otherwise. */
function CrashActions({
  cluster: c,
  busy,
  blockReason,
  recoverLabel,
  onRecover,
}: {
  cluster: ClusterSummary
  busy: boolean
  blockReason: string | null
  recoverLabel: string
  onRecover: () => void
}) {
  const { t } = useTranslation()
  const name = clusterName(c)
  const canOperate = c.effective_role === 'admin' || c.effective_role === 'operator'
  const diagnosis = diagnoseCrash(crashErrors(c))
  const pending =
    (c.master_status.state === 'crashed' && c.master_status.auto_restart_pending) ||
    (c.caves_enabled && c.caves_status?.state === 'crashed' && c.caves_status.auto_restart_pending)
  const base = `/clusters/${encodeURIComponent(c.cluster_id)}`
  const forWorld = (action: string) => t('dashboard.ariaFor', { action, name })
  const fixHref = diagnosis?.kind === 'mod' && canOperate ? `${base}/mods` : null

  return (
    <div className="flex flex-wrap items-center gap-x-2 gap-y-1.5">
      {!canOperate && <span className="text-[13px] text-muted-foreground">{t('dashboard.needOperator')}</span>}
      {canOperate && blockReason && <span className="text-[13px] text-muted-foreground">{blockReason}</span>}
      {fixHref && (
        <Link to={fixHref} aria-label={forWorld(t('dashboard.fixMod'))} className={buttonVariants({ size: 'sm' })}>
          {t('dashboard.fixMod')}
          <ArrowRight className="h-3.5 w-3.5" />
        </Link>
      )}
      <Link
        to={`${base}/console`}
        aria-label={forWorld(t('dashboard.viewLogs'))}
        className={buttonVariants({ size: 'sm', variant: fixHref ? 'outline' : 'default' })}
      >
        <Terminal className="h-3.5 w-3.5" />
        {t('dashboard.viewLogs')}
      </Link>
      {canOperate && !pending && (
        // Restarting is offered, but never as the loudest choice: it already failed at least once.
        <Button
          size="sm"
          variant="outline"
          disabled={busy || !!blockReason}
          onClick={onRecover}
          aria-label={forWorld(recoverLabel)}
        >
          {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <RotateCw className="h-3.5 w-3.5" />}
          {recoverLabel}
        </Button>
      )}
    </div>
  )
}

/** A world that crashed completely: pinned above everything. */
function CrashedWorldRow({
  cluster: c,
  busy,
  startBlockReason,
  onRecover,
}: {
  cluster: ClusterSummary
  busy: boolean
  startBlockReason: string | null
  onRecover: () => void
}) {
  const { t } = useTranslation()
  const name = clusterName(c)
  const overviewHref = `/clusters/${encodeURIComponent(c.cluster_id)}/overview`

  return (
    <li className="flex flex-wrap items-start gap-x-4 gap-y-3 rounded-xl border border-red-500/40 bg-red-500/[0.07] px-4 py-3.5 sm:px-5">
      <CircleAlert className="mt-0.5 h-5 w-5 shrink-0 text-red-700 dark:text-red-400" aria-hidden="true" />
      <div className="min-w-0 flex-1 basis-64 space-y-1">
        <p className="text-[14px] font-semibold text-foreground">
          <Link to={overviewHref} className="hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm">
            {name}
          </Link>
          <span className="font-normal text-muted-foreground"> · </span>
          {t('dashboard.shardCrashed', { shards: crashedShardNames(c, t).join('、') })}
        </p>
        <CrashDiagnosis cluster={c} />
      </div>
      <div className="ml-auto">
        <CrashActions
          cluster={c}
          busy={busy}
          blockReason={startBlockReason}
          recoverLabel={t('dashboard.recoverWorld')}
          onRecover={onRecover}
        />
      </div>
    </li>
  )
}

/** A world that is not running: one quiet line, so the lit campfires above keep the attention. */
function SleepingWorldRow({
  cluster: c,
  busy,
  startBlockReason,
  onStart,
}: {
  cluster: ClusterSummary
  busy: boolean
  startBlockReason: string | null
  onStart: () => void
}) {
  const { t } = useTranslation()
  const name = clusterName(c)
  const health = getClusterHealth(c.master_status, c.caves_enabled, c.caves_status)
  const canOperate = c.effective_role === 'admin' || c.effective_role === 'operator'
  const overviewHref = `/clusters/${encodeURIComponent(c.cluster_id)}/overview`
  const forWorld = (action: string) => t('dashboard.ariaFor', { action, name })
  const note = !canOperate ? t('dashboard.needOperator') : startBlockReason

  return (
    <li className="flex flex-wrap items-center gap-x-4 gap-y-2 px-4 py-3 sm:px-5">
      <DSTClockDial className="h-8 w-8 opacity-60" />
      <div className="min-w-0 flex-1 basis-48">
        <div className="flex min-w-0 items-center gap-2.5">
          <Link
            to={overviewHref}
            className="truncate text-[14px] font-semibold text-foreground hover:underline focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-sm"
          >
            {name}
          </Link>
          <ShardStateBadge state={health} />
        </div>
        <p className="mt-0.5 truncate text-[13px] text-muted-foreground">
          {[
            t(`dashboard.modes.${c.game_mode}`, c.game_mode),
            c.caves_enabled ? t('dashboard.worldsForestCaves') : t('dashboard.worldsForestOnly'),
            t('dashboard.maxPlayers', { count: c.max_players }),
          ].join(' · ')}
        </p>
      </div>
      <div className="ml-auto flex flex-wrap items-center justify-end gap-x-2 gap-y-1.5">
        {note && <span className="text-[13px] text-muted-foreground">{note}</span>}
        {canOperate && !isClusterActive(c) && (
          <Button
            size="sm"
            variant="outline"
            disabled={busy || !!startBlockReason}
            onClick={onStart}
            aria-label={forWorld(t('dashboard.startWorld'))}
          >
            {busy ? <Loader2 className="h-3.5 w-3.5 animate-spin" /> : <Play className="h-3.5 w-3.5" />}
            {t('dashboard.startWorld')}
          </Button>
        )}
        <Link
          to={overviewHref}
          aria-label={forWorld(t('dashboard.openCluster'))}
          className={cn(buttonVariants({ size: 'sm', variant: 'outline' }), 'group')}
        >
          <span>{t('dashboard.openCluster')}</span>
          <ArrowRight className="h-3.5 w-3.5 transition-transform duration-150 motion-safe:group-hover:translate-x-0.5" />
        </Link>
      </div>
    </li>
  )
}

/** The campfire: lit once the world reports its day and season, waking until then. */
function WorldClock({ cluster: c, running }: { cluster: ClusterSummary; running: boolean }) {
  const { t } = useTranslation()
  const info = c.world_info
  const known = Boolean(info?.season && info.season !== 'unknown' && info.day > 0)
  const SeasonIcon = SEASON_ICON[info?.season] ?? Flame
  const phaseLabel = info?.phase && info.phase !== 'unknown' ? t(`dashboard.phases.${info.phase}`, '') : ''
  const [dayPrefix, daySuffix] = t('dashboard.day', { day: '\u0000' }).split('\u0000')
  const lit = running && known

  return (
    <div className="flex items-center gap-3.5 min-w-0">
      <div className="relative h-14 w-14 shrink-0">
        {lit && (
          <span
            aria-hidden="true"
            className="absolute -inset-3 rounded-full auth-campfire-breathe"
            style={{
              background: 'radial-gradient(circle, var(--ember-glow) 0%, transparent 70%)',
            }}
          />
        )}
        <DSTClockDial
          phase={lit ? info.phase : undefined}
          className={cn('relative h-14 w-14', !lit && 'opacity-60')}
        />
      </div>
      <div className="min-w-0 tabular-nums">
        {lit ? (
          <>
            <div className="flex items-baseline gap-1 leading-none whitespace-nowrap">
              <span className="text-[13px] text-muted-foreground">{dayPrefix}</span>
              <span className="font-display text-[30px] text-foreground">{info.day}</span>
              <span className="text-[13px] text-muted-foreground">{daySuffix}</span>
            </div>
            <div className="mt-1.5 flex items-center gap-1.5 text-[13px] text-foreground">
              <SeasonIcon
                className="h-3.5 w-3.5 shrink-0"
                style={{ color: `var(--season-${info.season})` }}
                aria-hidden="true"
              />
              <span className="font-medium">{t(`dashboard.seasons.${info.season}`, info.season)}</span>
              {phaseLabel && (
                <>
                  <span aria-hidden="true" className="opacity-45">
                    ·
                  </span>
                  <span className="text-muted-foreground">{phaseLabel}</span>
                </>
              )}
            </div>
            {info.remaining_days_in_season > 0 && (
              <div className="mt-0.5 text-[12px] text-muted-foreground">
                {t('overview.seasonRemaining', { days: info.remaining_days_in_season })}
              </div>
            )}
          </>
        ) : (
          <>
            <div className="text-[14px] font-medium text-foreground">{t('dashboard.worldWaking')}</div>
            <div className="mt-0.5 text-[13px] text-muted-foreground text-pretty">{t('dashboard.worldWakingHint')}</div>
          </>
        )}
      </div>
    </div>
  )
}

function PlayerList({
  players,
  max,
  isEn,
}: {
  players: ClusterSummary['world_info']['online_players']
  max: number
  isEn: boolean
}) {
  const { t } = useTranslation()
  const shown = players.slice(0, MAX_PLAYER_CHIPS)
  const extra = players.length - shown.length

  return (
    <div className="min-w-0">
      <div className="flex items-baseline gap-1.5 tabular-nums">
        <span className="text-[13px] text-muted-foreground">{t('dashboard.onlineNow')}</span>
        <span className="text-[15px] font-semibold text-foreground">{players.length}</span>
        <span className="text-[13px] text-muted-foreground">/ {max}</span>
      </div>
      {players.length > 0 ? (
        <ul className="mt-2 flex flex-wrap gap-1.5" aria-label={t('dashboard.onlineNow')}>
          {shown.map((p) => (
            <li
              key={p.ku_id || p.name}
              className="inline-flex max-w-full items-center gap-1 rounded-md bg-[var(--surface-elevated)] px-2 py-1 text-[13px]"
            >
              <span className="truncate text-foreground">{p.name}</span>
              {p.prefab && formatCharacterName(p.prefab, isEn) !== p.name && (
                <>
                  <span aria-hidden="true" className="text-muted-foreground/60">
                    ·
                  </span>
                  <span className="shrink-0 text-muted-foreground">{formatCharacterName(p.prefab, isEn)}</span>
                </>
              )}
            </li>
          ))}
          {extra > 0 && (
            <li className="inline-flex items-center rounded-md px-1.5 py-1 text-[13px] text-muted-foreground">
              {t('dashboard.morePlayers', { count: extra })}
            </li>
          )}
        </ul>
      ) : (
        <p className="mt-2 text-[13px] text-muted-foreground">
          {t('dashboard.noOneOnline')}
        </p>
      )}
    </div>
  )
}

/** Everything a friend needs to get in, in the words they will type. */
function JoinInfo({
  cluster: c,
  connectHost,
  hostIsGuess,
  isAdmin,
}: {
  cluster: ClusterSummary
  connectHost: string
  hostIsGuess: boolean
  isAdmin: boolean
}) {
  const { t } = useTranslation()
  const [revealed, setRevealed] = useState(false)
  const connectCmd = c.master_port ? formatConnectCommand(connectHost, c.master_port, c.cluster_password) : ''

  const copyConnect = async () => {
    if (!connectCmd) return
    const ok = await copyTextWithFallback(connectCmd)
    if (ok) toast.success(t('dashboard.connectCopied'))
    else toast.error(t('common.copyFailed'))
  }

  // Without a token the world never reaches the Klei lobby, so join details would be a false promise.
  if (!c.has_token) {
    return (
      <div className="min-w-0 rounded-lg bg-[var(--surface-elevated)]/70 px-3.5 py-3">
        <div className="text-[13px] font-medium text-foreground">{t('dashboard.howToJoin')}</div>
        <p className="mt-1.5 text-[13px] text-muted-foreground text-pretty">{t('dashboard.joinBlockedByToken')}</p>
        {isAdmin ? (
          <Link
            to="/settings"
            className={cn(buttonVariants({ size: 'sm', variant: 'warning' }), 'mt-2.5 w-full')}
          >
            {t('dashboard.goSettingsToken')}
            <ArrowRight className="h-3.5 w-3.5" />
          </Link>
        ) : (
          <p className="mt-1.5 text-[13px] text-muted-foreground">{t('dashboard.tokenAskOwnerShort')}</p>
        )}
      </div>
    )
  }

  return (
    <div className="min-w-0 rounded-lg bg-[var(--surface-elevated)]/70 px-3.5 py-3">
      <div className="text-[13px] font-medium text-foreground">{t('dashboard.howToJoin')}</div>
      <dl className="mt-2 grid grid-cols-[auto_minmax(0,1fr)] items-center gap-x-3 gap-y-1 text-[13px]">
        <dt className="text-muted-foreground">{t('dashboard.lobbyName')}</dt>
        <dd className="flex min-w-0 items-center justify-end gap-0.5">
          <span className="min-w-0 break-all text-right text-foreground">{clusterName(c)}</span>
          <CopyButton value={clusterName(c)} label={t('dashboard.copyLobbyName')} />
        </dd>
        <dt className="text-muted-foreground">{t('dashboard.roomPassword')}</dt>
        <dd className="flex min-w-0 items-center justify-end gap-0.5">
          {c.cluster_password ? (
            <>
              <span className={cn('font-mono text-foreground', revealed ? 'truncate' : 'shrink-0')}>
                {revealed ? c.cluster_password : '••••••'}
              </span>
              <button
                type="button"
                onClick={() => setRevealed((v) => !v)}
                aria-label={revealed ? t('dashboard.hidePassword') : t('dashboard.showPassword')}
                className="relative inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer before:absolute before:-inset-2 before:content-[''] sm:before:hidden"
              >
                {revealed ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
              </button>
              <CopyButton value={c.cluster_password} label={t('dashboard.copyPassword')} />
            </>
          ) : (
            <span className="py-1.5 text-muted-foreground">{t('dashboard.noPassword')}</span>
          )}
        </dd>
      </dl>
      {connectCmd && (
        <div className="mt-2.5">
          <Button
            size="sm"
            className="w-full"
            onClick={() => void copyConnect()}
            aria-label={t('dashboard.ariaFor', { action: t('dashboard.copyDirectConnect'), name: clusterName(c) })}
          >
            <Copy className="h-3.5 w-3.5" />
            {t('dashboard.copyDirectConnect')}
          </Button>
          <p className="mt-1.5 text-[12px] leading-snug text-muted-foreground text-pretty">
            {t('dashboard.connectHowTo')}
          </p>
          {hostIsGuess && (
            <p className="mt-1.5 flex items-start gap-1.5 text-[12px] leading-snug text-amber-800 dark:text-amber-300">
              <AlertTriangle className="mt-px h-3 w-3 shrink-0" aria-hidden="true" />
              <span>{t('dashboard.hostGuessWarning', { host: connectHost })}</span>
            </p>
          )}
        </div>
      )}
    </div>
  )
}

function ShardDetail({
  label,
  port,
  status,
}: {
  label: string
  port?: number
  status: ShardStatus
}) {
  const { t } = useTranslation()
  const active = status.state === 'running' || status.state === 'starting'

  return (
    <div className="flex flex-wrap items-center gap-x-3 gap-y-1 text-[13px] tabular-nums">
      <span className="w-10 shrink-0 font-medium text-foreground">{label}</span>
      <ShardStateBadge state={status.state} />
      {port ? (
        <span className="inline-flex items-center gap-1 text-muted-foreground">
          {t('dashboard.port')}
          <code className="text-foreground/85">{port}</code>
          <CopyButton value={String(port)} label={t('dashboard.copyPort', { port })} />
        </span>
      ) : null}
      {active && (
        <span className="text-muted-foreground">
          CPU {status.cpu_percent}% · {t('dashboard.memory')} {Math.round(status.memory_mb)} MB ·{' '}
          {t('dashboard.uptime')} {formatUptime(status.uptime_sec)}
        </span>
      )}
      {status.restart_count > 0 && (
        <span className="text-amber-700 dark:text-amber-400">
          {t('dashboard.crashRestarts', { count: status.restart_count })}
        </span>
      )}
    </div>
  )
}

function HostStrip({
  metrics,
  steamStatus,
  isAdmin,
}: {
  metrics?: HostMetrics
  steamStatus?: SteamStatus
  isAdmin: boolean
}) {
  const { t } = useTranslation()

  return (
    <section
      aria-label={t('dashboard.hostStripLabel')}
      className="flex flex-wrap items-center gap-x-5 gap-y-2 rounded-[10px] border border-border px-4 py-2.5 text-[13px] tabular-nums"
    >
      <span className="font-medium text-muted-foreground">{t('dashboard.hostStripLabel')}</span>
      {metrics ? (
        <>
          <HostMetric label="CPU" percent={metrics.cpu_percent} value={`${Math.round(metrics.cpu_percent)}%`} />
          <HostMetric
            label={t('dashboard.memory')}
            percent={metrics.memory_percent}
            value={`${(metrics.memory_used_mb / 1024).toFixed(1)} / ${(metrics.memory_total_mb / 1024).toFixed(1)} GB`}
          />
          <HostMetric
            label={t('dashboard.disk')}
            percent={metrics.disk_percent}
            value={`${metrics.disk_used_gb} / ${metrics.disk_total_gb} GB`}
          />
        </>
      ) : (
        <span className="text-muted-foreground">—</span>
      )}
      {steamStatus && (steamStatus.installed || !isAdmin) && (
        <span className="inline-flex items-center gap-1.5 sm:ml-auto">
          <span className="text-muted-foreground">{t('dashboard.serverVersion')}</span>
          {steamStatus.installed ? (
            <>
              <code className="text-foreground">{steamStatus.local_version || '—'}</code>
              {steamStatus.update_needed && isAdmin && (
                <Link to="/steam" className="font-medium text-amber-800 hover:underline dark:text-amber-300">
                  {t('dashboard.serverUpdateAvailable')}
                </Link>
              )}
            </>
          ) : (
            <span className="text-amber-800 dark:text-amber-300">{t('steam.notInstalled')}</span>
          )}
        </span>
      )}
    </section>
  )
}

function HostMetric({ label, value, percent }: { label: string; value: string; percent: number }) {
  const { t } = useTranslation()
  const level = percent >= 85 ? 'critical' : percent >= 70 ? 'elevated' : 'normal'

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5',
        level === 'critical' && 'rounded-md bg-red-500/10 px-1.5 py-0.5 text-red-700 dark:text-red-400',
        level === 'elevated' && 'rounded-md bg-amber-500/10 px-1.5 py-0.5 text-amber-800 dark:text-amber-300',
      )}
    >
      {level !== 'normal' && <AlertTriangle className="h-3 w-3 shrink-0" aria-hidden="true" />}
      <span className={level === 'normal' ? 'text-muted-foreground' : undefined}>{label}</span>
      <span className={cn('font-medium', level === 'normal' && 'text-foreground')}>{value}</span>
      {level !== 'normal' && (
        <span className="font-medium">
          {level === 'critical' ? t('dashboard.usageCritical') : t('dashboard.usageElevated')}
        </span>
      )}
    </span>
  )
}

function EmptyLobby({
  isAdmin,
  serverInstalled,
  onCreate,
  onImport,
}: {
  isAdmin: boolean
  serverInstalled: boolean
  onCreate: () => void
  onImport: () => void
}) {
  const { t } = useTranslation()
  const steps = [
    { label: t('dashboard.quickStartStep1'), state: serverInstalled ? 'done' : 'current' },
    { label: t('dashboard.quickStartStep2'), state: serverInstalled ? 'current' : 'todo' },
    { label: t('dashboard.quickStartStep3'), state: 'todo' },
  ] as const

  return (
    <div className="rounded-xl border border-dashed border-border bg-card py-11 px-6 flex flex-col items-center justify-center text-center gap-5">
      <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-[var(--surface-elevated)] text-foreground">
        <Server className="h-5 w-5" />
      </div>
      <div className="space-y-1 max-w-lg">
        <h2 className="text-base font-semibold tracking-[-0.015em]">{t('dashboard.emptyTitle')}</h2>
        <p className="text-[13px] text-muted-foreground leading-relaxed text-pretty">{t('dashboard.emptyDesc')}</p>
      </div>
      <ol className="grid grid-cols-1 sm:grid-cols-3 gap-2.5 w-full max-w-2xl text-left">
        {steps.map((step, i) => (
          <li
            key={step.label}
            className={cn(
              'rounded-lg border px-3.5 py-3 text-[13px] space-y-1',
              step.state === 'done' && 'border-emerald-600/30 bg-emerald-500/5',
              step.state === 'current' && 'border-[var(--border-strong)] bg-[var(--surface-elevated)]',
              step.state === 'todo' && 'border-border bg-background/50',
            )}
          >
            <div className="flex items-center justify-between gap-1 text-[12px] font-medium">
              <span className="tabular-nums text-muted-foreground">{t('dashboard.stepN', { n: i + 1 })}</span>
              {step.state === 'done' && (
                <span className="inline-flex items-center gap-1 text-emerald-700 dark:text-emerald-400">
                  <CheckCircle2 className="h-3 w-3" />
                  {t('dashboard.stepInstalled')}
                </span>
              )}
              {step.state === 'current' && <span className="text-foreground">{t('dashboard.stepPending')}</span>}
            </div>
            <div className="font-medium text-foreground">{step.label}</div>
          </li>
        ))}
      </ol>
      {isAdmin && (
        <div className="flex flex-wrap items-center justify-center gap-2 pt-1">
          {!serverInstalled && (
            <Link to="/steam" className={buttonVariants({ variant: 'default' })}>
              <Download className="h-3.5 w-3.5" />
              {t('dashboard.goSteamSetup')}
            </Link>
          )}
          <Button variant={serverInstalled ? 'default' : 'outline'} onClick={onCreate}>
            <Plus className="h-3.5 w-3.5" />
            {t('dashboard.createCluster')}
          </Button>
          <Button variant="outline" onClick={onImport}>
            <Upload className="h-3.5 w-3.5" />
            {t('importCluster.btn')}
          </Button>
        </div>
      )}
    </div>
  )
}
