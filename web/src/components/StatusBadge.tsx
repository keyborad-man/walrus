import { useTranslation } from 'react-i18next'
import { Flame, Loader2, Snowflake, Flower2, Sun } from 'lucide-react'
import type { ShardState } from '@/types/api'
import { cn, type CompositeClusterHealth } from '@/lib/utils'

export function ShardStateBadge({ state }: { state?: ShardState | CompositeClusterHealth }) {
  const { t } = useTranslation()
  const st = state ?? 'stopped'

  if (st === 'running') {
    return (
      <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-xs font-medium text-foreground">
        <span className="relative inline-flex h-2 w-2 items-center justify-center shrink-0" aria-hidden="true">
          <span className="absolute inline-flex h-full w-full rounded-full bg-emerald-500/45 dst-live-ring" />
          <span className="relative inline-flex h-[7px] w-[7px] rounded-full bg-emerald-600 dark:bg-emerald-500" />
        </span>
        {t('common.running')}
      </span>
    )
  }
  if (st === 'degraded') {
    return (
      <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-xs font-medium text-amber-700 dark:text-amber-400">
        <span className="h-[7px] w-[7px] rounded-full bg-amber-500 shrink-0" aria-hidden="true" />
        {t('common.degraded')}
      </span>
    )
  }
  if (st === 'starting' || st === 'stopping') {
    return (
      <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-xs font-medium text-amber-700 dark:text-amber-400">
        <Loader2 className="h-3 w-3 animate-spin shrink-0" aria-hidden="true" />
        {t(`common.${st}`)}
      </span>
    )
  }
  if (st === 'crashed') {
    return (
      <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-xs font-medium text-red-700 dark:text-red-400">
        <span className="h-[7px] w-[7px] rounded-full bg-red-600 dark:bg-red-500 shrink-0" aria-hidden="true" />
        {t('common.crashed')}
      </span>
    )
  }
  return (
    <span className="inline-flex items-center gap-1.5 whitespace-nowrap text-xs font-medium text-muted-foreground">
      <span className="h-[7px] w-[7px] rounded-full bg-muted-foreground/45 shrink-0" aria-hidden="true" />
      {t('common.stopped')}
    </span>
  )
}

/** Pure inline SVG compact Day/Dusk/Night Clock Dial */
export function DSTClockDial({ phase, className }: { phase?: string; className?: string }) {
  const angle = phase === 'night' ? 310 : phase === 'dusk' ? 215 : 65
  const known = phase === 'day' || phase === 'dusk' || phase === 'night'

  return (
    <svg
      viewBox="0 0 24 24"
      className={cn('h-5 w-5 shrink-0 text-foreground', className)}
      aria-hidden="true"
      focusable="false"
    >
      <circle cx="12" cy="12" r="9.5" fill="none" stroke="currentColor" strokeWidth="1.6" opacity="0.16" />
      <path
        d="M 12 2.5 A 9.5 9.5 0 0 1 21.5 12 A 9.5 9.5 0 0 1 16.8 20.2"
        fill="none"
        stroke={known ? 'var(--phase-day)' : 'currentColor'}
        strokeWidth="2.1"
        strokeLinecap="round"
        className="transition-opacity duration-200"
        opacity={!known ? '0.2' : phase === 'day' ? '1' : '0.3'}
      />
      <path
        d="M 16.8 20.2 A 9.5 9.5 0 0 1 5.2 18.6"
        fill="none"
        stroke={known ? 'var(--phase-dusk)' : 'currentColor'}
        strokeWidth="2.1"
        strokeLinecap="round"
        className="transition-opacity duration-200"
        opacity={!known ? '0.2' : phase === 'dusk' ? '1' : '0.3'}
      />
      <path
        d="M 5.2 18.6 A 9.5 9.5 0 0 1 12 2.5"
        fill="none"
        stroke={known ? 'var(--phase-night)' : 'currentColor'}
        strokeWidth="2.1"
        strokeLinecap="round"
        className="transition-opacity duration-200"
        opacity={!known ? '0.2' : phase === 'night' ? '1' : '0.3'}
      />
      <line
        x1="12"
        y1="12"
        x2="12"
        y2="5.5"
        stroke="currentColor"
        strokeWidth="1.6"
        opacity={known ? 1 : 0.35}
        strokeLinecap="round"
        className="transition-transform duration-300 ease-in-out motion-reduce:transition-none"
        style={{ transform: `rotate(${angle}deg)`, transformOrigin: '12px 12px' }}
      />
      <circle cx="12" cy="12" r="1.4" fill="currentColor" />
    </svg>
  )
}

