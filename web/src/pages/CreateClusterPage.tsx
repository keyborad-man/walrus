import React, { useEffect, useMemo, useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertTriangle,
  ArrowLeft,
  ArrowRight,
  Check,
  ChevronLeft,
  ChevronRight,
  Compass,
  Feather,
  Flame,
  Infinity as InfinityIcon,
  Moon,
  Mountain,
  Network,
  Plus,
  Settings2,
  Sparkles,
  Trees,
} from 'lucide-react'
import { api } from '@/lib/api'
import { CLUSTER_ID_RE } from '@/lib/dst'
import { cn } from '@/lib/utils'
import {
  buildOverrideDiff,
  countModifiedAgainstBaseline,
  extractPresetOverrides,
  getPlaystyleById,
  PLAYSTYLES,
  suggestClusterId,
  type PlaystyleId,
} from '@/lib/worldHelpers'
import { useRegisterUnsaved } from '@/hooks/useUnsavedGuard'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Input, Label, SecretInput } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { CopyButton } from '@/components/ui/copy-button'
import { TabList } from '@/components/ui/tabs'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { PageCardsSkeleton } from '@/components/ui/skeleton'
import { WorldOptionsPane } from '@/components/WorldOptionsPane'

type WizardStep = 'playstyle' | 'configure'
type ConfigTab = 'settings' | 'forest' | 'caves'
type ServerNetworkMode = 'online' | 'lan' | 'offline'

const INTENTIONS = ['cooperative', 'social', 'competitive', 'madness'] as const
type Intention = (typeof INTENTIONS)[number]

function PlaystyleIcon({ id, className }: { id: PlaystyleId; className?: string }) {
  switch (id) {
    case 'relaxed':
      return <Feather className={className} />
    case 'endless':
      return <InfinityIcon className={className} />
    case 'survival':
      return <Flame className={className} />
    case 'wilderness':
      return <Compass className={className} />
    case 'lightsout':
      return <Moon className={className} />
  }
}

function InlineStepper<T extends string | number>({
  ariaLabel,
  options,
  value,
  onChange,
}: {
  ariaLabel: string
  options: { value: T; label: string }[]
  value: T
  onChange: (next: T) => void
}) {
  const idx = options.findIndex((o) => o.value === value)
  const canPrev = idx > 0
  const canNext = idx >= 0 && idx < options.length - 1

  return (
    <div
      role="group"
      aria-label={ariaLabel}
      className="flex h-9 items-center justify-between rounded-lg border border-input bg-[var(--surface-input)] px-1"
    >
      <button
        type="button"
        disabled={!canPrev}
        onClick={() => canPrev && onChange(options[idx - 1].value)}
        aria-label={`${ariaLabel} previous`}
        className={cn(
          'flex h-7 w-8 items-center justify-center rounded-md transition-colors',
          canPrev
            ? 'text-foreground hover:bg-secondary cursor-pointer'
            : 'text-muted-foreground/35 cursor-not-allowed',
        )}
      >
        <ChevronLeft className="h-4 w-4" />
      </button>

      <span className="px-2 text-sm font-medium text-foreground truncate">
        {idx >= 0 ? options[idx].label : String(value)}
      </span>

      <button
        type="button"
        disabled={!canNext}
        onClick={() => canNext && onChange(options[idx + 1].value)}
        aria-label={`${ariaLabel} next`}
        className={cn(
          'flex h-7 w-8 items-center justify-center rounded-md transition-colors',
          canNext
            ? 'text-foreground hover:bg-secondary cursor-pointer'
            : 'text-muted-foreground/35 cursor-not-allowed',
        )}
      >
        <ChevronRight className="h-4 w-4" />
      </button>
    </div>
  )
}

