import { useState } from 'react'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { RefreshCw, ScrollText } from 'lucide-react'
import { api } from '@/lib/api'
import { formatAuditAction } from '@/lib/dst'
import { cn, formatDateTime } from '@/lib/utils'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { SearchInput } from '@/components/SearchInput'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { PageHeader } from '@/components/PageHeader'
import { EmptyState } from '@/components/EmptyState'
import { TableRowsSkeleton } from '@/components/ui/skeleton'

export function AuditLogsPage() {
  const { t, i18n } = useTranslation()
  const isEn = i18n.language === 'en-US'
  const [actionFilter, setActionFilter] = useState('')

  const { data, isLoading, isFetching, isError, error, refetch } = useQuery({
    queryKey: ['audit-logs', actionFilter],
    queryFn: () => api.auditLogs(150, actionFilter),
  })
  const logs = data?.logs ?? []

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={t('nav.system')}
        title={t('audit.title')}
        subtitle={t('audit.subtitle')}
        actions={
          <div className="flex items-center gap-2 w-full sm:w-auto">
            <SearchInput
              value={actionFilter}
              onChange={setActionFilter}
              placeholder={t('audit.filterPlaceholder')}
              className="flex-1 sm:w-64"
            />
            <Button
              variant="outline"
              size="icon"
              aria-label={t('common.refresh')}
              title={t('common.refresh')}
              onClick={() => void refetch()}
            >
              <RefreshCw className={cn('h-4 w-4', isFetching && 'animate-spin')} />
            </Button>
          </div>
        }
      />

      {isError && <QueryErrorCard error={error} onRetry={() => void refetch()} compact />}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 tabular-nums">
            <ScrollText className="h-4 w-4 text-primary" />
            {t('audit.title')} ({logs.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <TableRowsSkeleton rows={6} />
          ) : logs.length === 0 ? (
            <EmptyState compact icon={<ScrollText className="h-5 w-5" />} title={t('audit.emptyLogs')} />
          ) : (
            <>
              {/* Mobile stacked cards (< md) */}
              <div className="divide-y divide-border md:hidden">
                {logs.map((log) => (
                  <div key={log.id} className="py-3 first:pt-0 last:pb-0 space-y-1.5 text-xs">
                    <div className="flex items-center justify-between gap-2">
                      <Badge variant="secondary" title={log.action}>
                        {formatAuditAction(log.action, isEn)}
                      </Badge>
                      <span className="text-muted-foreground tabular-nums shrink-0">
                        {formatDateTime(log.created_at)}
                      </span>
                    </div>
                    <div className="flex flex-wrap items-center gap-x-2 gap-y-0.5">
                      <span className="font-medium text-foreground">{log.actor}</span>
                      {log.target && <span className="font-mono text-muted-foreground break-all">{log.target}</span>}
                    </div>
                    {log.detail && <div className="text-muted-foreground break-words">{log.detail}</div>}
                    {log.ip && <div className="font-mono text-muted-foreground tabular-nums">{log.ip}</div>}
                  </div>
                ))}
              </div>

              {/* Desktop table (>= md) */}
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-left text-xs text-muted-foreground">
                      <th className="pb-3 pr-4 font-medium">{t('audit.time')}</th>
                      <th className="pb-3 pr-4 font-medium">{t('audit.actor')}</th>
                      <th className="pb-3 pr-4 font-medium">{t('audit.action')}</th>
                      <th className="pb-3 pr-4 font-medium">{t('audit.target')}</th>
                      <th className="pb-3 pr-4 font-medium">{t('audit.detail')}</th>
                      <th className="pb-3 font-medium">{t('audit.ip')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {logs.map((log) => (
                      <tr key={log.id} className="hover:bg-secondary/30">
                        <td className="py-2.5 pr-4 text-xs text-muted-foreground whitespace-nowrap tabular-nums">
                          {formatDateTime(log.created_at)}
                        </td>
                        <td className="py-2.5 pr-4 text-xs font-medium">{log.actor}</td>
                        <td className="py-2.5 pr-4">
                          <Badge variant="secondary" title={log.action}>
                            {formatAuditAction(log.action, isEn)}
                          </Badge>
                        </td>
                        <td className="py-2.5 pr-4 font-mono text-xs">{log.target}</td>
                        <td className="py-2.5 pr-4 text-xs text-muted-foreground max-w-sm truncate">{log.detail}</td>
                        <td className="py-2.5 font-mono text-xs text-muted-foreground tabular-nums">{log.ip}</td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </CardContent>
      </Card>
    </div>
  )
}
