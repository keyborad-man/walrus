import React, { useEffect, useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Bug,
  Calendar,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Compass,
  Filter,
  Flame,
  Globe,
  Moon,
  Mountain,
  RotateCcw,
  Shield,
  Skull,
  Sparkles,
  Sprout,
  Trash2,
  Trees,
} from 'lucide-react'
import { cn } from '@/lib/utils'
import { getWorldOptionIcon } from '@/lib/worldIcons'
import {
  countModifiedAgainstBaseline,
  resolveOptionValue,
  sortClientOptions,
} from '@/lib/worldHelpers'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Select } from '@/components/ui/input'
import { TabList } from '@/components/ui/tabs'
import { SearchInput } from '@/components/SearchInput'
import type { WorldOptionField, WorldOptionGroup, WorldOptionSection, WorldPreset } from '@/types/api'

interface Props {
  shard: 'master' | 'caves'
  schema: WorldOptionGroup[]
  presets: WorldPreset[]
  selectedPresetId: string
  presetTitle: string
  presetDescription: string
  playstyleLabel?: string
  baselineOverrides: Record<string, string>
  overrides: Record<string, string>
  onChangeOverride: (key: string, value: string) => void
  onResetAll: () => void
  onSelectPreset?: (presetId: string) => void
  onSwitchPlaystyle?: () => void
  onRemoveCaves?: () => void
}

function getGroupFallbackIcon(groupId: string) {
  if (groupId.includes('events')) return Calendar
  if (groupId.includes('survivors')) return Shield
  if (groupId.includes('lunar')) return Moon
  if (groupId.includes('giants')) return Skull
  if (groupId.includes('monsters')) return Flame
  if (groupId.includes('animals')) return Bug
  if (groupId.includes('portal')) return Sparkles
  if (groupId.includes('resources')) return Sprout
  if (groupId.includes('global')) return Globe
  return Compass
}

function OptionIcon({ optionKey, groupId, modified }: { optionKey: string; groupId: string; modified: boolean }) {
  const src = getWorldOptionIcon(optionKey)
  const [failed, setFailed] = useState(false)

  useEffect(() => {
    setFailed(false)
  }, [optionKey])

  if (!src || failed) {
    const Icon = getGroupFallbackIcon(groupId)
    return (
      <div
        aria-hidden="true"
        className={cn(
          'flex h-11 w-11 shrink-0 items-center justify-center rounded-md bg-secondary/70 text-muted-foreground ring-1',
          modified ? 'ring-amber-500/60 text-amber-600 dark:text-amber-400' : 'ring-border',
        )}
      >
        <Icon className="h-4 w-4" />
      </div>
    )
  }

  return (
    <div
      aria-hidden="true"
      className={cn(
        'h-11 w-11 shrink-0 overflow-hidden rounded-md bg-[#1d1814] shadow-xs ring-1 transition-shadow',
        modified ? 'ring-amber-500/70' : 'ring-black/20 dark:ring-white/12',
      )}
    >
      <img
        src={src}
        alt=""
        width={44}
        height={44}
        loading="lazy"
        decoding="async"
        draggable={false}
        onError={() => setFailed(true)}
        className="h-full w-full object-fill select-none"
      />
    </div>
  )
}

