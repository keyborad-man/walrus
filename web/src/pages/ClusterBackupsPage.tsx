import React, { useState } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { AlertTriangle, Archive, Clock, Download, Pencil, Play, Plus, RotateCcw, Trash2 } from 'lucide-react'
import { api } from '@/lib/api'
import { validateCronExpr } from '@/lib/dst'
import { cn, describeCron, formatBytes, formatDateTime } from '@/lib/utils'
import { useAuth } from '@/hooks/useAuth'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button, buttonVariants } from '@/components/ui/button'
import { Input, Label, Select } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { TabList } from '@/components/ui/tabs'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { Dialog } from '@/components/ui/dialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { PageHeader } from '@/components/PageHeader'
import { EmptyState } from '@/components/EmptyState'
import { TableRowsSkeleton } from '@/components/ui/skeleton'
import type { BackupRecord, ScheduledTask } from '@/types/api'

const CRON_PRESET_KEYS = [
  { key: 'daily4', expr: '0 4 * * *' },
  { key: 'every6h', expr: '0 */6 * * *' },
  { key: 'every12h', expr: '0 */12 * * *' },
  { key: 'hourly', expr: '0 * * * *' },
  { key: 'weeklySun3', expr: '0 3 * * 0' },
] as const

type ScheduleTaskType = 'backup' | 'restart' | 'announce' | 'update' | 'mod_update'

