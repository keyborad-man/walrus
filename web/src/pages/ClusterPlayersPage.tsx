import React, { useEffect, useMemo, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  Ban,
  Check,
  HeartPulse,
  Megaphone,
  MessageSquare,
  Plus,
  RefreshCw,
  Save,
  Shield,
  UserCheck,
  UserMinus,
  Users,
  X,
} from 'lucide-react'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { formatCharacterName, KU_ID_RE } from '@/lib/dst'
import { useRegisterUnsaved } from '@/hooks/useUnsavedGuard'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { TabList } from '@/components/ui/tabs'
import { CopyButton } from '@/components/ui/copy-button'
import { SearchInput } from '@/components/SearchInput'
import { StickySaveBar } from '@/components/StickySaveBar'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { PageHeader } from '@/components/PageHeader'
import type { OnlinePlayer } from '@/types/api'

type ListKind = 'adminlist' | 'whitelist' | 'blocklist'

export function ClusterPlayersPage() {
  const { id = '' } = useParams<{ id: string }>()
  const { t, i18n } = useTranslation()
  const isEn = i18n.language === 'en-US'
  const qc = useQueryClient()

  const { data: clusterData } = useQuery({
    queryKey: ['cluster', id],
    queryFn: () => api.getCluster(id),
    enabled: !!id,
  })
  const canOperate = clusterData?.effective_role === 'admin' || clusterData?.effective_role === 'operator'

  const {
    data: playersData,
    isError: playersError,
    error: playersErrObj,
    refetch: refetchPlayers,
  } = useQuery({
    queryKey: ['players', id],
    queryFn: () => api.getPlayers(id),
    refetchInterval: 5000,
    enabled: !!id,
  })

  const { data: chatData } = useQuery({
    queryKey: ['chat', id],
    queryFn: () => api.getChat(id, 150),
    refetchInterval: 4000,
    enabled: !!id,
  })

  const [activeList, setActiveList] = useState<ListKind>('adminlist')
  const [admins, setAdmins] = useState<string[]>([])
  const [whites, setWhites] = useState<string[]>([])
  const [blocks, setBlocks] = useState<string[]>([])
  const [initialListsJson, setInitialListsJson] = useState('')
  const [newKu, setNewKu] = useState('')
  const [chatFilter, setChatFilter] = useState('')
  const [announce, setAnnounce] = useState('')
  const [promotingKuId, setPromotingKuId] = useState<string | null>(null)
  const [pendingAction, setPendingAction] = useState<{ action: 'kick' | 'ban'; player: OnlinePlayer } | null>(null)

  const listsDirty = useMemo(() => {
    if (!initialListsJson) return false
    return JSON.stringify({ admins, whites, blocks }) !== initialListsJson
  }, [admins, whites, blocks, initialListsJson])

  useRegisterUnsaved('player-lists', listsDirty)

  useEffect(() => {
    if (!playersData?.lists || listsDirty) return
    const nextAdmins = playersData.lists.adminlist ?? []
    const nextWhites = playersData.lists.whitelist ?? []
    const nextBlocks = playersData.lists.blocklist ?? []
    setAdmins(nextAdmins)
    setWhites(nextWhites)
    setBlocks(nextBlocks)
    setInitialListsJson(JSON.stringify({ admins: nextAdmins, whites: nextWhites, blocks: nextBlocks }))
  }, [playersData?.lists, listsDirty])

  const savedAdmins = playersData?.lists?.adminlist ?? []
  const savedWhites = playersData?.lists?.whitelist ?? []
  const savedBlocks = playersData?.lists?.blocklist ?? []

  const historyMap = useMemo(() => {
    const m: Record<string, string> = {}
    for (const h of playersData?.lists?.history ?? []) {
      if (h.ku_id && h.last_name) m[h.ku_id] = h.last_name
    }
    for (const p of playersData?.world_info?.online_players ?? []) {
      if (p.ku_id && p.name) m[p.ku_id] = p.name
    }
    return m
  }, [playersData])

  const probeMut = useMutation({
    mutationFn: () => api.triggerProbe(id),
    onSuccess: () => {
      void qc.invalidateQueries({ queryKey: ['players', id] })
      toast.success(t('common.actionSuccess'))
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const actionMut = useMutation({
    mutationFn: ({ action, ku_id }: { action: 'kick' | 'ban' | 'revive'; ku_id: string; name?: string }) =>
      api.playerAction(id, action, ku_id),
    onSuccess: (_, vars) => {
      const displayName = vars.name || historyMap[vars.ku_id] || vars.ku_id
      const msgKey =
        vars.action === 'revive'
          ? 'players.toastRevived'
          : vars.action === 'ban'
            ? 'players.toastBanned'
            : 'players.toastKicked'
      toast.success(t(msgKey, { name: displayName }))
      setPendingAction(null)
      void qc.invalidateQueries({ queryKey: ['players', id] })
      void qc.invalidateQueries({ queryKey: ['chat', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const saveListsMut = useMutation({
    mutationFn: () => api.savePlayerLists(id, { adminlist: admins, whitelist: whites, blocklist: blocks }),
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setInitialListsJson(JSON.stringify({ admins, whites, blocks }))
      void qc.invalidateQueries({ queryKey: ['players', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const promoteAdminMut = useMutation({
    mutationFn: (vars: { kuId: string; name?: string }) => {
      const nextSavedAdmins = savedAdmins.includes(vars.kuId) ? savedAdmins : [...savedAdmins, vars.kuId]
      return api.savePlayerLists(id, {
        adminlist: nextSavedAdmins,
        whitelist: savedWhites,
        blocklist: savedBlocks,
      })
    },
    onMutate: (vars) => setPromotingKuId(vars.kuId),
    onSuccess: (_, vars) => {
      toast.success(t('players.toastPromoted', { name: vars.name || vars.kuId }))
      setAdmins((prev) => (prev.includes(vars.kuId) ? prev : [...prev, vars.kuId]))
      if (!listsDirty) {
        const nextSavedAdmins = savedAdmins.includes(vars.kuId) ? savedAdmins : [...savedAdmins, vars.kuId]
        setInitialListsJson(
          JSON.stringify({ admins: nextSavedAdmins, whites: savedWhites, blocks: savedBlocks }),
        )
      }
      void qc.invalidateQueries({ queryKey: ['players', id] })
    },
    onError: (err: Error) => toast.error(err.message),
    onSettled: () => setPromotingKuId(null),
  })

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        if (!canOperate || !listsDirty || saveListsMut.isPending) return
        e.preventDefault()
        saveListsMut.mutate()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [canOperate, listsDirty, saveListsMut])

  const announceMut = useMutation({
    mutationFn: (msg: string) => {
      const escaped = msg.replaceAll('\\', '\\\\').replaceAll('"', '\\"').replace(/\r?\n/g, ' ')
      return api.sendCommand(id, 'master', `c_announce("${escaped}")`)
    },
    onSuccess: () => {
      toast.success(t('players.toastAnnounced'))
      setAnnounce('')
      void qc.invalidateQueries({ queryKey: ['chat', id] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const currentList = activeList === 'adminlist' ? admins : activeList === 'whitelist' ? whites : blocks
  const setCurrentList = (next: string[]) => {
    if (activeList === 'adminlist') setAdmins(next)
    else if (activeList === 'whitelist') setWhites(next)
    else setBlocks(next)
  }

  const addKuToCurrent = (ku: string) => {
    const clean = ku.trim()
    if (!clean || currentList.includes(clean)) return
    setCurrentList([...currentList, clean])
  }

  const onlinePlayers = playersData?.world_info?.online_players ?? []
  const chatMessages = useMemo(() => {
    const raw = chatData?.messages ?? []
    const q = chatFilter.trim().toLowerCase()
    if (!q) return raw
    return raw.filter(
      (m) =>
        m.message.toLowerCase().includes(q) ||
        (m.sender ?? '').toLowerCase().includes(q) ||
        (m.ku_id ?? '').toLowerCase().includes(q),
    )
  }, [chatData, chatFilter])

  const kuInvalid = newKu.trim().length > 0 && !KU_ID_RE.test(newKu.trim())

  const handleAddKu = (e: React.FormEvent) => {
    e.preventDefault()
    if (kuInvalid) return
    addKuToCurrent(newKu)
    setNewKu('')
  }

  const discardLists = () => {
    if (!playersData?.lists) return
    setAdmins(playersData.lists.adminlist ?? [])
    setWhites(playersData.lists.whitelist ?? [])
    setBlocks(playersData.lists.blocklist ?? [])
  }

  const handleAnnounce = (e: React.FormEvent) => {
    e.preventDefault()
    if (!announce.trim()) return
    announceMut.mutate(announce.trim())
  }

  const renderSafeActions = (p: OnlinePlayer) => {
    const isAlreadyAdmin = savedAdmins.includes(p.ku_id)
    return (
      <>
        <Button
          size="sm"
          variant="outline"
          onClick={() => actionMut.mutate({ action: 'revive', ku_id: p.ku_id, name: p.name })}
        >
          <HeartPulse className="h-3.5 w-3.5 text-muted-foreground" />
          {t('players.revive')}
        </Button>
        <Button
          size="sm"
          variant="outline"
          disabled={isAlreadyAdmin || promotingKuId === p.ku_id}
          onClick={() => promoteAdminMut.mutate({ kuId: p.ku_id, name: p.name })}
        >
          <Shield className="h-3.5 w-3.5 text-muted-foreground" />
          {isAlreadyAdmin ? t('players.alreadyAdmin') : t('players.addAdmin')}
        </Button>
      </>
    )
  }

  const renderModerationActions = (p: OnlinePlayer) => (
    <>
      <Button size="sm" variant="warning" onClick={() => setPendingAction({ action: 'kick', player: p })}>
        <UserMinus className="h-3.5 w-3.5" />
        {t('players.kick')}
      </Button>
      <Button size="sm" variant="dangerOutline" onClick={() => setPendingAction({ action: 'ban', player: p })}>
        <Ban className="h-3.5 w-3.5" />
        {t('players.ban')}
      </Button>
    </>
  )

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={t('nav.groupOps')}
        title={t('players.title')}
        subtitle={t('players.subtitle')}
        actions={
          canOperate ? (
            <Button variant="outline" size="sm" disabled={probeMut.isPending} onClick={() => probeMut.mutate()}>
              <RefreshCw className={`h-3.5 w-3.5 ${probeMut.isPending ? 'animate-spin' : ''}`} />
              {t('players.refreshProbe')}
            </Button>
          ) : undefined
        }
      />

      {playersError && <QueryErrorCard error={playersErrObj} onRetry={() => void refetchPlayers()} compact />}

      {/* Online Players — Cards on Mobile, Table on Desktop */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 tabular-nums">
            <Users className="h-4 w-4 text-primary" />
            {t('players.onlineTitle')} ({onlinePlayers.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {onlinePlayers.length === 0 ? (
            <div className="py-8 text-center text-xs text-muted-foreground">{t('overview.noPlayers')}</div>
          ) : (
            <>
              {/* Mobile stacked rows (< md) */}
              <div className="divide-y divide-border md:hidden">
                {onlinePlayers.map((p) => (
                  <div key={p.ku_id} className="py-3.5 first:pt-0 last:pb-0 space-y-3">
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <div className="font-semibold text-sm truncate">{p.name}</div>
                        <div className="flex items-center gap-1 mt-0.5">
                          <span className="font-mono text-xs text-muted-foreground truncate">{p.ku_id}</span>
                          <CopyButton value={p.ku_id} />
                        </div>
                      </div>
                      <Badge variant="secondary" className="tabular-nums shrink-0">
                        {formatCharacterName(p.prefab, isEn)} · {t('dashboard.day', { day: p.age })}
                      </Badge>
                    </div>
                    {canOperate && (
                      <div className="flex flex-col gap-2">
                        <div className="flex flex-wrap gap-1.5">{renderSafeActions(p)}</div>
                        <div className="flex flex-wrap gap-1.5 pt-1 border-t border-border/60">
                          {renderModerationActions(p)}
                        </div>
                      </div>
                    )}
                  </div>
                ))}
              </div>

              {/* Desktop table (>= md) */}
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-left text-xs text-muted-foreground">
                      <th className="pb-2.5 pr-4 font-medium">{t('players.colPlayer')}</th>
                      <th className="pb-2.5 pr-4 font-medium">KU_ID</th>
                      <th className="pb-2.5 pr-4 font-medium">{t('players.colCharacter')}</th>
                      <th className="pb-2.5 text-right font-medium">{t('common.actions')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {onlinePlayers.map((p) => (
                      <tr key={p.ku_id} className="hover:bg-secondary/30">
                        <td className="py-2.5 pr-4 font-medium">{p.name}</td>
                        <td className="py-2.5 pr-4">
                          <div className="inline-flex items-center gap-1">
                            <span className="font-mono text-xs text-muted-foreground">{p.ku_id}</span>
                            <CopyButton value={p.ku_id} />
                          </div>
                        </td>
                        <td className="py-2.5 pr-4">
                          <Badge variant="secondary" className="tabular-nums">
                            {formatCharacterName(p.prefab, isEn)} · {t('dashboard.day', { day: p.age })}
                          </Badge>
                        </td>
                        <td className="py-2.5 text-right whitespace-nowrap">
                          {canOperate && (
                            <div className="inline-flex items-center gap-1.5">
                              {renderSafeActions(p)}
                              <span className="mx-0.5 h-4 w-px bg-border" aria-hidden="true" />
                              {renderModerationActions(p)}
                            </div>
                          )}
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

      {/* Lists & Chat Grid */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-6">
        {/* Three Lists Manager */}
        <Card className="flex flex-col">
          <CardHeader className="pb-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <CardTitle className="flex items-center gap-2">
                <UserCheck className="h-4 w-4 text-primary" />
                <span>{t('players.listsTitle')}</span>
                {listsDirty && <Badge variant="warning">{t('files.unsaved')}</Badge>}
              </CardTitle>
              {canOperate && (
                <div className="flex items-center gap-1.5">
                  {listsDirty && (
                    <Button size="sm" variant="outline" onClick={discardLists}>
                      {t('common.discard')}
                    </Button>
                  )}
                  <Button size="sm" disabled={saveListsMut.isPending || !listsDirty} onClick={() => saveListsMut.mutate()}>
                    <Save className="h-3.5 w-3.5" />
                    {t('players.saveLists')}
                  </Button>
                </div>
              )}
            </div>
            <TabList
              ariaLabel={t('players.listsTitle')}
              variant="pills"
              idPrefix="player-lists"
              className="pt-2"
              value={activeList}
              onChange={setActiveList}
              items={[
                { key: 'adminlist', label: `${t('players.adminList')} (${admins.length})` },
                { key: 'whitelist', label: `${t('players.whiteList')} (${whites.length})` },
                { key: 'blocklist', label: `${t('players.blockList')} (${blocks.length})` },
              ]}
            />
          </CardHeader>
          <CardContent
            id={`player-lists-panel-${activeList}`}
            role="tabpanel"
            aria-labelledby={`player-lists-tab-${activeList}`}
            className="space-y-4 flex-1"
          >
            {canOperate && (
              <form onSubmit={handleAddKu} className="space-y-1.5">
                <div className="flex gap-2">
                  <Input
                    value={newKu}
                    onChange={(e) => setNewKu(e.target.value)}
                    placeholder={t('players.addKuPlaceholder')}
                    aria-label={t('players.addKuPlaceholder')}
                    aria-invalid={kuInvalid}
                    className={cn('font-mono text-xs', kuInvalid && 'border-red-600 focus-visible:ring-red-600')}
                  />
                  <Button
                    type="submit"
                    size="icon"
                    aria-label={t('players.addKuBtn')}
                    title={t('players.addKuBtn')}
                    disabled={!newKu.trim() || kuInvalid}
                  >
                    <Plus className="h-4 w-4" />
                  </Button>
                </div>
                {kuInvalid && (
                  <p role="alert" className="text-xs font-medium text-red-700 dark:text-red-400">
                    {t('players.kuInvalidHint')}
                  </p>
                )}
              </form>
            )}

            {/* Quick pick from player history */}
            {canOperate && (playersData?.lists?.history?.length ?? 0) > 0 && (
              <div className="space-y-1.5">
                <div className="text-xs text-muted-foreground">{t('players.recentPlayersHint')}</div>
                <div className="flex flex-wrap gap-2">
                  {playersData!.lists.history.slice(0, 10).map((h) => {
                    const alreadyIn = currentList.includes(h.ku_id)
                    return (
                      <button
                        key={h.ku_id}
                        type="button"
                        disabled={alreadyIn}
                        onClick={() => addKuToCurrent(h.ku_id)}
                        className={cn(
                          'inline-flex min-h-11 sm:min-h-8 items-center gap-1.5 rounded-full border px-3 py-1.5 text-xs transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring',
                          alreadyIn
                            ? 'border-emerald-600/35 bg-emerald-600/10 text-emerald-800 dark:text-emerald-300 cursor-default'
                            : 'border-border bg-secondary/60 hover:border-primary/50 cursor-pointer',
                        )}
                      >
                        {alreadyIn && <Check className="h-3.5 w-3.5 shrink-0" />}
                        <span>{h.last_name || h.ku_id}</span>
                        <span className="font-mono text-muted-foreground">({h.ku_id})</span>
                      </button>
                    )
                  })}
                </div>
              </div>
            )}

            <div className="space-y-1.5 max-h-64 overflow-y-auto">
              {currentList.length === 0 ? (
                <div className="py-6 px-4 text-center text-xs text-muted-foreground leading-relaxed text-pretty">
                  {activeList === 'adminlist'
                    ? t('players.emptyAdminList')
                    : activeList === 'whitelist'
                      ? t('players.emptyWhiteList')
                      : t('players.emptyBlockList')}
                </div>
              ) : (
                currentList.map((ku) => (
                  <div
                    key={ku}
                    className="flex items-center justify-between gap-2 rounded-lg bg-secondary/35 px-3 py-2 text-xs"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <span className="font-mono font-medium truncate">{ku}</span>
                      <CopyButton value={ku} />
                      {historyMap[ku] && <Badge variant="secondary">{historyMap[ku]}</Badge>}
                    </div>
                    {canOperate && (
                      <button
                        type="button"
                        aria-label={`${t('players.removeFromList')} ${ku}`}
                        title={t('players.removeFromList')}
                        onClick={() => setCurrentList(currentList.filter((x) => x !== ku))}
                        className="flex h-11 w-11 sm:h-8 sm:w-8 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-red-600/12 hover:text-red-700 dark:hover:text-red-300 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer pressable"
                      >
                        <X className="h-3.5 w-3.5" />
                      </button>
                    )}
                  </div>
                ))
              )}
            </div>
          </CardContent>
        </Card>

        {/* Chat Stream + Announcement */}
        <Card className="flex flex-col">
          <CardHeader className="pb-3">
            <div className="flex flex-wrap items-center justify-between gap-2">
              <CardTitle className="flex items-center gap-2">
                <MessageSquare className="h-4 w-4 text-primary" />
                {t('players.chatTitle')}
              </CardTitle>
              <SearchInput
                value={chatFilter}
                onChange={setChatFilter}
                placeholder={t('players.chatSearch')}
                className="w-full sm:w-44"
                inputClassName="h-9 sm:h-8"
              />
            </div>
          </CardHeader>
          <CardContent className="flex-1 flex flex-col space-y-3">
            <div className="h-72 overflow-y-auto rounded-lg bg-[var(--surface-terminal)] p-3 space-y-1.5 text-xs font-mono">
              {chatMessages.length === 0 ? (
                <div className="h-full flex items-center justify-center text-slate-400 text-center px-4">
                  {t('players.emptyChat')}
                </div>
              ) : (
                chatMessages.map((m, i) => (
                  <div key={i} className="leading-relaxed break-words">
                    {m.timestamp && <span className="text-slate-400 mr-1.5">[{m.timestamp}]</span>}
                    {m.kind === 'say' && (
                      <>
                        <span className="text-sky-300 font-semibold">{m.sender}</span>
                        <span className="text-slate-400">: </span>
                        <span className="text-slate-100">{m.message}</span>
                      </>
                    )}
                    {m.kind === 'announcement' && (
                      <span className="text-amber-300">
                        [{t('players.announcementTag')}] {m.message}
                      </span>
                    )}
                    {m.kind === 'join' && <span className="text-emerald-400">{m.message}</span>}
                    {m.kind === 'leave' && <span className="text-slate-300">{m.message}</span>}
                    {m.kind === 'death' && <span className="text-red-400">{m.message}</span>}
                  </div>
                ))
              )}
            </div>

            {canOperate && (
              <form onSubmit={handleAnnounce} className="flex gap-2">
                <Input
                  value={announce}
                  onChange={(e) => setAnnounce(e.target.value)}
                  placeholder={t('players.announcePlaceholder')}
                  aria-label={t('players.announcePlaceholder')}
                />
                <Button type="submit" size="sm" disabled={!announce.trim() || announceMut.isPending}>
                  <Megaphone className="h-3.5 w-3.5" />
                  {t('players.sendAnnounce')}
                </Button>
              </form>
            )}
          </CardContent>
        </Card>
      </div>

      <StickySaveBar
        visible={canOperate && listsDirty}
        saving={saveListsMut.isPending}
        saveLabel={t('players.saveLists')}
        onDiscard={discardLists}
        onSave={() => saveListsMut.mutate()}
      />

      {/* Kick / Ban Confirmation */}
      <ConfirmDialog
        open={!!pendingAction}
        onClose={() => setPendingAction(null)}
        onConfirm={() => {
          if (pendingAction) {
            actionMut.mutate({
              action: pendingAction.action,
              ku_id: pendingAction.player.ku_id,
              name: pendingAction.player.name,
            })
          }
        }}
        title={pendingAction?.action === 'ban' ? t('players.banConfirmTitle') : t('players.kickConfirmTitle')}
        description={
          pendingAction?.action === 'ban'
            ? t('players.banConfirm', { name: pendingAction.player.name, kuId: pendingAction.player.ku_id })
            : t('players.kickConfirm', {
                name: pendingAction?.player.name ?? '',
                kuId: pendingAction?.player.ku_id ?? '',
              })
        }
        confirmLabel={pendingAction?.action === 'ban' ? t('players.ban') : t('players.kick')}
        tone={pendingAction?.action === 'ban' ? 'danger' : 'warning'}
        loading={actionMut.isPending}
      />
    </div>
  )
}
