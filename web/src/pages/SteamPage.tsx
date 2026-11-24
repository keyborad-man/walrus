import { useEffect, useRef, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { CheckCircle2, Download, RefreshCw, Terminal, AlertTriangle, Loader2 } from 'lucide-react'
import { api } from '@/lib/api'
import { formatDateTime } from '@/lib/utils'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { Label, Select } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { PageHeader } from '@/components/PageHeader'
import type { SteamLogLine, SteamStatus } from '@/types/api'

export function SteamPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()

  const { data, isError, error, refetch } = useQuery({
    queryKey: ['steam-status'],
    queryFn: api.steamStatus,
    refetchInterval: 5000,
  })

  const [liveStatus, setLiveStatus] = useState<SteamStatus | null>(null)
  const [logs, setLogs] = useState<SteamLogLine[]>([])
  const [branch, setBranch] = useState('public')
  const [validate, setValidate] = useState(true)
  const logContainerRef = useRef<HTMLDivElement | null>(null)

  const status = liveStatus ?? data?.status

  useEffect(() => {
    if (data?.status?.branch) {
      setBranch(data.status.branch)
    }
  }, [data?.status?.branch])

  useEffect(() => {
    let active = true
    let ws: WebSocket | null = null
    let timer: number | undefined

    const connect = () => {
      if (!active) return
      ws = new WebSocket(api.wsSteamLogsUrl())
      ws.onmessage = (ev) => {
        if (!active) return
        try {
          const msg = JSON.parse(ev.data)
          if (msg.type === 'history') {
            if (Array.isArray(msg.logs)) setLogs(msg.logs)
            if (msg.status) setLiveStatus(msg.status)
          } else if (msg.type === 'log' && msg.entry) {
            setLogs((prev) => {
              const incoming = msg.entry as SteamLogLine
              if (prev.some((x) => x.seq === incoming.seq && x.message === incoming.message)) {
                return prev
              }
              return [...prev.slice(-399), incoming]
            })
          } else if (msg.type === 'status' && msg.status) {
            setLiveStatus(msg.status)
            void qc.invalidateQueries({ queryKey: ['steam-status'] })
          }
        } catch {
          // ignore
        }
      }
      ws.onclose = () => {
        if (active) timer = window.setTimeout(connect, 3000)
      }
      ws.onerror = () => ws?.close()
    }

    connect()
    return () => {
      active = false
      if (timer) window.clearTimeout(timer)
      ws?.close()
    }
  }, [qc])

  useEffect(() => {
    if (logContainerRef.current) {
      logContainerRef.current.scrollTop = logContainerRef.current.scrollHeight
    }
  }, [logs])

  const checkMut = useMutation({
    mutationFn: api.steamCheck,
    onSuccess: (res) => {
      setLiveStatus(res.status)
      void qc.invalidateQueries({ queryKey: ['steam-status'] })
      toast.success(t('common.actionSuccess'))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const updateMut = useMutation({
    mutationFn: () => api.steamUpdate({ branch, validate }),
    onSuccess: (res) => {
      setLiveStatus(res.status)
      toast.success(t('steam.updating'))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const isRunning = status?.task_state === 'running'

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={t('nav.system')}
        title={t('steam.title')}
        subtitle={t('steam.subtitle')}
        actions={
          <>
            <Button variant="outline" disabled={checkMut.isPending} onClick={() => checkMut.mutate()}>
              <RefreshCw className={`h-4 w-4 ${checkMut.isPending ? 'animate-spin' : ''}`} />
              {t('steam.checkNow')}
            </Button>
            <Button disabled={isRunning || updateMut.isPending} onClick={() => updateMut.mutate()}>
              {isRunning ? <Loader2 className="h-4 w-4 animate-spin" /> : <Download className="h-4 w-4" />}
              {isRunning ? t('steam.updating') : t('steam.startUpdate')}
            </Button>
          </>
        }
      />

      {isError && <QueryErrorCard error={error} onRetry={() => void refetch()} compact />}

      {/* Status Summary Cards */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t('steam.localVersion')}</CardDescription>
            <CardTitle className="text-2xl font-mono flex items-center gap-2 flex-wrap tabular-nums">
              <span>{status?.local_version || '—'}</span>
              <Badge variant={status?.installed ? 'success' : 'warning'}>
                {status?.installed ? t('steam.installed') : t('steam.notInstalled')}
              </Badge>
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground tabular-nums">
            {t('steam.lastUpdatedLabel')}:{' '}
            {status?.last_updated_at ? formatDateTime(status.last_updated_at) : t('steam.neverRecorded')}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>{t('steam.remoteVersion')}</CardDescription>
            <CardTitle className="text-2xl font-mono flex items-center gap-2 flex-wrap tabular-nums">
              <span>{status?.remote_version || '—'}</span>
              {status?.remote_version &&
                (status.update_needed ? (
                  <Badge variant="warning">
                    <AlertTriangle className="h-3.5 w-3.5" />
                    {t('steam.updateAvailable')}
                  </Badge>
                ) : (
                  <Badge variant="success">
                    <CheckCircle2 className="h-3.5 w-3.5" />
                    {t('steam.upToDate')}
                  </Badge>
                ))}
            </CardTitle>
          </CardHeader>
          <CardContent className="text-xs text-muted-foreground tabular-nums">
            {t('steam.lastCheckedLabel')}:{' '}
            {status?.last_checked_at ? formatDateTime(status.last_checked_at) : t('steam.neverRecorded')}
          </CardContent>
        </Card>

        <Card>
          <CardHeader className="pb-2">
            <CardDescription>SteamCMD</CardDescription>
            <CardTitle className="text-base flex items-center gap-2 flex-wrap">
              <Badge
                variant={
                  status?.task_state === 'running'
                    ? 'warning'
                    : status?.task_state === 'success'
                      ? 'success'
                      : status?.task_state === 'failed'
                        ? 'destructive'
                        : 'secondary'
                }
              >
                {status?.task_state === 'running' ? (
                  <>
                    <Loader2 className="h-3 w-3 animate-spin shrink-0" aria-hidden="true" />
                    {t('steam.statusRunning')}
                  </>
                ) : status?.task_state === 'success' ? (
                  t('steam.statusSuccess')
                ) : status?.task_state === 'failed' ? (
                  t('steam.statusFailed')
                ) : (
                  t('steam.statusIdle')
                )}
              </Badge>
              {status?.runtime_mode && (
                <Badge variant="secondary">
                  {status.runtime_mode === 'mock' ? t('common.mockBadge') : t('common.linuxBadge')}
                </Badge>
              )}
            </CardTitle>
          </CardHeader>
          <CardContent className="space-y-3">
            <div className="space-y-1.5">
              <div className="flex items-center justify-between gap-2">
                <Label htmlFor="steam-branch" className="text-xs">
                  {t('settings.branch')}
                </Label>
                <Select
                  id="steam-branch"
                  className="w-36 h-9 sm:h-8 text-xs"
                  value={branch}
                  disabled={isRunning}
                  onChange={(e) => setBranch(e.target.value)}
                >
                  <option value="public">public</option>
                  <option value="updatebeta">updatebeta</option>
                </Select>
              </div>
              <p className="text-xs text-muted-foreground leading-relaxed">{t('steam.branchRunHint')}</p>
            </div>
            <label htmlFor="steam-validate" className="flex items-center justify-between gap-2 text-xs cursor-pointer">
              <span className="text-muted-foreground">{t('steam.validateFiles')}</span>
              <Switch
                id="steam-validate"
                checked={validate}
                disabled={isRunning}
                onCheckedChange={setValidate}
                ariaLabel={t('steam.validateFiles')}
              />
            </label>
          </CardContent>
        </Card>
      </div>

      {/* Live SteamCMD Log Terminal */}
      <Card>
        <CardHeader className="pb-3">
          <CardTitle className="flex items-center gap-2 text-sm">
            <Terminal className="h-4 w-4 text-primary" />
            {t('steam.terminalTitle')}
          </CardTitle>
        </CardHeader>
        <CardContent>
          <div
            ref={logContainerRef}
            className="h-[380px] overflow-y-auto rounded-xl border border-[var(--surface-terminal-border)] bg-[var(--surface-terminal)] p-4 font-mono text-xs leading-relaxed text-slate-200"
          >
            {logs.length === 0 ? (
              <div className="h-full flex items-center justify-center text-slate-400 text-center px-4">
                {t('steam.emptyLogs')}
              </div>
            ) : (
              <div className="space-y-0.5">
                {logs.map((l, i) => (
                  <div
                    key={`${l.seq}-${i}`}
                    className={
                      l.message.includes('Success') || l.message.includes('completed')
                        ? 'text-emerald-400'
                        : l.message.toLowerCase().includes('error') || l.message.toLowerCase().includes('failed')
                          ? 'text-red-400'
                          : l.message.startsWith('[walrus]')
                            ? 'text-amber-300'
                            : 'text-slate-200'
                    }
                  >
                    {l.message}
                  </div>
                ))}
              </div>
            )}
          </div>
        </CardContent>
      </Card>
    </div>
  )
}
