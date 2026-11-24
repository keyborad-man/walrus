import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

const badgeVariants = cva(
  'inline-flex items-center gap-1.5 whitespace-nowrap rounded-md border px-2 py-0.5 text-[11.5px] font-medium leading-4',
  {
    variants: {
      variant: {
        default: 'border-border bg-secondary text-foreground',
        secondary: 'border-border bg-[var(--surface-elevated)] text-muted-foreground',
        success:
          'border-emerald-600/25 bg-emerald-500/10 text-emerald-700 dark:border-emerald-500/25 dark:bg-emerald-500/12 dark:text-emerald-400',
        warning:
          'border-amber-600/30 bg-amber-500/10 text-amber-800 dark:border-amber-500/30 dark:bg-amber-500/12 dark:text-amber-300',
        destructive:
          'border-red-600/25 bg-red-500/10 text-red-700 dark:border-red-500/30 dark:bg-red-500/12 dark:text-red-400',
        info: 'border-border bg-[var(--surface-elevated)] text-foreground',
        muted: 'border-border bg-transparent text-muted-foreground',
      },
    },
    defaultVariants: {
      variant: 'default',
    },
  },
)

export interface BadgeProps extends React.HTMLAttributes<HTMLSpanElement>, VariantProps<typeof badgeVariants> {}

export function Badge({ className, variant, ...props }: BadgeProps) {
  return <span className={cn(badgeVariants({ variant }), className)} {...props} />
}
