import React, { useEffect, useMemo, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertTriangle,
  ArrowUpCircle,
  CheckCircle2,
  ChevronDown,
  ExternalLink,
  Globe,
  HelpCircle,
  KeyRound,
  Loader2,
  Network,
  PlugZap,
  RefreshCw,
  Save,
  ServerCog,
  Terminal,
} from 'lucide-react'
import { api } from '@/lib/api'
import { cn, formatDateTime, getClusterHealth } from '@/lib/utils'
import { useRegisterUnsaved } from '@/hooks/useUnsavedGuard'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input, Label, SecretInput, Select } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/ui/copy-button'
import { CollapsibleContent } from '@/components/ui/collapsible'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { StickySaveBar } from '@/components/StickySaveBar'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { PageHeader } from '@/components/PageHeader'
import { PageCardsSkeleton } from '@/components/ui/skeleton'

const MANUAL_UPGRADE_CMD = 'docker compose pull && docker compose up -d'
const ALLOWED_PROXY_SCHEMES = new Set(['http:', 'https:', 'socks5:', 'socks5h:'])

function isValidProxyUrl(raw: string): boolean {
  const s = raw.trim()
  if (!s) return true
  if (/\s/.test(s)) return false
  try {
    const u = new URL(s)
    if (!ALLOWED_PROXY_SCHEMES.has(u.protocol.toLowerCase())) return false
    if (!u.hostname || !u.port) return false
    const port = Number(u.port)
    return Number.isInteger(port) && port >= 1 && port <= 65535
  } catch {
    return false
  }
}

