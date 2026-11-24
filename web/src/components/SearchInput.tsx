import { Search, X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Input } from '@/components/ui/input'
import { cn } from '@/lib/utils'

interface SearchInputProps {
  value: string
  onChange: (next: string) => void
  placeholder: string
  ariaLabel?: string
  className?: string
  inputClassName?: string
}

export function SearchInput({
  value,
  onChange,
  placeholder,
  ariaLabel,
  className,
  inputClassName,
}: SearchInputProps) {
  const { t } = useTranslation()
  return (
    <div className={cn('relative', className)}>
      <Search className="pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-muted-foreground" />
      <Input
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder}
        aria-label={ariaLabel ?? placeholder}
        className={cn('pl-8 text-xs', value ? 'pr-8' : '', inputClassName)}
      />
      {value && (
        <button
          type="button"
          onClick={() => onChange('')}
          aria-label={t('common.clearSearch')}
          title={t('common.clearSearch')}
          className="absolute right-1.5 top-1/2 -translate-y-1/2 flex h-6 w-6 items-center justify-center rounded text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer pressable"
        >
          <X className="h-3.5 w-3.5" />
        </button>
      )}
    </div>
  )
}
