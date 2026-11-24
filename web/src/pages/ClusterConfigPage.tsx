import { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { AlertCircle, AlertTriangle, Info, Mountain, Network, RotateCw, Save, Settings2, Trees } from 'lucide-react'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { useRegisterUnsaved } from '@/hooks/useUnsavedGuard'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input, Label, SecretInput, Select } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { Checkbox } from '@/components/ui/checkbox'
import { Badge } from '@/components/ui/badge'
import { TabList } from '@/components/ui/tabs'
import { StickySaveBar } from '@/components/StickySaveBar'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { WorldVisualEditor } from '@/components/WorldVisualEditor'
import { PageHeader } from '@/components/PageHeader'
import { PageCardsSkeleton } from '@/components/ui/skeleton'
import type { ClusterINI, ServerINI } from '@/types/api'

type TabKey = 'room' | 'ports' | 'master' | 'caves'

export function ClusterConfigPage() {
  const { id = '' } = useParams<{ id: string }>()
  const { t } = useTranslation()
  const qc = useQueryClient()

  const [tab, setTab] = useState<TabKey>('room')
  const [pendingTab, setPendingTab] = useState<TabKey | null>(null)
  const [worldDirty, setWorldDirty] = useState(false)
  const [savedNeedRestart, setSavedNeedRestart] = useState(false)

  const [clusterINI, setClusterINI] = useState<ClusterINI | null>(null)
  const [masterINI, setMasterINI] = useState<ServerINI | null>(null)
  const [cavesINI, setCavesINI] = useState<ServerINI | null>(null)
  const [cavesEnabled, setCavesEnabled] = useState(true)
  const [autoRestart, setAutoRestart] = useState(true)
  const [modSync, setModSync] = useState(true)
  const [newToken, setNewToken] = useState('')
  const [clearToken, setClearToken] = useState(false)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['cluster', id],
    queryFn: () => api.getCluster(id),
    enabled: !!id,
  })

  const { data: presetsData } = useQuery({
    queryKey: ['presets'],
    queryFn: api.presets,
  })

  const resetFormFromData = () => {
    if (!data?.cluster) return
    setClusterINI(data.cluster.cluster_ini)
    setMasterINI(data.cluster.master_server_ini)
    setCavesINI(data.cluster.caves_server_ini ?? null)
    setCavesEnabled(data.cluster.meta.caves_enabled)
    setAutoRestart(data.cluster.meta.auto_restart)
    setModSync(data.cluster.meta.mod_sync_enabled)
    setNewToken('')
    setClearToken(false)
  }

  useEffect(() => {
    resetFormFromData()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data])

  const canOperate = data?.effective_role === 'admin' || data?.effective_role === 'operator'

  const isRoomPortsDirty = useMemo(() => {
    if (!data?.cluster || !clusterINI || !masterINI) return false
    const c = data.cluster
    if (newToken.trim().length > 0 || clearToken) return true
    if (cavesEnabled !== c.meta.caves_enabled) return true
    if (autoRestart !== c.meta.auto_restart) return true
    if (modSync !== c.meta.mod_sync_enabled) return true
    if (JSON.stringify(clusterINI) !== JSON.stringify(c.cluster_ini)) return true
    if (JSON.stringify(masterINI) !== JSON.stringify(c.master_server_ini)) return true
    if (JSON.stringify(cavesINI ?? null) !== JSON.stringify(c.caves_server_ini ?? null)) return true
    return false
  }, [data, clusterINI, masterINI, cavesINI, cavesEnabled, autoRestart, modSync, newToken, clearToken])

  useRegisterUnsaved('cluster-config', isRoomPortsDirty)

  const saveConfigMut = useMutation({
    mutationFn: () => {
      const payload: Parameters<typeof api.updateClusterConfig>[1] = {
        cluster_ini: clusterINI ?? undefined,
        master_server_ini: masterINI ?? undefined,
        caves_server_ini: cavesEnabled && cavesINI ? cavesINI : undefined,
        caves_enabled: cavesEnabled,
        auto_restart: autoRestart,
        mod_sync_enabled: modSync,
      }
      if (clearToken) {
        payload.cluster_token = ''
      } else if (newToken.trim()) {
        payload.cluster_token = newToken.trim()
      }
      return api.updateClusterConfig(id, payload)
    },
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setNewToken('')
      setClearToken(false)
      setSavedNeedRestart(true)
      void qc.invalidateQueries({ queryKey: ['cluster', id] })
      void qc.invalidateQueries({ queryKey: ['clusters'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const portValidationError = useMemo(() => {
    if (!clusterINI || !masterINI) return null
    const list = [
      Number(clusterINI.master_port),
      Number(masterINI.server_port),
      Number(masterINI.authentication_port),
      Number(masterINI.master_server_port),
      ...(cavesEnabled && cavesINI
        ? [
            Number(cavesINI.server_port),
            Number(cavesINI.authentication_port),
            Number(cavesINI.master_server_port),
          ]
        : []),
    ]
    if (list.some((p) => !Number.isInteger(p) || p < 1024 || p > 65535)) {
      return t('config.portInvalidRange')
    }
    if (new Set(list).size !== list.length) {
      return t('config.portConflict')
    }
    return null
  }, [clusterINI, masterINI, cavesEnabled, cavesINI, t])

  // Ctrl/Cmd + S shortcut on Room & Ports tabs
  useEffect(() => {
    if (tab !== 'room' && tab !== 'ports') return
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        if (!canOperate || !isRoomPortsDirty || !!portValidationError || saveConfigMut.isPending) return
        e.preventDefault()
        saveConfigMut.mutate()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [tab, canOperate, isRoomPortsDirty, portValidationError, saveConfigMut])

  const restartMut = useMutation({
    mutationFn: () => api.restartCluster(id),
    onSuccess: () => {
      toast.success(t('common.actionSuccess'))
      setSavedNeedRestart(false)
      void qc.invalidateQueries({ queryKey: ['cluster', id] })
      void qc.invalidateQueries({ queryKey: ['clusters'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  if (isError) {
    return <QueryErrorCard error={error} onRetry={() => void refetch()} />
  }

  if (isLoading || !clusterINI || !masterINI || !data?.cluster) {
    return <PageCardsSkeleton count={2} />
  }

  const isClusterRunning =
    data.cluster.master_status.state === 'running' ||
    (data.cluster.meta.caves_enabled && data.cluster.caves_status?.state === 'running')

  const masterPresets = (presetsData?.presets ?? []).filter((p) => p.shard === 'master')
  const cavesPresets = (presetsData?.presets ?? []).filter((p) => p.shard === 'caves')

  const tabs: { key: TabKey; label: string; icon: React.ReactNode; hidden?: boolean }[] = [
    { key: 'room', label: t('config.tabRoom'), icon: <Settings2 className="h-3.5 w-3.5" /> },
    { key: 'ports', label: t('config.tabPorts'), icon: <Network className="h-3.5 w-3.5" /> },
    { key: 'master', label: t('config.tabMasterWorld'), icon: <Trees className="h-3.5 w-3.5" /> },
    { key: 'caves', label: t('config.tabCavesWorld'), icon: <Mountain className="h-3.5 w-3.5" />, hidden: !cavesEnabled },
  ]

  const requestTabChange = (next: TabKey) => {
    if (next === tab) return
    const leavingWorld = tab === 'master' || tab === 'caves'
    const enteringWorld = next === 'master' || next === 'caves'
    if (leavingWorld && worldDirty) {
      setPendingTab(next)
      return
    }
    if (!leavingWorld && enteringWorld && isRoomPortsDirty) {
      setPendingTab(next)
      return
    }
    setTab(next)
  }

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={t('nav.groupCare')}
        title={t('config.title')}
        subtitle={
          <span className="inline-flex items-center gap-1.5 text-[13px]">
            <AlertCircle className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
            <span>{t('config.restartNotice')}</span>
          </span>
        }
      />

      {savedNeedRestart && isClusterRunning && canOperate && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-card px-3.5 py-2.5 text-[13px]">
          <div className="flex items-center gap-2.5 min-w-0">
            <span className="h-[7px] w-[7px] rounded-full bg-amber-500 shrink-0" aria-hidden="true" />
            <span className="text-foreground/90">{t('config.savedRestartPrompt')}</span>
          </div>
          <Button size="sm" variant="warning" disabled={restartMut.isPending} onClick={() => restartMut.mutate()}>
            <RotateCw className={cn('h-3.5 w-3.5', restartMut.isPending && 'animate-spin')} />
            {t('config.restartNow')}
          </Button>
        </div>
      )}

      {saveConfigMut.error && (tab === 'room' || tab === 'ports') && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-red-600/35 bg-red-600/10 px-4 py-3 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
        >
          <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
          <span className="break-words">{saveConfigMut.error.message}</span>
        </div>
      )}

      {/* Tab Bar */}
      <TabList
        ariaLabel={t('config.title')}
        idPrefix="cluster-config"
        value={tab}
        onChange={requestTabChange}
        items={tabs}
      />

      {/* Room Tab (Flattened sections — no nested cards) */}
      {tab === 'room' && (
        <div id="cluster-config-panel-room" role="tabpanel" aria-labelledby="cluster-config-tab-room" className="space-y-4">
          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2">
                <span>{t('config.tabRoom')}</span>
                {isRoomPortsDirty && <Badge variant="warning">{t('files.unsaved')}</Badge>}
              </CardTitle>
              <CardDescription>cluster.ini &amp; cluster_token.txt</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <Label htmlFor="cfg-name">{t('createCluster.clusterName')}</Label>
                  <Input
                    id="cfg-name"
                    disabled={!canOperate}
                    value={clusterINI.cluster_name}
                    onChange={(e) => setClusterINI({ ...clusterINI, cluster_name: e.target.value })}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="cfg-pwd">{t('createCluster.password')}</Label>
                  <Input
                    id="cfg-pwd"
                    disabled={!canOperate}
                    value={clusterINI.cluster_password}
                    onChange={(e) => setClusterINI({ ...clusterINI, cluster_password: e.target.value })}
                  />
                </div>
                <div className="space-y-1.5 sm:col-span-2">
                  <Label htmlFor="cfg-desc">{t('createCluster.description')}</Label>
                  <Input
                    id="cfg-desc"
                    disabled={!canOperate}
                    value={clusterINI.cluster_description}
                    onChange={(e) => setClusterINI({ ...clusterINI, cluster_description: e.target.value })}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="cfg-mode">{t('createCluster.gameMode')}</Label>
                  <Select
                    id="cfg-mode"
                    disabled={!canOperate}
                    value={clusterINI.game_mode}
                    onChange={(e) => setClusterINI({ ...clusterINI, game_mode: e.target.value })}
                  >
                    <option value="endless">{t('dashboard.modes.endless')}</option>
                    <option value="survival">{t('dashboard.modes.survival')}</option>
                    <option value="relaxed">{t('dashboard.modes.relaxed')}</option>
                    <option value="wilderness">{t('dashboard.modes.wilderness')}</option>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="cfg-intention">{t('createCluster.intention')}</Label>
                  <Select
                    id="cfg-intention"
                    disabled={!canOperate}
                    value={clusterINI.cluster_intention}
                    onChange={(e) => setClusterINI({ ...clusterINI, cluster_intention: e.target.value })}
                  >
                    <option value="cooperative">{t('config.intentions.cooperative')}</option>
                    <option value="social">{t('config.intentions.social')}</option>
                    <option value="competitive">{t('config.intentions.competitive')}</option>
                    <option value="madness">{t('config.intentions.madness')}</option>
                  </Select>
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="cfg-max">{t('createCluster.maxPlayers')}</Label>
                  <Input
                    id="cfg-max"
                    type="number"
                    min={1}
                    max={64}
                    disabled={!canOperate}
                    value={clusterINI.max_players}
                    onChange={(e) => setClusterINI({ ...clusterINI, max_players: Number(e.target.value) })}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="cfg-snaps">{t('config.maxSnapshots')}</Label>
                  <Input
                    id="cfg-snaps"
                    type="number"
                    min={1}
                    max={50}
                    disabled={!canOperate}
                    value={clusterINI.max_snapshots}
                    onChange={(e) => setClusterINI({ ...clusterINI, max_snapshots: Number(e.target.value) })}
                  />
                  <p className="text-xs text-muted-foreground">{t('config.maxSnapshotsHint')}</p>
                </div>
              </div>

              {/* Runtime Switches Section (Flat divider, no nested card) */}
              <div className="border-t border-border pt-5 space-y-3">
                <h3 className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                  {t('config.sectionSwitches')}
                </h3>
                <div className="grid grid-cols-1 sm:grid-cols-2 gap-x-8 gap-y-3">
                  <label htmlFor="cfg-sw-caves" className="flex items-center justify-between gap-3 py-1 cursor-pointer">
                    <span className="text-sm">{t('config.cavesToggle')}</span>
                    <Switch
                      id="cfg-sw-caves"
                      disabled={!canOperate}
                      checked={cavesEnabled}
                      onCheckedChange={setCavesEnabled}
                      ariaLabel={t('config.cavesToggle')}
                    />
                  </label>
                  <label htmlFor="cfg-sw-pause" className="flex items-center justify-between gap-3 py-1 cursor-pointer">
                    <span className="text-sm">{t('createCluster.pauseWhenEmpty')}</span>
                    <Switch
                      id="cfg-sw-pause"
                      disabled={!canOperate}
                      checked={clusterINI.pause_when_empty}
                      onCheckedChange={(v) => setClusterINI({ ...clusterINI, pause_when_empty: v })}
                      ariaLabel={t('createCluster.pauseWhenEmpty')}
                    />
                  </label>
                  <label htmlFor="cfg-sw-autorestart" className="flex items-center justify-between gap-3 py-1 cursor-pointer">
                    <span className="text-sm">{t('config.autoRestart')}</span>
                    <Switch
                      id="cfg-sw-autorestart"
                      disabled={!canOperate}
                      checked={autoRestart}
                      onCheckedChange={setAutoRestart}
                      ariaLabel={t('config.autoRestart')}
                    />
                  </label>
                  <label htmlFor="cfg-sw-pvp" className="flex items-center justify-between gap-3 py-1 cursor-pointer">
                    <span className="text-sm">{t('createCluster.pvp')}</span>
                    <Switch
                      id="cfg-sw-pvp"
                      disabled={!canOperate}
                      checked={clusterINI.pvp}
                      onCheckedChange={(v) => setClusterINI({ ...clusterINI, pvp: v })}
                      ariaLabel={t('createCluster.pvp')}
                    />
                  </label>
                  <label htmlFor="cfg-sw-modsync" className="flex items-center justify-between gap-3 py-1 cursor-pointer">
                    <span className="text-sm">{t('config.modSync')}</span>
                    <Switch
                      id="cfg-sw-modsync"
                      disabled={!canOperate}
                      checked={modSync}
                      onCheckedChange={setModSync}
                      ariaLabel={t('config.modSync')}
                    />
                  </label>
                </div>
              </div>

              {/* Cluster Token Override Section (Flat divider, no nested card) */}
              <div className="border-t border-border pt-5 space-y-2.5">
                <div className="flex items-center justify-between gap-2 flex-wrap">
                  <Label htmlFor="cfg-token">{t('config.tokenLabel')}</Label>
                  <Badge variant={data.cluster.has_cluster_token ? 'success' : 'warning'}>
                    {data.cluster.using_default_token
                      ? t('config.tokenUsingDefault')
                      : data.cluster.has_cluster_token
                        ? `${t('config.tokenCustom')}: ${data.cluster.masked_cluster_token}`
                        : t('dashboard.tokenMissing')}
                  </Badge>
                </div>
                <SecretInput
                  id="cfg-token"
                  disabled={!canOperate || clearToken}
                  placeholder={t('config.tokenPlaceholder')}
                  value={newToken}
                  onChange={(e) => setNewToken(e.target.value)}
                />
                {!data.cluster.using_default_token && data.cluster.has_cluster_token && canOperate && (
                  <label className="inline-flex items-center gap-2 text-xs text-muted-foreground cursor-pointer pt-1">
                    <Checkbox
                      checked={clearToken}
                      onCheckedChange={setClearToken}
                      ariaLabel={t('config.clearToken')}
                    />
                    <span>{t('config.clearToken')}</span>
                  </label>
                )}
              </div>

              {canOperate && (
                <div className="flex justify-end gap-2 pt-2 border-t border-border">
                  {isRoomPortsDirty && (
                    <Button variant="outline" disabled={saveConfigMut.isPending} onClick={resetFormFromData}>
                      {t('common.discard')}
                    </Button>
                  )}
                  <Button
                    disabled={saveConfigMut.isPending || !isRoomPortsDirty || !!portValidationError}
                    onClick={() => saveConfigMut.mutate()}
                  >
                    <Save className="h-4 w-4" />
                    {saveConfigMut.isPending ? t('common.saving') : t('config.saveConfig')}
                  </Button>
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {/* Ports Tab */}
      {tab === 'ports' && (
        <div id="cluster-config-panel-ports" role="tabpanel" aria-labelledby="cluster-config-tab-ports">
        <Card>
          <CardHeader>
            <CardTitle className="flex items-center gap-2">
              <span>{t('config.tabPorts')}</span>
              {isRoomPortsDirty && <Badge variant="warning">{t('files.unsaved')}</Badge>}
            </CardTitle>
            <CardDescription>server.ini &amp; cluster.ini [SHARD]</CardDescription>
          </CardHeader>
          <CardContent className="space-y-6">
            <div className="flex items-start gap-2 rounded-lg bg-secondary/50 px-3.5 py-2.5 text-xs text-muted-foreground">
              <Info className="h-4 w-4 text-primary shrink-0 mt-0.5" />
              <span className="leading-relaxed">{t('config.portsFirewallHint')}</span>
            </div>

            {portValidationError && (
              <div
                role="alert"
                className="flex items-start gap-2 rounded-lg border border-red-600/35 bg-red-600/10 px-3.5 py-2.5 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
              >
                <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
                <span>{portValidationError}</span>
              </div>
            )}

            <div className="space-y-1.5 max-w-sm">
              <Label htmlFor="cfg-cmport">{t('config.clusterMasterPort')}</Label>
              <Input
                id="cfg-cmport"
                type="number"
                disabled={!canOperate}
                value={clusterINI.master_port}
                onChange={(e) => setClusterINI({ ...clusterINI, master_port: Number(e.target.value) })}
              />
            </div>

            <div className="space-y-3 border-t border-border pt-5">
              <h3 className="text-sm font-semibold">{t('config.masterPorts')}</h3>
              <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                <div className="space-y-1.5">
                  <Label htmlFor="cfg-m-sport">{t('config.serverPort')}</Label>
                  <Input
                    id="cfg-m-sport"
                    type="number"
                    disabled={!canOperate}
                    value={masterINI.server_port}
                    onChange={(e) => setMasterINI({ ...masterINI, server_port: Number(e.target.value) })}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="cfg-m-aport">{t('config.authPort')}</Label>
                  <Input
                    id="cfg-m-aport"
                    type="number"
                    disabled={!canOperate}
                    value={masterINI.authentication_port}
                    onChange={(e) => setMasterINI({ ...masterINI, authentication_port: Number(e.target.value) })}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="cfg-m-qport">{t('config.steamPort')}</Label>
                  <Input
                    id="cfg-m-qport"
                    type="number"
                    disabled={!canOperate}
                    value={masterINI.master_server_port}
                    onChange={(e) => setMasterINI({ ...masterINI, master_server_port: Number(e.target.value) })}
                  />
                </div>
              </div>
            </div>

            {cavesEnabled && cavesINI && (
              <div className="space-y-3 border-t border-border pt-5">
                <h3 className="text-sm font-semibold">{t('config.cavesPorts')}</h3>
                <div className="grid grid-cols-1 sm:grid-cols-3 gap-4">
                  <div className="space-y-1.5">
                    <Label htmlFor="cfg-c-sport">{t('config.serverPort')}</Label>
                    <Input
                      id="cfg-c-sport"
                      type="number"
                      disabled={!canOperate}
                      value={cavesINI.server_port}
                      onChange={(e) => setCavesINI({ ...cavesINI, server_port: Number(e.target.value) })}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="cfg-c-aport">{t('config.authPort')}</Label>
                    <Input
                      id="cfg-c-aport"
                      type="number"
                      disabled={!canOperate}
                      value={cavesINI.authentication_port}
                      onChange={(e) => setCavesINI({ ...cavesINI, authentication_port: Number(e.target.value) })}
                    />
                  </div>
                  <div className="space-y-1.5">
                    <Label htmlFor="cfg-c-qport">{t('config.steamPort')}</Label>
                    <Input
                      id="cfg-c-qport"
                      type="number"
                      disabled={!canOperate}
                      value={cavesINI.master_server_port}
                      onChange={(e) => setCavesINI({ ...cavesINI, master_server_port: Number(e.target.value) })}
                    />
                  </div>
                </div>
              </div>
            )}

            {canOperate && (
              <div className="flex justify-end gap-2 pt-2 border-t border-border">
                {isRoomPortsDirty && (
                  <Button variant="outline" disabled={saveConfigMut.isPending} onClick={resetFormFromData}>
                    {t('common.discard')}
                  </Button>
                )}
                <Button
                  disabled={saveConfigMut.isPending || !isRoomPortsDirty || !!portValidationError}
                  onClick={() => saveConfigMut.mutate()}
                >
                  <Save className="h-4 w-4" />
                  {saveConfigMut.isPending ? t('common.saving') : t('config.saveConfig')}
                </Button>
              </div>
            )}
          </CardContent>
        </Card>
        </div>
      )}

      {/* Sticky Bottom Save Bar for Room & Ports tabs */}
      <StickySaveBar
        visible={canOperate && (tab === 'room' || tab === 'ports') && isRoomPortsDirty}
        saving={saveConfigMut.isPending}
        disabled={!!portValidationError}
        saveLabel={t('config.saveConfig')}
        onDiscard={resetFormFromData}
        onSave={() => saveConfigMut.mutate()}
      />

      {/* World Visual + Raw Lua Dual-Mode Editor Tabs */}
      {(tab === 'master' || tab === 'caves') && (
        <div
          id={`cluster-config-panel-${tab}`}
          role="tabpanel"
          aria-labelledby={`cluster-config-tab-${tab}`}
        >
          <WorldVisualEditor
            key={tab}
            clusterId={id}
            shard={tab}
            canOperate={canOperate}
            presets={tab === 'master' ? masterPresets : cavesPresets}
            isClusterRunning={isClusterRunning}
            onRestartCluster={() => restartMut.mutate()}
            restarting={restartMut.isPending}
            onDirtyChange={setWorldDirty}
            onSaved={() => setSavedNeedRestart(true)}
          />
        </div>
      )}

      {/* Unsaved Tab Switch Confirmation */}
      <ConfirmDialog
        open={pendingTab !== null}
        onClose={() => setPendingTab(null)}
        onConfirm={() => {
          if (pendingTab) {
            if (tab === 'room' || tab === 'ports') resetFormFromData()
            setWorldDirty(false)
            setTab(pendingTab)
            setPendingTab(null)
          }
        }}
        title={t('common.unsavedSwitchConfirmTitle')}
        description={t('common.unsavedSwitchConfirmDesc')}
        confirmLabel={t('common.discard')}
        tone="warning"
      />
    </div>
  )
}
