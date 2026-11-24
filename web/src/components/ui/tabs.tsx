import React, { useCallback, useEffect, useLayoutEffect, useMemo, useRef, useState } from 'react'
import { cn } from '@/lib/utils'

export interface TabOption<T extends string> {
  key: T
  label: string
  icon?: React.ReactNode
  badge?: string | number
  hidden?: boolean
}

interface TabListProps<T extends string> {
  ariaLabel: string
  value: T
  onChange: (next: T) => void
  items: TabOption<T>[]
  variant?: 'bar' | 'segmented' | 'pills'
  /** Per-tab panel prefix: sets id="${idPrefix}-tab-${key}" & aria-controls="${idPrefix}-panel-${key}" */
  idPrefix?: string
  /** Shared panel ID controlled by all tabs (e.g. a filterable log viewport) */
  controlsId?: string
  /** Subtle neutral active style for secondary filter bars */
  tone?: 'primary' | 'neutral'
  className?: string
}

export function TabList<T extends string>({
  ariaLabel,
  value,
  onChange,
  items,
  variant = 'bar',
  idPrefix,
  controlsId,
  tone = 'neutral',
  className,
}: TabListProps<T>) {
  const visible = useMemo(() => items.filter((item) => !item.hidden), [items])
  const visibleKeysSig = visible.map((v) => `${v.key}:${v.label}:${v.badge ?? ''}`).join('|')
  const activeIdx = visible.findIndex((item) => item.key === value)

  const containerRef = useRef<HTMLDivElement | null>(null)
  const btnRefs = useRef<(HTMLButtonElement | null)[]>([])
  const [indicator, setIndicator] = useState<{ left: number; top: number; width: number; height: number } | null>(null)
  const [hasMeasured, setHasMeasured] = useState(false)

  const measureIndicator = useCallback(() => {
    const btn = activeIdx >= 0 ? btnRefs.current[activeIdx] : null
    if (!btn) {
      setIndicator((prev) => (prev === null ? prev : null))
      return
    }
    const next = {
      left: btn.offsetLeft,
      top: btn.offsetTop,
      width: btn.offsetWidth,
      height: btn.offsetHeight,
    }
    setIndicator((prev) =>
      prev &&
      prev.left === next.left &&
      prev.top === next.top &&
      prev.width === next.width &&
      prev.height === next.height
        ? prev
        : next,
    )
  }, [activeIdx])

  useLayoutEffect(() => {
    measureIndicator()
    if (!hasMeasured) {
      const raf = window.requestAnimationFrame(() => setHasMeasured(true))
      return () => window.cancelAnimationFrame(raf)
    }
  }, [measureIndicator, visibleKeysSig, hasMeasured])

  useEffect(() => {
    const el = containerRef.current
    if (!el || typeof ResizeObserver === 'undefined') return
    const ro = new ResizeObserver(() => measureIndicator())
    ro.observe(el)
    return () => ro.disconnect()
  }, [measureIndicator])

  const handleKeyDown = (e: React.KeyboardEvent<HTMLButtonElement>, idx: number) => {
    if (visible.length <= 1) return
    let nextIdx: number | null = null
    if (e.key === 'ArrowRight') nextIdx = (idx + 1) % visible.length
    else if (e.key === 'ArrowLeft') nextIdx = (idx - 1 + visible.length) % visible.length
    else if (e.key === 'Home') nextIdx = 0
    else if (e.key === 'End') nextIdx = visible.length - 1
    if (nextIdx !== null) {
      e.preventDefault()
      const target = visible[nextIdx]
      onChange(target.key)
      btnRefs.current[nextIdx]?.focus()
    }
  }

  const containerCls =
    variant === 'segmented'
      ? 'relative inline-flex items-center rounded-lg border border-border bg-card p-0.5 max-w-full overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden'
      : variant === 'pills'
        ? 'relative flex flex-nowrap overflow-x-auto sm:flex-wrap gap-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden'
        : 'relative flex flex-nowrap overflow-x-auto sm:flex-wrap gap-1 border-b border-border pb-2 -mx-1 px-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden'

  const indicatorVisualCls =
    tone === 'primary'
      ? 'bg-primary rounded-md'
      : variant === 'bar'
        ? 'bg-[var(--surface-hover)] rounded-md'
        : variant === 'segmented'
          ? 'bg-[var(--surface-hover)] rounded-[6px]'
          : 'bg-[var(--surface-hover)] border border-border rounded-md'

  return (
    <div ref={containerRef} role="tablist" aria-label={ariaLabel} className={cn(containerCls, className)}>
      {indicator && (
        <span
          aria-hidden="true"
          className={cn(
            'pointer-events-none absolute left-0 top-0 z-0',
            indicatorVisualCls,
            hasMeasured &&
              'motion-safe:[transition:transform_200ms_var(--ease-out),width_200ms_var(--ease-out),height_200ms_var(--ease-out)]',
          )}
          style={{
            width: indicator.width,
            height: indicator.height,
            transform: `translate3d(${indicator.left}px, ${indicator.top}px, 0)`,
          }}
        />
      )}

      {visible.map((item, idx) => {
        const active = value === item.key
        const tabId = idPrefix ? `${idPrefix}-tab-${item.key}` : undefined
        const ariaControls = controlsId ?? (idPrefix ? `${idPrefix}-panel-${item.key}` : undefined)
        return (
          <button
            key={item.key}
            id={tabId}
            ref={(el) => {
              btnRefs.current[idx] = el
            }}
            type="button"
            role="tab"
            aria-selected={active}
            aria-controls={ariaControls}
            tabIndex={active ? 0 : -1}
            onClick={() => onChange(item.key)}
            onKeyDown={(e) => handleKeyDown(e, idx)}
            className={cn(
              'relative z-10 inline-flex shrink-0 whitespace-nowrap items-center justify-center gap-1.5 font-medium cursor-pointer transition-colors duration-150 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background tabular-nums',
              variant === 'segmented'
                ? 'min-h-8 sm:min-h-7 rounded-[6px] px-2.5 py-1 text-xs'
                : variant === 'pills'
                  ? 'min-h-9 sm:min-h-8 rounded-md px-2.5 py-1 text-xs'
                  : 'min-h-9 sm:min-h-8 rounded-md px-3 py-1.5 text-xs',
              active
                ? tone === 'primary'
                  ? 'text-primary-foreground font-medium'
                  : 'text-foreground font-medium'
                : 'text-muted-foreground hover:text-foreground',
            )}
          >
            {item.icon}
            <span>{item.label}</span>
            {item.badge !== undefined && (
              <span
                className={cn(
                  'ml-0.5 rounded px-1.5 py-0 text-[11px] leading-4 font-mono tabular-nums',
                  active
                    ? tone === 'primary'
                      ? 'bg-primary-foreground/20 text-primary-foreground'
                      : 'bg-background/80 text-foreground'
                    : 'bg-secondary/70 text-muted-foreground',
                )}
              >
                {item.badge}
              </span>
            )}
          </button>
        )
      })}
    </div>
  )
}
