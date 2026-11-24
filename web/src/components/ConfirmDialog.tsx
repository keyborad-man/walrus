import { useEffect, useState } from 'react'
import { AlertTriangle } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { Dialog } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input, Label } from '@/components/ui/input'
import { cn } from '@/lib/utils'

export interface ConfirmDialogProps {
  open: boolean
  onClose: () => void
  onConfirm: () => void
  title: string
  description: string
  confirmLabel?: string
  cancelLabel?: string
  tone?: 'danger' | 'warning'
  confirmTextMatch?: string
  loading?: boolean
}

export function ConfirmDialog({
  open,
  onClose,
  onConfirm,
  title,
  description,
  confirmLabel,
  cancelLabel,
  tone = 'warning',
  confirmTextMatch,
  loading = false,
}: ConfirmDialogProps) {
  const { t } = useTranslation()
  const [typed, setTyped] = useState('')

  useEffect(() => {
    if (!open) setTyped('')
  }, [open])

  const matchRequired = !!confirmTextMatch
  const canConfirm = !loading && (!matchRequired || typed.trim() === confirmTextMatch)

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={title}
      maxWidthClass="max-w-md"
      footer={
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose} disabled={loading}>
            {cancelLabel ?? t('common.cancel')}
          </Button>
          <Button
            type="button"
            variant={tone === 'danger' ? 'destructive' : 'warning'}
            disabled={!canConfirm}
            onClick={() => {
              onConfirm()
            }}
          >
            {confirmLabel ?? t('common.confirm')}
          </Button>
        </div>
      }
    >
      <div className="space-y-4">
        <div
          className={cn(
            'flex items-start gap-3 rounded-xl border p-3.5 text-xs leading-relaxed',
            tone === 'danger'
              ? 'border-red-600/30 bg-red-600/10 text-red-900 dark:border-red-500/35 dark:bg-red-500/12 dark:text-red-200'
              : 'border-amber-600/35 bg-amber-500/14 text-amber-950 dark:border-amber-500/35 dark:bg-amber-500/14 dark:text-amber-200',
          )}
        >
          <AlertTriangle
            className={cn(
              'h-4 w-4 shrink-0 mt-0.5',
              tone === 'danger' ? 'text-red-700 dark:text-red-400' : 'text-amber-700 dark:text-amber-400',
            )}
          />
          <p className="whitespace-pre-line">{description}</p>
        </div>

        {matchRequired && (
          <div className="space-y-1.5">
            <Label htmlFor="confirm-match-input">
              {t('common.confirmTypeHint', { value: confirmTextMatch })}
            </Label>
            <Input
              id="confirm-match-input"
              value={typed}
              onChange={(e) => setTyped(e.target.value)}
              placeholder={t('common.confirmTypePlaceholder', { value: confirmTextMatch })}
              className="font-mono"
              autoComplete="off"
            />
          </div>
        )}
      </div>
    </Dialog>
  )
}