export function ClusterBackupsPage() {
  const { id = '' } = useParams<{ id: string }>()
  const { t, i18n } = useTranslation()
  const { user } = useAuth()
  const qc = useQueryClient()
  const isEn = i18n.language === 'en-US'

  const [tab, setTab] = useState<'backups' | 'schedules'>('backups')
  const [note, setNote] = useState('')

  // Schedule dialog state
  const [schedOpen, setSchedOpen] = useState(false)
  const [editingTask, setEditingTask] = useState<ScheduledTask | null>(null)
  const [taskName, setTaskName] = useState('')
  const [taskType, setTaskType] = useState<ScheduleTaskType>('backup')
  const [cronExpr, setCronExpr] = useState('0 4 * * *')
  const [scopeAll, setScopeAll] = useState(false)
  const [countdownSec, setCountdownSec] = useState(60)
  const [announceMsg, setAnnounceMsg] = useState('')
  const [autoUpgrade, setAutoUpgrade] = useState(true)
  const [onlyWhenEmpty, setOnlyWhenEmpty] = useState(true)
  const [backupBeforeUpdate, setBackupBeforeUpdate] = useState(false)
  const [taskEnabled, setTaskEnabled] = useState(true)
  const [runningTaskId, setRunningTaskId] = useState<number | null>(null)

  // Confirm states
  const [restoreTarget, setRestoreTarget] = useState<BackupRecord | null>(null)
  const [deleteBackupTarget, setDeleteBackupTarget] = useState<BackupRecord | null>(null)
  const [deleteSchedTarget, setDeleteSchedTarget] = useState<ScheduledTask | null>(null)

  const { data: clusterData } = useQuery({
    queryKey: ['cluster', id],
    queryFn: () => api.getCluster(id),
    enabled: !!id,
  })
  const canOperate = clusterData?.effective_role === 'admin' || clusterData?.effective_role === 'operator'
  const isAdmin = user?.role === 'admin'
  const onlinePlayersCount = clusterData?.cluster.world_info?.online_players?.length ?? 0

  const {
    data,
    isLoading,
    isError: backupsIsError,
    error: backupsError,
    refetch: refetchBackups,
  } = useQuery({
    queryKey: ['backups', id],
    queryFn: () => api.listBackups(id),
    enabled: !!id,
  })
  const backups = data?.backups ?? []

  const {
    data: schedData,
    isLoading: loadingSched,
    isError: schedIsError,
    error: schedError,
    refetch: refetchSched,
  } = useQuery({
    queryKey: ['schedules', id],
    queryFn: () => api.listClusterSchedules(id),
    enabled: !!id,
    refetchInterval: tab === 'schedules' ? 5000 : false,
  })
  const tasks = schedData?.tasks ?? []

  const openCreateSched = () => {
    setEditingTask(null)
    setTaskName('')
    setTaskType('backup')
    setCronExpr('0 4 * * *')
    setScopeAll(false)
    setCountdownSec(60)
    setAnnounceMsg('')
    setAutoUpgrade(true)
    setOnlyWhenEmpty(true)
    setBackupBeforeUpdate(false)
    setTaskEnabled(true)
    setSchedOpen(true)
  }

  const openEditSched = (task: ScheduledTask) => {
    let parsed: Record<string, unknown> = {}
    try {
      parsed = JSON.parse(task.params_json || '{}')
    } catch {
      parsed = {}
    }
    setEditingTask(task)
    setTaskName(task.name)
    setTaskType((task.task_type as ScheduleTaskType) || 'backup')
    setCronExpr(task.cron_expr)
    setScopeAll(task.cluster_id === '*')
    setCountdownSec(typeof parsed.countdown_sec === 'number' ? parsed.countdown_sec : 60)
    setAnnounceMsg(
      typeof parsed.message === 'string'
        ? parsed.message
        : typeof parsed.note === 'string'
          ? parsed.note
          : '',
    )
    setAutoUpgrade(typeof parsed.auto_upgrade === 'boolean' ? parsed.auto_upgrade : true)
    setOnlyWhenEmpty(typeof parsed.only_when_empty === 'boolean' ? parsed.only_when_empty : true)
    setBackupBeforeUpdate(typeof parsed.backup_before_update === 'boolean' ? parsed.backup_before_update : false)
    setTaskEnabled(task.enabled)
    setSchedOpen(true)
  }

  const createMut = useMutation({
    mutationFn: () => api.createBackup(id, note),
    onSuccess: () => {
      toast.success(t('backups.toastCreated'))
      setNote('')
      void qc.invalidateQueries({ queryKey: ['backups', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const restoreMut = useMutation({
    mutationFn: (bid: number) => api.restoreBackup(id, bid),
    onSuccess: () => {
      toast.success(t('backups.restoredSuccess'))
      setRestoreTarget(null)
      void qc.invalidateQueries({ queryKey: ['backups', id] })
      void qc.invalidateQueries({ queryKey: ['cluster', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (bid: number) => api.deleteBackup(id, bid),
    onSuccess: () => {
      toast.success(t('common.actionSuccess'))
      setDeleteBackupTarget(null)
      void qc.invalidateQueries({ queryKey: ['backups', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const saveSchedMut = useMutation({
    mutationFn: () => {
      const params: Record<string, unknown> = {}
      if (taskType === 'restart') {
        params.countdown_sec = Math.max(0, Number(countdownSec) || 0)
        if (announceMsg.trim()) params.message = announceMsg.trim()
      } else if (taskType === 'announce') {
        params.message = announceMsg.trim()
      } else if (taskType === 'update' || taskType === 'mod_update') {
        params.auto_upgrade = autoUpgrade
        params.only_when_empty = onlyWhenEmpty
        params.backup_before_update = backupBeforeUpdate
        params.countdown_sec = Math.max(0, Number(countdownSec) || 0)
        if (announceMsg.trim()) params.message = announceMsg.trim()
      } else if (taskType === 'backup' && announceMsg.trim()) {
        params.note = announceMsg.trim()
      }
      const payload = {
        cluster_id: isAdmin && (scopeAll || taskType === 'update') ? '*' : id,
        name: taskName.trim(),
        task_type: taskType,
        cron_expr: cronExpr.trim(),
        params,
        enabled: taskEnabled,
      }
      return editingTask ? api.updateSchedule(editingTask.id, payload) : api.createSchedule(payload)
    },
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setSchedOpen(false)
      setEditingTask(null)
      setTaskName('')
      setAnnounceMsg('')
      void qc.invalidateQueries({ queryKey: ['schedules', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const toggleSchedMut = useMutation({
    mutationFn: (task: ScheduledTask) => {
      let parsed: Record<string, unknown> = {}
      try {
        parsed = JSON.parse(task.params_json || '{}')
      } catch {
        parsed = {}
      }
      return api.updateSchedule(task.id, {
        cluster_id: task.cluster_id,
        name: task.name,
        task_type: task.task_type,
        cron_expr: task.cron_expr,
        params: parsed,
        enabled: !task.enabled,
      })
    },
    onSuccess: () => void qc.invalidateQueries({ queryKey: ['schedules', id] }),
    onError: (err: Error) => toast.error(err.message),
  })

  const runSchedMut = useMutation({
    mutationFn: (tid: number) => api.runScheduleNow(tid),
    onMutate: (tid) => setRunningTaskId(tid),
    onSuccess: () => {
      toast.success(t('common.actionSuccess'))
      setTimeout(() => {
        void qc.invalidateQueries({ queryKey: ['schedules', id] })
        void qc.invalidateQueries({ queryKey: ['backups', id] })
      }, 1500)
    },
    onError: (err: Error) => toast.error(err.message),
    onSettled: () => setRunningTaskId(null),
  })

  const deleteSchedMut = useMutation({
    mutationFn: (tid: number) => api.deleteSchedule(tid),
    onSuccess: () => {
      toast.success(t('common.actionSuccess'))
      setDeleteSchedTarget(null)
      void qc.invalidateQueries({ queryKey: ['schedules', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const handleCreate = (e: React.FormEvent) => {
    e.preventDefault()
    createMut.mutate()
  }

  const cronValidation = validateCronExpr(cronExpr, isEn)
  const cronPreview = cronValidation.description

  const renderBackupActions = (b: BackupRecord) => (
    <>
      <a
        href={api.backupDownloadUrl(id, b.id)}
        download={b.filename}
        className={buttonVariants({ size: 'sm', variant: 'outline' })}
      >
        <Download className="h-3.5 w-3.5" />
        {t('backups.download')}
      </a>
      {canOperate && (
        <>
          <Button
            size="sm"
            variant="warning"
            disabled={restoreMut.isPending}
            onClick={() => setRestoreTarget(b)}
          >
            <RotateCcw className="h-3.5 w-3.5" />
            {t('backups.restore')}
          </Button>
          <Button
            size="icon"
            variant="ghost"
            aria-label={`${t('common.delete')} ${b.filename}`}
            title={t('common.delete')}
            className="text-red-700 hover:bg-red-600/12 dark:text-red-400 dark:hover:bg-red-500/12"
            disabled={deleteMut.isPending}
            onClick={() => setDeleteBackupTarget(b)}
          >
            <Trash2 className="h-4 w-4" />
          </Button>
        </>
      )}
    </>
  )

  return (
    <div className="space-y-6">
      <PageHeader eyebrow={t('nav.groupCare')} title={t('nav.backups')} subtitle={t('backups.subtitle')} />

      {/* Tab Switcher */}
      <TabList
        ariaLabel={t('nav.backups')}
        idPrefix="backups-tabs"
        value={tab}
        onChange={setTab}
        items={[
          {
            key: 'backups',
            label: `${t('schedules.tabBackups')} (${backups.length})`,
            icon: <Archive className="h-3.5 w-3.5" />,
          },
          {
            key: 'schedules',
            label: `${t('schedules.tabSchedules')} (${tasks.length})`,
            icon: <Clock className="h-3.5 w-3.5" />,
          },
        ]}
      />

      {/* Backups Tab */}
      {tab === 'backups' && (
        <div id="backups-tabs-panel-backups" role="tabpanel" aria-labelledby="backups-tabs-tab-backups" className="space-y-4">
          {backupsIsError && <QueryErrorCard error={backupsError} onRetry={() => void refetchBackups()} compact />}
          {canOperate && (
            <Card>
              <CardContent className="p-4 sm:p-5">
                <form onSubmit={handleCreate} className="flex flex-col sm:flex-row gap-3">
                  <Input
                    value={note}
                    onChange={(e) => setNote(e.target.value)}
                    placeholder={t('backups.notePlaceholder')}
                    aria-label={t('backups.notePlaceholder')}
                    className="flex-1"
                  />
                  <Button type="submit" disabled={createMut.isPending}>
                    <Plus className="h-4 w-4" />
                    {createMut.isPending ? t('common.saving') : t('backups.createBtn')}
                  </Button>
                </form>
              </CardContent>
            </Card>
          )}

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 tabular-nums">
                <Archive className="h-4 w-4 text-primary" />
                {t('backups.title')} ({backups.length})
              </CardTitle>
            </CardHeader>
            <CardContent>
              {isLoading ? (
                <TableRowsSkeleton rows={4} />
              ) : backups.length === 0 ? (
                <EmptyState compact icon={<Archive className="h-5 w-5" />} title={t('backups.title')} description={t('backups.emptyBackups')} />
              ) : (
                <>
                  {/* Mobile rows (< md) */}
                  <div className="divide-y divide-border md:hidden">
                    {backups.map((b) => (
                      <div key={b.id} className="py-3.5 first:pt-0 last:pb-0 space-y-2.5">
                        <div className="flex items-start justify-between gap-2">
                          <div className="min-w-0">
                            <div className="font-mono text-xs font-medium break-all">{b.filename}</div>
                            <div className="text-xs text-muted-foreground tabular-nums mt-0.5">
                              {formatBytes(b.size_bytes)} · {formatDateTime(b.created_at)}
                            </div>
                          </div>
                          <Badge variant={b.trigger === 'pre_restore' ? 'warning' : 'secondary'}>
                            {t(`backups.triggers.${b.trigger}`, b.trigger)}
                          </Badge>
                        </div>
                        {b.note && <p className="text-xs text-muted-foreground">{b.note}</p>}
                        <div className="flex flex-wrap items-center gap-1.5 pt-1">{renderBackupActions(b)}</div>
                      </div>
                    ))}
                  </div>

                  {/* Desktop table (>= md) */}
                  <div className="hidden md:block overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-border text-left text-xs text-muted-foreground">
                          <th className="pb-3 pr-4 font-medium">{t('backups.filename')}</th>
                          <th className="pb-3 pr-4 font-medium">{t('backups.trigger')}</th>
                          <th className="pb-3 pr-4 font-medium">{t('backups.size')}</th>
                          <th className="pb-3 pr-4 font-medium">{t('backups.note')}</th>
                          <th className="pb-3 pr-4 font-medium">{t('backups.createdAt')}</th>
                          <th className="pb-3 text-right font-medium">{t('common.actions')}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {backups.map((b) => (
                          <tr key={b.id} className="hover:bg-secondary/30">
                            <td className="py-3 pr-4 font-mono text-xs">{b.filename}</td>
                            <td className="py-3 pr-4">
                              <Badge variant={b.trigger === 'pre_restore' ? 'warning' : 'secondary'}>
                                {t(`backups.triggers.${b.trigger}`, b.trigger)}
                              </Badge>
                            </td>
                            <td className="py-3 pr-4 font-mono text-xs tabular-nums whitespace-nowrap">
                              {formatBytes(b.size_bytes)}
                            </td>
                            <td className="py-3 pr-4 text-xs text-muted-foreground max-w-xs truncate">
                              {b.note || '-'}
                            </td>
                            <td className="py-3 pr-4 text-xs text-muted-foreground whitespace-nowrap tabular-nums">
                              {formatDateTime(b.created_at)}
                            </td>
                            <td className="py-3 text-right whitespace-nowrap">
                              <div className="inline-flex items-center gap-1.5">{renderBackupActions(b)}</div>
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
        </div>
      )}

      {/* Schedules Tab */}
      {tab === 'schedules' && (
        <div id="backups-tabs-panel-schedules" role="tabpanel" aria-labelledby="backups-tabs-tab-schedules" className="space-y-4">
          {schedIsError && <QueryErrorCard error={schedError} onRetry={() => void refetchSched()} compact />}
          {canOperate && tasks.length > 0 && (
            <div className="flex justify-end">
              <Button onClick={openCreateSched}>
                <Plus className="h-4 w-4" />
                {t('schedules.addTask')}
              </Button>
            </div>
          )}

          <Card>
            <CardHeader>
              <CardTitle className="flex items-center gap-2 tabular-nums">
                <Clock className="h-4 w-4 text-primary" />
                {t('schedules.tabSchedules')} ({tasks.length})
              </CardTitle>
            </CardHeader>
            <CardContent>
              {loadingSched ? (
                <div className="py-10 text-center text-sm text-muted-foreground">{t('common.loading')}</div>
              ) : tasks.length === 0 ? (
                <div className="py-10 text-center space-y-3">
                  <p className="text-xs text-muted-foreground max-w-md mx-auto leading-relaxed text-balance">
                    {t('schedules.emptySchedules')}
                  </p>
                  {canOperate && (
                    <Button size="sm" variant="secondary" onClick={openCreateSched}>
                      <Plus className="h-3.5 w-3.5" />
                      {t('schedules.addTask')}
                    </Button>
                  )}
                </div>
              ) : (
                <>
                  {/* Mobile stacked cards (< md) */}
                  <div className="divide-y divide-border md:hidden">
                    {tasks.map((task) => {
                      const canModifyTask = isAdmin || task.cluster_id !== '*'
                      const desc = describeCron(task.cron_expr, isEn)
                      return (
                        <div key={task.id} className="py-3.5 first:pt-0 last:pb-0 space-y-2.5">
                          <div className="flex items-start justify-between gap-2">
                            <div className="min-w-0">
                              <div className="font-semibold text-sm truncate">{task.name}</div>
                              <div className="mt-1 flex flex-wrap items-center gap-1.5">
                                <Badge variant="default">
                                  {t(`schedules.types.${task.task_type}`, task.task_type)}
                                </Badge>
                                <span className="font-mono text-xs text-muted-foreground">
                                  {task.cluster_id === '*' ? t('schedules.scopeAll') : task.cluster_id}
                                </span>
                              </div>
                            </div>
                            <Switch
                              checked={task.enabled}
                              disabled={!canOperate || !canModifyTask}
                              ariaLabel={`${task.name} ${t('common.enabled')}`}
                              onCheckedChange={() => toggleSchedMut.mutate(task)}
                            />
                          </div>
                          <div className="text-xs text-muted-foreground space-y-0.5 tabular-nums">
                            <div>
                              <span className="font-mono text-foreground">{task.cron_expr}</span>
                              {desc ? ` · ${desc}` : ''}
                            </div>
                            <div>
                              {t('schedules.lastRun')}: {task.last_run_at ? formatDateTime(task.last_run_at) : '—'}
                              {task.last_status ? ` · ${task.last_status}` : ''}
                            </div>
                          </div>
                          {canOperate && canModifyTask && (
                            <div className="flex flex-wrap items-center gap-1.5 pt-1">
                              <Button
                                size="sm"
                                variant="secondary"
                                disabled={runningTaskId === task.id}
                                onClick={() => runSchedMut.mutate(task.id)}
                              >
                                <Play className="h-3.5 w-3.5" />
                                {t('schedules.runNow')}
                              </Button>
                              <Button
                                size="icon"
                                variant="ghost"
                                aria-label={`${t('common.edit')} ${task.name}`}
                                title={t('common.edit')}
                                onClick={() => openEditSched(task)}
                              >
                                <Pencil className="h-4 w-4" />
                              </Button>
                              <Button
                                size="icon"
                                variant="ghost"
                                aria-label={`${t('common.delete')} ${task.name}`}
                                title={t('common.delete')}
                                className="text-red-700 hover:bg-red-600/12 dark:text-red-400 dark:hover:bg-red-500/12"
                                onClick={() => setDeleteSchedTarget(task)}
                              >
                                <Trash2 className="h-4 w-4" />
                              </Button>
                            </div>
                          )}
                        </div>
                      )
                    })}
                  </div>

                  {/* Desktop table (>= md) */}
                  <div className="hidden md:block overflow-x-auto">
                    <table className="w-full text-sm">
                      <thead>
                        <tr className="border-b border-border text-left text-xs text-muted-foreground">
                          <th className="pb-3 pr-4 font-medium">{t('schedules.name')}</th>
                          <th className="pb-3 pr-4 font-medium">{t('schedules.type')}</th>
                          <th className="pb-3 pr-4 font-medium">Cron</th>
                          <th className="pb-3 pr-4 font-medium">{t('schedules.scope')}</th>
                          <th className="pb-3 pr-4 font-medium">{t('schedules.lastRun')}</th>
                          <th className="pb-3 pr-4 font-medium">{t('common.status')}</th>
                          <th className="pb-3 text-right font-medium">{t('common.actions')}</th>
                        </tr>
                      </thead>
                      <tbody className="divide-y divide-border">
                        {tasks.map((task) => {
                          const canModifyTask = isAdmin || task.cluster_id !== '*'
                          const desc = describeCron(task.cron_expr, isEn)
                          return (
                            <tr key={task.id} className="hover:bg-secondary/30">
                              <td className="py-3 pr-4 font-medium text-xs">{task.name}</td>
                              <td className="py-3 pr-4">
                                <Badge variant="default">{t(`schedules.types.${task.task_type}`, task.task_type)}</Badge>
                              </td>
                              <td className="py-3 pr-4 text-xs">
                                <div className="font-mono">{task.cron_expr}</div>
                                {desc && <div className="text-muted-foreground">{desc}</div>}
                              </td>
                              <td className="py-3 pr-4 font-mono text-xs">
                                {task.cluster_id === '*' ? t('schedules.scopeAll') : task.cluster_id}
                              </td>
                              <td className="py-3 pr-4 text-xs text-muted-foreground tabular-nums">
                                <div>{task.last_run_at ? formatDateTime(task.last_run_at) : '—'}</div>
                                {task.last_status && (
                                  <div
                                    title={task.last_status}
                                    className={cn(
                                      'font-mono text-xs truncate max-w-[220px]',
                                      task.last_status.startsWith('error:') && 'text-red-700 dark:text-red-400',
                                    )}
                                  >
                                    {task.last_status}
                                  </div>
                                )}
                              </td>
                              <td className="py-3 pr-4">
                                <Switch
                                  checked={task.enabled}
                                  disabled={!canOperate || !canModifyTask}
                                  ariaLabel={`${task.name} ${t('common.enabled')}`}
                                  onCheckedChange={() => toggleSchedMut.mutate(task)}
                                />
                              </td>
                              <td className="py-3 text-right whitespace-nowrap">
                                {canOperate && canModifyTask && (
                                  <div className="inline-flex items-center gap-1">
                                    <Button
                                      size="sm"
                                      variant="secondary"
                                      disabled={runningTaskId === task.id}
                                      onClick={() => runSchedMut.mutate(task.id)}
                                    >
                                      <Play className="h-3.5 w-3.5" />
                                      {t('schedules.runNow')}
                                    </Button>
                                    <Button
                                      size="icon"
                                      variant="ghost"
                                      aria-label={`${t('common.edit')} ${task.name}`}
                                      title={t('common.edit')}
                                      onClick={() => openEditSched(task)}
                                    >
                                      <Pencil className="h-4 w-4" />
                                    </Button>
                                    <Button
                                      size="icon"
                                      variant="ghost"
                                      aria-label={`${t('common.delete')} ${task.name}`}
                                      title={t('common.delete')}
                                      className="text-red-700 hover:bg-red-600/12 dark:text-red-400 dark:hover:bg-red-500/12"
                                      onClick={() => setDeleteSchedTarget(task)}
                                    >
                                      <Trash2 className="h-4 w-4" />
                                    </Button>
                                  </div>
                                )}
                              </td>
                            </tr>
                          )
                        })}
                      </tbody>
                    </table>
                  </div>
                </>
              )}
            </CardContent>
          </Card>
        </div>
      )}

      {/* Create / Edit Scheduled Task Dialog */}
      <Dialog
        open={schedOpen}
        onClose={() => {
          setSchedOpen(false)
          setEditingTask(null)
        }}
        title={editingTask ? `${t('schedules.editTask')}: ${editingTask.name}` : t('schedules.addTask')}
        footer={
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              onClick={() => {
                setSchedOpen(false)
                setEditingTask(null)
              }}
            >
              {t('common.cancel')}
            </Button>
            <Button
              type="submit"
              form="schedule-task-form"
              disabled={saveSchedMut.isPending || !cronValidation.valid || !taskName.trim()}
            >
              {t('common.save')}
            </Button>
          </div>
        }
      >
        <form
          id="schedule-task-form"
          onSubmit={(e) => {
            e.preventDefault()
            if (!cronValidation.valid) return
            saveSchedMut.mutate()
          }}
          className="space-y-4"
        >
          {saveSchedMut.error && (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-red-600/35 bg-red-600/10 px-3.5 py-2.5 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
            >
              <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
              <span className="break-words">{saveSchedMut.error.message}</span>
            </div>
          )}
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label htmlFor="sched-name">{t('schedules.name')}</Label>
              <Input
                id="sched-name"
                required
                value={taskName}
                onChange={(e) => setTaskName(e.target.value)}
                placeholder={t('schedules.namePlaceholder')}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="sched-type">{t('schedules.type')}</Label>
              <Select
                id="sched-type"
                value={taskType}
                onChange={(e) => setTaskType(e.target.value as ScheduleTaskType)}
              >
                <option value="backup">{t('schedules.types.backup')}</option>
                <option value="restart">{t('schedules.types.restart')}</option>
                <option value="announce">{t('schedules.types.announce')}</option>
                {isAdmin && <option value="update">{t('schedules.types.update')}</option>}
                <option value="mod_update">{t('schedules.types.mod_update')}</option>
              </Select>
            </div>
          </div>

          <div className="space-y-1.5">
            <Label htmlFor="sched-cron">{t('schedules.cronExpr')}</Label>
            <Input
              id="sched-cron"
              required
              aria-invalid={!cronValidation.valid}
              value={cronExpr}
              onChange={(e) => setCronExpr(e.target.value)}
              className={cn('font-mono', !cronValidation.valid && 'border-red-600 focus-visible:ring-red-600')}
            />
            {!cronValidation.valid ? (
              <div role="alert" className="text-xs font-medium text-red-700 dark:text-red-400">
                {t('schedules.cronInvalid')}
              </div>
            ) : (
              cronPreview && (
                <div className="text-xs font-medium text-emerald-800 dark:text-emerald-300">
                  {t('schedules.cronPreviewLabel')} {cronPreview}
                </div>
              )
            )}
            <p className="text-xs text-muted-foreground">{t('schedules.cronHint')}</p>
            <div className="flex flex-wrap gap-1.5 pt-1">
              {CRON_PRESET_KEYS.map((p) => (
                <button
                  key={p.expr}
                  type="button"
                  onClick={() => setCronExpr(p.expr)}
                  className={cn(
                    'inline-flex min-h-10 sm:min-h-7 items-center rounded-full border px-3 py-1.5 sm:px-2.5 sm:py-1 text-xs cursor-pointer transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                    cronExpr === p.expr
                      ? 'border-foreground/30 bg-secondary text-foreground font-semibold ring-1 ring-border'
                      : 'border-border bg-secondary/40 text-muted-foreground hover:text-foreground hover:border-foreground/25',
                  )}
                >
                  {t(`schedules.presets.${p.key}`)} ({p.expr})
                </button>
              ))}
            </div>
          </div>

          {taskType === 'restart' && (
            <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <Label htmlFor="sched-cd">{t('schedules.countdownSec')}</Label>
                <Input
                  id="sched-cd"
                  type="number"
                  min={0}
                  max={600}
                  value={countdownSec}
                  onChange={(e) => setCountdownSec(Number(e.target.value))}
                />
              </div>
              <div className="space-y-1.5">
                <Label htmlFor="sched-rmsg">{t('schedules.announceMsg')}</Label>
                <Input
                  id="sched-rmsg"
                  value={announceMsg}
                  onChange={(e) => setAnnounceMsg(e.target.value)}
                  placeholder={t('schedules.restartMsgPlaceholder')}
                />
              </div>
            </div>
          )}

          {taskType === 'announce' && (
            <div className="space-y-1.5">
              <Label htmlFor="sched-amsg">{t('schedules.announceMsg')}</Label>
              <Input
                id="sched-amsg"
                required
                value={announceMsg}
                onChange={(e) => setAnnounceMsg(e.target.value)}
                placeholder={t('schedules.announceMsgPlaceholder')}
              />
            </div>
          )}

          {(taskType === 'update' || taskType === 'mod_update') && (
            <div className="space-y-3">
              <label
                htmlFor="sched-auto-upgrade"
                className="flex items-start justify-between gap-3 rounded-lg border border-border p-3 cursor-pointer"
              >
                <div className="space-y-0.5">
                  <div className="text-sm font-medium">{t('schedules.autoUpgrade')}</div>
                  <p className="text-xs text-muted-foreground leading-relaxed">{t('schedules.autoUpgradeHint')}</p>
                </div>
                <Switch
                  id="sched-auto-upgrade"
                  checked={autoUpgrade}
                  onCheckedChange={setAutoUpgrade}
                  ariaLabel={t('schedules.autoUpgrade')}
                />
              </label>

              {autoUpgrade && (
                <>
                  <label
                    htmlFor="sched-only-empty"
                    className="flex items-center justify-between gap-3 rounded-lg border border-border p-3 cursor-pointer"
                  >
                    <span className="text-sm">
                      {taskType === 'mod_update' ? t('schedules.onlyWhenEmptyMods') : t('schedules.onlyWhenEmpty')}
                    </span>
                    <Switch
                      id="sched-only-empty"
                      checked={onlyWhenEmpty}
                      onCheckedChange={setOnlyWhenEmpty}
                      ariaLabel={
                        taskType === 'mod_update' ? t('schedules.onlyWhenEmptyMods') : t('schedules.onlyWhenEmpty')
                      }
                    />
                  </label>

                  <label
                    htmlFor="sched-backup-before"
                    className="flex items-start justify-between gap-3 rounded-lg border border-border p-3 cursor-pointer"
                  >
                    <div className="space-y-0.5">
                      <div className="text-sm">{t('schedules.backupBeforeUpdate')}</div>
                      <p className="text-xs text-muted-foreground leading-relaxed">
                        {t('schedules.backupBeforeUpdateHint')}
                      </p>
                    </div>
                    <Switch
                      id="sched-backup-before"
                      checked={backupBeforeUpdate}
                      onCheckedChange={setBackupBeforeUpdate}
                      ariaLabel={t('schedules.backupBeforeUpdate')}
                    />
                  </label>

                  {!onlyWhenEmpty && (
                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                      <div className="space-y-1.5">
                        <Label htmlFor="sched-upd-cd">{t('schedules.countdownSec')}</Label>
                        <Input
                          id="sched-upd-cd"
                          type="number"
                          min={0}
                          max={600}
                          value={countdownSec}
                          onChange={(e) => setCountdownSec(Number(e.target.value))}
                        />
                      </div>
                      <div className="space-y-1.5">
                        <Label htmlFor="sched-upd-msg">{t('schedules.announceMsg')}</Label>
                        <Input
                          id="sched-upd-msg"
                          value={announceMsg}
                          onChange={(e) => setAnnounceMsg(e.target.value)}
                          placeholder={
                            taskType === 'mod_update'
                              ? t('schedules.modUpdateMsgPlaceholder')
                              : t('schedules.updateMsgPlaceholder')
                          }
                        />
                      </div>
                    </div>
                  )}
                </>
              )}
            </div>
          )}

          {isAdmin && taskType !== 'update' && (
            <label
              htmlFor="sched-scope-all"
              className="flex items-center justify-between rounded-lg border border-border p-3 cursor-pointer"
            >
              <span className="text-sm">{t('schedules.scopeAll')}</span>
              <Switch
                id="sched-scope-all"
                checked={scopeAll}
                onCheckedChange={setScopeAll}
                ariaLabel={t('schedules.scopeAll')}
              />
            </label>
          )}

          <label
            htmlFor="sched-enabled"
            className="flex items-center justify-between rounded-lg border border-border p-3 cursor-pointer"
          >
            <span className="text-sm">{t('common.enabled')}</span>
            <Switch
              id="sched-enabled"
              checked={taskEnabled}
              onCheckedChange={setTaskEnabled}
              ariaLabel={t('common.enabled')}
            />
          </label>
        </form>
      </Dialog>

      {/* Confirmations */}
      <ConfirmDialog
        open={!!restoreTarget}
        onClose={() => setRestoreTarget(null)}
        onConfirm={() => {
          if (restoreTarget) restoreMut.mutate(restoreTarget.id)
        }}
        title={t('backups.restoreConfirmTitle')}
        description={
          restoreTarget
            ? [
                t('backups.restoreConfirmDetails', {
                  name: restoreTarget.filename,
                  time: formatDateTime(restoreTarget.created_at),
                }),
                restoreTarget.note ? t('backups.restoreConfirmNote', { note: restoreTarget.note }) : '',
                onlinePlayersCount > 0
                  ? t('backups.restoreConfirmOnlineWarn', { count: onlinePlayersCount })
                  : t('backups.restoreConfirmOfflineHint'),
              ]
                .filter(Boolean)
                .join('\n')
            : ''
        }
        confirmLabel={t('backups.restore')}
        tone="warning"
        loading={restoreMut.isPending}
      />

      <ConfirmDialog
        open={!!deleteBackupTarget}
        onClose={() => setDeleteBackupTarget(null)}
        onConfirm={() => {
          if (deleteBackupTarget) deleteMut.mutate(deleteBackupTarget.id)
        }}
        title={t('backups.deleteConfirmTitle')}
        description={t('backups.deleteConfirm', { name: deleteBackupTarget?.filename ?? '' })}
        confirmLabel={t('common.delete')}
        tone="danger"
        loading={deleteMut.isPending}
      />

      <ConfirmDialog
        open={!!deleteSchedTarget}
        onClose={() => setDeleteSchedTarget(null)}
        onConfirm={() => {
          if (deleteSchedTarget) deleteSchedMut.mutate(deleteSchedTarget.id)
        }}
        title={t('schedules.deleteConfirmTitle')}
        description={t('schedules.deleteConfirm', { name: deleteSchedTarget?.name ?? '' })}
        confirmLabel={t('common.delete')}
        tone="danger"
        loading={deleteSchedMut.isPending}
      />
    </div>
  )
}