export function SettingsPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['system-settings'],
    queryFn: api.getSettings,
  })
  const { data: versionData, refetch: refetchVersion } = useQuery({
    queryKey: ['system-version'],
    queryFn: api.systemVersion,
    retry: false,
  })
  const { data: clustersData } = useQuery({
    queryKey: ['clusters'],
    queryFn: api.listClusters,
  })

  const [portStart, setPortStart] = useState(10990)
  const [portEnd, setPortEnd] = useState(11060)
  const [defaultToken, setDefaultToken] = useState('')
  const [steamKey, setSteamKey] = useState('')
  const [clearSteamKey, setClearSteamKey] = useState(false)
  const [proxyEnabled, setProxyEnabled] = useState(false)
  const [proxyUrl, setProxyUrl] = useState('')
  const [clearProxyUrl, setClearProxyUrl] = useState(false)
  const [proxyTestFeedback, setProxyTestFeedback] = useState<
    | { ok: true; latencyMs: number; target: string }
    | { ok: false; message: string }
    | null
  >(null)
  const [branch, setBranch] = useState('public')
  const [arch, setArch] = useState('64')
  const [autoUpdate, setAutoUpdate] = useState('manual')
  const [backupRetain, setBackupRetain] = useState(20)
  const [showTokenGuide, setShowTokenGuide] = useState(false)
  const [showReleaseNotes, setShowReleaseNotes] = useState(false)
  const [confirmUpgradeOpen, setConfirmUpgradeOpen] = useState(false)
  const [upgradePhase, setUpgradePhase] = useState<'idle' | 'pulling' | 'restarting' | 'reconnecting' | 'failed'>('idle')
  const [upgradeMessage, setUpgradeMessage] = useState('')
  const [upgradeError, setUpgradeError] = useState('')
  const upgradeStartVersionRef = useRef<string>('')
  const seenDisconnectRef = useRef<boolean>(false)
  const restartingAtRef = useRef<number>(0)

  const versionInfo = versionData?.info
  const runningClusterCount = useMemo(() => {
    const list = clustersData?.clusters ?? []
    return list.filter((c) => {
      const h = getClusterHealth(c.master_status, c.caves_enabled, c.caves_status)
      return h === 'running' || h === 'starting' || h === 'degraded'
    }).length
  }, [clustersData])

  // Sync backend-reported active upgrade state if user refreshes while pulling/restarting.
  useEffect(() => {
    const st = versionInfo?.upgrade?.state
    if (!st || st === 'idle') return
    if (st === 'pulling' && upgradePhase === 'idle') {
      upgradeStartVersionRef.current = versionInfo?.current_version ?? ''
      setUpgradePhase('pulling')
      setUpgradeMessage(versionInfo?.upgrade?.message || '')
      setUpgradeError('')
    } else if (st === 'restarting' && upgradePhase !== 'reconnecting') {
      upgradeStartVersionRef.current = versionInfo?.current_version ?? ''
      restartingAtRef.current = Date.now()
      setUpgradePhase('restarting')
      setUpgradeMessage(versionInfo?.upgrade?.message || '')
      setUpgradeError('')
    } else if (st === 'failed' && upgradePhase === 'pulling') {
      setUpgradePhase('failed')
      setUpgradeError(versionInfo?.upgrade?.error || versionInfo?.upgrade?.message || t('settings.upgradeFailed'))
    }
  }, [versionInfo, upgradePhase, t])

  // Poll upgrade progress during pulling -> restarting -> reconnecting -> reload.
  useEffect(() => {
    if (upgradePhase !== 'pulling' && upgradePhase !== 'restarting' && upgradePhase !== 'reconnecting') return
    let cancelled = false

    const timer = window.setInterval(async () => {
      if (cancelled) return

      if (upgradePhase === 'pulling') {
        try {
          const res = await api.systemVersion()
          if (cancelled) return
          qc.setQueryData(['system-version'], res)
          const up = res.info.upgrade
          if (up.state === 'pulling') {
            setUpgradeMessage(up.message || '')
          } else if (up.state === 'restarting') {
            restartingAtRef.current = Date.now()
            setUpgradePhase('restarting')
            setUpgradeMessage(up.message || '')
          } else if (up.state === 'failed') {
            setUpgradePhase('failed')
            setUpgradeError(up.error || up.message || t('settings.upgradeFailed'))
          }
        } catch {
          seenDisconnectRef.current = true
          restartingAtRef.current = Date.now()
          setUpgradePhase('reconnecting')
        }
        return
      }

      // restarting or reconnecting: probe /healthz until the new container responds
      try {
        const hz = await api.healthz()
        if (cancelled) return
        const versionChanged =
          Boolean(upgradeStartVersionRef.current) && Boolean(hz.version) && hz.version !== upgradeStartVersionRef.current
        const waitedEnough = restartingAtRef.current > 0 && Date.now() - restartingAtRef.current > 8000
        if (seenDisconnectRef.current || versionChanged || waitedEnough) {
          toast.success(t('settings.upgradeComplete'))
          setUpgradePhase('idle')
          window.setTimeout(() => {
            window.location.reload()
          }, 900)
        }
      } catch {
        seenDisconnectRef.current = true
        if (upgradePhase !== 'reconnecting') {
          setUpgradePhase('reconnecting')
        }
      }
    }, upgradePhase === 'pulling' ? 1500 : 2000)

    return () => {
      cancelled = true
      window.clearInterval(timer)
    }
  }, [upgradePhase, qc, t])

  const checkVersionMut = useMutation({
    mutationFn: api.checkSystemVersion,
    onSuccess: (res) => {
      qc.setQueryData(['system-version'], res)
      if (res.info.update_available) {
        toast.success(t('settings.checkUpdateDoneAvailable', { version: res.info.latest_version || 'latest' }))
      } else {
        toast.success(t('settings.checkUpdateDoneUpToDate'))
      }
    },
    onError: (err: Error) => {
      void refetchVersion()
      toast.error(err.message)
    },
  })

  const upgradeMut = useMutation({
    mutationFn: () => api.triggerSystemUpgrade(versionInfo?.docker?.target_image || undefined),
    onSuccess: (res) => {
      setConfirmUpgradeOpen(false)
      qc.setQueryData(['system-version'], res)
      upgradeStartVersionRef.current = res.info.current_version
      seenDisconnectRef.current = false
      restartingAtRef.current = 0
      setUpgradeError('')
      setUpgradeMessage(res.info.upgrade.message || '')
      setUpgradePhase('pulling')
    },
    onError: (err: Error) => {
      setConfirmUpgradeOpen(false)
      toast.error(err.message)
    },
  })

  const resetFromData = () => {
    if (!data?.settings) return
    setPortStart(data.settings.port_pool_start)
    setPortEnd(data.settings.port_pool_end)
    setBranch(data.settings.dst_branch)
    setArch(data.settings.dst_arch)
    setAutoUpdate(data.settings.auto_update_mode)
    setBackupRetain(data.settings.backup_max_retain)
    setDefaultToken('')
    setSteamKey('')
    setClearSteamKey(false)
    setProxyEnabled(Boolean(data.settings.proxy_enabled))
    setProxyUrl(data.settings.has_proxy_auth ? '' : data.settings.proxy_url || '')
    setClearProxyUrl(false)
    setProxyTestFeedback(null)
  }

  useEffect(() => {
    resetFromData()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data])

  const isDirty = useMemo(() => {
    if (!data?.settings) return false
    const s = data.settings
    const proxyUrlChanged = s.has_proxy_auth
      ? proxyUrl.trim().length > 0 || clearProxyUrl
      : proxyUrl.trim() !== (s.proxy_url || '')
    return (
      Number(portStart) !== s.port_pool_start ||
      Number(portEnd) !== s.port_pool_end ||
      branch !== s.dst_branch ||
      arch !== s.dst_arch ||
      autoUpdate !== s.auto_update_mode ||
      Number(backupRetain) !== s.backup_max_retain ||
      defaultToken.trim().length > 0 ||
      steamKey.trim().length > 0 ||
      clearSteamKey ||
      proxyEnabled !== Boolean(s.proxy_enabled) ||
      proxyUrlChanged
    )
  }, [
    data,
    portStart,
    portEnd,
    branch,
    arch,
    autoUpdate,
    backupRetain,
    defaultToken,
    steamKey,
    clearSteamKey,
    proxyEnabled,
    proxyUrl,
    clearProxyUrl,
  ])

  const startNum = Number(portStart)
  const endNum = Number(portEnd)
  const portPoolValid =
    Number.isInteger(startNum) &&
    Number.isInteger(endNum) &&
    startNum >= 1024 &&
    endNum <= 65535 &&
    endNum - startNum >= 6
  const portPoolCapacity = portPoolValid ? Math.floor((endNum - startNum + 1) / 7) : 0

  const proxySettings = data?.settings
  const proxySyntaxValid = isValidProxyUrl(proxyUrl)
  const hasEffectiveProxySource =
    proxyUrl.trim().length > 0 ||
    Boolean(proxySettings?.has_proxy_auth && !clearProxyUrl) ||
    Boolean(proxySettings?.has_default_proxy_url)
  const proxyMissingUrl = proxyEnabled && !hasEffectiveProxySource
  const proxyValid = proxySyntaxValid && !proxyMissingUrl
  const formValid = portPoolValid && proxyValid

  const testProxyMut = useMutation({
    mutationFn: () => api.testProxy(proxyUrl.trim() || undefined),
    onSuccess: (res) => {
      const r = res.result
      setProxyTestFeedback({ ok: true, latencyMs: r.latency_ms, target: r.target })
      toast.success(t('settings.proxyTestSuccess', { target: r.target, latency: r.latency_ms }))
    },
    onError: (err: Error) => {
      setProxyTestFeedback({ ok: false, message: err.message })
      toast.error(err.message)
    },
  })

  useRegisterUnsaved('system-settings', isDirty)

  const saveMut = useMutation({
    mutationFn: () => {
      const payload: Record<string, unknown> = {
        port_pool_start: Number(portStart),
        port_pool_end: Number(portEnd),
        dst_branch: branch,
        dst_arch: arch,
        auto_update_mode: autoUpdate,
        backup_max_retain: Number(backupRetain),
        proxy_enabled: proxyEnabled,
      }
      if (defaultToken.trim()) payload.default_cluster_token = defaultToken.trim()
      if (steamKey.trim()) payload.steam_api_key = steamKey.trim()
      else if (clearSteamKey) payload.steam_api_key = ''
      if (data?.settings.has_proxy_auth) {
        if (proxyUrl.trim()) payload.proxy_url = proxyUrl.trim()
        else if (clearProxyUrl) payload.proxy_url = ''
      } else if (proxyUrl.trim() !== (data?.settings.proxy_url || '')) {
        payload.proxy_url = proxyUrl.trim()
      }
      return api.updateSettings(payload)
    },
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setDefaultToken('')
      setSteamKey('')
      setClearSteamKey(false)
      setClearProxyUrl(false)
      void qc.invalidateQueries({ queryKey: ['system-settings'] })
      void qc.invalidateQueries({ queryKey: ['workshop-search'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        if (!isDirty || !formValid || saveMut.isPending) return
        e.preventDefault()
        saveMut.mutate()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [isDirty, formValid, saveMut])

  if (isError) {
    return <QueryErrorCard error={error} onRetry={() => void refetch()} />
  }

  if (isLoading || !data) {
    return <PageCardsSkeleton count={3} />
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!formValid) return
    saveMut.mutate()
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <PageHeader
        eyebrow={t('nav.system')}
        title={t('settings.title')}
        subtitle={t('settings.subtitle')}
        badge={isDirty ? <Badge variant="warning">{t('files.unsaved')}</Badge> : undefined}
      />

      {/* Runtime Info */}
      <Card>
        <CardHeader>
          <CardTitle>{t('settings.runtimeInfo')}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap items-center gap-3 text-xs">
          <Badge variant={data.runtime_mode === 'mock' ? 'warning' : 'success'}>
            {data.runtime_mode === 'mock' ? t('common.mockBadge') : t('common.linuxBadge')}
          </Badge>
          {versionInfo && (
            <Badge variant="secondary" className="font-mono">
              {versionInfo.current_version}
              {versionInfo.commit && versionInfo.commit !== 'none' ? ` · ${versionInfo.commit}` : ''}
            </Badge>
          )}
          <div className="flex items-start gap-1.5 min-w-0 text-muted-foreground">
            <span className="shrink-0 pt-0.5">{t('settings.dataDir')}:</span>
            <code className="font-mono text-foreground break-all pt-0.5">{data.data_dir}</code>
            <CopyButton value={data.data_dir} />
          </div>
        </CardContent>
      </Card>

      {/* Walrus Version & Docker One-Click Upgrade */}
      {versionInfo && (
        <Card id="version">
          <CardHeader className="flex flex-col sm:flex-row sm:items-start sm:justify-between gap-3">
            <div className="space-y-1 min-w-0">
              <CardTitle className="flex items-center gap-2 flex-wrap">
                <ArrowUpCircle className="h-4 w-4 text-primary shrink-0" />
                <span>{t('settings.versionSection')}</span>
                {versionInfo.update_available ? (
                  <Badge variant="warning">
                    {t('settings.statusUpdateAvailable', { version: versionInfo.latest_version || 'latest' })}
                  </Badge>
                ) : versionInfo.last_checked_at && !versionInfo.last_check_error ? (
                  <Badge variant="success">
                    <CheckCircle2 className="h-3 w-3" />
                    {t('settings.statusUpToDate')}
                  </Badge>
                ) : (
                  <Badge variant="secondary">{t('settings.statusNotChecked')}</Badge>
                )}
              </CardTitle>
              <CardDescription>{t('settings.versionSectionHint')}</CardDescription>
            </div>
            <div className="flex items-center gap-2 shrink-0 flex-wrap">
              <Button
                type="button"
                variant="outline"
                size="sm"
                disabled={
                  checkVersionMut.isPending ||
                  upgradePhase === 'pulling' ||
                  upgradePhase === 'restarting' ||
                  upgradePhase === 'reconnecting'
                }
                onClick={() => checkVersionMut.mutate()}
              >
                <RefreshCw className={cn('h-3.5 w-3.5', checkVersionMut.isPending && 'animate-spin')} />
                {checkVersionMut.isPending ? t('settings.checkingUpdate') : t('settings.checkUpdate')}
              </Button>
              {versionInfo.docker.can_self_upgrade && (
                <Button
                  type="button"
                  size="sm"
                  variant={versionInfo.update_available ? 'default' : 'outline'}
                  disabled={
                    upgradeMut.isPending ||
                    upgradePhase === 'pulling' ||
                    upgradePhase === 'restarting' ||
                    upgradePhase === 'reconnecting'
                  }
                  onClick={() => setConfirmUpgradeOpen(true)}
                >
                  {upgradePhase === 'pulling' || upgradePhase === 'restarting' || upgradePhase === 'reconnecting' ? (
                    <Loader2 className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <ArrowUpCircle className="h-3.5 w-3.5" />
                  )}
                  {upgradePhase === 'pulling' || upgradePhase === 'restarting' || upgradePhase === 'reconnecting'
                    ? t('settings.upgrading')
                    : versionInfo.update_available
                      ? t('settings.upgradeNow')
                      : t('settings.reinstallLatest')}
                </Button>
              )}
            </div>
          </CardHeader>

          <CardContent className="space-y-4 text-xs">
            {/* Version Metadata Grid */}
            <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-4 gap-3 rounded-lg border border-border bg-secondary/25 p-3">
              <div className="space-y-0.5 min-w-0">
                <div className="text-muted-foreground">{t('settings.currentVersion')}</div>
                <div className="font-mono font-medium text-foreground truncate">
                  {versionInfo.current_version}
                  {versionInfo.commit && versionInfo.commit !== 'none' ? ` (${versionInfo.commit})` : ''}
                </div>
              </div>
              <div className="space-y-0.5 min-w-0">
                <div className="text-muted-foreground">{t('settings.latestVersion')}</div>
                <div className="flex items-center gap-1.5 min-w-0">
                  <span className="font-mono font-medium text-foreground truncate">
                    {versionInfo.latest_version || '—'}
                  </span>
                  {versionInfo.source && versionInfo.source !== 'none' && (
                    <Badge variant="muted" className="text-[10.5px] px-1.5 py-0">
                      {t(`settings.sourceLabels.${versionInfo.source}`, versionInfo.source)}
                    </Badge>
                  )}
                </div>
              </div>
              <div className="space-y-0.5 min-w-0">
                <div className="text-muted-foreground">{t('settings.buildTime')}</div>
                <div className="font-mono text-foreground truncate">
                  {versionInfo.build_time && versionInfo.build_time !== 'unknown'
                    ? formatDateTime(versionInfo.build_time)
                    : '—'}
                </div>
              </div>
              <div className="space-y-0.5 min-w-0">
                <div className="text-muted-foreground">{t('settings.lastChecked')}</div>
                <div className="font-mono text-foreground truncate">
                  {versionInfo.last_checked_at ? formatDateTime(versionInfo.last_checked_at) : '—'}
                </div>
              </div>
            </div>

            {/* Current / Target Docker Image Info */}
            {versionInfo.docker.in_docker && (
              <div className="flex flex-wrap items-center gap-x-5 gap-y-2 text-muted-foreground">
                {versionInfo.docker.current_image && (
                  <div className="flex items-center gap-1.5 min-w-0">
                    <span className="shrink-0">{t('settings.currentImage')}:</span>
                    <code className="font-mono text-foreground break-all">{versionInfo.docker.current_image}</code>
                  </div>
                )}
                {versionInfo.docker.target_image && (
                  <div className="flex items-center gap-1.5 min-w-0">
                    <span className="shrink-0">{t('settings.targetImage')}:</span>
                    <code className="font-mono text-foreground break-all">{versionInfo.docker.target_image}</code>
                  </div>
                )}
                {versionInfo.docker.can_self_upgrade && (
                  <Badge variant="success" className="text-[11px]">
                    {t('settings.dockerUpgradeReady')}
                  </Badge>
                )}
              </div>
            )}

            {/* Release Link & Collapsible Release Notes */}
            {(versionInfo.release_url || versionInfo.release_notes) && (
              <div className="space-y-1.5">
                <div className="flex items-center gap-3 flex-wrap">
                  {versionInfo.release_notes && (
                    <button
                      type="button"
                      aria-expanded={showReleaseNotes}
                      onClick={() => setShowReleaseNotes((v) => !v)}
                      className="inline-flex items-center gap-1 rounded text-xs font-medium text-primary hover:underline cursor-pointer"
                    >
                      <span>{t('settings.releaseNotesToggle')}</span>
                      <ChevronDown
                        className={cn(
                          'h-3.5 w-3.5 transition-transform duration-200 ease-out',
                          showReleaseNotes && 'rotate-180',
                        )}
                      />
                    </button>
                  )}
                  {versionInfo.release_url && (
                    <a
                      href={versionInfo.release_url}
                      target="_blank"
                      rel="noopener noreferrer"
                      className="inline-flex items-center gap-1 text-xs font-medium text-muted-foreground hover:text-foreground hover:underline"
                    >
                      <span>{t('settings.viewRelease')}</span>
                      <ExternalLink className="h-3 w-3" />
                    </a>
                  )}
                </div>
                {versionInfo.release_notes && (
                  <CollapsibleContent open={showReleaseNotes}>
                    <pre className="mt-1.5 max-h-56 overflow-auto whitespace-pre-wrap rounded-lg border border-border bg-secondary/35 p-3 font-mono text-[11.5px] leading-relaxed text-foreground">
                      {versionInfo.release_notes}
                    </pre>
                  </CollapsibleContent>
                )}
              </div>
            )}

            {/* Live Upgrade Progress / Failure Status */}
            {(upgradePhase === 'pulling' || upgradePhase === 'restarting' || upgradePhase === 'reconnecting') && (
              <div
                role="status"
                aria-live="polite"
                className="flex items-start gap-2.5 rounded-lg border border-amber-600/30 bg-amber-500/10 px-3.5 py-2.5 text-amber-950 dark:border-amber-500/35 dark:text-amber-200"
              >
                <Loader2 className="h-4 w-4 shrink-0 mt-0.5 animate-spin text-amber-700 dark:text-amber-400" />
                <div className="space-y-0.5 min-w-0">
                  <div className="font-medium">
                    {upgradePhase === 'pulling'
                      ? t('settings.pullingImage')
                      : upgradePhase === 'restarting'
                        ? t('settings.restartingContainer')
                        : t('settings.reconnecting')}
                  </div>
                  {upgradeMessage && (
                    <div className="font-mono text-[11px] opacity-85 break-all">{upgradeMessage}</div>
                  )}
                </div>
              </div>
            )}

            {upgradePhase === 'failed' && (
              <div
                role="alert"
                className="flex items-start gap-2.5 rounded-lg border border-red-600/30 bg-red-600/10 px-3.5 py-2.5 text-red-900 dark:border-red-500/35 dark:text-red-200"
              >
                <AlertTriangle className="h-4 w-4 shrink-0 mt-0.5 text-red-700 dark:text-red-400" />
                <div className="space-y-0.5 min-w-0">
                  <div className="font-medium">{t('settings.upgradeFailed')}</div>
                  {upgradeError && <div className="font-mono text-[11px] break-all">{upgradeError}</div>}
                </div>
              </div>
            )}

            {/* Graceful Fallback when Docker Socket is not mounted or not in Docker */}
            {!versionInfo.docker.can_self_upgrade && (
              <div className="space-y-2 rounded-lg border border-border bg-secondary/35 p-3">
                <div className="flex items-start gap-2 text-muted-foreground leading-relaxed">
                  <Terminal className="h-3.5 w-3.5 shrink-0 mt-0.5 text-foreground" />
                  <span>
                    {versionInfo.docker.unavailable_reason === 'socket_not_mounted'
                      ? `${t('settings.dockerSocketMissing')} ${t('settings.dockerSocketGuide')}`
                      : t('settings.dockerNotInContainer')}
                  </span>
                </div>
                <div className="flex items-center justify-between gap-2 rounded-md border border-border bg-background px-2.5 py-1.5">
                  <code className="font-mono text-[11.5px] text-foreground break-all">{MANUAL_UPGRADE_CMD}</code>
                  <CopyButton value={MANUAL_UPGRADE_CMD} />
                </div>
              </div>
            )}
          </CardContent>
        </Card>
      )}

      {/* Port Pool */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Network className="h-4 w-4 text-primary" />
            {t('settings.portPool')}
          </CardTitle>
          <CardDescription>{t('settings.portPoolHint')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label htmlFor="set-pstart">{t('settings.portStart')}</Label>
              <Input
                id="set-pstart"
                type="number"
                min={1024}
                max={65000}
                aria-invalid={!portPoolValid}
                className={cn(!portPoolValid && 'border-red-600 focus-visible:ring-red-600')}
                value={portStart}
                onChange={(e) => setPortStart(Number(e.target.value))}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="set-pend">{t('settings.portEnd')}</Label>
              <Input
                id="set-pend"
                type="number"
                min={1030}
                max={65535}
                aria-invalid={!portPoolValid}
                className={cn(!portPoolValid && 'border-red-600 focus-visible:ring-red-600')}
                value={portEnd}
                onChange={(e) => setPortEnd(Number(e.target.value))}
              />
            </div>
          </div>
          {!portPoolValid ? (
            <p role="alert" className="text-xs font-medium text-red-700 dark:text-red-400">
              {t('settings.portPoolInvalid')}
            </p>
          ) : (
            <p className="text-xs text-muted-foreground tabular-nums">
              {t('settings.portPoolCapacity', { count: portPoolCapacity })}
            </p>
          )}
        </CardContent>
      </Card>

      {/* Tokens */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <KeyRound className="h-4 w-4 text-primary" />
            {t('settings.tokensSection')}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1.5">
            <div className="flex items-center justify-between gap-2 flex-wrap">
              <Label htmlFor="set-token">{t('settings.defaultToken')}</Label>
              {data.settings.has_default_token && (
                <Badge variant="success" className="font-mono">
                  {data.settings.default_cluster_token}
                </Badge>
              )}
            </div>
            <SecretInput
              id="set-token"
              placeholder={
                data.settings.has_default_token ? t('settings.secretPlaceholder') : t('settings.tokenPlaceholderUnset')
              }
              value={defaultToken}
              onChange={(e) => setDefaultToken(e.target.value)}
            />
            <div>
              <button
                type="button"
                aria-expanded={showTokenGuide}
                onClick={() => setShowTokenGuide((v) => !v)}
                className="inline-flex items-center gap-1 rounded text-xs font-medium text-primary hover:underline transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer"
              >
                <HelpCircle className="h-3.5 w-3.5" />
                <span>{t('auth.tokenGuideToggle')}</span>
                <ChevronDown className={`h-3.5 w-3.5 transition-transform duration-200 ease-out ${showTokenGuide ? 'rotate-180' : ''}`} />
              </button>
              <CollapsibleContent open={showTokenGuide}>
                <div className="pt-1.5">
                  <div className="rounded-lg bg-secondary/45 px-3 py-2 text-xs text-muted-foreground leading-relaxed">
                    {t('auth.tokenGuideSteps')}
                  </div>
                </div>
              </CollapsibleContent>
            </div>
          </div>
          <div className="space-y-1.5">
            <div className="flex items-center justify-between gap-2 flex-wrap">
              <Label htmlFor="set-steamkey">{t('settings.steamApiKey')}</Label>
              {data.settings.has_steam_api_key && !clearSteamKey ? (
                <div className="flex items-center gap-2">
                  <Badge variant="success" className="font-mono">
                    {data.settings.steam_api_key}
                  </Badge>
                  <button
                    type="button"
                    onClick={() => {
                      setClearSteamKey(true)
                      setSteamKey('')
                    }}
                    className="text-xs font-medium text-muted-foreground hover:text-foreground underline underline-offset-4 cursor-pointer"
                  >
                    {data.settings.has_default_steam_api_key
                      ? t('settings.clearSteamKeyToDefault')
                      : t('settings.clearSteamKey')}
                  </button>
                </div>
              ) : data.settings.has_default_steam_api_key ? (
                <Badge variant="info">{t('settings.steamKeyUsingDefault')}</Badge>
              ) : null}
            </div>
            <SecretInput
              id="set-steamkey"
              placeholder={
                data.settings.has_steam_api_key && !clearSteamKey
                  ? t('settings.secretPlaceholder')
                  : data.settings.has_default_steam_api_key
                    ? t('settings.steamKeyPlaceholderUsingDefault')
                    : t('settings.steamKeyPlaceholderUnset')
              }
              value={steamKey}
              onChange={(e) => {
                setSteamKey(e.target.value)
                if (e.target.value.trim()) setClearSteamKey(false)
              }}
            />
          </div>
        </CardContent>
      </Card>

      {/* Network Proxy */}
      <Card id="proxy">
        <CardHeader>
          <CardTitle className="flex items-center gap-2 flex-wrap">
            <Globe className="h-4 w-4 text-primary shrink-0" />
            <span>{t('settings.proxySection')}</span>
            <Badge variant={proxyEnabled ? 'success' : 'secondary'}>
              {proxyEnabled ? t('settings.proxyEnabledBadge') : t('settings.proxyDisabledBadge')}
            </Badge>
            {data.settings.has_default_proxy_url && !data.settings.has_proxy_url && !proxyUrl.trim() && (
              <Badge variant="info">{t('settings.proxyUsingDefaultBadge')}</Badge>
            )}
          </CardTitle>
          <CardDescription>{t('settings.proxySectionHint')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="flex items-center justify-between gap-3 rounded-lg border border-border bg-secondary/25 px-3.5 py-2.5">
            <Label htmlFor="set-proxy-enabled" className="cursor-pointer font-medium">
              {t('settings.proxyEnabled')}
            </Label>
            <Switch
              id="set-proxy-enabled"
              checked={proxyEnabled}
              onCheckedChange={(checked) => {
                setProxyEnabled(checked)
                setProxyTestFeedback(null)
              }}
            />
          </div>

          <div className="space-y-1.5">
            <div className="flex items-center justify-between gap-2 flex-wrap">
              <Label htmlFor="set-proxy-url">{t('settings.proxyUrlLabel')}</Label>
              {data.settings.has_proxy_auth && !clearProxyUrl ? (
                <div className="flex items-center gap-2">
                  <Badge variant="success" className="font-mono">
                    {data.settings.proxy_url}
                  </Badge>
                  <button
                    type="button"
                    onClick={() => {
                      setClearProxyUrl(true)
                      setProxyUrl('')
                      setProxyTestFeedback(null)
                    }}
                    className="text-xs font-medium text-muted-foreground hover:text-foreground underline underline-offset-4 cursor-pointer"
                  >
                    {data.settings.has_default_proxy_url
                      ? t('settings.proxyClearToDefault')
                      : t('settings.proxyClear')}
                  </button>
                </div>
              ) : null}
            </div>

            <div className="flex flex-col sm:flex-row gap-2">
              <Input
                id="set-proxy-url"
                type="text"
                autoComplete="off"
                spellCheck={false}
                aria-invalid={!proxyValid}
                className={cn(
                  'font-mono text-xs sm:flex-1',
                  !proxyValid && 'border-red-600 focus-visible:ring-red-600',
                )}
                placeholder={
                  data.settings.has_proxy_auth && !clearProxyUrl
                    ? t('settings.proxyUrlPlaceholderAuthSaved')
                    : data.settings.has_default_proxy_url
                      ? t('settings.proxyUrlPlaceholderUsingDefault', {
                          url: data.settings.default_proxy_url || 'WALRUS_PROXY_URL',
                        })
                      : t('settings.proxyUrlPlaceholder')
                }
                value={proxyUrl}
                onChange={(e) => {
                  setProxyUrl(e.target.value)
                  if (e.target.value.trim()) setClearProxyUrl(false)
                  setProxyTestFeedback(null)
                }}
              />
              <Button
                type="button"
                variant="outline"
                size="sm"
                className="shrink-0 sm:h-9"
                disabled={testProxyMut.isPending || !proxySyntaxValid || !hasEffectiveProxySource}
                onClick={() => testProxyMut.mutate()}
              >
                {testProxyMut.isPending ? (
                  <Loader2 className="h-3.5 w-3.5 animate-spin" />
                ) : (
                  <PlugZap className="h-3.5 w-3.5" />
                )}
                {testProxyMut.isPending ? t('settings.proxyTesting') : t('settings.proxyTestBtn')}
              </Button>
            </div>

            {!proxySyntaxValid ? (
              <p role="alert" className="text-xs font-medium text-red-700 dark:text-red-400">
                {t('settings.proxyUrlInvalid')}
              </p>
            ) : proxyMissingUrl ? (
              <p role="alert" className="text-xs font-medium text-red-700 dark:text-red-400">
                {t('settings.proxyUrlRequired')}
              </p>
            ) : (
              <p className="text-xs text-muted-foreground leading-relaxed">{t('settings.proxyUrlHint')}</p>
            )}

            {proxyTestFeedback && (
              <div
                role={proxyTestFeedback.ok ? 'status' : 'alert'}
                className={cn(
                  'flex items-start gap-2 rounded-lg border px-3 py-2 text-xs',
                  proxyTestFeedback.ok
                    ? 'border-emerald-600/30 bg-emerald-500/10 text-emerald-950 dark:border-emerald-500/35 dark:text-emerald-200'
                    : 'border-red-600/30 bg-red-600/10 text-red-900 dark:border-red-500/35 dark:text-red-200',
                )}
              >
                {proxyTestFeedback.ok ? (
                  <CheckCircle2 className="h-3.5 w-3.5 shrink-0 mt-0.5 text-emerald-700 dark:text-emerald-400" />
                ) : (
                  <AlertTriangle className="h-3.5 w-3.5 shrink-0 mt-0.5 text-red-700 dark:text-red-400" />
                )}
                <div>
                  {proxyTestFeedback.ok
                    ? t('settings.proxyTestSuccess', {
                        target: proxyTestFeedback.target,
                        latency: proxyTestFeedback.latencyMs,
                      })
                    : proxyTestFeedback.message}
                </div>
              </div>
            )}
          </div>
        </CardContent>
      </Card>

      {/* DST Server Params */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ServerCog className="h-4 w-4 text-primary" />
            {t('settings.dstSection')}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <Label htmlFor="set-branch">{t('settings.branch')}</Label>
            <Select id="set-branch" value={branch} onChange={(e) => setBranch(e.target.value)}>
              <option value="public">{t('settings.branches.public')}</option>
              <option value="updatebeta">{t('settings.branches.updatebeta')}</option>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="set-arch">{t('settings.arch')}</Label>
            <Select id="set-arch" value={arch} onChange={(e) => setArch(e.target.value)}>
              <option value="64">{t('settings.arches.64')}</option>
              <option value="32">{t('settings.arches.32')}</option>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="set-autoupd">{t('settings.autoUpdate')}</Label>
            <Select id="set-autoupd" value={autoUpdate} onChange={(e) => setAutoUpdate(e.target.value)}>
              <option value="manual">{t('settings.autoUpdateModes.manual')}</option>
              <option value="when_empty">{t('settings.autoUpdateModes.when_empty')}</option>
              <option value="scheduled">{t('settings.autoUpdateModes.scheduled')}</option>
            </Select>
            <p className="text-xs text-muted-foreground leading-relaxed">{t('settings.autoUpdateHint')}</p>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="set-retain">{t('settings.backupRetain')}</Label>
            <Input
              id="set-retain"
              type="number"
              min={1}
              max={200}
              value={backupRetain}
              onChange={(e) => setBackupRetain(Number(e.target.value))}
            />
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-end gap-2">
        {isDirty && (
          <Button type="button" variant="outline" disabled={saveMut.isPending} onClick={resetFromData}>
            {t('common.discard')}
          </Button>
        )}
        <Button type="submit" disabled={saveMut.isPending || !isDirty || !formValid}>
          <Save className="h-4 w-4" />
          {saveMut.isPending ? t('common.saving') : t('common.save')}
        </Button>
      </div>

      <StickySaveBar
        visible={isDirty}
        saving={saveMut.isPending}
        disabled={!formValid}
        onDiscard={resetFromData}
        onSave={() => {
          if (formValid) saveMut.mutate()
        }}
      />

      <ConfirmDialog
        open={confirmUpgradeOpen}
        onClose={() => setConfirmUpgradeOpen(false)}
        onConfirm={() => upgradeMut.mutate()}
        loading={upgradeMut.isPending}
        tone="warning"
        title={t('settings.upgradeConfirmTitle')}
        description={
          runningClusterCount > 0
            ? `${t('settings.upgradeConfirmRunningWarn', { count: runningClusterCount })}\n\n${t('settings.upgradeConfirmDesc', { image: versionInfo?.docker?.target_image || 'ghcr.io/keyborad-man/walrus:latest' })}`
            : t('settings.upgradeConfirmDesc', {
                image: versionInfo?.docker?.target_image || 'ghcr.io/keyborad-man/walrus:latest',
              })
        }
        confirmLabel={t('settings.upgradeNow')}
      />
    </form>
  )
}
