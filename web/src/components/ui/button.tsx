import * as React from 'react'
import { cva, type VariantProps } from 'class-variance-authority'
import { cn } from '@/lib/utils'

export const buttonVariants = cva(
  'inline-flex items-center justify-center gap-1.5 whitespace-nowrap rounded-lg text-[13px] font-medium [transition:color_150ms_ease,background-color_150ms_ease,border-color_150ms_ease,opacity_150ms_ease,transform_140ms_var(--ease-out),scale_140ms_var(--ease-out)] focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none cursor-pointer motion-safe:active:scale-[0.985]',
  {
    variants: {
      variant: {
        default:
          'bg-primary text-primary-foreground border border-transparent hover:opacity-90 disabled:bg-secondary disabled:text-muted-foreground disabled:border-border',
        secondary:
          'bg-[var(--surface-elevated)] text-foreground border border-border hover:bg-[var(--surface-hover)] hover:border-[var(--border-strong)] disabled:opacity-50',
        outline:
          'border border-[var(--border-strong)] bg-card text-foreground hover:bg-[var(--surface-hover)] disabled:opacity-50',
        ghost:
          'border border-transparent text-muted-foreground hover:bg-[var(--surface-hover)] hover:text-foreground disabled:opacity-50',
        warning:
          'border border-amber-500/35 bg-amber-500/10 text-amber-800 hover:bg-amber-500/18 hover:border-amber-500/50 dark:border-amber-500/30 dark:bg-amber-500/10 dark:text-amber-300 dark:hover:bg-amber-500/18 disabled:opacity-50',
        dangerOutline:
          'border border-red-500/35 bg-transparent text-red-700 hover:bg-red-500/10 hover:border-red-500/55 dark:border-red-500/35 dark:text-red-400 dark:hover:bg-red-500/14 disabled:opacity-50',
        destructive:
          'bg-destructive text-destructive-foreground border border-transparent hover:bg-red-700 disabled:bg-secondary disabled:text-muted-foreground disabled:border-border',
        success:
          'bg-emerald-700 text-white border border-transparent hover:bg-emerald-600 dark:bg-emerald-600 dark:hover:bg-emerald-500 disabled:bg-secondary disabled:text-muted-foreground disabled:border-border',
      },
      size: {
        default: 'h-9 sm:h-[34px] px-3.5 py-1.5',
        sm: 'min-h-9 sm:min-h-[30px] rounded-md px-2.5 py-1 text-xs',
        lg: 'h-10 rounded-lg px-5 text-sm',
        icon: 'h-9 w-9 sm:h-8 sm:w-8 rounded-md',
      },
    },
    defaultVariants: {
      variant: 'default',
      size: 'default',
    },
  },
)

export interface ButtonProps
  extends React.ButtonHTMLAttributes<HTMLButtonElement>,
    VariantProps<typeof buttonVariants> {}

export const Button = React.forwardRef<HTMLButtonElement, ButtonProps>(
  ({ className, variant, size, type = 'button', ...props }, ref) => (
    <button ref={ref} type={type} className={cn(buttonVariants({ variant, size, className }))} {...props} />
  ),
)
Button.displayName = 'Button'
