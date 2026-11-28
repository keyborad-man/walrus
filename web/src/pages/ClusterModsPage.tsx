import React, { useEffect, useState } from 'react'
import { Link, useParams, useSearchParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertTriangle,
  Code2,
  Download,
  FolderArchive,
  Mountain,
  Package,
  Plus,
  RefreshCw,
  RotateCcw,
  RotateCw,
  Save,
  Search,
  Settings,
  Star,
  Trash2,
  Trees,
  Upload,
} from 'lucide-react'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { useAuth } from '@/hooks/useAuth'
import { useRegisterUnsaved } from '@/hooks/useUnsavedGuard'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input, Label, Select } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { TabList } from '@/components/ui/tabs'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { Dialog } from '@/components/ui/dialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { PageHeader } from '@/components/PageHeader'
import { StickySaveBar } from '@/components/StickySaveBar'
import { EmptyState } from '@/components/EmptyState'
import { PageCardsSkeleton } from '@/components/ui/skeleton'
import type { ClusterModItem, ModCache, ModConfigOption } from '@/types/api'

type ModsTab = 'configured' | 'favorites' | 'workshop' | 'local' | 'raw'

function formatCompactCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1).replace(/\.0$/, '')}M`
  if (n >= 1_000) return `${(n / 1_000).toFixed(1).replace(/\.0$/, '')}K`
  return String(n)
}

function ModThumbnail({ src, alt }: { src?: string; alt: string }) {
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    setFailed(false)
  }, [src])

  if (!src || failed) {
    return (
      <div
        className="h-12 w-12 rounded-lg border border-border bg-secondary/75 text-muted-foreground flex items-center justify-center shrink-0"
        aria-hidden="true"
      >
        <Package className="h-5 w-5" />
      </div>
    )
  }

  return (
    <img
      src={src}
      alt={alt}
      loading="lazy"
      decoding="async"
      referrerPolicy="no-referrer"
      onError={() => setFailed(true)}
      className="h-12 w-12 rounded-lg object-cover border border-border bg-secondary/40 shrink-0"
    />
  )
}

function FavoriteToggle({
  active,
  label,
  disabled,
  onClick,
}: {
  active: boolean
  label: string
  disabled?: boolean
  onClick: () => void
}) {
  return (
    <Button
      size="icon"
      variant="ghost"
      aria-pressed={active}
      aria-label={label}
      title={label}
      disabled={disabled}
      onClick={onClick}
      className={active ? 'text-amber-600 dark:text-amber-400' : 'text-muted-foreground'}
    >
      <Star className={cn('h-4 w-4', active && 'fill-current')} />
    </Button>
  )
}

export function ClusterModsPage() {
  const { id = '' } = useParams<{ id: string }>()
  const { t, i18n } = useTranslation()
  const { user } = useAuth()
  const qc = useQueryClient()

  // ?mod=workshop-123&shard=caves arrives from the dashboard's crash notice: open that shard and point at the mod.
  const [searchParams] = useSearchParams()
  const focusModId = searchParams.get('mod')
  const [tab, setTab] = useState<ModsTab>('configured')
  const [pendingTab, setPendingTab] = useState<ModsTab | null>(null)
  const [shard, setShard] = useState<'master' | 'caves'>(searchParams.get('shard') === 'caves' ? 'caves' : 'master')
  const [pendingShard, setPendingShard] = useState<'master' | 'caves' | null>(null)
  const [quickInput, setQuickInput] = useState('')
  const [isCollection, setIsCollection] = useState(false)
  const [rawLua, setRawLua] = useState('')
  const [initialRawLua, setInitialRawLua] = useState('')
  const [redownloadingModId, setRedownloadingModId] = useState<string | null>(null)
  const [bulkBusy, setBulkBusy] = useState(false)
  const [modsNeedRestart, setModsNeedRestart] = useState(false)
  const [restartConfirmOpen, setRestartConfirmOpen] = useState(false)

  // Workshop search
  const [searchQuery, setSearchQuery] = useState('')
  const [submittedQuery, setSubmittedQuery] = useState('')

  // Visual config modal
  const [configMod, setConfigMod] = useState<ClusterModItem | null>(null)
  const [draftOpts, setDraftOpts] = useState<Record<string, unknown>>({})

  // Confirm dialogs
  const [removeTarget, setRemoveTarget] = useState<ClusterModItem | null>(null)
  const [deleteLocalTarget, setDeleteLocalTarget] = useState<ModCache | null>(null)

  const { data: clusterData } = useQuery({
    queryKey: ['cluster', id],
    queryFn: () => api.getCluster(id),
    enabled: !!id,
  })
  const cavesEnabled = clusterData?.cluster.meta.caves_enabled ?? false
  const canOperate = clusterData?.effective_role === 'admin' || clusterData?.effective_role === 'operator'
  const isAdmin = user?.role === 'admin'
  const isClusterRunning =
    clusterData?.cluster.master_status.state === 'running' ||
    (cavesEnabled && clusterData?.cluster.caves_status?.state === 'running')
  const onlinePlayersCount = clusterData?.cluster.world_info?.online_players?.length ?? 0

  const {
    data: modsData,
    isLoading,
    isError: modsIsError,
    error: modsError,
    refetch: refetchMods,
  } = useQuery({
    queryKey: ['cluster-mods', id, shard, i18n.language],
    queryFn: () => api.getClusterMods(id, shard, i18n.language),
    enabled: !!id,
  })

  // DST never updates server mods on its own here, and a server on an old version turns away
  // every player who already has the new one, so check the Workshop whenever the page opens.
  const hasWorkshopMods = (modsData?.mods ?? []).some((m) => !m.is_local)
  const {
    data: updatesData,
    isError: updatesIsError,
    error: updatesError,
    refetch: refetchUpdates,
  } = useQuery({
    queryKey: ['cluster-mod-updates', id],
    queryFn: () => api.checkClusterModUpdates(id),
    enabled: !!id && hasWorkshopMods,
    retry: false,
  })
  const outdatedModIds = new Set((updatesData?.outdated ?? []).filter((o) => !o.missing).map((o) => o.mod_id))

  const { data: libraryData } = useQuery({
    queryKey: ['mod-library'],
    queryFn: api.listModLibrary,
    enabled: tab === 'local',
  })

  const { data: favoritesData } = useQuery({
    queryKey: ['mod-favorites'],
    queryFn: api.listModFavorites,
  })
  const favorites = favoritesData?.favorites ?? []
  const favoriteIds = new Set(favorites.map((f) => f.mod_id))

  const { data: searchData, isFetching: searching, error: searchErr } = useQuery({
    queryKey: ['workshop-search', submittedQuery],
    queryFn: () => api.searchWorkshop(submittedQuery, 1),
    enabled: tab === 'workshop',
    retry: false,
  })

  useEffect(() => {
    if (modsData?.raw_lua !== undefined) {
      setRawLua(modsData.raw_lua)
      setInitialRawLua(modsData.raw_lua)
    }
  }, [modsData?.raw_lua])

  const mods = modsData?.mods ?? []
  const modSync = modsData?.mod_sync_enabled ?? true
  const focusMod = focusModId ? mods.find((m) => m.mod_id === focusModId) : undefined
  const focusShard = searchParams.get('shard') === 'caves' ? 'caves' : 'master'

  // Once the suspect mod is off, restart right here instead of sending the owner back to the dashboard.
  // If the other shard is still up, only the crashed one is started so nobody gets kicked.
  const recoverMut = useMutation({
    mutationFn: () => api.startCluster(id, isClusterRunning ? focusShard : undefined),
    onSuccess: () => {
      toast.success(t('mods.recoverStarted', { name: clusterData?.cluster.meta.display_name || id }))
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      void qc.invalidateQueries({ queryKey: ['cluster', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  useEffect(() => {
    if (!focusMod) return
    const reduce = window.matchMedia('(prefers-reduced-motion: reduce)').matches
    document.getElementById(`mod-card-${focusMod.mod_id}`)?.scrollIntoView({ block: 'center', behavior: reduce ? 'auto' : 'smooth' })
  }, [focusMod])
  const installedIds = new Set(mods.map((m) => m.mod_id))
  const rawDirty = rawLua !== initialRawLua

  useRegisterUnsaved('cluster-mods-raw', tab === 'raw' && rawDirty)

  const refreshMods = () => {
    void qc.invalidateQueries({ queryKey: ['cluster-mods', id] })
    void qc.invalidateQueries({ queryKey: ['cluster-mod-updates', id] })
    void qc.invalidateQueries({ queryKey: ['mod-library'] })
  }

  const syncMut = useMutation({
    mutationFn: (enabled: boolean) => api.setModSync(id, enabled),
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const addWorkshopMut = useMutation({
    mutationFn: (vars: { input: string; is_collection: boolean }) =>
      api.addWorkshopMod(id, {
        shard: modSync ? 'master' : shard,
        input: vars.input,
        is_collection: vars.is_collection,
        locale: i18n.language,
      }),
    onSuccess: (res) => {
      toast.success(`${t('common.savedSuccess')} (+${res.added.length})`)
      setQuickInput('')
      setModsNeedRestart(true)
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const addLocalMut = useMutation({
    mutationFn: (modId: string) =>
      api.addLocalModToCluster(id, { shard: modSync ? 'master' : shard, mod_id: modId, locale: i18n.language }),
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setModsNeedRestart(true)
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const saveModMut = useMutation({
    mutationFn: (vars: { modId: string; enabled: boolean; configuration_options: Record<string, unknown> }) =>
      api.saveClusterMod(id, vars.modId, {
        shard: modSync ? 'master' : shard,
        enabled: vars.enabled,
        configuration_options: vars.configuration_options,
      }),
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setConfigMod(null)
      setModsNeedRestart(true)
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const removeModMut = useMutation({
    mutationFn: (mod: ClusterModItem) => api.removeClusterMod(id, mod.mod_id, modSync ? 'master' : shard),
    onSuccess: (_, mod) => {
      toast.success(t('mods.toastRemoved', { name: mod.name || mod.mod_id }))
      setRemoveTarget(null)
      setModsNeedRestart(true)
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const redownloadMut = useMutation({
    mutationFn: (modId: string) => api.redownloadMod(modId, i18n.language),
    onMutate: (modId) => setRedownloadingModId(modId),
    onSuccess: () => {
      toast.success(t('common.actionSuccess'))
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
    onSettled: () => setRedownloadingModId(null),
  })

  const updateModsMut = useMutation({
    mutationFn: () => api.updateClusterMods(id),
    onSuccess: (res) => {
      if (res.updated > 0) {
        toast.success(t('mods.toastModsUpdated', { count: res.updated }))
        setModsNeedRestart(true)
      } else {
        toast.success(t('mods.toastModsUpToDate'))
      }
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const saveRawMut = useMutation({
    mutationFn: () => api.saveRawModOverrides(id, modSync ? 'master' : shard, rawLua),
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setInitialRawLua(rawLua)
      setModsNeedRestart(true)
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const restartClusterMut = useMutation({
    mutationFn: () => api.restartCluster(id),
    onSuccess: () => {
      toast.success(t('dashboard.toastRestarted', { name: clusterData?.cluster.cluster_ini.cluster_name || id }))
      setModsNeedRestart(false)
      setRestartConfirmOpen(false)
      void qc.invalidateQueries({ queryKey: ['cluster', id] })
      void qc.invalidateQueries({ queryKey: ['clusters'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const handleBulkRedownload = async () => {
    const workshopMods = mods.filter((m) => !m.is_local)
    if (workshopMods.length === 0) return
    setBulkBusy(true)
    let succeeded = 0
    let failed = 0
    let lastErr = ''
    try {
      // Run sequentially because SteamCMD only executes one task at a time.
      for (const m of workshopMods) {
        setRedownloadingModId(m.mod_id)
        try {
          await api.redownloadMod(m.mod_id, i18n.language)
          succeeded++
        } catch (err) {
          failed++
          lastErr = err instanceof Error ? err.message : String(err)
        }
      }
      if (failed === 0) {
        toast.success(t('mods.toastBulkRedownloaded', { count: succeeded }))
      } else if (succeeded > 0) {
        toast.warning(t('mods.toastBulkRedownloadPartial', { succeeded, failed, detail: lastErr }))
      } else {
        toast.error(lastErr || t('mods.toastBulkRedownloadFailed'))
      }
      refreshMods()
    } finally {
      setRedownloadingModId(null)
      setBulkBusy(false)
    }
  }

  const favoriteMut = useMutation({
    // Resolves to true when the mod was removed from favorites, false when it was added.
    mutationFn: async (vars: { modId: string; name: string; preview_url?: string; configuration_options?: Record<string, unknown> }) => {
      if (favoriteIds.has(vars.modId)) {
        await api.deleteModFavorite(vars.modId)
        return true
      }
      await api.saveModFavorite(vars.modId, {
        name: vars.name,
        preview_url: vars.preview_url,
        configuration_options: vars.configuration_options,
      })
      return false
    },
    onSuccess: (removed, vars) => {
      const name = vars.name || vars.modId
      if (removed) toast.success(t('mods.toastUnfavorited', { name }))
      else if (vars.configuration_options) toast.success(t('mods.toastFavoritedWithConfig', { name }))
      else toast.success(t('mods.toastFavorited', { name }))
      void qc.invalidateQueries({ queryKey: ['mod-favorites'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const addFavoritesMut = useMutation({
    mutationFn: (modIds: string[]) =>
      api.addFavoriteModsToCluster(id, { shard: modSync ? 'master' : shard, mod_ids: modIds, locale: i18n.language }),
    onSuccess: (res) => {
      toast.success(t('mods.toastFavoritesAdded', { count: res.added.length }))
      if (res.added.length > 0) setModsNeedRestart(true)
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const uploadLocalMut = useMutation({
    mutationFn: (file: File) => api.uploadLocalMod(file, i18n.language),
    onSuccess: () => {
      toast.success(t('common.actionSuccess'))
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteLocalMut = useMutation({
    mutationFn: (modId: string) => api.deleteLocalMod(modId),
    onSuccess: () => {
      toast.success(t('common.actionSuccess'))
      setDeleteLocalTarget(null)
      refreshMods()
    },
    onError: (err: Error) => toast.error(err.message),
  })

  // Ctrl/Cmd + S in Raw Lua tab
  useEffect(() => {
    if (tab !== 'raw' || !canOperate) return
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's' && rawDirty && !saveRawMut.isPending) {
        e.preventDefault()
        saveRawMut.mutate()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [tab, canOperate, rawDirty, saveRawMut])

  const openConfigModal = (m: ClusterModItem) => {
    const initial: Record<string, unknown> = { ...(m.configuration_options ?? {}) }
    for (const opt of m.schema_options ?? []) {
      if (initial[opt.name] === undefined && opt.default !== undefined) {
        initial[opt.name] = opt.default
      }
    }
    setDraftOpts(initial)
    setConfigMod(m)
  }

  const handleQuickAdd = (e: React.FormEvent) => {
    e.preventDefault()
    if (!quickInput.trim()) return
    addWorkshopMut.mutate({ input: quickInput.trim(), is_collection: isCollection })
  }

  const localMods = (libraryData?.mods ?? []).filter((m) => m.is_local)
  const favoritesToAdd = favorites.filter((f) => !installedIds.has(f.mod_id)).map((f) => f.mod_id)

  const requestTabChange = (next: ModsTab) => {
    if (next === tab) return
    if (tab === 'raw' && rawDirty) {
      setPendingTab(next)
      return
    }
    setTab(next)
  }

  const requestShardChange = (next: 'master' | 'caves') => {
    if (next === shard) return
    if (tab === 'raw' && rawDirty) {
      setPendingShard(next)
      return
    }
    setShard(next)
  }

  return (
    <div className="space-y-6">
      {/* Header + Sync Control */}
      <PageHeader
        eyebrow={t('nav.groupCare')}
        title={t('mods.title')}
        subtitle={t('mods.subtitle')}
        actions={
          cavesEnabled ? (
            <div className="flex items-center gap-2.5 flex-wrap">
              <div className="flex items-center gap-2.5 rounded-lg border border-border bg-card px-3 py-1.5 text-xs">
                <span className="font-medium">{t('mods.syncLabel')}</span>
                <Switch
                  checked={modSync}
                  disabled={!canOperate || syncMut.isPending}
                  onCheckedChange={(v) => syncMut.mutate(v)}
                  ariaLabel={t('mods.syncLabel')}
                />
              </div>
              {!modSync && (
                <TabList
                  ariaLabel={t('mods.syncLabel')}
                  variant="segmented"
                  value={shard}
                  onChange={requestShardChange}
                  items={[
                    { key: 'master', label: t('common.master'), icon: <Trees className="h-3.5 w-3.5" /> },
                    { key: 'caves', label: t('common.caves'), icon: <Mountain className="h-3.5 w-3.5" /> },
                  ]}
                />
              )}
            </div>
          ) : undefined
        }
      />

      {modsNeedRestart && isClusterRunning && canOperate && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-card px-3.5 py-2.5 text-[13px]">
          <div className="flex items-center gap-2.5 min-w-0">
            <span className="h-[7px] w-[7px] rounded-full bg-amber-500 shrink-0" aria-hidden="true" />
            <span className="text-foreground/90">{t('mods.savedRestartPrompt')}</span>
          </div>
          <Button
            size="sm"
            variant="warning"
            disabled={restartClusterMut.isPending}
            onClick={() => {
              if (onlinePlayersCount > 0) setRestartConfirmOpen(true)
              else restartClusterMut.mutate()
            }}
          >
            <RotateCw className={cn('h-3.5 w-3.5', restartClusterMut.isPending && 'animate-spin')} />
            {t('config.restartNow')}
          </Button>
        </div>
      )}

      {/* Tabs */}
      <TabList
        ariaLabel={t('mods.title')}
        idPrefix="cluster-mods"
        value={tab}
        onChange={requestTabChange}
        items={[
          { key: 'configured', label: `${t('mods.tabConfigured')} (${mods.length})`, icon: <Package className="h-3.5 w-3.5" /> },
          { key: 'favorites', label: `${t('mods.tabFavorites')} (${favorites.length})`, icon: <Star className="h-3.5 w-3.5" /> },
          { key: 'workshop', label: t('mods.tabWorkshop'), icon: <Search className="h-3.5 w-3.5" /> },
          { key: 'local', label: t('mods.tabLocal'), icon: <FolderArchive className="h-3.5 w-3.5" /> },
          { key: 'raw', label: t('mods.tabRaw'), icon: <Code2 className="h-3.5 w-3.5" /> },
        ]}
      />

      {modsIsError && <QueryErrorCard error={modsError} onRetry={() => void refetchMods()} compact />}

      {/* Configured Mods Tab */}
      {tab === 'configured' && (
        <div id="cluster-mods-panel-configured" role="tabpanel" aria-labelledby="cluster-mods-tab-configured" className="space-y-4">
          {canOperate && (
            <Card>
              <CardContent className="p-4 sm:p-5">
                <form onSubmit={handleQuickAdd} className="space-y-3">
                  <div className="flex flex-col sm:flex-row gap-2">
                    <Input
                      value={quickInput}
                      onChange={(e) => setQuickInput(e.target.value)}
                      placeholder={t('mods.quickAddPlaceholder')}
                      aria-label={t('mods.quickAddPlaceholder')}
                      className="flex-1"
                    />
                    <Button type="submit" disabled={!quickInput.trim() || addWorkshopMut.isPending}>
                      <Plus className="h-4 w-4" />
                      {addWorkshopMut.isPending ? t('common.loading') : t('mods.addBtn')}
                    </Button>
                  </div>
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <label className="inline-flex items-center gap-2 text-xs text-muted-foreground cursor-pointer">
                      <Switch
                        checked={isCollection}
                        onCheckedChange={setIsCollection}
                        ariaLabel={t('mods.collectionMode')}
                      />
                      <span>{t('mods.collectionMode')}</span>
                    </label>
                    {mods.filter((m) => !m.is_local).length > 1 && (
                      <Button
                        type="button"
                        size="sm"
                        variant="outline"
                        disabled={bulkBusy}
                        onClick={() => void handleBulkRedownload()}
                      >
                        <RefreshCw className={cn('h-3.5 w-3.5', bulkBusy && 'animate-spin')} />
                        {t('mods.bulkRedownloadAll')}
                      </Button>
                    )}
                  </div>
                </form>
              </CardContent>
            </Card>
          )}

          {focusModId && !isLoading && !focusMod && (
            <p className="flex items-center gap-2 rounded-lg border border-amber-500/30 bg-amber-500/[0.06] px-3 py-2 text-[13px] text-amber-800 dark:text-amber-300">
              <AlertTriangle className="h-3.5 w-3.5 shrink-0" aria-hidden="true" />
              {t('mods.crashSuspectMissing', { id: focusModId })}
            </p>
          )}
          {outdatedModIds.size > 0 && (
            <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-amber-500/30 bg-amber-500/[0.06] px-3.5 py-2.5 text-[13px]">
              <p className="flex items-start gap-2 min-w-0 text-amber-900 dark:text-amber-300">
                <Download className="mt-0.5 h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                <span>{t('mods.updatesAvailable', { count: outdatedModIds.size })}</span>
              </p>
              {canOperate && (
                <Button
                  size="sm"
                  variant="warning"
                  disabled={updateModsMut.isPending || bulkBusy}
                  onClick={() => updateModsMut.mutate()}
                >
                  <RefreshCw className={cn('h-3.5 w-3.5', updateModsMut.isPending && 'animate-spin')} />
                  {t('mods.updateAll')}
                </Button>
              )}
            </div>
          )}
          {updatesIsError && (
            <p className="flex flex-wrap items-center gap-x-2 gap-y-1 text-xs text-muted-foreground">
              <span>{t('mods.updatesCheckFailed', { detail: updatesError.message })}</span>
              <button
                type="button"
                className="underline underline-offset-2 hover:text-foreground"
                onClick={() => void refetchUpdates()}
              >
                {t('mods.updatesRetry')}
              </button>
            </p>
          )}
          {isLoading ? (
            <PageCardsSkeleton count={2} />
          ) : mods.length === 0 ? (
            <EmptyState
              icon={<Package className="h-5 w-5" />}
              title={t('mods.tabConfigured')}
              description={t('mods.emptyConfigured')}
              action={
                <Button size="sm" variant="secondary" onClick={() => setTab('workshop')}>
                  <Search className="h-3.5 w-3.5" />
                  {t('mods.tabWorkshop')}
                </Button>
              }
            />
          ) : (
            <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
              {mods.map((m) => (
                <Card
                  key={m.mod_id}
                  id={`mod-card-${m.mod_id}`}
                  interactive={m.enabled}
                  className={cn(
                    !m.enabled && 'border-dashed bg-muted/40',
                    outdatedModIds.has(m.mod_id) && 'mod-update-glow',
                    focusMod?.mod_id === m.mod_id && 'border-red-500/60 ring-2 ring-red-500/25',
                  )}
                >
                  <CardContent className="p-4 sm:p-5 space-y-3">
                    {focusMod?.mod_id === m.mod_id && (
                      <div className="space-y-2 rounded-md bg-red-500/[0.08] px-2.5 py-2 text-xs text-red-800 dark:text-red-300">
                        <p className="flex items-start gap-1.5">
                          <AlertTriangle className="mt-px h-3.5 w-3.5 shrink-0" aria-hidden="true" />
                          <span>{m.enabled ? t('mods.crashSuspect') : t('mods.crashSuspectDisabled')}</span>
                        </p>
                        {!m.enabled && canOperate && (
                          <div className="flex flex-wrap items-center gap-2 pl-5">
                            <Button size="sm" disabled={recoverMut.isPending} onClick={() => recoverMut.mutate()}>
                              <RefreshCw className={cn('h-3.5 w-3.5', recoverMut.isPending && 'animate-spin')} />
                              {isClusterRunning
                                ? t('mods.recoverShard', { shard: t(focusShard === 'caves' ? 'common.cavesShort' : 'common.masterShort') })
                                : t('mods.recoverWorld')}
                            </Button>
                            <Link to="/" className="text-xs text-foreground underline underline-offset-2 hover:opacity-80">
                              {t('mods.backToDashboard')}
                            </Link>
                          </div>
                        )}
                      </div>
                    )}
                    <div className="flex items-start justify-between gap-3">
                      <div className="flex items-start gap-3 min-w-0">
                        <ModThumbnail src={m.preview_url} alt={m.name} />
                        <div className="min-w-0">
                          <h2 className="font-semibold text-sm truncate">{m.name}</h2>
                          <div className="flex items-center gap-1.5 flex-wrap mt-1">
                            <Badge variant="secondary" className="font-mono">
                              {m.mod_id}
                            </Badge>
                            {m.is_local && <Badge variant="info">{t('mods.localBadge')}</Badge>}
                            {m.version && <Badge variant="muted">v{m.version}</Badge>}
                            {outdatedModIds.has(m.mod_id) && (
                              <Badge variant="warning">{t('mods.updateAvailableBadge')}</Badge>
                            )}
                          </div>
                        </div>
                      </div>
                      <Switch
                        checked={m.enabled}
                        disabled={!canOperate || saveModMut.isPending}
                        ariaLabel={`${m.name} ${t('common.enabled')}`}
                        onCheckedChange={(enabled) =>
                          saveModMut.mutate({
                            modId: m.mod_id,
                            enabled,
                            configuration_options: m.configuration_options ?? {},
                          })
                        }
                      />
                    </div>

                    {m.description && (
                      <p className="text-xs text-muted-foreground line-clamp-2 text-pretty">{m.description}</p>
                    )}

                    {m.client_only && (
                      <div className="flex items-center gap-1.5 rounded-lg border border-amber-600/35 bg-amber-500/12 px-2.5 py-1.5 text-xs text-amber-900 dark:border-amber-500/30 dark:text-amber-300">
                        <AlertTriangle className="h-3.5 w-3.5 shrink-0" />
                        <span>{t('mods.clientOnlyWarn')}</span>
                      </div>
                    )}

                    {!m.downloaded && !m.is_local && (
                      <div className="text-xs text-amber-900 dark:text-amber-300">{t('mods.notDownloaded')}</div>
                    )}

                    <div className="flex items-center justify-between gap-2 pt-2 border-t border-border">
                      <div className="flex items-center gap-1.5 flex-wrap">
                        <Button
                          size="sm"
                          variant="secondary"
                          disabled={!canOperate}
                          onClick={() => openConfigModal(m)}
                        >
                          <Settings className="h-3.5 w-3.5" />
                          {t('mods.configure')}
                          {m.schema_options?.length > 0 && ` (${m.schema_options.length})`}
                        </Button>
                        {!m.is_local && canOperate && (
                          <Button
                            size="sm"
                            variant="outline"
                            disabled={redownloadingModId === m.mod_id}
                            onClick={() => redownloadMut.mutate(m.mod_id)}
                          >
                            <RefreshCw className={`h-3.5 w-3.5 ${redownloadingModId === m.mod_id ? 'animate-spin' : ''}`} />
                            {t('mods.redownload')}
                          </Button>
                        )}
                      </div>
                      {canOperate && (
                        <div className="flex items-center gap-1">
                          <FavoriteToggle
                            active={favoriteIds.has(m.mod_id)}
                            label={favoriteIds.has(m.mod_id) ? t('mods.unfavorite') : t('mods.favoriteWithConfig')}
                            disabled={favoriteMut.isPending}
                            onClick={() =>
                              favoriteMut.mutate({
                                modId: m.mod_id,
                                name: m.name,
                                preview_url: m.preview_url,
                                configuration_options: m.configuration_options ?? {},
                              })
                            }
                          />
                          <Button
                            size="icon"
                            variant="ghost"
                            aria-label={`${t('mods.remove')} ${m.name}`}
                            title={t('mods.remove')}
                            className="text-red-700 hover:bg-red-600/12 dark:text-red-400 dark:hover:bg-red-500/12"
                            onClick={() => setRemoveTarget(m)}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        </div>
                      )}
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          )}
        </div>
      )}

      {/* Favorites Tab */}
      {tab === 'favorites' && (
        <div id="cluster-mods-panel-favorites" role="tabpanel" aria-labelledby="cluster-mods-tab-favorites" className="space-y-4">
          {favorites.length === 0 ? (
            <EmptyState
              icon={<Star className="h-5 w-5" />}
              title={t('mods.tabFavorites')}
              description={t('mods.emptyFavorites')}
            />
          ) : (
            <>
              <div className="flex flex-wrap items-center justify-between gap-3">
                <p className="text-xs text-muted-foreground text-pretty max-w-2xl">{t('mods.favoritesHint')}</p>
                {canOperate && (
                  <Button
                    size="sm"
                    disabled={favoritesToAdd.length === 0 || addFavoritesMut.isPending}
                    onClick={() => addFavoritesMut.mutate(favoritesToAdd)}
                  >
                    <Plus className="h-3.5 w-3.5" />
                    {t('mods.addAllFavorites', { count: favoritesToAdd.length })}
                  </Button>
                )}
              </div>
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {favorites.map((f) => {
                  const added = installedIds.has(f.mod_id)
                  const savedCount = f.configuration_options ? Object.keys(f.configuration_options).length : 0
                  return (
                    <Card key={f.mod_id} className="flex flex-col">
                      <CardContent className="p-4 sm:p-5 flex-1 flex flex-col justify-between gap-3">
                        <div className="flex items-start gap-3">
                          <ModThumbnail src={f.preview_url} alt={f.name || f.mod_id} />
                          <div className="min-w-0">
                            <h2 className="font-semibold text-sm line-clamp-1">{f.name || f.mod_id}</h2>
                            <div className="flex items-center gap-1.5 flex-wrap mt-1">
                              <Badge variant="secondary" className="font-mono">
                                {f.mod_id}
                              </Badge>
                              {f.is_local && <Badge variant="info">{t('mods.localBadge')}</Badge>}
                              <Badge variant="muted">
                                {savedCount > 0
                                  ? t('mods.favoriteConfigSaved', { count: savedCount })
                                  : t('mods.favoriteConfigDefault')}
                              </Badge>
                            </div>
                          </div>
                        </div>
                        {canOperate && (
                          <div className="pt-2 border-t border-border flex items-center justify-end gap-1.5">
                            <FavoriteToggle
                              active
                              label={t('mods.unfavorite')}
                              disabled={favoriteMut.isPending}
                              onClick={() => favoriteMut.mutate({ modId: f.mod_id, name: f.name })}
                            />
                            <Button
                              size="sm"
                              variant={added ? 'secondary' : 'default'}
                              disabled={added || addFavoritesMut.isPending}
                              onClick={() => addFavoritesMut.mutate([f.mod_id])}
                            >
                              <Download className="h-3.5 w-3.5" />
                              {added ? t('mods.alreadyAdded') : t('mods.addToCluster')}
                            </Button>
                          </div>
                        )}
                      </CardContent>
                    </Card>
                  )
                })}
              </div>
            </>
          )}
        </div>
      )}

      {/* Workshop Search Tab */}
      {tab === 'workshop' && (
        <div id="cluster-mods-panel-workshop" role="tabpanel" aria-labelledby="cluster-mods-tab-workshop" className="space-y-4">
          <form
            onSubmit={(e) => {
              e.preventDefault()
              setSubmittedQuery(searchQuery)
            }}
            className="flex gap-2"
          >
            <Input
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder={t('mods.searchPlaceholder')}
              aria-label={t('mods.searchPlaceholder')}
              className="flex-1"
            />
            <Button type="submit" disabled={searching}>
              <Search className="h-4 w-4" />
              {t('mods.searchBtn')}
            </Button>
          </form>

          {searchData?.is_mock && (
            <div className="flex flex-wrap items-center justify-between gap-2 rounded-lg border border-amber-600/30 bg-amber-500/10 px-3.5 py-2.5 text-xs text-amber-900 dark:border-amber-500/30 dark:text-amber-300">
              <span>{t('mods.mockCatalogHint')}</span>
              {isAdmin && (
                <Link
                  to="/settings"
                  className="font-medium underline underline-offset-4 hover:opacity-80 shrink-0"
                >
                  {t('mods.configureApiKey')}
                </Link>
              )}
            </div>
          )}

          {searchErr && (
            <div className="rounded-lg border border-red-600/35 bg-red-600/10 px-4 py-3 text-xs text-red-800 dark:border-red-500/35 dark:text-red-300">
              {(searchErr as Error).message}
            </div>
          )}

          {searching && !searchData ? (
            <PageCardsSkeleton count={3} />
          ) : !searchErr && searchData && searchData.items.length === 0 ? (
            <EmptyState
              icon={<Search className="h-5 w-5" />}
              title={t('mods.searchEmptyTitle')}
              description={searchData.is_mock ? t('mods.searchEmptyCuratedDesc') : t('mods.searchEmptyRemoteDesc')}
              action={
                submittedQuery ? (
                  <Button
                    size="sm"
                    variant="secondary"
                    onClick={() => {
                      setSearchQuery('')
                      setSubmittedQuery('')
                    }}
                  >
                    {searchData.is_mock ? t('mods.showAllCurated') : t('common.clearSearch')}
                  </Button>
                ) : undefined
              }
            />
          ) : (
            <div className="space-y-3">
              {searchData?.is_mock && !submittedQuery && searchData.items.length > 0 && (
                <div className="flex items-center justify-between text-xs text-muted-foreground">
                  <span className="font-medium text-foreground">{t('mods.curatedSectionTitle')}</span>
                  <span className="tabular-nums">{searchData.items.length}</span>
                </div>
              )}
              <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-4">
                {(searchData?.items ?? []).map((item) => {
                  const added = installedIds.has(item.mod_id)
                  return (
                    <Card key={item.workshop_id} className="flex flex-col">
                      <CardContent className="p-4 sm:p-5 flex-1 flex flex-col justify-between gap-3">
                        <div className="space-y-3">
                          <div className="flex items-start gap-3">
                            <ModThumbnail src={item.preview_url} alt={item.title} />
                            <div className="min-w-0">
                              <h2 className="font-semibold text-sm line-clamp-1">{item.title}</h2>
                              <div className="flex items-center gap-2 font-mono text-xs text-muted-foreground tabular-nums">
                                <span>ID: {item.workshop_id}</span>
                                {item.subscriptions > 0 && (
                                  <span>· {t('mods.subscriptionsCount', { value: formatCompactCount(item.subscriptions) })}</span>
                                )}
                              </div>
                            </div>
                          </div>
                          <p className="text-xs text-muted-foreground line-clamp-3">{item.description}</p>
                          {item.is_client_only && <Badge variant="warning">{t('mods.clientOnlyWarn')}</Badge>}
                        </div>
                        <div className="pt-2 border-t border-border flex items-center justify-end gap-1.5">
                          {canOperate && (
                            <FavoriteToggle
                              active={favoriteIds.has(item.mod_id)}
                              label={favoriteIds.has(item.mod_id) ? t('mods.unfavorite') : t('mods.favorite')}
                              disabled={favoriteMut.isPending}
                              onClick={() =>
                                favoriteMut.mutate({ modId: item.mod_id, name: item.title, preview_url: item.preview_url })
                              }
                            />
                          )}
                          <Button
                            size="sm"
                            variant={added ? 'secondary' : 'default'}
                            disabled={added || !canOperate || addWorkshopMut.isPending}
                            onClick={() => addWorkshopMut.mutate({ input: item.workshop_id, is_collection: false })}
                          >
                            <Download className="h-3.5 w-3.5" />
                            {added ? t('mods.alreadyAdded') : t('mods.addToCluster')}
                          </Button>
                        </div>
                      </CardContent>
                    </Card>
                  )
                })}
              </div>
            </div>
          )}
        </div>
      )}

      {/* Local / Private Mods Tab */}
      {tab === 'local' && (
        <div id="cluster-mods-panel-local" role="tabpanel" aria-labelledby="cluster-mods-tab-local" className="space-y-4">
          {isAdmin && (
            <Card>
              <CardHeader>
                <CardTitle className="flex items-center gap-2 text-base">
                  <Upload className="h-4 w-4 text-primary" />
                  {t('mods.uploadTitle')}
                </CardTitle>
                <CardDescription>{t('mods.uploadDesc')}</CardDescription>
              </CardHeader>
              <CardContent>
                <label className="inline-flex items-center gap-2 rounded-lg border border-dashed border-[var(--border-strong)] bg-[var(--surface-elevated)] px-4 py-2 text-[13px] font-medium text-foreground hover:bg-[var(--surface-hover)] focus-within:ring-2 focus-within:ring-ring cursor-pointer pressable">
                  <Upload className="h-4 w-4" />
                  <span>{uploadLocalMut.isPending ? t('common.loading') : t('mods.uploadBtn')}</span>
                  <input
                    type="file"
                    accept=".zip"
                    className="sr-only"
                    disabled={uploadLocalMut.isPending}
                    onChange={(e) => {
                      const file = e.target.files?.[0]
                      if (file) uploadLocalMut.mutate(file)
                      e.target.value = ''
                    }}
                  />
                </label>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle>
                {t('mods.localLibrary')} ({localMods.length})
              </CardTitle>
            </CardHeader>
            <CardContent>
              {localMods.length === 0 ? (
                <div className="py-8 text-center text-xs text-muted-foreground">{t('mods.emptyLocal')}</div>
              ) : (
                <div className="divide-y divide-border">
                  {localMods.map((m) => {
                    const added = installedIds.has(m.mod_id)
                    return (
                      <div key={m.mod_id} className="py-3 flex flex-wrap items-center justify-between gap-3">
                        <div className="min-w-0">
                          <div className="font-medium text-sm">{m.name}</div>
                          <div className="text-xs text-muted-foreground font-mono">
                            {m.mod_id} {m.version ? `· v${m.version}` : ''} {m.author ? `· ${m.author}` : ''}
                          </div>
                        </div>
                        <div className="flex items-center gap-2">
                          {canOperate && (
                            <FavoriteToggle
                              active={favoriteIds.has(m.mod_id)}
                              label={favoriteIds.has(m.mod_id) ? t('mods.unfavorite') : t('mods.favorite')}
                              disabled={favoriteMut.isPending}
                              onClick={() => favoriteMut.mutate({ modId: m.mod_id, name: m.name })}
                            />
                          )}
                          <Button
                            size="sm"
                            variant={added ? 'secondary' : 'default'}
                            disabled={added || !canOperate || addLocalMut.isPending}
                            onClick={() => addLocalMut.mutate(m.mod_id)}
                          >
                            {added ? t('mods.alreadyAdded') : t('mods.addToCluster')}
                          </Button>
                          {isAdmin && (
                            <Button
                              size="icon"
                              variant="ghost"
                              aria-label={`${t('common.delete')} ${m.name}`}
                              title={t('common.delete')}
                              className="text-red-700 hover:bg-red-600/12 dark:text-red-400 dark:hover:bg-red-500/12"
                              onClick={() => setDeleteLocalTarget(m)}
                            >
                              <Trash2 className="h-4 w-4" />
                            </Button>
                          )}
                        </div>
                      </div>
                    )
                  })}
                </div>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {/* Raw modoverrides.lua Tab */}
      {tab === 'raw' && (
        <div id="cluster-mods-panel-raw" role="tabpanel" aria-labelledby="cluster-mods-tab-raw">
        <Card>
          <CardHeader className="flex flex-row items-center justify-between gap-2">
            <div>
              <CardTitle className="flex items-center gap-2">
                <span>modoverrides.lua</span>
                {rawDirty && <Badge variant="warning">{t('files.unsaved')}</Badge>}
              </CardTitle>
              <CardDescription>
                {modSync ? t('mods.syncedScope') : shard === 'master' ? t('common.master') : t('common.caves')}
                {canOperate ? ` · ${t('worldVisual.shortcutHint')}` : ''}
              </CardDescription>
            </div>
            {canOperate && (
              <div className="flex items-center gap-2">
                {rawDirty && (
                  <Button size="sm" variant="outline" onClick={() => setRawLua(initialRawLua)}>
                    {t('common.discard')}
                  </Button>
                )}
                <Button size="sm" disabled={saveRawMut.isPending || !rawDirty} onClick={() => saveRawMut.mutate()}>
                  <Save className="h-3.5 w-3.5" />
                  {t('common.save')}
                </Button>
              </div>
            )}
          </CardHeader>
          <CardContent className="space-y-3">
            {saveRawMut.error && (
              <div
                role="alert"
                className="flex items-start gap-2 rounded-lg border border-red-600/35 bg-red-600/10 px-3.5 py-2.5 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
              >
                <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
                <span className="break-words">{saveRawMut.error.message}</span>
              </div>
            )}
            <textarea
              readOnly={!canOperate}
              aria-label="modoverrides.lua"
              value={rawLua}
              onChange={(e) => setRawLua(e.target.value)}
              spellCheck={false}
              className="w-full h-[460px] rounded-xl border border-[var(--surface-terminal-border)] bg-[var(--surface-terminal)] p-4 font-mono text-xs leading-relaxed text-slate-200 focus:outline-none focus:ring-2 focus:ring-ring"
            />
          </CardContent>
        </Card>
        {canOperate && (
          <div className="mt-4">
            <StickySaveBar
              visible={rawDirty}
              saving={saveRawMut.isPending}
              onDiscard={() => setRawLua(initialRawLua)}
              onSave={() => saveRawMut.mutate()}
            />
          </div>
        )}
        </div>
      )}

      {/* Dynamic Mod Options Config Modal */}
      <Dialog
        open={!!configMod}
        onClose={() => setConfigMod(null)}
        title={configMod ? `${t('mods.configModalTitle')}: ${configMod.name}` : ''}
        subtitle={configMod?.mod_id}
        maxWidthClass="max-w-2xl"
        footer={
          configMod ? (
            <div className="flex flex-wrap items-center justify-between gap-2">
              <Button
                type="button"
                variant="outline"
                size="sm"
                onClick={() => {
                  const defs: Record<string, unknown> = {}
                  for (const opt of configMod.schema_options ?? []) {
                    if (opt.default !== undefined) defs[opt.name] = opt.default
                  }
                  setDraftOpts(defs)
                }}
              >
                <RotateCcw className="h-3.5 w-3.5" />
                {t('mods.resetDefaults')}
              </Button>
              <div className="flex gap-2">
                <Button type="button" variant="outline" onClick={() => setConfigMod(null)}>
                  {t('common.cancel')}
                </Button>
                <Button
                  type="button"
                  disabled={saveModMut.isPending}
                  onClick={() =>
                    saveModMut.mutate({
                      modId: configMod.mod_id,
                      enabled: configMod.enabled,
                      configuration_options: draftOpts,
                    })
                  }
                >
                  <Save className="h-4 w-4" />
                  {t('common.save')}
                </Button>
              </div>
            </div>
          ) : undefined
        }
      >
        {configMod && (
          <div className="space-y-3">
            {!configMod.schema_options || configMod.schema_options.length === 0 ? (
              <p className="text-sm text-muted-foreground py-6 text-center">{t('mods.noOptions')}</p>
            ) : (
              configMod.schema_options.map((opt: ModConfigOption, optIdx: number) => {
                const currentVal = draftOpts[opt.name] ?? opt.default
                const serializedCurrent = JSON.stringify(currentVal)
                const selectId = `mod-opt-${opt.name}-${optIdx}`
                return (
                  <div
                    key={`${opt.name}-${optIdx}`}
                    className="flex flex-col sm:flex-row sm:items-center justify-between gap-2 rounded-xl border border-border bg-secondary/25 p-3"
                  >
                    <div className="min-w-0">
                      <Label htmlFor={selectId} className="text-sm">
                        {opt.label || opt.name}
                      </Label>
                      {opt.hover && <p className="text-xs text-muted-foreground mt-0.5">{opt.hover}</p>}
                    </div>
                    <Select
                      id={selectId}
                      className="sm:w-56 shrink-0"
                      value={serializedCurrent}
                      onChange={(e) => {
                        try {
                          const parsed = JSON.parse(e.target.value)
                          setDraftOpts((prev) => ({ ...prev, [opt.name]: parsed }))
                        } catch {
                          setDraftOpts((prev) => ({ ...prev, [opt.name]: e.target.value }))
                        }
                      }}
                    >
                      {opt.options.map((ch, idx) => (
                        <option key={idx} value={JSON.stringify(ch.data)}>
                          {ch.description || String(ch.data)}
                        </option>
                      ))}
                    </Select>
                  </div>
                )
              })
            )}
          </div>
        )}
      </Dialog>

      {/* Confirmations */}
      <ConfirmDialog
        open={!!removeTarget}
        onClose={() => setRemoveTarget(null)}
        onConfirm={() => {
          if (removeTarget) removeModMut.mutate(removeTarget)
        }}
        title={t('mods.removeConfirmTitle')}
        description={t('mods.removeConfirm', { name: removeTarget?.name ?? removeTarget?.mod_id ?? '' })}
        confirmLabel={t('mods.remove')}
        tone="warning"
        loading={removeModMut.isPending}
      />

      <ConfirmDialog
        open={restartConfirmOpen}
        onClose={() => setRestartConfirmOpen(false)}
        onConfirm={() => restartClusterMut.mutate()}
        title={t('dashboard.restartOnlineConfirmTitle')}
        description={t('dashboard.restartOnlineConfirmDesc', {
          name: clusterData?.cluster.cluster_ini.cluster_name || id,
          count: onlinePlayersCount,
        })}
        confirmLabel={t('config.restartNow')}
        tone="warning"
        loading={restartClusterMut.isPending}
      />

      <ConfirmDialog
        open={!!deleteLocalTarget}
        onClose={() => setDeleteLocalTarget(null)}
        onConfirm={() => {
          if (deleteLocalTarget) deleteLocalMut.mutate(deleteLocalTarget.mod_id)
        }}
        title={t('mods.deleteLocalConfirmTitle')}
        description={t('mods.deleteLocalConfirm', { name: deleteLocalTarget?.name ?? deleteLocalTarget?.mod_id ?? '' })}
        confirmLabel={t('common.delete')}
        tone="danger"
        loading={deleteLocalMut.isPending}
      />
      <ConfirmDialog
        open={pendingTab !== null || pendingShard !== null}
        onClose={() => {
          setPendingTab(null)
          setPendingShard(null)
        }}
        onConfirm={() => {
          setRawLua(initialRawLua)
          if (pendingTab) {
            setTab(pendingTab)
            setPendingTab(null)
          }
          if (pendingShard) {
            setShard(pendingShard)
            setPendingShard(null)
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