export function CreateClusterPage() {
  const { t, i18n } = useTranslation()
  const isEn = i18n.language === 'en-US'
  const navigate = useNavigate()
  const qc = useQueryClient()

  const fallbackSuffixRef = useRef(String(Math.floor(1000 + Math.random() * 9000)))
  const clusterNameRef = useRef<HTMLInputElement | null>(null)
  const clusterIdRef = useRef<HTMLInputElement | null>(null)

  const [step, setStep] = useState<WizardStep>('playstyle')
  const [activeTab, setActiveTab] = useState<ConfigTab>('settings')

  // Step 1: Playstyle & presets
  const [playstyleId, setPlaystyleId] = useState<PlaystyleId>('endless')
  const [pendingStyleSwitch, setPendingStyleSwitch] = useState<PlaystyleId | null>(null)

  // Step 2: Settings tab fields
  const [clusterName, setClusterName] = useState('')
  const [clusterId, setClusterId] = useState('')
  const [clusterIdTouched, setClusterIdTouched] = useState(false)
  const [description, setDescription] = useState('')
  const [intention, setIntention] = useState<Intention>('cooperative')
  const [pvp, setPvp] = useState(false)
  const [maxPlayers, setMaxPlayers] = useState(6)
  const [password, setPassword] = useState('')
  const [networkMode, setNetworkMode] = useState<ServerNetworkMode>('online')
  const [pauseWhenEmpty, setPauseWhenEmpty] = useState(true)
  const [autoRestart, setAutoRestart] = useState(true)
  const [clusterToken, setClusterToken] = useState('')

  // Step 2: Forest & Caves world overrides
  const [cavesEnabled, setCavesEnabled] = useState(true)
  const [masterOverrides, setMasterOverrides] = useState<Record<string, string>>({})
  const [cavesOverrides, setCavesOverrides] = useState<Record<string, string>>({})
  const [overridesInitialized, setOverridesInitialized] = useState(false)

  const { data: presetsData, isLoading: presetsLoading } = useQuery({
    queryKey: ['presets'],
    queryFn: api.presets,
  })
  const { data: schemaData, isLoading: schemaLoading } = useQuery({
    queryKey: ['world-schema'],
    queryFn: api.worldSchema,
  })
  const { data: portsData } = useQuery({
    queryKey: ['next-ports', 'create-page'],
    queryFn: api.previewNextPorts,
  })

  const masterPresets = useMemo(
    () => (presetsData?.presets ?? []).filter((p) => p.shard === 'master'),
    [presetsData],
  )
  const cavesPresets = useMemo(
    () => (presetsData?.presets ?? []).filter((p) => p.shard === 'caves'),
    [presetsData],
  )

  const selectedPlaystyle = useMemo(() => getPlaystyleById(playstyleId), [playstyleId])

  const selectedMasterPreset = useMemo(
    () => masterPresets.find((p) => p.id === selectedPlaystyle.presetId) ?? masterPresets[0] ?? null,
    [masterPresets, selectedPlaystyle.presetId],
  )
  const selectedCavesPreset = useMemo(() => cavesPresets[0] ?? null, [cavesPresets])

  const masterBaseline = useMemo(
    () => (selectedMasterPreset ? extractPresetOverrides(selectedMasterPreset.lua) : {}),
    [selectedMasterPreset],
  )
  const cavesBaseline = useMemo(
    () => (selectedCavesPreset ? extractPresetOverrides(selectedCavesPreset.lua) : {}),
    [selectedCavesPreset],
  )

  // Seed initial overrides once presets load
  useEffect(() => {
    if (overridesInitialized || !selectedMasterPreset) return
    setMasterOverrides(extractPresetOverrides(selectedMasterPreset.lua))
    if (selectedCavesPreset) {
      setCavesOverrides(extractPresetOverrides(selectedCavesPreset.lua))
    }
    setOverridesInitialized(true)
  }, [overridesInitialized, selectedMasterPreset, selectedCavesPreset])

  const masterSchema = schemaData?.master ?? []
  const cavesSchema = schemaData?.caves ?? []

  const masterModifiedCount = useMemo(
    () => countModifiedAgainstBaseline(masterSchema, masterOverrides, masterBaseline),
    [masterSchema, masterOverrides, masterBaseline],
  )
  const cavesModifiedCount = useMemo(
    () => (cavesEnabled ? countModifiedAgainstBaseline(cavesSchema, cavesOverrides, cavesBaseline) : 0),
    [cavesEnabled, cavesSchema, cavesOverrides, cavesBaseline],
  )

  const clusterIdInvalid = clusterId.trim().length > 0 && !CLUSTER_ID_RE.test(clusterId.trim())

  const hasUnsavedWork =
    step === 'configure' &&
    (clusterName.trim().length > 0 ||
      description.trim().length > 0 ||
      masterModifiedCount > 0 ||
      cavesModifiedCount > 0)

  const applyPlaystyle = (nextId: PlaystyleId) => {
    setPlaystyleId(nextId)
    const def = getPlaystyleById(nextId)
    const preset = masterPresets.find((p) => p.id === def.presetId)
    setMasterOverrides(preset ? extractPresetOverrides(preset.lua) : {})
    setPendingStyleSwitch(null)
  }

  const requestPlaystyleChange = (nextId: PlaystyleId) => {
    if (nextId === playstyleId) return
    if (step === 'configure' && masterModifiedCount > 0) {
      setPendingStyleSwitch(nextId)
      return
    }
    applyPlaystyle(nextId)
  }

  const handleClusterNameChange = (nextName: string) => {
    setClusterName(nextName)
    if (!clusterIdTouched) {
      if (nextName.trim()) {
        setClusterId(suggestClusterId(nextName, fallbackSuffixRef.current))
      } else {
        setClusterId('')
      }
    }
  }

  const createMut = useMutation({
    mutationFn: api.createCluster,
    onSuccess: (res) => {
      toast.success(t('createWorld.createdToast', { name: res.cluster.cluster_ini.cluster_name }))
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      navigate(`/clusters/${encodeURIComponent(res.cluster.cluster_id)}/overview`, { replace: true })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  useRegisterUnsaved('create-world-wizard', hasUnsavedWork && !createMut.isSuccess)

  const handleGenerateWorld = () => {
    const trimmedName = clusterName.trim()
    const trimmedId = clusterId.trim()

    if (!trimmedName) {
      setActiveTab('settings')
      toast.error(t('createWorld.nameRequired'))
      window.setTimeout(() => clusterNameRef.current?.focus(), 30)
      return
    }
    if (!trimmedId || !CLUSTER_ID_RE.test(trimmedId)) {
      setActiveTab('settings')
      toast.error(t('createCluster.clusterIdInvalid'))
      window.setTimeout(() => clusterIdRef.current?.focus(), 30)
      return
    }

    const masterDiff = buildOverrideDiff(masterSchema, masterOverrides, masterBaseline)
    const cavesDiff = cavesEnabled ? buildOverrideDiff(cavesSchema, cavesOverrides, cavesBaseline) : {}

    createMut.mutate({
      cluster_id: trimmedId,
      cluster_name: trimmedName,
      cluster_description: description.trim(),
      cluster_password: password,
      game_mode: selectedPlaystyle.gameMode,
      max_players: Math.min(64, Math.max(1, Number(maxPlayers) || 6)),
      pvp,
      pause_when_empty: pauseWhenEmpty,
      cluster_intention: intention,
      offline_cluster: networkMode === 'offline',
      lan_only_cluster: networkMode === 'lan',
      caves_enabled: cavesEnabled,
      master_preset: selectedPlaystyle.presetId,
      caves_preset: 'caves_default',
      master_overrides: Object.keys(masterDiff).length > 0 ? masterDiff : undefined,
      caves_overrides: cavesEnabled && Object.keys(cavesDiff).length > 0 ? cavesDiff : undefined,
      cluster_token: clusterToken.trim(),
      auto_restart: autoRestart,
    })
  }

  if (presetsLoading || schemaLoading) {
    return (
      <div className="space-y-6">
        <PageCardsSkeleton count={2} />
      </div>
    )
  }

  // =========================================================================
  // STEP 1: Choose Server Playstyle (mirrors DST Screenshot 1)
  // =========================================================================
  if (step === 'playstyle') {
    return (
      <div className="mx-auto max-w-5xl space-y-6 pb-8">
        <div className="flex items-center justify-between gap-3">
          <Button type="button" size="sm" variant="ghost" onClick={() => navigate('/')}>
            <ArrowLeft className="h-4 w-4" />
            {t('createWorld.backToDashboard')}
          </Button>
          <span className="text-xs text-muted-foreground font-mono">1 / 2</span>
        </div>

        <div className="rounded-2xl border border-border bg-card p-6 sm:p-8 space-y-7 shadow-xs">
          <div className="text-center space-y-1.5">
            <h1 className="text-xl sm:text-2xl font-semibold tracking-tight text-foreground">
              {t('createWorld.choosePlaystyleTitle')}
            </h1>
            <p className="text-xs sm:text-sm text-muted-foreground">
              {t('createWorld.choosePlaystyleSubtitle')}
            </p>
          </div>

          {/* 5 Playstyle Cards */}
          <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-3">
            {PLAYSTYLES.map((style) => {
              const active = style.id === playstyleId
              const title = isEn ? style.nameEn : style.nameZh
              const tags = isEn ? style.tagsEn : style.tagsZh
              return (
                <button
                  key={style.id}
                  type="button"
                  aria-pressed={active}
                  onClick={() => applyPlaystyle(style.id)}
                  onDoubleClick={() => {
                    applyPlaystyle(style.id)
                    setStep('configure')
                  }}
                  className={cn(
                    'group relative flex flex-col rounded-xl border p-3.5 text-left transition-all duration-150 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                    active
                      ? 'border-foreground/50 bg-secondary/45 ring-2 ring-foreground/15 shadow-sm -translate-y-0.5'
                      : 'border-border bg-card hover:border-border-strong hover:bg-secondary/25',
                  )}
                >
                  {/* Visual Crest Area */}
                  <div
                    className={cn(
                      'relative flex h-28 w-full flex-col items-center justify-center rounded-lg border bg-gradient-to-b mb-3 overflow-hidden',
                      style.accentClass,
                    )}
                  >
                    <PlaystyleIcon
                      id={style.id}
                      className={cn(
                        'h-9 w-9 transition-transform duration-200',
                        active ? 'scale-110 text-foreground' : 'text-foreground/75 group-hover:scale-105',
                      )}
                    />
                    <span className="mt-2 text-[10px] font-mono uppercase tracking-widest text-muted-foreground">
                      {style.nameEn}
                    </span>

                    {active && (
                      <span className="absolute right-2 top-2 flex h-5 w-5 items-center justify-center rounded-full bg-primary text-primary-foreground shadow-xs">
                        <Check className="h-3 w-3" />
                      </span>
                    )}
                  </div>

                  {/* Card Title */}
                  <div className="text-center space-y-2 flex-1 flex flex-col justify-between">
                    <div className="text-sm font-semibold text-foreground">{title}</div>
                    <div className="flex flex-wrap items-center justify-center gap-1">
                      {tags.map((tag) => (
                        <span
                          key={tag}
                          className={cn(
                            'inline-block rounded-md border px-1.5 py-0.5 text-[10px] font-medium leading-tight',
                            active ? style.badgeClass : 'border-border bg-secondary/60 text-muted-foreground',
                          )}
                        >
                          {tag}
                        </span>
                      ))}
                    </div>
                  </div>
                </button>
              )
            })}
          </div>

          {/* Selected Playstyle Detail Description (mirrors DST bottom caption) */}
          <div className="mx-auto max-w-2xl rounded-xl border border-border bg-secondary/35 px-5 py-4 text-center space-y-1.5">
            <div className="inline-flex items-center gap-2 text-xs font-semibold text-foreground">
              <PlaystyleIcon id={selectedPlaystyle.id} className="h-3.5 w-3.5" />
              <span>{isEn ? selectedPlaystyle.nameEn : selectedPlaystyle.nameZh}</span>
            </div>
            <p className="text-xs sm:text-sm leading-relaxed text-muted-foreground">
              {isEn ? selectedPlaystyle.detailDescEn : selectedPlaystyle.detailDescZh}
            </p>
          </div>

          {/* Step 1 Footer */}
          <div className="flex flex-wrap items-center justify-between gap-3 pt-2 border-t border-border">
            <Button type="button" variant="outline" onClick={() => navigate('/')}>
              {t('common.cancel')}
            </Button>
            <Button type="button" onClick={() => setStep('configure')}>
              <span>{t('createWorld.nextConfigure')}</span>
              <ArrowRight className="h-4 w-4" />
            </Button>
          </div>
        </div>
      </div>
    )
  }

  // =========================================================================
  // STEP 2: Tabbed World Creation Workbench (mirrors DST Screenshots 2, 3, 4)
  // =========================================================================
  const topTabs: {
    key: ConfigTab
    label: string
    icon: React.ReactNode
    badge?: string | number
  }[] = [
    {
      key: 'settings',
      label: t('createWorld.tabSettings'),
      icon: <Settings2 className="h-3.5 w-3.5" />,
    },
    {
      key: 'forest',
      label: t('createWorld.tabForest'),
      icon: <Trees className="h-3.5 w-3.5" />,
      badge: masterModifiedCount > 0 ? masterModifiedCount : undefined,
    },
    {
      key: 'caves',
      label: t('createWorld.tabCaves'),
      icon: <Mountain className="h-3.5 w-3.5" />,
      badge: !cavesEnabled
        ? t('createWorld.cavesOffBadge')
        : cavesModifiedCount > 0
          ? cavesModifiedCount
          : undefined,
    },
  ]

  const totalCustomCount = masterModifiedCount + cavesModifiedCount
  const styleLabel = isEn ? selectedPlaystyle.nameEn : selectedPlaystyle.nameZh

  return (
    <div className="space-y-4 pb-6">
      {/* Top Bar: DST-style Main Tabs (Settings | Forest | Caves) */}
      <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-3">
        <div className="space-y-0.5">
          <div className="flex items-center gap-2">
            <h1 className="text-lg font-semibold tracking-tight text-foreground">
              {t('createWorld.pageTitle')}
            </h1>
            <Badge variant="muted" className="text-[11px] font-medium">
              {styleLabel}
            </Badge>
          </div>
          <p className="text-xs text-muted-foreground">{t('createWorld.pageSubtitle')}</p>
        </div>

        <TabList
          ariaLabel={t('createWorld.pageTitle')}
          variant="segmented"
          idPrefix="create-world"
          value={activeTab}
          onChange={setActiveTab}
          items={topTabs}
        />
      </div>

      {createMut.error && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-red-600/35 bg-red-600/10 px-4 py-3 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
        >
          <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
          <span className="break-words">{createMut.error.message}</span>
        </div>
      )}

      {/* TAB 1: SETTINGS (mirrors DST Screenshot 2) */}
      {activeTab === 'settings' && (
        <div id="create-world-panel-settings" role="tabpanel" className="mx-auto max-w-4xl space-y-4">
          {/* World Summary Banner */}
          <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-border bg-card px-4 py-3.5">
            <div className="flex items-center gap-3 min-w-0">
              <div
                className={cn(
                  'flex h-12 w-12 shrink-0 items-center justify-center rounded-lg border bg-gradient-to-b',
                  selectedPlaystyle.accentClass,
                )}
              >
                <PlaystyleIcon id={selectedPlaystyle.id} className="h-5 w-5 text-foreground" />
              </div>
              <div className="min-w-0">
                <div className="text-sm font-semibold text-foreground truncate">
                  {clusterName.trim() || t('createWorld.unnamedWorld')}
                </div>
                <div className="flex flex-wrap items-center gap-2 text-xs text-muted-foreground mt-0.5">
                  <span>{t('createWorld.brandNewWorld')}</span>
                  {clusterId.trim() && (
                    <>
                      <span>·</span>
                      <span className="font-mono text-[11px]">{clusterId.trim()}</span>
                    </>
                  )}
                </div>
              </div>
            </div>

            <div className="flex items-center gap-1.5">
              <Badge variant="secondary" className="gap-1">
                <Trees className="h-3 w-3" />
                {t('createWorld.tabForest')}
              </Badge>
              <Badge variant={cavesEnabled ? 'secondary' : 'muted'} className="gap-1">
                <Mountain className="h-3 w-3" />
                {cavesEnabled ? t('createWorld.tabCaves') : t('createWorld.cavesOffBadge')}
              </Badge>
            </div>
          </div>

          {/* Primary Settings Table (DST row-by-row style) */}
          <div className="rounded-xl border border-border bg-card divide-y divide-border/70">
            {/* Server Name */}
            <div className="grid grid-cols-1 sm:grid-cols-[180px_minmax(0,1fr)] items-center gap-2 sm:gap-4 px-4 sm:px-5 py-3.5">
              <Label htmlFor="cw-name" className="text-xs font-medium text-muted-foreground sm:text-right">
                {t('createCluster.clusterName')} <span className="text-red-500">*</span>
              </Label>
              <Input
                ref={clusterNameRef}
                id="cw-name"
                required
                placeholder={t('createCluster.clusterNamePlaceholder')}
                value={clusterName}
                onChange={(e) => handleClusterNameChange(e.target.value)}
              />
            </div>

            {/* Cluster Directory ID */}
            <div className="grid grid-cols-1 sm:grid-cols-[180px_minmax(0,1fr)] items-start sm:items-center gap-2 sm:gap-4 px-4 sm:px-5 py-3.5">
              <Label htmlFor="cw-cid" className="text-xs font-medium text-muted-foreground sm:text-right">
                {t('createCluster.clusterId')} <span className="text-red-500">*</span>
              </Label>
              <div className="space-y-1">
                <Input
                  ref={clusterIdRef}
                  id="cw-cid"
                  required
                  aria-invalid={clusterIdInvalid}
                  className={cn('font-mono', clusterIdInvalid && 'border-red-600 focus-visible:ring-red-600')}
                  placeholder="friends_s1"
                  value={clusterId}
                  onChange={(e) => {
                    setClusterIdTouched(true)
                    setClusterId(e.target.value)
                  }}
                />
                {clusterIdInvalid ? (
                  <p role="alert" className="text-[11px] font-medium text-red-600 dark:text-red-400">
                    {t('createCluster.clusterIdInvalid')}
                  </p>
                ) : (
                  <p className="text-[11px] text-muted-foreground">{t('createCluster.clusterIdHint')}</p>
                )}
              </div>
            </div>

            {/* Description */}
            <div className="grid grid-cols-1 sm:grid-cols-[180px_minmax(0,1fr)] items-center gap-2 sm:gap-4 px-4 sm:px-5 py-3.5">
              <Label htmlFor="cw-desc" className="text-xs font-medium text-muted-foreground sm:text-right">
                {t('createCluster.description')}
              </Label>
              <Input
                id="cw-desc"
                placeholder={t('createWorld.descriptionPlaceholder')}
                value={description}
                onChange={(e) => setDescription(e.target.value)}
              />
            </div>

            {/* Server Intention (DST radio-style selector row) */}
            <div className="grid grid-cols-1 sm:grid-cols-[180px_minmax(0,1fr)] items-center gap-2 sm:gap-4 px-4 sm:px-5 py-3.5">
              <span className="text-xs font-medium text-muted-foreground sm:text-right">
                {t('createCluster.intention')}
              </span>
              <div className="grid grid-cols-2 sm:grid-cols-4 gap-2" role="radiogroup" aria-label={t('createCluster.intention')}>
                {INTENTIONS.map((item) => {
                  const selected = intention === item
                  return (
                    <button
                      key={item}
                      type="button"
                      role="radio"
                      aria-checked={selected}
                      onClick={() => setIntention(item)}
                      className={cn(
                        'flex items-center justify-center gap-2 rounded-lg border px-3 py-2 text-xs font-medium transition-colors cursor-pointer',
                        selected
                          ? 'border-foreground/45 bg-secondary text-foreground'
                          : 'border-border bg-card text-muted-foreground hover:text-foreground hover:bg-secondary/40',
                      )}
                    >
                      <span
                        className={cn(
                          'h-2.5 w-2.5 rounded-full border transition-colors',
                          selected ? 'border-primary bg-primary' : 'border-muted-foreground/50',
                        )}
                      />
                      <span className="truncate">{t(`config.intentions.${item}`).replace(/\s*\([^)]*\)/, '')}</span>
                    </button>
                  )
                })}
              </div>
            </div>

            {/* Playstyle / Game Mode */}
            <div className="grid grid-cols-1 sm:grid-cols-[180px_minmax(0,1fr)] items-center gap-2 sm:gap-4 px-4 sm:px-5 py-3.5">
              <span className="text-xs font-medium text-muted-foreground sm:text-right">
                {t('createWorld.playstyleField')}
              </span>
              <div className="flex items-center gap-2">
                <div className="flex-1 min-w-0">
                  <InlineStepper
                    ariaLabel={t('createWorld.playstyleField')}
                    value={playstyleId}
                    onChange={(next) => requestPlaystyleChange(next)}
                    options={PLAYSTYLES.map((p) => ({
                      value: p.id,
                      label: isEn ? p.nameEn : p.nameZh,
                    }))}
                  />
                </div>
                <Button
                  type="button"
                  size="sm"
                  variant="outline"
                  onClick={() => setStep('playstyle')}
                  className="shrink-0 h-9"
                >
                  {t('createWorld.chooseStyleCards')}
                </Button>
              </div>
            </div>

            {/* PVP */}
            <div className="grid grid-cols-1 sm:grid-cols-[180px_minmax(0,1fr)] items-center gap-2 sm:gap-4 px-4 sm:px-5 py-3.5">
              <span className="text-xs font-medium text-muted-foreground sm:text-right">
                {t('createWorld.pvpField')}
              </span>
              <InlineStepper
                ariaLabel={t('createWorld.pvpField')}
                value={pvp ? 'on' : 'off'}
                onChange={(v) => setPvp(v === 'on')}
                options={[
                  { value: 'off', label: t('createWorld.off') },
                  { value: 'on', label: t('createWorld.on') },
                ]}
              />
            </div>

            {/* Max Players */}
            <div className="grid grid-cols-1 sm:grid-cols-[180px_minmax(0,1fr)] items-center gap-2 sm:gap-4 px-4 sm:px-5 py-3.5">
              <Label htmlFor="cw-players" className="text-xs font-medium text-muted-foreground sm:text-right">
                {t('createWorld.playersField')}
              </Label>
              <div className="flex h-9 items-center justify-between rounded-lg border border-input bg-[var(--surface-input)] px-1">
                <button
                  type="button"
                  disabled={maxPlayers <= 1}
                  onClick={() => setMaxPlayers((n) => Math.max(1, n - 1))}
                  aria-label="Decrease players"
                  className={cn(
                    'flex h-7 w-8 items-center justify-center rounded-md transition-colors',
                    maxPlayers > 1
                      ? 'text-foreground hover:bg-secondary cursor-pointer'
                      : 'text-muted-foreground/35 cursor-not-allowed',
                  )}
                >
                  <ChevronLeft className="h-4 w-4" />
                </button>
                <input
                  id="cw-players"
                  type="number"
                  min={1}
                  max={64}
                  value={maxPlayers}
                  onChange={(e) => {
                    const parsed = Number(e.target.value)
                    if (!Number.isNaN(parsed)) setMaxPlayers(Math.min(64, Math.max(1, parsed)))
                  }}
                  className="w-20 bg-transparent text-center text-sm font-medium tabular-nums text-foreground focus:outline-none"
                />
                <button
                  type="button"
                  disabled={maxPlayers >= 64}
                  onClick={() => setMaxPlayers((n) => Math.min(64, n + 1))}
                  aria-label="Increase players"
                  className={cn(
                    'flex h-7 w-8 items-center justify-center rounded-md transition-colors',
                    maxPlayers < 64
                      ? 'text-foreground hover:bg-secondary cursor-pointer'
                      : 'text-muted-foreground/35 cursor-not-allowed',
                  )}
                >
                  <ChevronRight className="h-4 w-4" />
                </button>
              </div>
            </div>

            {/* Password */}
            <div className="grid grid-cols-1 sm:grid-cols-[180px_minmax(0,1fr)] items-center gap-2 sm:gap-4 px-4 sm:px-5 py-3.5">
              <Label htmlFor="cw-pwd" className="text-xs font-medium text-muted-foreground sm:text-right">
                {t('createWorld.passwordField')}
              </Label>
              <Input
                id="cw-pwd"
                type="text"
                placeholder={t('createWorld.passwordPlaceholder')}
                value={password}
                onChange={(e) => setPassword(e.target.value)}
              />
            </div>

            {/* Server Mode (Online / LAN / Offline) */}
            <div className="grid grid-cols-1 sm:grid-cols-[180px_minmax(0,1fr)] items-center gap-2 sm:gap-4 px-4 sm:px-5 py-3.5">
              <span className="text-xs font-medium text-muted-foreground sm:text-right">
                {t('createWorld.serverModeField')}
              </span>
              <InlineStepper
                ariaLabel={t('createWorld.serverModeField')}
                value={networkMode}
                onChange={setNetworkMode}
                options={[
                  { value: 'online', label: t('createWorld.serverModeOnline') },
                  { value: 'lan', label: t('createWorld.serverModeLan') },
                  { value: 'offline', label: t('createWorld.serverModeOffline') },
                ]}
              />
            </div>
          </div>

          {/* Dedicated Server Ops & Ports Section */}
          <div className="rounded-xl border border-border bg-card p-4 sm:p-5 space-y-4">
            <div className="flex items-center justify-between gap-2">
              <h2 className="text-xs font-semibold tracking-wide text-foreground">
                {t('createWorld.dedicatedOpsTitle')}
              </h2>
              <span className="text-[11px] text-muted-foreground">{t('createWorld.dedicatedOpsHint')}</span>
            </div>

            <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
              <label
                htmlFor="cw-pause"
                className="flex items-center justify-between gap-3 rounded-lg border border-border bg-secondary/30 px-3.5 py-2.5 cursor-pointer"
              >
                <span className="text-xs font-medium text-foreground">{t('createCluster.pauseWhenEmpty')}</span>
                <Switch
                  id="cw-pause"
                  checked={pauseWhenEmpty}
                  onCheckedChange={setPauseWhenEmpty}
                  ariaLabel={t('createCluster.pauseWhenEmpty')}
                />
              </label>

              <label
                htmlFor="cw-autorestart"
                className="flex items-center justify-between gap-3 rounded-lg border border-border bg-secondary/30 px-3.5 py-2.5 cursor-pointer"
              >
                <span className="text-xs font-medium text-foreground">{t('createCluster.autoRestart')}</span>
                <Switch
                  id="cw-autorestart"
                  checked={autoRestart}
                  onCheckedChange={setAutoRestart}
                  ariaLabel={t('createCluster.autoRestart')}
                />
              </label>
            </div>

            <div className="space-y-1.5">
              <Label htmlFor="cw-token">{t('createCluster.clusterToken')}</Label>
              <SecretInput
                id="cw-token"
                placeholder="pds-g^KU_..."
                value={clusterToken}
                onChange={(e) => setClusterToken(e.target.value)}
              />
            </div>

            {portsData?.ports && (
              <div className="rounded-lg border border-border bg-secondary/40 px-3.5 py-2.5 text-xs space-y-1">
                <div className="flex items-center gap-1.5 font-medium text-foreground">
                  <Network className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
                  <span>{t('createCluster.allocatedPorts')}</span>
                </div>
                <div className="flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-xs text-muted-foreground tabular-nums">
                  <span className="inline-flex items-center gap-0.5">
                    Master :{portsData.ports.master_server_port}
                    <CopyButton value={String(portsData.ports.master_server_port)} />
                  </span>
                  {cavesEnabled && (
                    <span className="inline-flex items-center gap-0.5">
                      Caves :{portsData.ports.caves_server_port}
                      <CopyButton value={String(portsData.ports.caves_server_port)} />
                    </span>
                  )}
                  <span>ShardBus :{portsData.ports.cluster_master_port}</span>
                </div>
              </div>
            )}
          </div>
        </div>
      )}

      {/* TAB 2: FOREST (mirrors DST Screenshots 3 & 4) */}
      {activeTab === 'forest' && (
        <div id="create-world-panel-forest" role="tabpanel">
          <WorldOptionsPane
            shard="master"
            schema={masterSchema}
            presets={masterPresets}
            selectedPresetId={selectedPlaystyle.presetId}
            presetTitle={
              selectedMasterPreset
                ? isEn
                  ? selectedMasterPreset.name_en
                  : selectedMasterPreset.name_zh
                : styleLabel
            }
            presetDescription={isEn ? selectedPlaystyle.shortDescEn : selectedPlaystyle.shortDescZh}
            playstyleLabel={styleLabel}
            baselineOverrides={masterBaseline}
            overrides={masterOverrides}
            onChangeOverride={(k, v) => setMasterOverrides((prev) => ({ ...prev, [k]: v }))}
            onResetAll={() => setMasterOverrides({ ...masterBaseline })}
            onSelectPreset={(presetId) => {
              const targetStyle = PLAYSTYLES.find((p) => p.presetId === presetId)
              if (targetStyle) requestPlaystyleChange(targetStyle.id)
            }}
            onSwitchPlaystyle={() => setStep('playstyle')}
          />
        </div>
      )}

      {/* TAB 3: CAVES (mirrors DST Caves Tab + Add/Remove Caves) */}
      {activeTab === 'caves' && (
        <div id="create-world-panel-caves" role="tabpanel">
          {!cavesEnabled ? (
            <div className="mx-auto max-w-xl rounded-2xl border border-dashed border-border-strong bg-card p-8 sm:p-10 text-center space-y-4">
              <div className="mx-auto flex h-14 w-14 items-center justify-center rounded-2xl bg-secondary text-foreground">
                <Mountain className="h-7 w-7" />
              </div>
              <div className="space-y-1.5">
                <h2 className="text-base font-semibold text-foreground">{t('createWorld.cavesEmptyTitle')}</h2>
                <p className="text-xs sm:text-sm text-muted-foreground leading-relaxed">
                  {t('createWorld.cavesEmptyDesc')}
                </p>
              </div>
              <Button
                type="button"
                onClick={() => {
                  setCavesEnabled(true)
                  if (Object.keys(cavesOverrides).length === 0) {
                    setCavesOverrides({ ...cavesBaseline })
                  }
                }}
              >
                <Plus className="h-4 w-4" />
                {t('createWorld.addCaves')}
              </Button>
            </div>
          ) : (
            <WorldOptionsPane
              shard="caves"
              schema={cavesSchema}
              presets={cavesPresets}
              selectedPresetId="caves_default"
              presetTitle={
                selectedCavesPreset
                  ? isEn
                    ? selectedCavesPreset.name_en
                    : selectedCavesPreset.name_zh
                  : t('createWorld.tabCaves')
              }
              presetDescription={t('createWorld.cavesPresetDesc')}
              baselineOverrides={cavesBaseline}
              overrides={cavesOverrides}
              onChangeOverride={(k, v) => setCavesOverrides((prev) => ({ ...prev, [k]: v }))}
              onResetAll={() => setCavesOverrides({ ...cavesBaseline })}
              onRemoveCaves={() => setCavesEnabled(false)}
            />
          )}
        </div>
      )}

      {/* Sticky Bottom Action Bar (mirrors DST bottom-left Back & bottom-right Generate World) */}
      <div className="sticky bottom-4 z-20 flex items-center justify-between gap-3 rounded-xl border border-[var(--border-strong)] bg-popover/95 backdrop-blur-md px-4 py-3 shadow-[0_14px_34px_-10px_rgba(0,0,0,0.28)] dark:shadow-[0_14px_34px_-10px_rgba(0,0,0,0.55)]">
        <Button type="button" variant="outline" onClick={() => setStep('playstyle')}>
          <ArrowLeft className="h-4 w-4" />
          {t('createWorld.backBtn')}
        </Button>

        <div className="hidden md:flex items-center gap-2 text-xs text-muted-foreground truncate">
          <span className="font-medium text-foreground">{styleLabel}</span>
          <span>·</span>
          <span>
            {cavesEnabled ? t('createWorld.summaryDualShard') : t('createWorld.summaryForestOnly')}
          </span>
          <span>·</span>
          <span className="tabular-nums">{t('createWorld.summaryPlayers', { count: maxPlayers })}</span>
          {totalCustomCount > 0 && (
            <>
              <span>·</span>
              <span className="inline-flex items-center gap-1 text-amber-600 dark:text-amber-400 font-medium tabular-nums">
                <Sparkles className="h-3 w-3" />
                {t('createWorld.summaryCustomCount', { count: totalCustomCount })}
              </span>
            </>
          )}
        </div>

        <Button
          type="button"
          disabled={createMut.isPending}
          onClick={handleGenerateWorld}
          className="min-w-[128px]"
        >
          {createMut.isPending ? t('createWorld.generating') : t('createWorld.generateWorld')}
        </Button>
      </div>

      {/* Confirm playstyle switch when custom forest tweaks exist */}
      <ConfirmDialog
        open={pendingStyleSwitch !== null}
        onClose={() => setPendingStyleSwitch(null)}
        onConfirm={() => {
          if (pendingStyleSwitch) applyPlaystyle(pendingStyleSwitch)
        }}
        title={t('createWorld.switchPlaystyleTitle')}
        description={t('createWorld.switchPlaystyleDesc', { count: masterModifiedCount })}
        confirmLabel={t('createWorld.switchPlaystyleConfirm')}
        tone="warning"
      />
    </div>
  )
}
