import React from 'react'
import { cn } from '@/lib/utils'

interface PageHeaderProps {
  /** Kept for backwards compatibility; visually omitted in Precision Instrument style to avoid redundant header layers */
  eyebrow?: string
  title: React.ReactNode
  subtitle?: React.ReactNode
  badge?: React.ReactNode
  actions?: React.ReactNode
  className?: string
  /** Extra classes for the title row, e.g. to hide it visually where the app bar already shows the title. */
  titleClassName?: string
  /** Extra classes for the actions container. */
  actionsClassName?: string
}

export function PageHeader({ title, subtitle, badge, actions, className, titleClassName, actionsClassName }: PageHeaderProps) {
  return (
    <div className={cn('flex flex-col md:flex-row md:items-end md:justify-between gap-3.5', className)}>
      <div className="space-y-1 min-w-0">
        <div className={cn('flex items-center gap-2 flex-wrap', titleClassName)}>
          <h1 className="text-[20px] sm:text-[22px] font-semibold tracking-[-0.02em] leading-tight text-foreground">
            {title}
          </h1>
          {badge}
        </div>
        {subtitle && (
          <p className="text-[13px] text-muted-foreground leading-relaxed text-pretty max-w-2xl">{subtitle}</p>
        )}
      </div>
      {actions && <div className={cn('flex flex-wrap items-center gap-2 shrink-0', actionsClassName)}>{actions}</div>}
    </div>
  )
}