function OptionStepperCard({
  opt,
  groupId,
  value,
  baselineValue,
  isEn,
  onChange,
  onReset,
}: {
  opt: WorldOptionField
  groupId: string
  value: string
  baselineValue: string
  isEn: boolean
  onChange: (next: string) => void
  onReset: () => void
}) {
  const { t } = useTranslation()
  const label = isEn ? opt.label_en : opt.label_zh
  const isModified = value !== baselineValue
  const currentIdx = opt.choices.findIndex((c) => c.value === value)
  const canPrev = currentIdx > 0
  const canNext = currentIdx >= 0 && currentIdx < opt.choices.length - 1
  const singleChoice = opt.choices.length <= 1

  const step = (delta: -1 | 1) => {
    if (singleChoice) return
    const nextIdx = currentIdx + delta
    if (nextIdx >= 0 && nextIdx < opt.choices.length) {
      onChange(opt.choices[nextIdx].value)
    }
  }

  return (
    <div
      className={cn(
        'group flex items-center gap-2.5 rounded-lg border p-2.5 transition-colors',
        isModified
          ? 'border-amber-500/45 bg-amber-500/[0.06] dark:bg-amber-500/[0.08]'
          : 'border-border/80 bg-card hover:border-border-strong',
      )}
    >
      <OptionIcon optionKey={opt.key} groupId={groupId} modified={isModified} />

      <div className="min-w-0 flex-1 space-y-1">
        <div className="flex items-center justify-between gap-1">
          <span
            className={cn(
              'text-xs font-medium leading-tight truncate',
              isModified ? 'text-foreground' : 'text-foreground/90',
            )}
            title={`${label} (${opt.key})`}
          >
            {label}
          </span>
          {isModified && (
            <button
              type="button"
              onClick={onReset}
              aria-label={`${t('worldVisual.resetDefault')} ${label}`}
              title={t('worldVisual.resetDefault')}
              className="flex h-4 w-4 shrink-0 items-center justify-center rounded text-amber-600 dark:text-amber-400 hover:bg-amber-500/15 cursor-pointer"
            >
              <RotateCcw className="h-3 w-3" />
            </button>
          )}
        </div>

        {/* DST-style < Value > stepper with native select overlay for direct jumping */}
        <div className="flex items-center gap-0.5 rounded-md border border-border bg-secondary/50 p-0.5">
          <button
            type="button"
            disabled={!canPrev}
            onClick={() => step(-1)}
            aria-label={`${label} -`}
            className={cn(
              'flex h-6 w-6 shrink-0 items-center justify-center rounded transition-colors',
              canPrev
                ? 'text-foreground/80 hover:bg-background hover:text-foreground cursor-pointer'
                : 'text-muted-foreground/35 cursor-not-allowed',
            )}
          >
            <ChevronLeft className="h-3.5 w-3.5" />
          </button>

          <div className="relative flex-1 min-w-0">
            <select
              aria-label={label}
              disabled={singleChoice}
              value={value}
              onChange={(e) => onChange(e.target.value)}
              className={cn(
                'w-full appearance-none bg-transparent px-1 py-0.5 text-center text-xs font-medium truncate focus:outline-none focus-visible:ring-1 focus-visible:ring-ring rounded',
                singleChoice ? 'text-muted-foreground cursor-default' : 'text-foreground cursor-pointer',
                isModified && 'text-amber-700 dark:text-amber-300 font-semibold',
              )}
            >
              {currentIdx < 0 && <option value={value}>{value}</option>}
              {opt.choices.map((ch) => (
                <option key={ch.value} value={ch.value} className="bg-popover text-popover-foreground">
                  {isEn ? ch.label_en : ch.label_zh}
                </option>
              ))}
            </select>
          </div>

          <button
            type="button"
            disabled={!canNext}
            onClick={() => step(1)}
            aria-label={`${label} +`}
            className={cn(
              'flex h-6 w-6 shrink-0 items-center justify-center rounded transition-colors',
              canNext
                ? 'text-foreground/80 hover:bg-background hover:text-foreground cursor-pointer'
                : 'text-muted-foreground/35 cursor-not-allowed',
            )}
          >
            <ChevronRight className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    </div>
  )
}

