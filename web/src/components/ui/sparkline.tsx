import { cn } from '@/lib/utils'

interface SparklineProps {
  values: number[]
  tone?: 'emerald' | 'amber' | 'red' | 'neutral'
  height?: number
  className?: string
  /** Auto-fit vertical range to emphasize micro-fluctuations (Precision Instrument style) */
  autoScale?: boolean
}

export function Sparkline({
  values,
  tone = 'neutral',
  height = 32,
  className,
  autoScale = true,
}: SparklineProps) {
  const width = 200
  const padY = 4
  const usableH = height - padY * 2

  const safeValues =
    values.length >= 2
      ? values
      : values.length === 1
        ? [values[0], values[0]]
        : [0, 0]

  let lo = 0
  let hi = 100
  if (autoScale) {
    const minVal = Math.min(...safeValues)
    const maxVal = Math.max(...safeValues)
    const span = maxVal - minVal
    const pad = span > 0 ? span * 0.35 : Math.max(2, Math.abs(maxVal) * 0.1)
    lo = Math.max(0, minVal - pad)
    hi = maxVal + pad
    if (hi <= lo) hi = lo + 1
  }

  const pts = safeValues.map((v, i) => {
    const clamped = autoScale ? v : Math.min(100, Math.max(0, v))
    const ratio = (clamped - lo) / (hi - lo)
    const x = (i / Math.max(1, safeValues.length - 1)) * width
    const y = padY + (1 - Math.min(1, Math.max(0, ratio))) * usableH
    return { x, y }
  })

  const linePath = pts.map((p, i) => `${i === 0 ? 'M' : 'L'} ${p.x.toFixed(1)} ${p.y.toFixed(1)}`).join(' ')

  const strokeCls =
    tone === 'red'
      ? 'stroke-red-500'
      : tone === 'amber'
        ? 'stroke-amber-500'
        : tone === 'emerald'
          ? 'stroke-emerald-500'
          : 'stroke-zinc-500 dark:stroke-zinc-400'

  return (
    <svg
      viewBox={`0 0 ${width} ${height}`}
      preserveAspectRatio="none"
      className={cn('w-full overflow-visible block', className)}
      style={{ height }}
      aria-hidden="true"
      focusable="false"
    >
      <path
        d={linePath}
        fill="none"
        className={strokeCls}
        strokeWidth="1.5"
        strokeLinecap="round"
        strokeLinejoin="round"
        vectorEffect="non-scaling-stroke"
      />
    </svg>
  )
}