/** Precision 16-segment World Clock Dial for Overview */
export function DSTClockHeroDial({
  day,
  phase = 'day',
  season = 'autumn',
  className,
}: {
  day?: number
  phase?: string
  season?: string
  className?: string
}) {
  const angle = phase === 'night' ? 310 : phase === 'dusk' ? 215 : 68
  const ticks = Array.from({ length: 16 }, (_, i) => i * 22.5)

  const seasonTone = `var(--season-${['winter', 'spring', 'summer'].includes(season) ? season : 'autumn'})`

  return (
    <svg
      viewBox="0 0 96 96"
      className={cn('h-20 w-20 shrink-0 select-none text-foreground', className)}
      aria-hidden="true"
      focusable="false"
    >
      <circle cx="48" cy="48" r="45" fill="none" stroke="currentColor" strokeWidth="1" opacity="0.14" />

      {ticks.map((deg) => (
        <line
          key={deg}
          x1="48"
          y1="6.5"
          x2="48"
          y2={deg % 90 === 0 ? '11' : '9'}
          stroke="currentColor"
          strokeWidth={deg % 90 === 0 ? '1.2' : '0.8'}
          opacity={deg % 90 === 0 ? '0.4' : '0.18'}
          transform={`rotate(${deg} 48 48)`}
        />
      ))}

      {/* Day segment */}
      <path
        d="M 48 16 A 32 32 0 0 1 80 48 A 32 32 0 0 1 64 75.7"
        fill="none"
        stroke="var(--phase-day)"
        strokeWidth="5.5"
        strokeLinecap="butt"
        className="transition-opacity duration-300"
        opacity={phase === 'day' || !phase || phase === 'unknown' ? '0.95' : '0.24'}
      />
      {/* Dusk segment */}
      <path
        d="M 64 75.7 A 32 32 0 0 1 22.2 66.8"
        fill="none"
        stroke="var(--phase-dusk)"
        strokeWidth="5.5"
        strokeLinecap="butt"
        className="transition-opacity duration-300"
        opacity={phase === 'dusk' ? '0.95' : '0.24'}
      />
      {/* Night segment */}
      <path
        d="M 22.2 66.8 A 32 32 0 0 1 48 16"
        fill="none"
        stroke="var(--phase-night)"
        strokeWidth="5.5"
        strokeLinecap="butt"
        className="transition-opacity duration-300"
        opacity={phase === 'night' ? '0.95' : '0.24'}
      />

      <circle cx="48" cy="48" r="24" fill="var(--card)" stroke="currentColor" strokeWidth="1" opacity="0.16" />
      <circle cx="48" cy="48" r="23.5" fill="var(--card)" />

      <g
        className="transition-transform duration-500 [transition-timing-function:var(--ease-out)] motion-reduce:transition-none"
        style={{ transform: `rotate(${angle}deg)`, transformOrigin: '48px 48px' }}
      >
        <polygon points="48,12 45.2,20.5 50.8,20.5" fill="currentColor" />
      </g>

      {day && day > 0 ? (
        <>
          <text
            x="48"
            y={day >= 1000 ? 54 : day >= 100 ? 55 : 57}
            textAnchor="middle"
            fill="var(--foreground)"
            fontSize={day >= 1000 ? 15 : day >= 100 ? 20 : 26}
            className="font-display"
          >
            {day}
          </text>
        </>
      ) : (
        <circle cx="48" cy="48" r="3.5" fill={seasonTone} opacity="0.7" />
      )}
    </svg>
  )
}

export function SeasonBadge({
  season,
  day,
  phase,
  size = 'sm',
}: {
  season: string
  day: number
  phase?: string
  size?: 'sm' | 'md'
}) {
  const { t } = useTranslation()
  if (!season || season === 'unknown' || day <= 0) return null

  const seasonIcon: Record<string, React.ReactNode> = {
    autumn: <Flame className="h-3.5 w-3.5 shrink-0 text-[var(--season-autumn)]" />,
    winter: <Snowflake className="h-3.5 w-3.5 shrink-0 text-[var(--season-winter)]" />,
    spring: <Flower2 className="h-3.5 w-3.5 shrink-0 text-[var(--season-spring)]" />,
    summer: <Sun className="h-3.5 w-3.5 shrink-0 text-[var(--season-summer)]" />,
  }
  const phaseLabel = phase && phase !== 'unknown' ? t(`dashboard.phases.${phase}`, '') : ''

  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 whitespace-nowrap font-medium tabular-nums text-xs text-muted-foreground',
        size === 'md' ? 'text-[12.5px]' : 'text-xs',
      )}
    >
      {size === 'md' ? <DSTClockDial phase={phase} className="h-4 w-4" /> : (seasonIcon[season] ?? seasonIcon.autumn)}
      <span className="text-foreground">{t(`dashboard.seasons.${season}`, season)}</span>
      <span aria-hidden="true" className="opacity-45">
        ·
      </span>
      <span className="text-foreground">{t('dashboard.day', { day })}</span>
      {phaseLabel && size === 'md' && <span className="text-muted-foreground">({phaseLabel})</span>}
    </span>
  )
}
