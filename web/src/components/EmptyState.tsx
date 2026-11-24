import React from 'react'
import { cn } from '@/lib/utils'

interface EmptyStateProps {
  icon: React.ReactNode
  title: string
  description?: string
  action?: React.ReactNode
  className?: string
  compact?: boolean
}

export function EmptyState({ icon, title, description, action, className, compact = false }: EmptyStateProps) {
  return (
    <div
      className={cn(
        'flex flex-col items-center justify-center text-center rounded-[10px] border border-dashed border-border bg-[var(--surface-elevated)]/30',
        compact ? 'py-7 px-4 gap-2' : 'py-11 px-6 gap-3',
        className,
      )}
    >
      <div className="flex h-10 w-10 items-center justify-center rounded-lg border border-border bg-card text-muted-foreground">
        {icon}
      </div>
      <div className="space-y-1 max-w-md">
        <div className="text-[13.5px] font-medium text-foreground tracking-[-0.01em]">{title}</div>
        {description && <p className="text-xs text-muted-foreground leading-relaxed text-pretty">{description}</p>}
      </div>
      {action && <div className="pt-1 flex flex-wrap items-center justify-center gap-2">{action}</div>}
    </div>
  )
}
