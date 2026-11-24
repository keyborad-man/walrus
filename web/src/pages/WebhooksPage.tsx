import React, { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { AlertTriangle, Bell, CheckCircle2, Pencil, Plus, Send, Trash2, XCircle } from 'lucide-react'
import { api, formatApiError } from '@/lib/api'
import { formatDateTime } from '@/lib/utils'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input, Label, SecretInput, Select } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Checkbox } from '@/components/ui/checkbox'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { Dialog } from '@/components/ui/dialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { PageHeader } from '@/components/PageHeader'
import { EmptyState } from '@/components/EmptyState'
import { PageCardsSkeleton } from '@/components/ui/skeleton'
import type { WebhookConfig } from '@/types/api'

const CRITICAL_EVENTS = ['server.crash', 'steam.update_available', 'backup.failed']

const TEMPLATES = [
  { id: 'discord', label: 'Discord' },
  { id: 'feishu', label: '飞书 (Feishu / Lark)' },
  { id: 'dingtalk', label: '钉钉 (DingTalk)' },
  { id: 'wecom', label: '企业微信 (WeCom)' },
  { id: 'telegram', label: 'Telegram Bot' },
  { id: 'bark', label: 'Bark (iOS Push)' },
  { id: 'generic', label: 'Generic JSON Webhook' },
]

