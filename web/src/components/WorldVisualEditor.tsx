import { useCallback, useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertTriangle,
  ChevronDown,
  Code2,
  Filter,
  RotateCcw,
  RotateCw,
  Save,
  Sliders,
  Sparkles,
} from 'lucide-react'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { getWorldOptionIcon } from '@/lib/worldIcons'
import { useRegisterUnsaved } from '@/hooks/useUnsavedGuard'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Select } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { TabList } from '@/components/ui/tabs'
import { CollapsibleContent } from '@/components/ui/collapsible'
import { SearchInput } from '@/components/SearchInput'
import { StickySaveBar } from '@/components/StickySaveBar'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { PageCardsSkeleton } from '@/components/ui/skeleton'
import type { WorldOptionField, WorldOptionGroup, WorldOptionSection, WorldPreset } from '@/types/api'

interface Props {
  clusterId: string
  shard: 'master' | 'caves'
  canOperate: boolean
  presets: WorldPreset[]
  isClusterRunning?: boolean
  onRestartCluster?: () => void
  restarting?: boolean
  onDirtyChange?: (dirty: boolean) => void
  onSaved?: () => void
}

const CUSTOMIZED_PRESET_VALUE = '__customized__'

type SectionFilter = 'all' | WorldOptionSection

const SECTION_ORDER: WorldOptionSection[] = ['settings', 'worldgen']

/**
 * Resolves an option value against its choices.
 * Legacy configs may store "default" for DST enabled/disabled options (whose real
 * choices are "none" / "always"); DST treats "default" as the option's default.
 */
function resolveOptionValue(opt: WorldOptionField, raw: unknown): string {
  if (raw === undefined || raw === null) return opt.default
  const str = String(raw)
  if (str === 'default' && opt.choices.every((ch) => ch.value !== 'default')) {
    return opt.default
  }
  return str
}

/**
 * Mirrors DST scripts/map/customize.lua option sorting within a group:
 * items with an explicit client `order` come first in ascending order,
 * followed by unordered items sorted by the active client language label.
 */
function sortClientOptions(options: WorldOptionField[], isEn: boolean): WorldOptionField[] {
  return [...options].sort((a, b) => {
    const ao = a.order ?? 0
    const bo = b.order ?? 0
    if (ao !== bo) {
      if (ao === 0) return 1
      if (bo === 0) return -1
      return ao - bo
    }
    const la = isEn ? a.label_en : a.label_zh
    const lb = isEn ? b.label_en : b.label_zh
    return la < lb ? -1 : la > lb ? 1 : 0
  })
}

function extractPresetLevelId(lua: string): string {
  return lua.match(/^\s*id\s*=\s*["']([^"']+)["']/m)?.[1] ?? ''
}

function extractPresetOverrides(lua: string): Record<string, string> {
  const block = lua.match(/overrides\s*=\s*\{([\s\S]*?)\}/)?.[1]
  if (!block) return {}
  const out: Record<string, string> = {}
  for (const m of block.matchAll(/([A-Za-z0-9_]+)\s*=\s*(?:"([^"]*)"|'([^']*)'|(true|false|-?\d+(?:\.\d+)?))/g)) {
    out[m[1]] = m[2] ?? m[3] ?? m[4] ?? ''
  }
  return out
}

function WorldOptionIcon({ optionKey, modified }: { optionKey: string; modified: boolean }) {
  const src = getWorldOptionIcon(optionKey)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    setFailed(false)
  }, [optionKey])

  if (!src || failed) {
    return (
      <div
        aria-hidden="true"
        className={cn(
          'flex h-12 w-12 shrink-0 items-center justify-center rounded-md bg-secondary/60 text-muted-foreground ring-1',
          modified ? 'ring-primary/45' : 'ring-border',
        )}
      >
        <Sliders className="h-4 w-4" />
      </div>
    )
  }

  return (
    <div
      aria-hidden="true"
      className={cn(
        'h-12 w-12 shrink-0 overflow-hidden rounded-md bg-[#1d1814] shadow-xs ring-1 transition-shadow',
        modified ? 'ring-primary/50' : 'ring-black/20 dark:ring-white/12',
      )}
    >
      <img
        src={src}
        alt=""
        width={48}
        height={48}
        loading="lazy"
        decoding="async"
        draggable={false}
        onError={() => setFailed(true)}
        className="h-full w-full object-fill select-none"
      />
    </div>
  )
}