export function WorldOptionsPane({
  shard,
  schema,
  presets,
  selectedPresetId,
  presetTitle,
  presetDescription,
  playstyleLabel,
  baselineOverrides,
  overrides,
  onChangeOverride,
  onResetAll,
  onSelectPreset,
  onSwitchPlaystyle,
  onRemoveCaves,
}: Props) {
  const { t, i18n } = useTranslation()
  const isEn = i18n.language === 'en-US'

  const [section, setSection] = useState<WorldOptionSection>('settings')
  const [search, setSearch] = useState('')
  const [onlyModified, setOnlyModified] = useState(false)
  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({})

  const sortedSchema = useMemo(
    () =>
      schema.map((g) => ({
        ...g,
        options: sortClientOptions(g.options, isEn),
      })),
    [schema, isEn],
  )

  const totalModified = useMemo(
    () => countModifiedAgainstBaseline(sortedSchema, overrides, baselineOverrides),
    [sortedSchema, overrides, baselineOverrides],
  )

  const settingsModCount = useMemo(
    () =>
      countModifiedAgainstBaseline(
        sortedSchema.filter((g) => g.section === 'settings'),
        overrides,
        baselineOverrides,
      ),
    [sortedSchema, overrides, baselineOverrides],
  )

  const worldgenModCount = useMemo(
    () =>
      countModifiedAgainstBaseline(
        sortedSchema.filter((g) => g.section === 'worldgen'),
        overrides,
        baselineOverrides,
      ),
    [sortedSchema, overrides, baselineOverrides],
  )

  const visibleGroups = useMemo(() => {
    const q = search.trim().toLowerCase()
    return sortedSchema
      .filter((g) => g.section === section)
      .map((g) => ({
        ...g,
        options: g.options.filter((opt) => {
          const cur = resolveOptionValue(opt, overrides[opt.key])
          const base = resolveOptionValue(opt, baselineOverrides[opt.key])
          if (onlyModified && cur === base) return false
          if (!q) return true
          return (
            opt.label_zh.toLowerCase().includes(q) ||
            opt.label_en.toLowerCase().includes(q) ||
            opt.key.toLowerCase().includes(q) ||
            g.name_zh.toLowerCase().includes(q) ||
            g.name_en.toLowerCase().includes(q)
          )
        }),
      }))
      .filter((g) => g.options.length > 0)
  }, [sortedSchema, section, search, onlyModified, overrides, baselineOverrides])

  const sectionTotalOptions = useMemo(
    () => sortedSchema.filter((g) => g.section === section).reduce((sum, g) => sum + g.options.length, 0),
    [sortedSchema, section],
  )

  const ShardIcon = shard === 'master' ? Trees : Mountain

  return (
    <div className="grid grid-cols-1 lg:grid-cols-[260px_minmax(0,1fr)] gap-4 items-start">
      {/* Left Sidebar: World Preset (mirrors DST left preset panel) */}
      <aside className="rounded-xl border border-border bg-card p-4 space-y-4 lg:sticky lg:top-4">
        <div className="flex items-center justify-between gap-2 border-b border-border pb-2.5">
          <div className="flex items-center gap-2 text-xs font-semibold tracking-wide text-foreground">
            <ShardIcon className="h-4 w-4 text-muted-foreground" />
            <span>{t('createWorld.worldPresetTitle')}</span>
          </div>
          {totalModified > 0 && (
            <Badge variant="warning" className="tabular-nums">
              {t('worldVisual.modifiedCount', { count: totalModified })}
            </Badge>
          )}
        </div>

        <div className="space-y-2.5">
          <div className="rounded-lg border border-border bg-secondary/40 px-3 py-2.5 space-y-1.5">
            <div className="flex items-center justify-between gap-2">
              <span className="text-sm font-semibold text-foreground truncate">{presetTitle}</span>
              {playstyleLabel && (
                <Badge variant="secondary" className="shrink-0 text-[11px]">
                  {playstyleLabel}
                </Badge>
              )}
            </div>
            <p className="text-xs leading-relaxed text-muted-foreground">{presetDescription}</p>
          </div>

          {presets.length > 1 && onSelectPreset && (
            <div className="space-y-1.5">
              <label
                htmlFor={`preset-select-${shard}`}
                className="block text-[11px] font-medium text-muted-foreground"
              >
                {t('createWorld.selectWorldPreset')}
              </label>
              <Select
                id={`preset-select-${shard}`}
                value={selectedPresetId}
                onChange={(e) => onSelectPreset(e.target.value)}
                className="h-9 text-xs"
              >
                {presets.map((p) => (
                  <option key={p.id} value={p.id}>
                    {isEn ? p.name_en : p.name_zh}
                  </option>
                ))}
              </Select>
            </div>
          )}
        </div>

        <div className="space-y-2 pt-2 border-t border-border">
          <Button
            type="button"
            size="sm"
            variant="outline"
            disabled={totalModified === 0}
            onClick={onResetAll}
            className="w-full justify-center"
          >
            <RotateCcw className="h-3.5 w-3.5" />
            {t('createWorld.resetPresetChanges')}
          </Button>

          {onSwitchPlaystyle && (
            <Button
              type="button"
              size="sm"
              variant="ghost"
              onClick={onSwitchPlaystyle}
              className="w-full justify-center text-muted-foreground hover:text-foreground"
            >
              {t('createWorld.changePlaystyle')}
            </Button>
          )}
        </div>

        <div className="rounded-lg bg-secondary/35 px-3 py-2 text-[11px] text-muted-foreground leading-relaxed">
          {t('createWorld.presetSidebarHint', {
            settings: sortedSchema.filter((g) => g.section === 'settings').reduce((s, g) => s + g.options.length, 0),
            worldgen: sortedSchema.filter((g) => g.section === 'worldgen').reduce((s, g) => s + g.options.length, 0),
          })}
        </div>
      </aside>

      {/* Right Main Panel: World Settings / World Generation Sub-tabs + Grouped 3-Col Grid */}
      <div className="rounded-xl border border-border bg-card overflow-hidden min-w-0">
        {/* Sub-tab & Filter Header */}
        <div className="border-b border-border bg-secondary/20 px-4 py-3 space-y-3">
          <div className="flex flex-wrap items-center justify-between gap-2.5">
            <TabList
              ariaLabel={t('worldVisual.sectionLabel')}
              variant="segmented"
              controlsId={`create-world-section-${shard}`}
              value={section}
              onChange={setSection}
              items={[
                {
                  key: 'settings',
                  label: t('worldVisual.sectionSettings'),
                  badge: settingsModCount > 0 ? settingsModCount : undefined,
                },
                {
                  key: 'worldgen',
                  label: t('worldVisual.sectionWorldgen'),
                  badge: worldgenModCount > 0 ? worldgenModCount : undefined,
                },
              ]}
            />

            <div className="flex items-center gap-2">
              <span className="hidden sm:inline text-xs text-muted-foreground tabular-nums">
                {t('createWorld.optionsCount', { count: sectionTotalOptions })}
              </span>
              {shard === 'caves' && onRemoveCaves && (
                <Button type="button" size="sm" variant="outline" onClick={onRemoveCaves} className="text-red-600 dark:text-red-400 hover:bg-red-500/10 border-red-500/30">
                  <Trash2 className="h-3.5 w-3.5" />
                  {t('createWorld.removeCaves')}
                </Button>
              )}
            </div>
          </div>

          <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-2">
            <SearchInput
              value={search}
              onChange={setSearch}
              placeholder={t('worldVisual.searchOptions')}
              className="flex-1 sm:max-w-xs"
              inputClassName="h-8"
            />
            <button
              type="button"
              aria-pressed={onlyModified}
              onClick={() => setOnlyModified((v) => !v)}
              className={cn(
                'inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border px-3 text-xs font-medium transition-colors cursor-pointer tabular-nums',
                onlyModified
                  ? 'border-amber-500/45 bg-amber-500/12 text-amber-700 dark:text-amber-300'
                  : 'border-border bg-card text-muted-foreground hover:text-foreground',
              )}
            >
              <Filter className="h-3.5 w-3.5" />
              <span>
                {t('worldVisual.onlyModified')} ({section === 'settings' ? settingsModCount : worldgenModCount})
              </span>
            </button>
          </div>
        </div>

        {/* Grouped Options Grid */}
        <div
          id={`create-world-section-${shard}`}
          role="tabpanel"
          className="p-4 space-y-5 max-h-[68vh] overflow-y-auto"
        >
          {visibleGroups.length === 0 ? (
            <div className="py-12 text-center text-xs text-muted-foreground">
              {t('worldVisual.noMatchingOptions')}
            </div>
          ) : (
            visibleGroups.map((group) => {
              const isCollapsed = !search.trim() && !onlyModified && Boolean(collapsedGroups[group.id])
              const groupMod = group.options.filter(
                (o) => resolveOptionValue(o, overrides[o.key]) !== resolveOptionValue(o, baselineOverrides[o.key]),
              ).length

              return (
                <section key={group.id} className="space-y-2.5">
                  <button
                    type="button"
                    aria-expanded={!isCollapsed}
                    onClick={() =>
                      setCollapsedGroups((prev) => ({
                        ...prev,
                        [group.id]: !isCollapsed,
                      }))
                    }
                    className="flex w-full items-center justify-between gap-2 rounded-lg bg-secondary/55 px-3.5 py-2 text-left hover:bg-secondary/80 transition-colors cursor-pointer"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <h3 className="text-xs font-semibold tracking-wide text-foreground">
                        {isEn ? group.name_en : group.name_zh}
                      </h3>
                      <span className="text-[11px] text-muted-foreground tabular-nums">({group.options.length})</span>
                      {groupMod > 0 && (
                        <Badge variant="warning" className="tabular-nums text-[10px] px-1.5 py-0">
                          {t('worldVisual.modifiedCount', { count: groupMod })}
                        </Badge>
                      )}
                    </div>
                    <ChevronDown
                      className={cn(
                        'h-4 w-4 text-muted-foreground transition-transform duration-150',
                        !isCollapsed && 'rotate-180',
                      )}
                    />
                  </button>

                  {!isCollapsed && (
                    <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-3 gap-2">
                      {group.options.map((opt) => {
                        const cur = resolveOptionValue(opt, overrides[opt.key])
                        const base = resolveOptionValue(opt, baselineOverrides[opt.key])
                        return (
                          <OptionStepperCard
                            key={opt.key}
                            opt={opt}
                            groupId={group.id}
                            value={cur}
                            baselineValue={base}
                            isEn={isEn}
                            onChange={(val) => onChangeOverride(opt.key, val)}
                            onReset={() => onChangeOverride(opt.key, base)}
                          />
                        )
                      })}
                    </div>
                  )}
                </section>
              )
            })
          )}
        </div>
      </div>
    </div>
  )
}
