import { cn } from '@/lib/utils'

interface SwitchProps {
  checked: boolean
  onCheckedChange: (checked: boolean) => void
  disabled?: boolean
  id?: string
  ariaLabel?: string
  ariaLabelledBy?: string
}

export function Switch({ checked, onCheckedChange, disabled, id, ariaLabel, ariaLabelledBy }: SwitchProps) {
  return (
    <button
      id={id}
      type="button"
      role="switch"
      aria-checked={checked}
      aria-label={ariaLabel}
      aria-labelledby={ariaLabelledBy}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        "relative inline-flex h-6 w-11 sm:h-5 sm:w-9 shrink-0 cursor-pointer items-center rounded-full border pressable focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background disabled:cursor-not-allowed disabled:opacity-50 before:content-[''] before:absolute before:-inset-2 sm:before:hidden",
        checked
          ? 'border-primary bg-primary'
          : 'border-[var(--border-strong)] bg-secondary',
      )}
    >
      <span
        className={cn(
          'pointer-events-none block h-4.5 w-4.5 sm:h-3.5 sm:w-3.5 rounded-full transition-transform duration-200 ease-out',
          checked
            ? 'translate-x-5 sm:translate-x-4 bg-primary-foreground'
            : 'translate-x-0.5 bg-muted-foreground',
        )}
      />
    </button>
  )
}
