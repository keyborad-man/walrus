import * as React from 'react'
import { createPortal } from 'react-dom'
import { X } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { cn } from '@/lib/utils'

interface DialogProps {
  open: boolean
  onClose: () => void
  title: string
  subtitle?: string
  children: React.ReactNode
  footer?: React.ReactNode
  maxWidthClass?: string
  variant?: 'modal' | 'sheet'
}

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

export function Dialog({
  open,
  onClose,
  title,
  subtitle,
  children,
  footer,
  maxWidthClass = 'max-w-xl',
  variant = 'modal',
}: DialogProps) {
  const { t } = useTranslation()
  const titleId = React.useId()
  const descId = React.useId()
  const panelRef = React.useRef<HTMLDivElement | null>(null)
  const previousFocusRef = React.useRef<HTMLElement | null>(null)
  const onCloseRef = React.useRef(onClose)
  const [mounted, setMounted] = React.useState(open)
  const [entered, setEntered] = React.useState(false)

  React.useEffect(() => {
    onCloseRef.current = onClose
  }, [onClose])

  React.useEffect(() => {
    if (open) {
      setMounted(true)
      let raf2 = 0
      const raf1 = window.requestAnimationFrame(() => {
        raf2 = window.requestAnimationFrame(() => setEntered(true))
      })
      return () => {
        window.cancelAnimationFrame(raf1)
        window.cancelAnimationFrame(raf2)
      }
    }
    setEntered(false)
    const timer = window.setTimeout(() => setMounted(false), variant === 'sheet' ? 260 : 200)
    return () => window.clearTimeout(timer)
  }, [open, variant])

  React.useEffect(() => {
    if (!open) return
    const active = document.activeElement as HTMLElement | null
    if (!previousFocusRef.current && active && !panelRef.current?.contains(active)) {
      previousFocusRef.current = active
    }
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    return () => {
      document.body.style.overflow = prevOverflow
      const toRestore = previousFocusRef.current
      previousFocusRef.current = null
      toRestore?.focus?.()
    }
  }, [open])

  React.useEffect(() => {
    if (!open || !mounted) return

    const focusTimer = window.setTimeout(() => {
      const panel = panelRef.current
      if (!panel) return
      const first = panel.querySelector<HTMLElement>(FOCUSABLE_SELECTOR)
      if (first) first.focus()
      else panel.focus()
    }, 20)

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onCloseRef.current()
        return
      }
      if (e.key === 'Tab' && panelRef.current) {
        const focusables = Array.from(panelRef.current.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
          (el) => el.offsetParent !== null,
        )
        if (focusables.length === 0) {
          e.preventDefault()
          return
        }
        const first = focusables[0]
        const last = focusables[focusables.length - 1]
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault()
          last.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }

    window.addEventListener('keydown', onKey)
    return () => {
      window.clearTimeout(focusTimer)
      window.removeEventListener('keydown', onKey)
    }
  }, [open, mounted])

  if (!mounted || typeof document === 'undefined') return null

  return createPortal(
    <div
      className={cn(
        'fixed inset-0 z-[60] flex justify-center',
        variant === 'sheet' ? 'items-end sm:items-center p-0 sm:p-4' : 'items-center p-4',
        !entered && 'pointer-events-none',
      )}
    >
      <div
        className={cn(
          'fixed inset-0 bg-black/60 backdrop-blur-[2px] transition-opacity duration-200',
          entered ? 'opacity-100' : 'opacity-0',
        )}
        onClick={onClose}
        aria-hidden="true"
      />
      <div
        ref={panelRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={titleId}
        aria-describedby={subtitle ? descId : undefined}
        tabIndex={-1}
        className={cn(
          'relative z-10 flex flex-col w-full overflow-hidden border border-[var(--border-strong)] bg-popover text-popover-foreground shadow-[0_24px_60px_-14px_rgba(0,0,0,0.45)] focus:outline-none',
          variant === 'sheet'
            ? cn(
                'rounded-t-xl rounded-b-none sm:rounded-xl max-h-[85vh] sm:max-h-[90vh] sm:origin-center [transition:translate_260ms_var(--ease-drawer),transform_260ms_var(--ease-drawer),opacity_200ms_var(--ease-out),scale_200ms_var(--ease-out)] motion-reduce:[transition:opacity_200ms_ease]',
                entered
                  ? 'translate-y-0 opacity-100 scale-100'
                  : 'motion-safe:translate-y-full sm:motion-safe:translate-y-0 sm:motion-safe:scale-[0.98] opacity-0',
              )
            : cn(
                'rounded-xl max-h-[90vh] origin-center [transition:opacity_200ms_var(--ease-out),scale_200ms_var(--ease-out),translate_200ms_var(--ease-out)] motion-reduce:[transition:opacity_200ms_ease]',
                entered
                  ? 'opacity-100 scale-100 translate-y-0'
                  : 'opacity-0 motion-safe:scale-[0.98] motion-safe:translate-y-1.5',
              ),
          maxWidthClass,
        )}
      >
        <div className="shrink-0 flex items-start justify-between gap-4 border-b border-border bg-popover px-5 py-3.5">
          <div className="min-w-0">
            <h2 id={titleId} className="text-[15px] font-semibold tracking-[-0.015em]">
              {title}
            </h2>
            {subtitle && (
              <p id={descId} className="mt-1 text-xs text-muted-foreground leading-relaxed">
                {subtitle}
              </p>
            )}
          </div>
          <button
            type="button"
            onClick={onClose}
            aria-label={t('common.close')}
            title={t('common.close')}
            className="flex h-8 w-8 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-[var(--surface-hover)] hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer pressable"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
        <div className="flex-1 overflow-y-auto px-5 py-4">{children}</div>
        {footer && (
          <div className="shrink-0 border-t border-border bg-[var(--surface-elevated)]/60 px-5 py-3">
            {footer}
          </div>
        )}
      </div>
    </div>,
    document.body,
  )
}