export function WorldVisualEditor({
  clusterId,
  shard,
  canOperate,
  presets,
  isClusterRunning = false,
  onRestartCluster,
  restarting = false,
  onDirtyChange,
  onSaved,
}: Props) {
  const { t, i18n } = useTranslation()
  const qc = useQueryClient()
  const isEn = i18n.language === 'en-US'

  const [mode, setMode] = useState<'visual' | 'source'>('visual')
  const [pendingMode, setPendingMode] = useState<'visual' | 'source' | null>(null)
  const [mobileFiltersOpen, setMobileFiltersOpen] = useState(false)
  const [activeSection, setActiveSection] = useState<SectionFilter>('all')
  const [activeGroup, setActiveGroup] = useState<string>('all')
  const [onlyModified, setOnlyModified] = useState(false)
  const [search, setSearch] = useState('')
  const [draftOverrides, setDraftOverrides] = useState<Record<string, unknown>>({})
  const [initialOverridesJson, setInitialOverridesJson] = useState('{}')
  const [rawLua, setRawLua] = useState('')
  const [initialRawLua, setInitialRawLua] = useState('')
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({})
  const [pendingPreset, setPendingPreset] = useState<WorldPreset | null>(null)
  const [justSaved, setJustSaved] = useState(false)
  const [syncNonce, setSyncNonce] = useState(0)

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['world-lua', clusterId, shard],
    queryFn: () => api.readWorldLua(clusterId, shard),
    enabled: !!clusterId,
  })

  useEffect(() => {
    if (!data) return
    const parsedOverrides = { ...(data.parsed?.overrides ?? {}) }
    setRawLua(data.lua)
    setInitialRawLua(data.lua)
    setDraftOverrides(parsedOverrides)
    setInitialOverridesJson(JSON.stringify(parsedOverrides))
    setJustSaved(false)
    if (data.schema.length > 0) {
      const defaults: Record<string, boolean> = {}
      data.schema.forEach((g, idx) => {
        const hasMod = g.options.some((o) => resolveOptionValue(o, parsedOverrides[o.key]) !== o.default)
        defaults[g.id] = idx === 0 || hasMod
      })
      setExpandedGroups(defaults)
    }
  }, [data, syncNonce])

  const isVisualDirty = useMemo(
    () => JSON.stringify(draftOverrides) !== initialOverridesJson,
    [draftOverrides, initialOverridesJson],
  )
  const isSourceDirty = rawLua !== initialRawLua
  const isDirty = mode === 'visual' ? isVisualDirty : isSourceDirty
  const anyDirty = isVisualDirty || isSourceDirty

  useEffect(() => {
    onDirtyChange?.(anyDirty)
  }, [anyDirty, onDirtyChange])

  useRegisterUnsaved(`world-${clusterId}-${shard}`, anyDirty)

  const requestModeChange = (next: 'visual' | 'source') => {
    if (next === mode) return
    if (isDirty) {
      setPendingMode(next)
      return
    }
    setMode(next)
  }

  const saveVisualMut = useMutation({
    mutationFn: () => api.patchWorldOverrides(clusterId, shard, draftOverrides),
    onSuccess: (res) => {
      setRawLua(res.lua)
      setInitialRawLua(res.lua)
      setInitialOverridesJson(JSON.stringify(draftOverrides))
      setJustSaved(true)
      onSaved?.()
      toast.success(t('common.savedSuccess'))
      void qc.invalidateQueries({ queryKey: ['world-lua', clusterId, shard] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const saveSourceMut = useMutation({
    mutationFn: (lua: string) => api.writeWorldLua(clusterId, shard, lua),
    onSuccess: async (_, savedLua) => {
      setRawLua(savedLua)
      setInitialRawLua(savedLua)
      setJustSaved(true)
      setPendingPreset(null)
      onSaved?.()
      toast.success(t('common.savedSuccess'))
      await qc.invalidateQueries({ queryKey: ['world-lua', clusterId, shard] })
      setSyncNonce((n) => n + 1)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const triggerSave = useCallback(() => {
    if (!canOperate) return
    if (mode === 'visual') {
      if (!saveVisualMut.isPending) saveVisualMut.mutate()
    } else {
      if (!saveSourceMut.isPending) saveSourceMut.mutate(rawLua)
    }
  }, [canOperate, mode, rawLua, saveSourceMut, saveVisualMut])

  // Keyboard Ctrl/Cmd + S support
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        if (!canOperate || !isDirty) return
        e.preventDefault()
        triggerSave()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [canOperate, isDirty, triggerSave])

  const handleDiscard = () => {
    if (!data) return
    const parsedOverrides = { ...(data.parsed?.overrides ?? {}) }
    setDraftOverrides(parsedOverrides)
    setRawLua(data.lua)
  }

  const schema = useMemo<WorldOptionGroup[]>(() => {
    const raw = data?.schema ?? []
    return raw.map((g) => ({
      ...g,
      options: sortClientOptions(g.options, isEn),
    }))
  }, [data?.schema, isEn])

  const sectionSchema = useMemo(
    () => (activeSection === 'all' ? schema : schema.filter((g) => g.section === activeSection)),
    [schema, activeSection],
  )

  const handleSectionChange = (next: SectionFilter) => {
    setActiveSection(next)
    if (activeGroup !== 'all') {
      const stillVisible =
        next === 'all' || schema.some((g) => g.id === activeGroup && g.section === next)
      if (!stillVisible) setActiveGroup('all')
    }
    if (next !== 'all') {
      const targetGroups = schema.filter((g) => g.section === next)
      if (targetGroups.length > 0 && targetGroups.every((g) => !expandedGroups[g.id])) {
        setExpandedGroups((prev) => ({ ...prev, [targetGroups[0].id]: true }))
      }
    }
  }

  const knownKeys = useMemo(() => {
    const s = new Set<string>()
    for (const g of schema) {
      for (const opt of g.options) s.add(opt.key)
    }
    return s
  }, [schema])

  const customPreservedKeys = useMemo(() => {
    return Object.entries(draftOverrides).filter(([k]) => !knownKeys.has(k))
  }, [draftOverrides, knownKeys])

  const matchedPreset = useMemo(() => {
    const levelId = data?.parsed?.id?.trim()
    if (!levelId) return null
    return presets.find((p) => extractPresetLevelId(p.lua) === levelId) ?? null
  }, [data?.parsed?.id, presets])

  const isCustomizedFromPreset = useMemo(() => {
    if (!matchedPreset) return false
    const presetOv = extractPresetOverrides(matchedPreset.lua)
    for (const g of schema) {
      for (const opt of g.options) {
        const cur = resolveOptionValue(opt, draftOverrides[opt.key])
        const expected = resolveOptionValue(opt, presetOv[opt.key])
        if (cur !== expected) return true
      }
    }
    for (const [k, v] of Object.entries(draftOverrides)) {
      if (!knownKeys.has(k) && (presetOv[k] === undefined || String(v) !== presetOv[k])) {
        return true
      }
    }
    for (const [k, pv] of Object.entries(presetOv)) {
      if (!knownKeys.has(k) && String(draftOverrides[k] ?? '') !== pv) {
        return true
      }
    }
    return false
  }, [matchedPreset, schema, draftOverrides, knownKeys])

  const presetSelectValue = matchedPreset
    ? isCustomizedFromPreset
      ? CUSTOMIZED_PRESET_VALUE
      : matchedPreset.id
    : ''

  const modifiedCount = useMemo(() => {
    let c = 0
    for (const g of schema) {
      for (const opt of g.options) {
        if (resolveOptionValue(opt, draftOverrides[opt.key]) !== opt.default) c++
      }
    }
    return c
  }, [schema, draftOverrides])

  const sectionModifiedCount = useMemo(() => {
    let c = 0
    for (const g of sectionSchema) {
      for (const opt of g.options) {
        if (resolveOptionValue(opt, draftOverrides[opt.key]) !== opt.default) c++
      }
    }
    return c
  }, [sectionSchema, draftOverrides])

  const filteredGroups = useMemo(() => {
    const q = search.trim().toLowerCase()
    return sectionSchema
      .filter((g) => activeGroup === 'all' || g.id === activeGroup)
      .map((g) => ({
        ...g,
        options: g.options.filter((o) => {
          const cur = resolveOptionValue(o, draftOverrides[o.key])
          if (onlyModified && cur === o.default) return false
          return (
            !q ||
            o.label_zh.toLowerCase().includes(q) ||
            o.label_en.toLowerCase().includes(q) ||
            o.key.toLowerCase().includes(q)
          )
        }),
      }))
      .filter((g) => g.options.length > 0)
  }, [sectionSchema, activeGroup, search, onlyModified, draftOverrides])

  const groupedBySection = useMemo(() => {
    return SECTION_ORDER.map((section) => ({
      section,
      groups: filteredGroups.filter((g) => g.section === section),
    })).filter((entry) => entry.groups.length > 0)
  }, [filteredGroups])

  const autoExpandAll = activeGroup !== 'all' || onlyModified || search.trim().length > 0
  const allVisibleExpanded = sectionSchema.length > 0 && sectionSchema.every((g) => expandedGroups[g.id])

  if (isError) {
    return <QueryErrorCard error={error} onRetry={() => void refetch()} />
  }

  if (isLoading || !data) {
    return <PageCardsSkeleton count={2} />
  }

  const saving = saveVisualMut.isPending || saveSourceMut.isPending
  const saveError = saveVisualMut.error || saveSourceMut.error

  const presetSelectOptions = (
    <>
      {matchedPreset && isCustomizedFromPreset ? (
        <option value={CUSTOMIZED_PRESET_VALUE} disabled>
          {t('config.presetCustomized', {
            name: isEn ? matchedPreset.name_en : matchedPreset.name_zh,
          })}
        </option>
      ) : (
        <option value="" disabled>
          {t('config.applyPreset')}
        </option>
      )}
      {presets.map((p) => (
        <option key={p.id} value={p.id}>
          {isEn ? p.name_en : p.name_zh}
        </option>
      ))}
    </>
  )

  return (
    <div className="space-y-4">
      {!onSaved && justSaved && isClusterRunning && onRestartCluster && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-card px-3.5 py-2.5 text-[13px]">
          <div className="flex items-center gap-2.5 min-w-0">
            <span className="h-[7px] w-[7px] rounded-full bg-amber-500 shrink-0" aria-hidden="true" />
            <span className="text-foreground/90">{t('config.savedRestartPrompt')}</span>
          </div>
          <Button size="sm" variant="warning" disabled={restarting} onClick={onRestartCluster}>
            <RotateCw className={cn('h-3.5 w-3.5', restarting && 'animate-spin')} />
            {t('config.restartNow')}
          </Button>
        </div>
      )}

      {saveError && (
        <div
          role="alert"
          className="flex items-start gap-2 rounded-xl border border-red-600/35 bg-red-600/10 px-4 py-3 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
        >
          <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
          <span className="break-words">{saveError.message}</span>
        </div>
      )}

      <Card>
        <CardHeader className="space-y-4">
          {/* Row 1: Title/Status + Dual-Mode Toggle + Preset + Save */}
          <div className="flex flex-col lg:flex-row lg:items-center justify-between gap-3">
            <div>
              <CardTitle className="flex items-center gap-2 flex-wrap">
                <span>{shard === 'master' ? t('config.tabMasterWorld') : t('config.tabCavesWorld')}</span>
                {modifiedCount > 0 && (
                  <Badge variant="default" className="tabular-nums">
                    {t('worldVisual.modifiedCount', { count: modifiedCount })}
                  </Badge>
                )}
                {isDirty && <Badge variant="warning">{t('files.unsaved')}</Badge>}
              </CardTitle>
              <CardDescription className="mt-1">
                {shard === 'master' ? 'Master/leveldataoverride.lua' : 'Caves/leveldataoverride.lua'}
                {canOperate && <span className="hidden sm:inline"> · {t('worldVisual.shortcutHint')}</span>}
              </CardDescription>
            </div>

            <div className="flex flex-wrap items-center gap-2">
              <TabList
                ariaLabel={shard === 'master' ? t('config.tabMasterWorld') : t('config.tabCavesWorld')}
                variant="segmented"
                controlsId={`world-editor-panel-${shard}`}
                value={mode}
                onChange={requestModeChange}
                items={[
                  {
                    key: 'visual',
                    label: t('worldVisual.visualMode'),
                    icon: <Sliders className="h-3.5 w-3.5" />,
                  },
                  {
                    key: 'source',
                    label: t('worldVisual.sourceMode'),
                    icon: <Code2 className="h-3.5 w-3.5" />,
                  },
                ]}
              />

              {canOperate && (
                <>
                  <Select
                    aria-label={t('config.applyPreset')}
                    title={t('config.applyPreset')}
                    className="hidden sm:flex w-52 h-8 text-xs"
                    value={presetSelectValue}
                    onChange={(e) => {
                      const found = presets.find((p) => p.id === e.target.value)
                      if (found) setPendingPreset(found)
                    }}
                  >
                    {presetSelectOptions}
                  </Select>

                  <Button size="sm" disabled={saving || !isDirty} onClick={triggerSave}>
                    <Save className="h-3.5 w-3.5" />
                    {saving ? t('common.saving') : mode === 'visual' ? t('worldVisual.saveVisual') : t('config.saveWorld')}
                  </Button>
                </>
              )}
            </div>
          </div>

          {/* Row 2: Client Section Switcher + Visual Filter Toolbar */}
          {mode === 'visual' && (
            <div className="space-y-2.5 pt-3 border-t border-border">
              {/* Primary row: DST client section switcher (All / World Settings / World Generation) + Desktop Search */}
              <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2.5">
                <TabList
                  ariaLabel={t('worldVisual.sectionLabel')}
                  variant="segmented"
                  controlsId={`world-editor-section-${shard}`}
                  value={activeSection}
                  onChange={handleSectionChange}
                  items={[
                    { key: 'all', label: t('worldVisual.sectionAll') },
                    { key: 'settings', label: t('worldVisual.sectionSettings') },
                    { key: 'worldgen', label: t('worldVisual.sectionWorldgen') },
                  ]}
                />

                <SearchInput
                  value={search}
                  onChange={setSearch}
                  placeholder={t('worldVisual.searchOptions')}
                  className="hidden sm:block w-52 shrink-0"
                  inputClassName="h-8"
                />
              </div>

              {/* Mobile compact row: Search + Filter toggle */}
              <div className="flex items-center gap-2 sm:hidden">
                <SearchInput
                  value={search}
                  onChange={setSearch}
                  placeholder={t('worldVisual.searchOptions')}
                  className="flex-1"
                  inputClassName="h-11"
                />
                <Button
                  type="button"
                  size="sm"
                  variant={mobileFiltersOpen || onlyModified || activeGroup !== 'all' ? 'secondary' : 'outline'}
                  aria-expanded={mobileFiltersOpen}
                  onClick={() => setMobileFiltersOpen((v) => !v)}
                  className="shrink-0 h-11"
                >
                  <Filter className="h-3.5 w-3.5" />
                  <span>{t('common.filter')}</span>
                  {(onlyModified || activeGroup !== 'all') && (
                    <span className="h-1.5 w-1.5 rounded-full bg-primary" aria-hidden="true" />
                  )}
                </Button>
              </div>

              <div
                className={cn(
                  'flex-col sm:flex-row sm:items-center justify-between gap-2.5',
                  mobileFiltersOpen ? 'flex' : 'hidden sm:flex',
                )}
              >
                <div className="flex flex-wrap items-center gap-2">
                  {canOperate && (
                    <Select
                      aria-label={t('config.applyPreset')}
                      title={t('config.applyPreset')}
                      className="w-full h-11 text-xs sm:hidden"
                      value={presetSelectValue}
                      onChange={(e) => {
                        const found = presets.find((p) => p.id === e.target.value)
                        if (found) setPendingPreset(found)
                      }}
                    >
                      {presetSelectOptions}
                    </Select>
                  )}

                  <Select
                    aria-label={t('worldVisual.categoryLabel')}
                    value={activeGroup}
                    onChange={(e) => setActiveGroup(e.target.value)}
                    className="w-full sm:w-52 h-11 sm:h-8 text-xs"
                  >
                    <option value="all">{t('worldVisual.allCategories')}</option>
                    {activeSection === 'all'
                      ? SECTION_ORDER.map((section) => {
                          const groups = schema.filter((g) => g.section === section)
                          if (groups.length === 0) return null
                          const secLabel =
                            section === 'settings'
                              ? t('worldVisual.sectionSettings')
                              : t('worldVisual.sectionWorldgen')
                          return (
                            <optgroup key={section} label={secLabel}>
                              {groups.map((g) => (
                                <option key={g.id} value={g.id}>
                                  {isEn ? g.name_en : g.name_zh} ({g.options.length})
                                </option>
                              ))}
                            </optgroup>
                          )
                        })
                      : sectionSchema.map((g) => (
                          <option key={g.id} value={g.id}>
                            {isEn ? g.name_en : g.name_zh} ({g.options.length})
                          </option>
                        ))}
                  </Select>

                  <button
                    type="button"
                    aria-pressed={onlyModified}
                    onClick={() => setOnlyModified((v) => !v)}
                    className={cn(
                      'inline-flex h-11 sm:h-8 items-center gap-1.5 rounded-lg border px-3 text-xs font-medium transition-colors cursor-pointer tabular-nums focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                      onlyModified
                        ? 'border-foreground/25 bg-secondary text-foreground font-semibold ring-1 ring-border'
                        : 'border-border bg-secondary/45 text-muted-foreground hover:text-foreground hover:border-foreground/25',
                    )}
                  >
                    <Filter className="h-3.5 w-3.5" />
                    {t('worldVisual.onlyModified')} ({sectionModifiedCount})
                  </button>

                  {!autoExpandAll && (
                    <Button
                      size="sm"
                      variant="ghost"
                      onClick={() => {
                        setExpandedGroups((prev) => {
                          const next = { ...prev }
                          sectionSchema.forEach((g, idx) => {
                            next[g.id] = allVisibleExpanded ? idx === 0 : true
                          })
                          return next
                        })
                      }}
                    >
                      {allVisibleExpanded ? t('worldVisual.collapseAll') : t('worldVisual.expandAll')}
                    </Button>
                  )}
                </div>
              </div>
            </div>
          )}
        </CardHeader>

        <CardContent id={`world-editor-panel-${shard}`} role="tabpanel">
          {mode === 'visual' ? (
            <div id={`world-editor-section-${shard}`} className="space-y-6">
              {groupedBySection.length === 0 ? (
                <div className="py-10 text-center text-xs text-muted-foreground">
                  {t('worldVisual.noMatchingOptions')}
                </div>
              ) : (
                groupedBySection.map(({ section, groups }) => {
                  const sectionOptCount = groups.reduce((acc, g) => acc + g.options.length, 0)
                  return (
                    <section key={section} className="space-y-3">
                      <div className="flex flex-wrap items-baseline justify-between gap-x-3 gap-y-1 border-b border-border/70 pb-1.5">
                        <div className="flex items-center gap-2">
                          <h2 className="text-xs font-semibold tracking-wide text-foreground">
                            {section === 'settings'
                              ? t('worldVisual.sectionSettings')
                              : t('worldVisual.sectionWorldgen')}
                          </h2>
                          <span className="text-xs text-muted-foreground tabular-nums">({sectionOptCount})</span>
                        </div>
                        <span className="text-[11px] text-muted-foreground">
                          {section === 'settings'
                            ? t('worldVisual.sectionSettingsHint')
                            : t('worldVisual.sectionWorldgenHint')}
                        </span>
                      </div>

                      <div className="space-y-3">
                        {groups.map((g) => {
                          const groupModCount = g.options.filter(
                            (opt) => resolveOptionValue(opt, draftOverrides[opt.key]) !== opt.default,
                          ).length
                          const isOpen = autoExpandAll || Boolean(expandedGroups[g.id])

                          return (
                            <div key={g.id}>
                              <button
                                type="button"
                                aria-expanded={isOpen}
                                onClick={() => setExpandedGroups((prev) => ({ ...prev, [g.id]: !isOpen }))}
                                className="flex w-full items-center justify-between gap-2 rounded-lg bg-secondary/45 px-3.5 py-2.5 text-left hover:bg-secondary/70 transition-colors cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                              >
                                <div className="flex items-center gap-2 min-w-0">
                                  <h3 className="text-xs font-semibold tracking-wide text-foreground">
                                    {isEn ? g.name_en : g.name_zh}
                                  </h3>
                                  <span className="text-xs text-muted-foreground tabular-nums">
                                    ({g.options.length})
                                  </span>
                                  {groupModCount > 0 && (
                                    <Badge variant="default" className="tabular-nums">
                                      {t('worldVisual.modifiedCount', { count: groupModCount })}
                                    </Badge>
                                  )}
                                </div>
                                <ChevronDown
                                  className={cn(
                                    'h-4 w-4 text-muted-foreground transition-transform duration-200 ease-out',
                                    isOpen && 'rotate-180',
                                  )}
                                />
                              </button>

                              <CollapsibleContent open={isOpen}>
                                <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-2.5 pt-2.5 px-1 pb-1">
                                  {g.options.map((opt) => {
                                    const cur = resolveOptionValue(opt, draftOverrides[opt.key])
                                    const isModified = cur !== opt.default
                                    const labelText = isEn ? opt.label_en : opt.label_zh
                                    const hasChoice = opt.choices.some((ch) => ch.value === cur)
                                    return (
                                      <div
                                        key={opt.key}
                                        className={cn(
                                          'flex items-center gap-3 rounded-lg p-2.5 sm:px-3 sm:py-2.5 transition-colors',
                                          isModified
                                            ? 'bg-accent/70 ring-1 ring-primary/35 dark:bg-primary/10 dark:ring-primary/35'
                                            : 'bg-secondary/25 hover:bg-secondary/40',
                                        )}
                                      >
                                        <WorldOptionIcon optionKey={opt.key} modified={isModified} />
                                        <div className="min-w-0 flex-1 space-y-1.5">
                                          <div
                                            className="text-xs font-medium leading-tight truncate"
                                            title={`${labelText} (${opt.key})`}
                                          >
                                            {labelText}
                                          </div>
                                          <div className="flex items-center gap-1.5">
                                            <Select
                                              disabled={!canOperate}
                                              aria-label={labelText}
                                              className="flex-1 min-w-0 h-10 sm:h-8 text-xs"
                                              value={cur}
                                              onChange={(e) =>
                                                setDraftOverrides((prev) => ({
                                                  ...prev,
                                                  [opt.key]: e.target.value,
                                                }))
                                              }
                                            >
                                              {!hasChoice && <option value={cur}>{cur}</option>}
                                              {opt.choices.map((ch) => (
                                                <option key={ch.value} value={ch.value}>
                                                  {isEn ? ch.label_en : ch.label_zh}
                                                </option>
                                              ))}
                                            </Select>
                                            {canOperate && isModified && (
                                              <button
                                                type="button"
                                                aria-label={`${t('worldVisual.resetDefault')} ${labelText}`}
                                                title={t('worldVisual.resetDefault')}
                                                onClick={() =>
                                                  setDraftOverrides((prev) => ({
                                                    ...prev,
                                                    [opt.key]: opt.default,
                                                  }))
                                                }
                                                className="flex h-10 w-10 sm:h-8 sm:w-8 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer pressable"
                                              >
                                                <RotateCcw className="h-3.5 w-3.5" />
                                              </button>
                                            )}
                                          </div>
                                        </div>
                                      </div>
                                    )
                                  })}
                                </div>
                              </CollapsibleContent>
                            </div>
                          )
                        })}
                      </div>
                    </section>
                  )
                })
              )}

              {/* Custom / Mod Extended Overrides Preserved Losslessly */}
              {customPreservedKeys.length > 0 && (
                <div className="rounded-lg bg-sky-600/10 p-4 space-y-2">
                  <div className="flex items-center gap-1.5 text-xs font-medium text-sky-900 dark:text-sky-300">
                    <Sparkles className="h-3.5 w-3.5 shrink-0" />
                    {t('worldVisual.customPreserved')} ({customPreservedKeys.length})
                  </div>
                  <p className="text-xs text-muted-foreground">{t('worldVisual.customPreservedHint')}</p>
                  <div className="flex flex-wrap gap-1.5">
                    {customPreservedKeys.map(([k, v]) => (
                      <Badge key={k} variant="secondary" className="font-mono">
                        {k} = {String(v)}
                      </Badge>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ) : (
            <textarea
              readOnly={!canOperate}
              aria-label={shard === 'master' ? 'Master/leveldataoverride.lua' : 'Caves/leveldataoverride.lua'}
              value={rawLua}
              onChange={(e) => setRawLua(e.target.value)}
              spellCheck={false}
              className="w-full h-[500px] rounded-xl border border-[var(--surface-terminal-border)] bg-[var(--surface-terminal)] p-4 font-mono text-xs leading-relaxed text-slate-200 focus:outline-none focus:ring-2 focus:ring-ring"
            />
          )}
        </CardContent>
      </Card>

      {/* Sticky Bottom Unsaved Changes Bar */}
      <StickySaveBar
        visible={canOperate && isDirty}
        saving={saving}
        saveLabel={mode === 'visual' ? t('worldVisual.saveVisual') : t('config.saveWorld')}
        onDiscard={handleDiscard}
        onSave={triggerSave}
      />

      {/* Confirm Mode Switch when Draft is Unsaved */}
      <ConfirmDialog
        open={pendingMode !== null}
        onClose={() => setPendingMode(null)}
        onConfirm={() => {
          if (pendingMode) {
            handleDiscard()
            setMode(pendingMode)
            setPendingMode(null)
          }
        }}
        title={t('worldVisual.modeSwitchConfirmTitle')}
        description={t('worldVisual.modeSwitchConfirmDesc')}
        confirmLabel={t('common.discard')}
        tone="warning"
      />

      {/* Confirm Official Preset Overwrite */}
      <ConfirmDialog
        open={!!pendingPreset}
        onClose={() => setPendingPreset(null)}
        onConfirm={() => {
          if (!pendingPreset) return
          saveSourceMut.mutate(pendingPreset.lua)
        }}
        title={t('config.applyPresetConfirmTitle')}
        description={t('config.applyPresetConfirmDesc', {
          name: pendingPreset ? (isEn ? pendingPreset.name_en : pendingPreset.name_zh) : '',
          shard: shard === 'master' ? t('common.master') : t('common.caves'),
        })}
        confirmLabel={t('config.applyPreset')}
        tone="warning"
        loading={saveSourceMut.isPending}
      />
    </div>
  )
}
