import { useEffect, useState } from 'react'
import { Save } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/components/ui/button'
import { cn } from '@/lib/utils'

interface StickySaveBarProps {
  visible: boolean
  saving?: boolean
  disabled?: boolean
  saveLabel?: string
  onDiscard: () => void
  onSave: () => void
}

export function StickySaveBar({
  visible,
  saving = false,
  disabled = false,
  saveLabel,
  onDiscard,
  onSave,
}: StickySaveBarProps) {
  const { t } = useTranslation()
  const [mounted, setMounted] = useState(visible)
  const [entered, setEntered] = useState(false)

  useEffect(() => {
    if (visible) {
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
    const timer = window.setTimeout(() => setMounted(false), 180)
    return () => window.clearTimeout(timer)
  }, [visible])

  if (!mounted) return null

  return (
    <div
      role="status"
      aria-live="polite"
      inert={!entered}
      className={cn(
        'sticky bottom-4 z-20 flex flex-wrap items-center justify-between gap-3 rounded-[10px] border border-[var(--border-strong)] bg-popover px-4 py-2.5 shadow-[0_14px_34px_-10px_rgba(0,0,0,0.45)] [transition:translate_200ms_var(--ease-out),transform_200ms_var(--ease-out),opacity_180ms_var(--ease-out)] motion-reduce:[transition:opacity_180ms_ease]',
        entered ? 'translate-y-0 opacity-100' : 'motion-safe:translate-y-3 opacity-0 pointer-events-none',
      )}
    >
      <div className="flex items-center gap-2 text-xs font-medium text-foreground">
        <span className="h-[7px] w-[7px] rounded-full bg-amber-500 shrink-0" aria-hidden="true" />
        <span>{t('common.unsavedChanges')}</span>
      </div>
      <div className="flex items-center gap-2">
        <Button type="button" size="sm" variant="outline" disabled={saving} onClick={onDiscard}>
          {t('common.discard')}
        </Button>
        <Button type="button" size="sm" disabled={saving || disabled} onClick={onSave}>
          <Save className="h-3.5 w-3.5" />
          {saving ? t('common.saving') : (saveLabel ?? t('common.save'))}
        </Button>
      </div>
    </div>
  )
}
