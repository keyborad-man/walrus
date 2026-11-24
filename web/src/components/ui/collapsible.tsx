import * as React from 'react'
import { cn } from '@/lib/utils'

interface CollapsibleContentProps {
  open: boolean
  children: React.ReactNode
  className?: string
  innerClassName?: string
}

export function CollapsibleContent({
  open,
  children,
  className,
  innerClassName,
}: CollapsibleContentProps) {
  return (
    <div
      aria-hidden={!open}
      inert={!open}
      className={cn(
        'grid [transition:grid-template-rows_200ms_var(--ease-out),opacity_180ms_ease] motion-reduce:[transition:opacity_180ms_ease]',
        open ? 'grid-rows-[1fr] opacity-100' : 'grid-rows-[0fr] opacity-0',
        className,
      )}
    >
      <div className={cn('overflow-hidden', innerClassName)}>{children}</div>
    </div>
  )
}
