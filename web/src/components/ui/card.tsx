import * as React from 'react'
import { cn } from '@/lib/utils'

export interface CardProps extends React.HTMLAttributes<HTMLDivElement> {
  /** Subtle hover elevation & border highlight for clickable / focal cards */
  interactive?: boolean
}

export const Card = React.forwardRef<HTMLDivElement, CardProps>(({ className, interactive = false, ...props }, ref) => (
  <div
    ref={ref}
    className={cn(
      'rounded-[10px] surface-card text-card-foreground',
      interactive && 'surface-interactive surface-lift',
      className,
    )}
    {...props}
  />
))
Card.displayName = 'Card'

export const CardHeader = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => (
    <div ref={ref} className={cn('flex flex-col space-y-1 p-4 sm:p-5', className)} {...props} />
  ),
)
CardHeader.displayName = 'CardHeader'

export interface CardTitleProps extends React.HTMLAttributes<HTMLHeadingElement> {
  as?: 'h1' | 'h2' | 'h3' | 'h4'
}

export const CardTitle = React.forwardRef<HTMLHeadingElement, CardTitleProps>(
  ({ className, as: Comp = 'h2', ...props }, ref) => (
    <Comp ref={ref} className={cn('text-[14px] font-semibold leading-snug tracking-[-0.01em]', className)} {...props} />
  ),
)
CardTitle.displayName = 'CardTitle'

export const CardDescription = React.forwardRef<HTMLParagraphElement, React.HTMLAttributes<HTMLParagraphElement>>(
  ({ className, ...props }, ref) => (
    <p ref={ref} className={cn('text-xs text-muted-foreground leading-relaxed', className)} {...props} />
  ),
)
CardDescription.displayName = 'CardDescription'

export const CardContent = React.forwardRef<HTMLDivElement, React.HTMLAttributes<HTMLDivElement>>(
  ({ className, ...props }, ref) => <div ref={ref} className={cn('p-4 sm:p-5 pt-0 sm:pt-0', className)} {...props} />,
)
CardContent.displayName = 'CardContent'
