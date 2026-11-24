import { useEffect, useRef, useState } from 'react'
import { Check, Copy } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { cn } from '@/lib/utils'

interface CopyButtonProps {
  value: string
  label?: string
  className?: string
}

export async function copyTextWithFallback(text: string): Promise<boolean> {
  if (typeof navigator !== 'undefined' && navigator.clipboard && window.isSecureContext) {
    try {
      await navigator.clipboard.writeText(text)
      return true
    } catch {
      // fall through to execCommand fallback (e.g. LAN HTTP NAS access)
    }
  }
  const ta = document.createElement('textarea')
  ta.value = text
  ta.setAttribute('readonly', '')
  ta.style.position = 'fixed'
  ta.style.top = '-9999px'
  ta.style.left = '-9999px'
  ta.style.opacity = '0'
  document.body.appendChild(ta)
  try {
    ta.focus({ preventScroll: true })
    ta.select()
    ta.setSelectionRange(0, ta.value.length)
    return document.execCommand('copy')
  } catch {
    return false
  } finally {
    document.body.removeChild(ta)
  }
}

export function CopyButton({ value, label, className }: CopyButtonProps) {
  const { t } = useTranslation()
  const [copied, setCopied] = useState(false)
  const timerRef = useRef<number | null>(null)

  useEffect(() => {
    return () => {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current)
    }
  }, [])

  const handleCopy = async () => {
    const ok = await copyTextWithFallback(value)
    if (ok) {
      if (timerRef.current !== null) window.clearTimeout(timerRef.current)
      setCopied(true)
      toast.success(`${t('common.copied')}: ${value}`)
      timerRef.current = window.setTimeout(() => {
        setCopied(false)
        timerRef.current = null
      }, 1600)
    } else {
      toast.error(t('common.copyFailed'))
    }
  }

  return (
    <button
      type="button"
      onClick={() => void handleCopy()}
      aria-label={label ?? `${t('common.copy')} ${value}`}
      title={label ?? t('common.copy')}
      className={cn(
        'relative inline-flex h-7 w-7 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-secondary hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer pressable before:content-[""] before:absolute before:-inset-2 sm:before:hidden',
        className,
      )}
    >
      <span className="relative inline-flex h-3.5 w-3.5 items-center justify-center" aria-hidden="true">
        <Copy
          className={cn(
            'absolute inset-0 h-3.5 w-3.5 [transition:opacity_160ms_var(--ease-out),scale_160ms_var(--ease-out),filter_160ms_var(--ease-out)] motion-reduce:[transition:opacity_160ms_ease]',
            copied ? 'opacity-0 motion-safe:scale-90 motion-safe:blur-[2px]' : 'opacity-100 scale-100 blur-none',
          )}
        />
        <Check
          className={cn(
            'absolute inset-0 h-3.5 w-3.5 text-emerald-700 dark:text-emerald-400 [transition:opacity_160ms_var(--ease-out),scale_160ms_var(--ease-out),filter_160ms_var(--ease-out)] motion-reduce:[transition:opacity_160ms_ease]',
            copied ? 'opacity-100 scale-100 blur-none' : 'opacity-0 motion-safe:scale-90 motion-safe:blur-[2px]',
          )}
        />
      </span>
    </button>
  )
}
