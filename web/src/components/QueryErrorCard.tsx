import { AlertTriangle, RefreshCw } from 'lucide-react'
import { useTranslation } from 'react-i18next'
import { formatError } from '@/lib/api'
import { Card, CardContent } from '@/components/ui/card'
import { Button } from '@/components/ui/button'

interface QueryErrorCardProps {
  error: unknown
  onRetry: () => void
  compact?: boolean
}

export function QueryErrorCard({ error, onRetry, compact = false }: QueryErrorCardProps) {
  const { t } = useTranslation()
  const message = error ? formatError(error) : t('common.loadFailed')

  if (compact) {
    return (
      <div
        role="alert"
        className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-red-600/35 bg-red-600/10 px-4 py-3 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
      >
        <div className="flex items-center gap-2 min-w-0">
          <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0" />
          <span className="break-words">{message}</span>
        </div>
        <Button size="sm" variant="outline" onClick={onRetry}>
          <RefreshCw className="h-3.5 w-3.5" />
          {t('common.retry')}
        </Button>
      </div>
    )
  }

  return (
    <Card className="border-red-600/35 bg-red-600/5 dark:border-red-500/35">
      <CardContent
        role="alert"
        className="py-10 px-6 sm:py-10 sm:px-6 flex flex-col items-center justify-center text-center gap-3"
      >
        <AlertTriangle className="h-7 w-7 text-red-700 dark:text-red-400" />
        <div className="space-y-1 max-w-md">
          <div className="text-sm font-semibold text-red-900 dark:text-red-200">{t('common.loadFailed')}</div>
          <p className="text-xs text-muted-foreground break-words">{message}</p>
        </div>
        <Button size="sm" variant="outline" onClick={onRetry}>
          <RefreshCw className="h-3.5 w-3.5" />
          {t('common.retry')}
        </Button>
      </CardContent>
    </Card>
  )
}
