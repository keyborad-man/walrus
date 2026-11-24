import * as React from 'react'
import { Eye, EyeOff } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

export const Input = React.forwardRef<HTMLInputElement, React.InputHTMLAttributes<HTMLInputElement>>(
  ({ className, type, ...props }, ref) => (
    <input
      ref={ref}
      type={type}
      className={cn(
        'flex h-9 sm:h-[34px] w-full rounded-lg border border-input bg-[var(--surface-input)] px-3 py-1 text-[13px] text-foreground transition-[border-color,box-shadow,background-color] duration-150 placeholder:text-muted-foreground/70 hover:border-[var(--border-strong)] focus-visible:outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/25 disabled:cursor-not-allowed disabled:opacity-55',
        className,
      )}
      {...props}
    />
  ),
)
Input.displayName = 'Input'

export const SecretInput = React.forwardRef<
  HTMLInputElement,
  Omit<React.InputHTMLAttributes<HTMLInputElement>, 'type'>
>(({ className, disabled, ...props }, ref) => {
  const { t } = useTranslation()
  const [revealed, setRevealed] = React.useState(false)

  return (
    <div className="relative">
      <Input
        ref={ref}
        type={revealed ? 'text' : 'password'}
        disabled={disabled}
        className={cn('pr-10', className)}
        {...props}
      />
      <button
        type="button"
        disabled={disabled}
        aria-label={revealed ? t('common.hideSecret') : t('common.showSecret')}
        aria-pressed={revealed}
        title={revealed ? t('common.hideSecret') : t('common.showSecret')}
        onClick={() => setRevealed((v) => !v)}
        className="absolute right-1 top-1/2 -translate-y-1/2 flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:pointer-events-none disabled:opacity-45 cursor-pointer pressable"
      >
        {revealed ? <EyeOff className="h-3.5 w-3.5" /> : <Eye className="h-3.5 w-3.5" />}
      </button>
    </div>
  )
})
SecretInput.displayName = 'SecretInput'

export const Select = React.forwardRef<HTMLSelectElement, React.SelectHTMLAttributes<HTMLSelectElement>>(
  ({ className, ...props }, ref) => (
    <select
      ref={ref}
      className={cn(
        'flex h-9 sm:h-[34px] w-full rounded-lg border border-input bg-[var(--surface-input)] px-3 py-1 text-[13px] text-foreground transition-[border-color,box-shadow,background-color] duration-150 hover:border-[var(--border-strong)] focus-visible:outline-none focus-visible:border-ring focus-visible:ring-2 focus-visible:ring-ring/25 disabled:cursor-not-allowed disabled:opacity-55',
        className,
      )}
      {...props}
    />
  ),
)
Select.displayName = 'Select'

export const Label = React.forwardRef<HTMLLabelElement, React.LabelHTMLAttributes<HTMLLabelElement>>(
  ({ className, ...props }, ref) => (
    <label ref={ref} className={cn('text-xs font-medium text-foreground/85 leading-snug', className)} {...props} />
  ),
)
Label.displayName = 'Label'
