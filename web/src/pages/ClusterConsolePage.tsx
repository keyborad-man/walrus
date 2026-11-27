import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertTriangle,
  Bookmark,
  ChevronDown,
  CloudRain,
  Columns2,
  Copy,
  Flame,
  Flower2,
  HeartPulse,
  History,
  Mountain,
  Play,
  Plus,
  RefreshCw,
  Save,
  Send,
  Snowflake,
  Sparkles,
  Sun,
  Terminal as TerminalIcon,
  Trash2,
  Trees,
  Users,
  Wand2,
  Wifi,
  WifiOff,
  X,
} from 'lucide-react'
import { api, formatApiError, formatError } from '@/lib/api'
import { cn } from '@/lib/utils'
import { Button } from '@/components/ui/button'
import { Input, Label, Select } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { Dialog } from '@/components/ui/dialog'
import { TabList } from '@/components/ui/tabs'
import { CollapsibleContent } from '@/components/ui/collapsible'
import { copyTextWithFallback } from '@/components/ui/copy-button'
import { SearchInput } from '@/components/SearchInput'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { ShardStateBadge } from '@/components/StatusBadge'
import type { CommandSnippet, LogEntry, ShardStatus } from '@/types/api'

const CLEAN_GROUND_LUA = `local n=0; for _,v in pairs(Ents) do if v.prefab=="spoiled_food" or v.prefab=="rottenegg" or v.prefab=="ash" or v.prefab=="stinger" or v.prefab=="houndstooth" or v.prefab=="silk" then if not v.components or not v.components.inventoryitem or not v.components.inventoryitem.owner then v:Remove(); n=n+1 end end end; c_announce("Cleaned "..n.." dropped items")`
const REVIVE_ALL_LUA = `for _,p in ipairs(AllPlayers) do if p:HasTag("playerghost") then p:PushEvent("respawnfromghost") end end; c_announce("All ghost players revived!")`
const COUNT_ENTITIES_LUA = `local n=0; for _ in pairs(Ents) do n=n+1 end; print("Total world entities: "..n)`

type PendingGMConfirm =
  | { kind: 'rollback'; days: number }
  | { kind: 'skipDay' }
  | { kind: 'regen' }
  | { kind: 'deleteSnippet'; snippet: CommandSnippet }

type LevelFilter = 'all' | 'error' | 'warn' | 'chat'

interface ShardStreamState {
  connected: boolean
  shardStatus: ShardStatus | null
  entries: LogEntry[]
  clear: () => void
  send: (cmd: string, feedbackLabel?: string) => void
}

/** Persistent per-shard WebSocket log stream (keeps buffered logs when switching tabs) */
function useShardStream(clusterId: string, shard: 'master' | 'caves', enabled: boolean): ShardStreamState {
  const { t } = useTranslation()
  const [connected, setConnected] = useState(false)
  const [shardStatus, setShardStatus] = useState<ShardStatus | null>(null)
  const [entries, setEntries] = useState<LogEntry[]>([])

  const wsRef = useRef<WebSocket | null>(null)
  const pendingCmdLabelsRef = useRef<Map<string, string>>(new Map())
  const cmdSeqRef = useRef(0)
  const tRef = useRef(t)
  tRef.current = t

  // Reset buffer only when cluster changes
  useEffect(() => {
    setEntries([])
    setShardStatus(null)
  }, [clusterId])

  useEffect(() => {
    if (!clusterId || !enabled) {
      setConnected(false)
      return
    }
    let active = true
    let retryTimer: number | undefined

    const connect = () => {
      if (!active) return
      const ws = new WebSocket(api.wsLogsUrl(clusterId, shard))
      wsRef.current = ws

      ws.onopen = () => {
        if (active) setConnected(true)
      }
      ws.onmessage = (ev) => {
        if (!active) return
        try {
          const msg = JSON.parse(ev.data)
          if (msg.type === 'history' && Array.isArray(msg.entries)) {
            setEntries(msg.entries)
          } else if (msg.type === 'log' && msg.entry) {
            setEntries((prev) => {
              const incoming = msg.entry as LogEntry
              if (prev.length > 0 && prev[prev.length - 1].seq === incoming.seq && prev[prev.length - 1].message === incoming.message) {
                return prev
              }
              const next = [...prev, incoming]
              return next.length > 800 ? next.slice(next.length - 800) : next
            })
          } else if (msg.type === 'status' && msg.status) {
            setShardStatus(msg.status as ShardStatus)
          } else if (msg.type === 'command_ack') {
            const reqId = typeof msg.req_id === 'string' ? msg.req_id : ''
            const label = reqId ? pendingCmdLabelsRef.current.get(reqId) : undefined
            if (reqId) pendingCmdLabelsRef.current.delete(reqId)
            if (label) toast.success(tRef.current('console.toastCommandSent', { label }))
          } else if (msg.type === 'error' && (msg.code || msg.error)) {
            if (typeof msg.req_id === 'string') {
              pendingCmdLabelsRef.current.delete(msg.req_id)
            }
            toast.error(formatApiError(msg))
          }
        } catch {
          // ignore
        }
      }
      ws.onclose = () => {
        if (!active) return
        setConnected(false)
        pendingCmdLabelsRef.current.clear()
        retryTimer = window.setTimeout(connect, 2500)
      }
      ws.onerror = () => ws.close()
    }

    connect()
    return () => {
      active = false
      pendingCmdLabelsRef.current.clear()
      if (retryTimer) window.clearTimeout(retryTimer)
      wsRef.current?.close()
      wsRef.current = null
    }
  }, [clusterId, shard, enabled])

  const clear = useCallback(() => setEntries([]), [])

  const send = useCallback(
    (cmd: string, feedbackLabel?: string) => {
      const clean = cmd.trim()
      if (!clean) return
      if (wsRef.current && wsRef.current.readyState === WebSocket.OPEN) {
        const reqId = `cmd-${++cmdSeqRef.current}`
        if (feedbackLabel) {
          pendingCmdLabelsRef.current.set(reqId, feedbackLabel)
        }
        wsRef.current.send(JSON.stringify({ type: 'command', command: clean, req_id: reqId }))
      } else {
        void api
          .sendCommand(clusterId, shard, clean)
          .then(() => {
            if (feedbackLabel) toast.success(tRef.current('console.toastCommandSent', { label: feedbackLabel }))
          })
          .catch((err: Error) => toast.error(err.message))
      }
    },
    [clusterId, shard],
  )

  return { connected, shardStatus, entries, clear, send }
}