export function WebhooksPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['webhooks'],
    queryFn: api.listWebhooks,
  })

  const webhooks = data?.webhooks ?? []
  const deliveries = data?.deliveries ?? []
  const allEvents = data?.all_events ?? []

  const [open, setOpen] = useState(false)
  const [editing, setEditing] = useState<WebhookConfig | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<WebhookConfig | null>(null)
  const [testingId, setTestingId] = useState<number | null>(null)
  const [name, setName] = useState('')
  const [template, setTemplate] = useState('discord')
  const [url, setUrl] = useState('')
  const [secret, setSecret] = useState('')
  const [events, setEvents] = useState<string[]>(CRITICAL_EVENTS)
  const [enabled, setEnabled] = useState(true)

  const openCreate = () => {
    setEditing(null)
    setName('')
    setTemplate('discord')
    setUrl('')
    setSecret('')
    setEvents(['server.crash', 'steam.update_available', 'backup.failed'])
    setEnabled(true)
    setOpen(true)
  }

  const openEdit = (w: WebhookConfig) => {
    setEditing(w)
    setName(w.name)
    setTemplate(w.template)
    setUrl(w.url)
    setSecret('')
    setEvents(w.events ?? [])
    setEnabled(w.enabled)
    setOpen(true)
  }

  const saveMut = useMutation({
    mutationFn: () =>
      editing
        ? api.updateWebhook(editing.id, { name, template, url, secret, events, enabled })
        : api.createWebhook({ name, template, url, secret, events, enabled }),
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setOpen(false)
      void qc.invalidateQueries({ queryKey: ['webhooks'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (wid: number) => api.deleteWebhook(wid),
    onSuccess: () => {
      toast.success(t('common.actionSuccess'))
      setDeleteTarget(null)
      void qc.invalidateQueries({ queryKey: ['webhooks'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const testMut = useMutation({
    mutationFn: (wid: number) => api.testWebhook(wid),
    onMutate: (wid) => setTestingId(wid),
    onSuccess: (res) => {
      void qc.invalidateQueries({ queryKey: ['webhooks'] })
      if (res.delivery.success) {
        toast.success(t('webhooks.testSentSuccess', { code: res.delivery.status_code }))
      } else {
        toast.error(formatApiError({ error: res.delivery.error, status: res.delivery.status_code }))
      }
    },
    onError: (err: Error) => toast.error(err.message),
    onSettled: () => setTestingId(null),
  })

  const toggleEvent = (ev: string) => {
    setEvents((prev) => (prev.includes(ev) ? prev.filter((x) => x !== ev) : [...prev, ev]))
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    saveMut.mutate()
  }

  const formatEventLabel = (ev: string) => t(`webhooks.eventLabels.${ev}`, ev)

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={t('nav.system')}
        title={t('webhooks.title')}
        subtitle={t('webhooks.subtitle')}
        actions={
          webhooks.length > 0 ? (
            <Button onClick={openCreate}>
              <Plus className="h-4 w-4" />
              {t('webhooks.addHook')}
            </Button>
          ) : undefined
        }
      />

      {isError && <QueryErrorCard error={error} onRetry={() => void refetch()} compact />}

      {/* Webhooks List */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {isLoading ? (
          <div className="col-span-full">
            <PageCardsSkeleton count={2} />
          </div>
        ) : webhooks.length === 0 ? (
          <div className="col-span-full">
            <EmptyState
              icon={<Bell className="h-5 w-5" />}
              title={t('webhooks.title')}
              description={t('webhooks.emptyHooks')}
              action={
                <Button size="sm" variant="secondary" onClick={openCreate}>
                  <Plus className="h-3.5 w-3.5" />
                  {t('webhooks.addHook')}
                </Button>
              }
            />
          </div>
        ) : (
          webhooks.map((w) => (
            <Card key={w.id} interactive={w.enabled} className={!w.enabled ? 'border-dashed bg-muted/40' : undefined}>
              <CardHeader className="pb-2">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <CardTitle className="flex items-center gap-2">
                      <Bell className="h-4 w-4 text-primary shrink-0" />
                      <span className="truncate">{w.name}</span>
                    </CardTitle>
                    <div className="mt-1 font-mono text-xs text-muted-foreground truncate">{w.url}</div>
                  </div>
                  <div className="flex items-center gap-1.5 shrink-0">
                    <Badge variant="default">{w.template}</Badge>
                    <Badge variant={w.enabled ? 'success' : 'muted'}>
                      {w.enabled ? t('common.enabled') : t('common.disabled')}
                    </Badge>
                  </div>
                </div>
              </CardHeader>
              <CardContent className="space-y-3">
                <div className="flex flex-wrap gap-1.5">
                  {w.events.map((ev) => (
                    <Badge key={ev} variant="secondary">
                      {formatEventLabel(ev)}
                    </Badge>
                  ))}
                </div>
                <div className="flex items-center justify-between pt-2 border-t border-border">
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={testingId === w.id}
                    onClick={() => testMut.mutate(w.id)}
                  >
                    <Send className="h-3.5 w-3.5" />
                    {t('webhooks.testBtn')}
                  </Button>
                  <div className="flex items-center gap-1">
                    <Button
                      size="icon"
                      variant="ghost"
                      aria-label={`${t('common.edit')} ${w.name}`}
                      title={t('common.edit')}
                      onClick={() => openEdit(w)}
                    >
                      <Pencil className="h-4 w-4" />
                    </Button>
                    <Button
                      size="icon"
                      variant="ghost"
                      aria-label={`${t('common.delete')} ${w.name}`}
                      title={t('common.delete')}
                      className="text-red-700 hover:bg-red-600/12 dark:text-red-400 dark:hover:bg-red-500/12"
                      onClick={() => setDeleteTarget(w)}
                    >
                      <Trash2 className="h-4 w-4" />
                    </Button>
                  </div>
                </div>
              </CardContent>
            </Card>
          ))
        )}
      </div>

      {/* Recent Deliveries */}
      <Card>
        <CardHeader>
          <CardTitle className="text-base tabular-nums">
            {t('webhooks.deliveriesTitle')} ({deliveries.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {deliveries.length === 0 ? (
            <div className="py-8 text-center text-xs text-muted-foreground">{t('webhooks.emptyDeliveries')}</div>
          ) : (
            <>
              {/* Mobile stacked cards (< md) */}
              <div className="divide-y divide-border md:hidden">
                {deliveries.map((d) => (
                  <div key={d.id} className="py-3 first:pt-0 last:pb-0 space-y-1.5 text-xs">
                    <div className="flex items-center justify-between gap-2">
                      <span className="font-medium truncate">{formatEventLabel(d.event)}</span>
                      <Badge variant={d.success ? 'success' : 'destructive'} className="tabular-nums shrink-0">
                        {d.success ? <CheckCircle2 className="h-3.5 w-3.5" /> : <XCircle className="h-3.5 w-3.5" />}
                        {d.status_code || (d.success ? 'OK' : 'ERR')}
                      </Badge>
                    </div>
                    <div className="text-muted-foreground tabular-nums">{formatDateTime(d.created_at)}</div>
                    <div className="text-muted-foreground break-words">{d.error || t('webhooks.delivered')}</div>
                  </div>
                ))}
              </div>

              {/* Desktop table (>= md) */}
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full text-xs">
                  <thead>
                    <tr className="border-b border-border text-left text-muted-foreground">
                      <th className="pb-2.5 pr-4 font-medium">{t('audit.time')}</th>
                      <th className="pb-2.5 pr-4 font-medium">{t('webhooks.eventCol')}</th>
                      <th className="pb-2.5 pr-4 font-medium">{t('common.status')}</th>
                      <th className="pb-2.5 font-medium">{t('audit.detail')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {deliveries.map((d) => (
                      <tr key={d.id}>
                        <td className="py-2.5 pr-4 text-muted-foreground whitespace-nowrap tabular-nums">
                          {formatDateTime(d.created_at)}
                        </td>
                        <td className="py-2.5 pr-4">{formatEventLabel(d.event)}</td>
                        <td className="py-2.5 pr-4">
                          <Badge variant={d.success ? 'success' : 'destructive'} className="tabular-nums">
                            {d.success ? <CheckCircle2 className="h-3.5 w-3.5" /> : <XCircle className="h-3.5 w-3.5" />}
                            {d.status_code || (d.success ? 'OK' : 'ERR')}
                          </Badge>
                        </td>
                        <td className="py-2.5 text-muted-foreground max-w-md truncate">
                          {d.error || t('webhooks.delivered')}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </>
          )}
        </CardContent>
      </Card>

      {/* Create / Edit Dialog */}
      <Dialog
        open={open}
        onClose={() => setOpen(false)}
        title={editing ? `${t('common.edit')}: ${editing.name}` : t('webhooks.addHook')}
        maxWidthClass="max-w-xl"
        footer={
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => setOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" form="webhook-form" disabled={saveMut.isPending}>
              {t('common.save')}
            </Button>
          </div>
        }
      >
        <form id="webhook-form" onSubmit={handleSubmit} className="space-y-4">
          {saveMut.error && (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-red-600/35 bg-red-600/10 px-3.5 py-2.5 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
            >
              <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
              <span className="break-words">{saveMut.error.message}</span>
            </div>
          )}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label htmlFor="wh-name">{t('webhooks.name')}</Label>
              <Input
                id="wh-name"
                required
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder={t('webhooks.namePlaceholder')}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="wh-tpl">{t('webhooks.template')}</Label>
              <Select id="wh-tpl" value={template} onChange={(e) => setTemplate(e.target.value)}>
                {TEMPLATES.map((tp) => (
                  <option key={tp.id} value={tp.id}>
                    {tp.label}
                  </option>
                ))}
              </Select>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="wh-url">{t('webhooks.url')}</Label>
            <Input
              id="wh-url"
              required
              type="url"
              value={url}
              onChange={(e) => setUrl(e.target.value)}
              placeholder="https://..."
            />
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="wh-secret">{t('webhooks.secret')}</Label>
            <SecretInput
              id="wh-secret"
              value={secret}
              onChange={(e) => setSecret(e.target.value)}
              placeholder={editing ? t('settings.secretPlaceholder') : ''}
            />
            {t(`webhooks.secretHints.${template}`, '') && (
              <p className="text-xs text-muted-foreground">{t(`webhooks.secretHints.${template}`, '')}</p>
            )}
          </div>

          <div className="space-y-2">
            <div className="flex items-center justify-between gap-2 flex-wrap">
              <Label>{t('webhooks.events')}</Label>
              <div className="flex items-center gap-1.5">
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="h-7 px-2 text-xs"
                  onClick={() => setEvents(CRITICAL_EVENTS.filter((e) => allEvents.includes(e)))}
                >
                  {t('webhooks.presetCritical')}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  variant="ghost"
                  className="h-7 px-2 text-xs"
                  onClick={() => setEvents([...allEvents])}
                >
                  {t('webhooks.presetAll')}
                </Button>
              </div>
            </div>
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5 rounded-lg bg-secondary/30 p-3">
              {allEvents.map((ev) => (
                <label key={ev} title={ev} className="flex items-center gap-2 text-xs cursor-pointer">
                  <Checkbox
                    checked={events.includes(ev)}
                    onCheckedChange={() => toggleEvent(ev)}
                    ariaLabel={formatEventLabel(ev)}
                  />
                  <span className="truncate">{formatEventLabel(ev)}</span>
                </label>
              ))}
            </div>
          </div>

          <label
            htmlFor="wh-enabled"
            className="flex items-center justify-between rounded-lg border border-border p-3 cursor-pointer"
          >
            <span className="text-sm">{t('common.enabled')}</span>
            <Switch id="wh-enabled" checked={enabled} onCheckedChange={setEnabled} ariaLabel={t('common.enabled')} />
          </label>
        </form>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => {
          if (deleteTarget) deleteMut.mutate(deleteTarget.id)
        }}
        title={t('webhooks.deleteConfirmTitle')}
        description={t('webhooks.deleteConfirm', { name: deleteTarget?.name ?? '' })}
        confirmLabel={t('common.delete')}
        tone="danger"
        loading={deleteMut.isPending}
      />
    </div>
  )
}
