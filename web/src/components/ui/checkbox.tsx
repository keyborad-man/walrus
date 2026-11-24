import { Check } from 'lucide-react'
import { cn } from '@/lib/utils'

interface CheckboxProps {
  checked: boolean
  onCheckedChange: (checked: boolean) => void
  id?: string
  disabled?: boolean
  ariaLabel?: string
  className?: string
}

export function Checkbox({ checked, onCheckedChange, id, disabled, ariaLabel, className }: CheckboxProps) {
  return (
    <button
      id={id}
      type="button"
      role="checkbox"
      aria-checked={checked}
      aria-label={ariaLabel}
      disabled={disabled}
      onClick={() => onCheckedChange(!checked)}
      className={cn(
        'inline-flex h-4.5 w-4.5 shrink-0 items-center justify-center rounded border pressable focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50 cursor-pointer',
        checked
          ? 'border-primary bg-primary text-primary-foreground'
          : 'border-input bg-background hover:border-primary/60',
        className,
      )}
    >
      <Check
        aria-hidden="true"
        className={cn(
          'h-3 w-3 stroke-[3] [transition:opacity_150ms_var(--ease-out),scale_150ms_var(--ease-out)] motion-reduce:[transition:opacity_150ms_ease]',
          checked ? 'opacity-100 scale-100' : 'opacity-0 motion-safe:scale-90',
        )}
      />
    </button>
  )
}