export function ClusterConsolePage() {
  const { id = '' } = useParams<{ id: string }>()
  const { t } = useTranslation()
  const qc = useQueryClient()

  const { data } = useQuery({
    queryKey: ['cluster', id],
    queryFn: () => api.getCluster(id),
    enabled: !!id,
  })

  const { data: snippetsData } = useQuery({
    queryKey: ['snippets'],
    queryFn: api.listSnippets,
  })

  const cavesEnabled = data?.cluster.meta.caves_enabled ?? false
  const canOperate = data?.effective_role === 'admin' || data?.effective_role === 'operator'

  const [shard, setShard] = useState<'master' | 'caves'>('master')
  const [splitView, setSplitView] = useState(false)
  const [autoScroll, setAutoScroll] = useState(true)
  const [filter, setFilter] = useState('')
  const [levelFilter, setLevelFilter] = useState<LevelFilter>('all')
  const [cmdInput, setCmdInput] = useState('')
  const [history, setHistory] = useState<string[]>([])
  const [histIdx, setHistIdx] = useState(-1)
  const [showDesktopGM, setShowDesktopGM] = useState(true)
  const [mobileGMOpen, setMobileGMOpen] = useState(false)
  const [seasonOpen, setSeasonOpen] = useState(false)
  const [dangerOpen, setDangerOpen] = useState(false)
  const [pendingConfirm, setPendingConfirm] = useState<PendingGMConfirm | null>(null)

  // Snippet dialog
  const [snippetOpen, setSnippetOpen] = useState(false)
  const [snipName, setSnipName] = useState('')
  const [snipCmd, setSnipCmd] = useState('')

  // Keep Forest & Caves streams connected smoothly so switching shards never wipes terminal history
  const masterStream = useShardStream(id, 'master', Boolean(id))
  const cavesStream = useShardStream(id, 'caves', Boolean(id && cavesEnabled))

  // Disable split view automatically if switching to a forest-only cluster
  useEffect(() => {
    if (!cavesEnabled) {
      setSplitView(false)
      setShard('master')
    }
  }, [cavesEnabled])

  const activeStream = shard === 'caves' && cavesEnabled ? cavesStream : masterStream

  // Ctrl+L clears active terminal viewport
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'l') {
        e.preventDefault()
        if (splitView) {
          masterStream.clear()
          cavesStream.clear()
        } else {
          activeStream.clear()
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [activeStream, cavesStream, masterStream, splitView])

  const createSnippetMut = useMutation({
    mutationFn: () => api.createSnippet({ name: snipName.trim(), shard, command: snipCmd.trim() }),
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setSnipName('')
      setSnipCmd('')
      setSnippetOpen(false)
      void qc.invalidateQueries({ queryKey: ['snippets'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteSnippetMut = useMutation({
    mutationFn: (sid: number) => api.deleteSnippet(sid),
    onSuccess: () => {
      setPendingConfirm(null)
      void qc.invalidateQueries({ queryKey: ['snippets'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const filterLogEntries = useCallback(
    (raw: LogEntry[]) => {
      const q = filter.trim().toLowerCase()
      return raw.filter((e) => {
        if (levelFilter !== 'all' && e.level !== levelFilter) return false
        if (q && !e.message.toLowerCase().includes(q)) return false
        return true
      })
    },
    [filter, levelFilter],
  )

  const filteredMaster = useMemo(() => filterLogEntries(masterStream.entries), [filterLogEntries, masterStream.entries])
  const filteredCaves = useMemo(() => filterLogEntries(cavesStream.entries), [filterLogEntries, cavesStream.entries])

  const dispatchCommand = (cmd: string, feedbackLabel?: string) => {
    const clean = cmd.trim()
    if (!clean || !canOperate) return
    activeStream.send(clean, feedbackLabel)
    setMobileGMOpen(false)
  }

  const handleSend = (e: React.FormEvent) => {
    e.preventDefault()
    const cmd = cmdInput.trim()
    if (!cmd || !canOperate) return
    dispatchCommand(cmd)
    setHistory((prev) => [cmd, ...prev.slice(0, 49)])
    setHistIdx(-1)
    setCmdInput('')
  }

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowUp' && history.length > 0) {
      e.preventDefault()
      const nextIdx = Math.min(histIdx + 1, history.length - 1)
      setHistIdx(nextIdx)
      setCmdInput(history[nextIdx])
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      if (histIdx <= 0) {
        setHistIdx(-1)
        setCmdInput('')
      } else {
        const nextIdx = histIdx - 1
        setHistIdx(nextIdx)
        setCmdInput(history[nextIdx])
      }
    }
  }

  const copyEntries = async (rows: LogEntry[]) => {
    if (rows.length === 0) return
    const text = rows.map((r) => r.message).join('\n')
    const ok = await copyTextWithFallback(text)
    if (ok) {
      toast.success(t('console.copiedLogs', { count: rows.length }))
    } else {
      toast.error(t('common.copyFailed'))
    }
  }

  const snippets = snippetsData?.snippets ?? []

  const gmContent = (
    <div className="space-y-5">
      {/* Safe & Reversible World Ops */}
      <div className="space-y-2">
        <div className="eyebrow">{t('gm.worldOps')}</div>
        <div className="grid grid-cols-2 gap-1.5">
          <Button
            size="sm"
            variant="secondary"
            className="justify-start"
            onClick={() => dispatchCommand('c_save()', t('gm.saveNow'))}
          >
            <Save className="h-3.5 w-3.5 text-emerald-700 dark:text-emerald-400 shrink-0" />
            <span className="truncate">{t('gm.saveNow')}</span>
          </Button>
          <Button
            size="sm"
            variant="outline"
            className="justify-start"
            onClick={() => setPendingConfirm({ kind: 'skipDay' })}
          >
            <Sun className="h-3.5 w-3.5 text-sky-700 dark:text-sky-400 shrink-0" />
            <span className="truncate">{t('gm.skipDay')}</span>
          </Button>
          <Button
            size="sm"
            variant="warning"
            className="justify-start"
            onClick={() => setPendingConfirm({ kind: 'rollback', days: 1 })}
          >
            <History className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">{t('gm.rollback1')}</span>
          </Button>
          <Button
            size="sm"
            variant="warning"
            className="justify-start"
            onClick={() => setPendingConfirm({ kind: 'rollback', days: 3 })}
          >
            <History className="h-3.5 w-3.5 shrink-0" />
            <span className="truncate">{t('gm.rollback3')}</span>
          </Button>
        </div>
        <div className="space-y-1 pt-1">
          <Button
            size="sm"
            variant="secondary"
            className="w-full justify-start"
            title={t('gm.cleanGroundHint')}
            onClick={() => dispatchCommand(CLEAN_GROUND_LUA, t('gm.cleanGround'))}
          >
            <Sparkles className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
            <span>{t('gm.cleanGround')}</span>
          </Button>
          <p className="text-[11px] text-muted-foreground leading-relaxed">{t('gm.cleanGroundHint')}</p>
        </div>
      </div>

      {/* Players & Entities */}
      <div className="space-y-2">
        <div className="eyebrow">{t('gm.playerOps')}</div>
        <div className="grid grid-cols-1 gap-1.5">
          <Button
            size="sm"
            variant="secondary"
            className="justify-start"
            onClick={() => dispatchCommand(REVIVE_ALL_LUA, t('gm.reviveAll'))}
          >
            <HeartPulse className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
            {t('gm.reviveAll')}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            className="justify-start"
            onClick={() => dispatchCommand('c_listallplayers()', t('gm.listPlayers'))}
          >
            <Users className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
            {t('gm.listPlayers')}
          </Button>
          <Button
            size="sm"
            variant="secondary"
            className="justify-start"
            onClick={() => dispatchCommand(COUNT_ENTITIES_LUA, t('gm.countEntities'))}
          >
            <Sparkles className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
            {t('gm.countEntities')}
          </Button>
        </div>
      </div>

      {/* Season & Weather (Collapsible) */}
      <div className="pt-2.5 border-t border-border/80">
        <button
          type="button"
          aria-expanded={seasonOpen}
          onClick={() => setSeasonOpen((v) => !v)}
          className="flex w-full items-center justify-between eyebrow hover:text-foreground transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded cursor-pointer py-1"
        >
          <span>{t('gm.seasonOps')}</span>
          <ChevronDown className={cn('h-4 w-4 transition-transform duration-200 ease-out', seasonOpen && 'rotate-180')} />
        </button>
        <CollapsibleContent open={seasonOpen}>
          <div className="grid grid-cols-2 gap-1.5 pt-2">
            <Button
              size="sm"
              variant="secondary"
              className="justify-start"
              onClick={() => dispatchCommand('TheWorld:PushEvent("ms_setseason", "autumn")', t('gm.setAutumn'))}
            >
              <Flame className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              {t('gm.setAutumn')}
            </Button>
            <Button
              size="sm"
              variant="secondary"
              className="justify-start"
              onClick={() => dispatchCommand('TheWorld:PushEvent("ms_setseason", "winter")', t('gm.setWinter'))}
            >
              <Snowflake className="h-3.5 w-3.5 text-sky-700 dark:text-sky-400 shrink-0" />
              {t('gm.setWinter')}
            </Button>
            <Button
              size="sm"
              variant="secondary"
              className="justify-start"
              onClick={() => dispatchCommand('TheWorld:PushEvent("ms_setseason", "spring")', t('gm.setSpring'))}
            >
              <Flower2 className="h-3.5 w-3.5 text-emerald-700 dark:text-emerald-400 shrink-0" />
              {t('gm.setSpring')}
            </Button>
            <Button
              size="sm"
              variant="secondary"
              className="justify-start"
              onClick={() => dispatchCommand('TheWorld:PushEvent("ms_setseason", "summer")', t('gm.setSummer'))}
            >
              <Sun className="h-3.5 w-3.5 text-orange-700 dark:text-orange-400 shrink-0" />
              {t('gm.setSummer')}
            </Button>
            <Button
              size="sm"
              variant="outline"
              className="justify-start"
              onClick={() => dispatchCommand('TheWorld:PushEvent("ms_forceprecipitation", true)', t('gm.startRain'))}
            >
              <CloudRain className="h-3.5 w-3.5 text-sky-700 dark:text-sky-400 shrink-0" />
              {t('gm.startRain')}
            </Button>
            <Button
              size="sm"
              variant="outline"
              className="justify-start"
              onClick={() => dispatchCommand('TheWorld:PushEvent("ms_forceprecipitation", false)', t('gm.stopRain'))}
            >
              <CloudRain className="h-3.5 w-3.5 text-muted-foreground shrink-0" />
              {t('gm.stopRain')}
            </Button>
          </div>
        </CollapsibleContent>
      </div>

      {/* Custom Snippets */}
      <div className="space-y-2 pt-2.5 border-t border-border/80">
        <div className="flex items-center justify-between">
          <span className="eyebrow flex items-center gap-1">
            <Bookmark className="h-3.5 w-3.5" />
            {t('gm.customSnippets')}
          </span>
          <button
            type="button"
            onClick={() => setSnippetOpen(true)}
            className="text-xs font-medium text-primary hover:underline inline-flex items-center gap-0.5 cursor-pointer py-1 px-1.5 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
          >
            <Plus className="h-3.5 w-3.5" />
            {t('common.create')}
          </button>
        </div>
        {snippets.length === 0 ? (
          <p className="text-xs text-muted-foreground leading-relaxed">{t('gm.emptySnippets')}</p>
        ) : (
          <div className="space-y-1.5">
            {snippets.map((s) => (
              <div
                key={s.id}
                className="flex items-center justify-between gap-1.5 rounded-xl border border-border bg-[var(--surface-elevated)]/70 px-2.5 py-1.5 text-xs"
              >
                <button
                  type="button"
                  onClick={() => dispatchCommand(s.command, s.name)}
                  className="flex flex-1 items-center gap-1.5 min-w-0 text-left hover:text-primary cursor-pointer py-1 sm:py-0.5 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  title={s.command}
                >
                  <Play className="h-3.5 w-3.5 shrink-0 text-primary" />
                  <span className="truncate font-medium">{s.name}</span>
                </button>
                <button
                  type="button"
                  aria-label={`${t('common.delete')} ${s.name}`}
                  title={t('common.delete')}
                  onClick={() => setPendingConfirm({ kind: 'deleteSnippet', snippet: s })}
                  className="flex h-9 w-9 sm:h-7 sm:w-7 items-center justify-center rounded-lg text-muted-foreground hover:bg-red-600/12 hover:text-red-700 dark:hover:text-red-300 shrink-0 cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring pressable"
                >
                  <X className="h-3.5 w-3.5" />
                </button>
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Isolated Collapsible High-Risk World Reset */}
      <div className="pt-2.5 border-t border-border/80">
        <button
          type="button"
          aria-expanded={dangerOpen}
          onClick={() => setDangerOpen((v) => !v)}
          className="flex w-full items-center justify-between text-xs font-medium text-muted-foreground hover:text-red-700 dark:hover:text-red-300 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded cursor-pointer py-1"
        >
          <span className="flex items-center gap-1.5">
            <AlertTriangle className="h-3.5 w-3.5 text-red-700 dark:text-red-400" />
            {t('gm.dangerOps')}
          </span>
          <ChevronDown className={cn('h-4 w-4 transition-transform duration-200 ease-out', dangerOpen && 'rotate-180')} />
        </button>
        <CollapsibleContent open={dangerOpen}>
          <div className="pt-2">
            <Button
              size="sm"
              variant="dangerOutline"
              className="w-full justify-start"
              onClick={() => setPendingConfirm({ kind: 'regen' })}
            >
              <RefreshCw className="h-3.5 w-3.5 shrink-0" />
              <span>{t('gm.regenWorld')}</span>
            </Button>
          </div>
        </CollapsibleContent>
      </div>
    </div>
  )

  return (
    <div className="flex flex-col gap-3.5 lg:h-[calc(100vh-9.5rem)] lg:min-h-[540px]">
      {/* Row 1: Stable Header Bar */}
      <div className="flex flex-wrap items-center justify-between gap-2.5">
        <div className="flex items-center gap-2.5 flex-wrap">
          <h1 className="text-xl sm:text-2xl font-semibold tracking-[-0.025em]">{t('console.title')}</h1>
          <TabList
            ariaLabel={t('console.title')}
            variant="segmented"
            controlsId="console-log-viewport"
            value={shard}
            onChange={(next) => {
              setShard(next)
            }}
            items={[
              {
                key: 'master',
                label: t('common.master'),
                icon: <Trees className="h-3.5 w-3.5 text-emerald-600 dark:text-emerald-400" />,
              },
              {
                key: 'caves',
                label: t('common.caves'),
                icon: <Mountain className="h-3.5 w-3.5 text-muted-foreground" />,
                hidden: !cavesEnabled,
              },
            ]}
          />
          {!splitView && (
            <div className="flex items-center gap-1.5" aria-live="polite">
              {activeStream.shardStatus && <ShardStateBadge state={activeStream.shardStatus.state} />}
              <Badge variant={activeStream.connected ? 'success' : 'warning'}>
                {activeStream.connected ? <Wifi className="h-3.5 w-3.5" /> : <WifiOff className="h-3.5 w-3.5" />}
                {activeStream.connected ? t('console.connected') : t('console.disconnected')}
              </Badge>
            </div>
          )}
        </div>

        <div className="flex items-center gap-2">
          {cavesEnabled && (
            <Button
              size="sm"
              variant={splitView ? 'secondary' : 'outline'}
              aria-pressed={splitView}
              className="hidden xl:inline-flex"
              onClick={() => setSplitView((v) => !v)}
            >
              <Columns2 className="h-3.5 w-3.5 text-primary" />
              {splitView ? t('console.singleView') : t('console.splitView')}
            </Button>
          )}
          {canOperate && !showDesktopGM && (
            <Button
              size="sm"
              variant="outline"
              className="hidden lg:inline-flex"
              onClick={() => setShowDesktopGM(true)}
            >
              <Wand2 className="h-3.5 w-3.5 text-primary" />
              {t('console.openGM')}
            </Button>
          )}
        </div>
      </div>

      {/* Main Split: Terminal Column + Collapsible Desktop GM Sidebar */}
      <div className="flex-1 flex flex-col lg:flex-row gap-4 lg:min-h-0">
        {/* Terminal Column */}
        <div className="flex-1 flex flex-col min-w-0 lg:min-h-0 gap-2.5">
          {/* Filter & Controls Toolbar */}
          <div className="flex flex-col sm:flex-row sm:items-center sm:justify-between gap-2">
            <div className="flex items-center gap-2 w-full sm:w-auto sm:flex-1 min-w-0">
              <SearchInput
                value={filter}
                onChange={setFilter}
                placeholder={t('console.filterPlaceholder')}
                className="flex-1 sm:flex-initial sm:w-56 min-w-0"
                inputClassName="h-9 sm:h-8"
              />
              {/* Mobile compact level Select */}
              <Select
                aria-label={t('console.levelFilterLabel')}
                value={levelFilter}
                onChange={(e) => setLevelFilter(e.target.value as LevelFilter)}
                className="w-24 h-9 text-xs sm:hidden shrink-0"
              >
                <option value="all">{t('console.levelAll')}</option>
                <option value="error">{t('console.levelError')}</option>
                <option value="warn">{t('console.levelWarn')}</option>
                <option value="chat">{t('console.levelChat')}</option>
              </Select>
              {/* Desktop/Tablet sliding segmented level filter */}
              <div className="hidden sm:block shrink-0">
                <TabList
                  ariaLabel={t('console.levelFilterLabel')}
                  variant="segmented"
                  value={levelFilter}
                  onChange={setLevelFilter}
                  items={[
                    { key: 'all', label: t('console.levelAll') },
                    { key: 'error', label: t('console.levelError') },
                    { key: 'warn', label: t('console.levelWarn') },
                    { key: 'chat', label: t('console.levelChat') },
                  ]}
                />
              </div>
            </div>

            <div className="flex items-center justify-between sm:justify-end gap-2 shrink-0">
              <label
                htmlFor="console-autoscroll"
                className="inline-flex items-center gap-1.5 text-xs text-muted-foreground cursor-pointer"
              >
                <Switch
                  id="console-autoscroll"
                  checked={autoScroll}
                  onCheckedChange={setAutoScroll}
                  ariaLabel={t('console.autoScroll')}
                />
                <span>{t('console.autoScroll')}</span>
              </label>
              <Button
                size="sm"
                variant="outline"
                className="h-9 sm:h-8"
                onClick={() => void copyEntries(shard === 'caves' && cavesEnabled ? filteredCaves : filteredMaster)}
              >
                <Copy className="h-3.5 w-3.5" />
                <span className="hidden sm:inline">{t('console.copyLogs')}</span>
              </Button>
              <Button
                size="sm"
                variant="outline"
                className="h-9 sm:h-8"
                onClick={() => {
                  if (splitView) {
                    masterStream.clear()
                    cavesStream.clear()
                  } else {
                    activeStream.clear()
                  }
                }}
                title="Ctrl/Cmd + L"
              >
                <Trash2 className="h-3.5 w-3.5" />
                {t('console.clear')}
              </Button>
            </div>
          </div>

          {/* Terminal Viewport(s) */}
          {splitView && cavesEnabled ? (
            <div id="console-log-viewport" className="grid grid-cols-1 xl:grid-cols-2 gap-3 lg:flex-1 lg:min-h-0">
              <TerminalPane
                title={t('common.master')}
                icon={<Trees className="h-3.5 w-3.5 text-emerald-400" />}
                active={shard === 'master'}
                onFocusPane={() => setShard('master')}
                connected={masterStream.connected}
                state={masterStream.shardStatus?.state}
                entries={filteredMaster}
                autoScroll={autoScroll}
                emptyText={t('console.emptyLogs')}
                onCopy={() => void copyEntries(filteredMaster)}
                onClear={masterStream.clear}
              />
              <TerminalPane
                title={t('common.caves')}
                icon={<Mountain className="h-3.5 w-3.5 text-slate-400" />}
                active={shard === 'caves'}
                onFocusPane={() => setShard('caves')}
                connected={cavesStream.connected}
                state={cavesStream.shardStatus?.state}
                entries={filteredCaves}
                autoScroll={autoScroll}
                emptyText={t('console.emptyLogs')}
                onCopy={() => void copyEntries(filteredCaves)}
                onClear={cavesStream.clear}
              />
            </div>
          ) : (
            <TerminalPane
              id="console-log-viewport"
              title={shard === 'caves' && cavesEnabled ? t('common.caves') : t('common.master')}
              icon={
                shard === 'caves' && cavesEnabled ? (
                  <Mountain className="h-3.5 w-3.5 text-slate-400" />
                ) : (
                  <Trees className="h-3.5 w-3.5 text-slate-400" />
                )
              }
              active
              connected={activeStream.connected}
              state={activeStream.shardStatus?.state}
              entries={shard === 'caves' && cavesEnabled ? filteredCaves : filteredMaster}
              autoScroll={autoScroll}
              emptyText={t('console.emptyLogs')}
              onCopy={() => void copyEntries(shard === 'caves' && cavesEnabled ? filteredCaves : filteredMaster)}
              onClear={activeStream.clear}
            />
          )}

          {/* Command Prompt Input */}
          {canOperate ? (
            <form onSubmit={handleSend} className="flex items-center gap-2 shrink-0 min-w-0">
              <Button
                type="button"
                variant="secondary"
                aria-label={t('console.openGM')}
                title={t('console.openGM')}
                className="lg:hidden shrink-0 h-11 px-3"
                onClick={() => setMobileGMOpen(true)}
              >
                <Wand2 className="h-4 w-4 text-primary" />
                <span>{t('console.gmShort')}</span>
              </Button>
              <div className="relative flex-1 min-w-0">
                <TerminalIcon
                  className="pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 h-3.5 w-3.5 text-primary"
                  aria-hidden="true"
                />
                <Input
                  value={cmdInput}
                  onChange={(e) => setCmdInput(e.target.value)}
                  onKeyDown={handleKeyDown}
                  placeholder={t('console.inputPlaceholder')}
                  aria-label={t('console.inputPlaceholder')}
                  className="min-w-0 w-full h-11 sm:h-9 pl-9 pr-20 font-mono text-xs"
                />
                <span className="pointer-events-none hidden sm:inline-flex items-center rounded-md border border-border bg-secondary/75 px-1.5 py-0.5 font-mono text-[10px] text-muted-foreground absolute right-2 top-1/2 -translate-y-1/2">
                  {shard === 'caves' && cavesEnabled ? t('common.cavesShort') : t('common.masterShort')}
                </span>
              </div>
              <Button type="submit" className="shrink-0 h-11 sm:h-9" disabled={!cmdInput.trim()}>
                <Send className="h-4 w-4" />
                {t('console.send')}
              </Button>
            </form>
          ) : (
            <div className="rounded-xl border border-border bg-[var(--surface-elevated)]/65 px-4 py-2.5 text-xs text-muted-foreground">
              {t('console.readOnlyHint')}
            </div>
          )}
        </div>

        {/* Desktop GM Toolbox Sidebar */}
        {canOperate && showDesktopGM && (
          <aside className="hidden lg:flex lg:flex-col w-76 shrink-0 rounded-[10px] surface-card overflow-hidden">
            <div className="flex items-center justify-between gap-2 border-b border-border px-4 py-3 shrink-0">
              <h2 className="text-xs font-semibold tracking-tight flex items-center gap-1.5 text-foreground">
                <Wand2 className="h-3.5 w-3.5 text-primary" />
                {t('gm.title')}
              </h2>
              <button
                type="button"
                onClick={() => setShowDesktopGM(false)}
                aria-label={t('common.close')}
                title={t('common.close')}
                className="flex h-7 w-7 items-center justify-center rounded-lg text-muted-foreground hover:bg-secondary hover:text-foreground cursor-pointer pressable"
              >
                <X className="h-3.5 w-3.5" />
              </button>
            </div>
            <div className="flex-1 overflow-y-auto p-4">{gmContent}</div>
          </aside>
        )}
      </div>

      {/* Mobile GM Toolbox Modal */}
      <Dialog
        open={mobileGMOpen}
        onClose={() => setMobileGMOpen(false)}
        title={t('gm.title')}
        maxWidthClass="max-w-lg"
        variant="sheet"
      >
        {gmContent}
      </Dialog>

      {/* Save Custom Snippet Dialog */}
      <Dialog open={snippetOpen} onClose={() => setSnippetOpen(false)} title={t('gm.addSnippet')}>
        <form
          onSubmit={(e) => {
            e.preventDefault()
            createSnippetMut.mutate()
          }}
          className="space-y-4"
        >
          {createSnippetMut.error && (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-xl border border-red-600/35 bg-red-600/10 px-3.5 py-2.5 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
            >
              <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
              <span className="break-words">{formatError(createSnippetMut.error)}</span>
            </div>
          )}
          <div className="space-y-1.5">
            <Label htmlFor="gm-snip-name">{t('gm.snippetName')}</Label>
            <Input
              id="gm-snip-name"
              required
              value={snipName}
              onChange={(e) => setSnipName(e.target.value)}
              placeholder={t('gm.snippetNamePlaceholder')}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="gm-snip-cmd">{t('gm.snippetCmd')}</Label>
            <Input
              id="gm-snip-cmd"
              required
              value={snipCmd}
              onChange={(e) => setSnipCmd(e.target.value)}
              placeholder='c_give("goldnugget", 20)'
              className="font-mono text-xs"
            />
          </div>
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => setSnippetOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" disabled={createSnippetMut.isPending}>
              {t('common.save')}
            </Button>
          </div>
        </form>
      </Dialog>

      {/* Destructive / High-Risk Confirmations */}
      <ConfirmDialog
        open={pendingConfirm?.kind === 'skipDay'}
        onClose={() => setPendingConfirm(null)}
        onConfirm={() => {
          dispatchCommand('TheWorld:PushEvent("ms_nextcycle")', t('gm.skipDay'))
          setPendingConfirm(null)
        }}
        title={t('gm.skipDayConfirmTitle')}
        description={t('gm.skipDayConfirmDesc')}
        confirmLabel={t('gm.skipDay')}
        tone="warning"
      />

      <ConfirmDialog
        open={pendingConfirm?.kind === 'rollback'}
        onClose={() => setPendingConfirm(null)}
        onConfirm={() => {
          if (pendingConfirm?.kind === 'rollback') {
            dispatchCommand(
              `c_rollback(${pendingConfirm.days})`,
              pendingConfirm.days === 3 ? t('gm.rollback3') : t('gm.rollback1'),
            )
            setPendingConfirm(null)
          }
        }}
        title={t('overview.rollbackConfirmTitle')}
        description={t('overview.rollbackConfirmDesc', {
          days: pendingConfirm?.kind === 'rollback' ? pendingConfirm.days : 1,
        })}
        confirmLabel={
          pendingConfirm?.kind === 'rollback' && pendingConfirm.days === 3 ? t('gm.rollback3') : t('gm.rollback1')
        }
        tone="warning"
      />

      <ConfirmDialog
        open={pendingConfirm?.kind === 'regen'}
        onClose={() => setPendingConfirm(null)}
        onConfirm={() => {
          dispatchCommand('c_regenerateworld()', t('gm.regenWorld'))
          setPendingConfirm(null)
        }}
        title={t('gm.regenConfirmTitle')}
        description={t('gm.regenConfirm')}
        confirmLabel={t('gm.regenWorld')}
        tone="danger"
        confirmTextMatch={id}
      />

      <ConfirmDialog
        open={pendingConfirm?.kind === 'deleteSnippet'}
        onClose={() => setPendingConfirm(null)}
        onConfirm={() => {
          if (pendingConfirm?.kind === 'deleteSnippet') {
            deleteSnippetMut.mutate(pendingConfirm.snippet.id)
          }
        }}
        title={t('console.deleteSnippetTitle')}
        description={t('console.deleteSnippetDesc', {
          name: pendingConfirm?.kind === 'deleteSnippet' ? pendingConfirm.snippet.name : '',
        })}
        confirmLabel={t('common.delete')}
        tone="danger"
        loading={deleteSnippetMut.isPending}
      />
    </div>
  )
}

function TerminalPane({
  id,
  title,
  icon,
  active,
  onFocusPane,
  connected,
  state,
  entries,
  autoScroll,
  emptyText,
  onCopy,
  onClear,
}: {
  id?: string
  title: string
  icon: React.ReactNode
  active: boolean
  onFocusPane?: () => void
  connected: boolean
  state?: ShardStatus['state']
  entries: LogEntry[]
  autoScroll: boolean
  emptyText: string
  onCopy: () => void
  onClear: () => void
}) {
  const { t } = useTranslation()
  const scrollRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    if (autoScroll && scrollRef.current) {
      scrollRef.current.scrollTop = scrollRef.current.scrollHeight
    }
  }, [entries, autoScroll])

  return (
    <div
      id={id}
      onClick={onFocusPane}
      className={cn(
        'flex flex-col overflow-hidden rounded-[10px] border bg-[var(--surface-terminal)] h-[50vh] min-h-[280px] lg:h-auto lg:flex-1 lg:min-h-0 transition-colors',
        active && onFocusPane
          ? 'border-[var(--border-strong)] ring-1 ring-ring/30'
          : 'border-[var(--surface-terminal-border)]',
      )}
    >
      {/* Terminal Status Bar */}
      <div className="flex items-center justify-between gap-2 border-b border-[var(--surface-terminal-border)] bg-white/[0.025] px-3.5 py-2 text-[11px] text-slate-300 shrink-0">
        <div className="flex items-center gap-2 min-w-0">
          {icon}
          <span className="font-semibold text-slate-100 truncate">{title}</span>
          <span className="inline-flex items-center gap-1 font-mono text-slate-400 tabular-nums">
            <span
              className={cn('h-1.5 w-1.5 rounded-full', connected ? 'bg-emerald-400' : 'bg-amber-400')}
              aria-hidden="true"
            />
            {state ? t(`common.${state}`) : connected ? t('console.connected') : t('console.disconnected')}
          </span>
        </div>
        <div className="flex items-center gap-2 shrink-0">
          <span className="font-mono text-slate-500 tabular-nums">{t('console.linesCount', { count: entries.length })}</span>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              onCopy()
            }}
            aria-label={t('console.copyLogs')}
            title={t('console.copyLogs')}
            className="inline-flex h-6 w-6 items-center justify-center rounded text-slate-400 hover:bg-white/10 hover:text-slate-100 cursor-pointer"
          >
            <Copy className="h-3 w-3" />
          </button>
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation()
              onClear()
            }}
            aria-label={t('console.clear')}
            title={t('console.clear')}
            className="inline-flex h-6 w-6 items-center justify-center rounded text-slate-400 hover:bg-white/10 hover:text-slate-100 cursor-pointer"
          >
            <Trash2 className="h-3 w-3" />
          </button>
        </div>
      </div>

      {/* Log Stream */}
      <div
        role="log"
        aria-label={title}
        aria-live="off"
        ref={scrollRef}
        className="flex-1 overflow-y-auto p-3.5 sm:p-4 font-mono text-xs leading-relaxed"
      >
        {entries.length === 0 ? (
          <div className="h-full flex items-center justify-center text-slate-400 text-center px-4">{emptyText}</div>
        ) : (
          <div className="space-y-0.5">
            {entries.map((entry, idx) => (
              <div
                key={`${entry.seq}-${idx}`}
                className={cn(
                  'break-words [overflow-wrap:anywhere] whitespace-pre-wrap rounded px-1.5 py-0.5',
                  entry.level === 'error' && 'bg-red-500/8 text-red-400 border-l-2 border-red-500/70',
                  entry.level === 'warn' && 'bg-amber-500/8 text-amber-300 border-l-2 border-amber-500/70',
                  entry.level === 'chat' && 'text-sky-300',
                  entry.level === 'system' && 'text-emerald-400',
                  entry.level === 'info' && 'text-slate-200',
                )}
              >
                {entry.message}
              </div>
            ))}
          </div>
        )}
      </div>
    </div>
  )
}
