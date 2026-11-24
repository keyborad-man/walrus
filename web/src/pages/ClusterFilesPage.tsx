import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  ChevronRight,
  Download,
  FileCode,
  FileText,
  Folder,
  FolderOpen,
  Lock,
  RefreshCw,
  RotateCw,
  Save,
  Trash2,
} from 'lucide-react'
import { api } from '@/lib/api'
import { cn, formatBytes, formatDateTime } from '@/lib/utils'
import { useRegisterUnsaved } from '@/hooks/useUnsavedGuard'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button, buttonVariants } from '@/components/ui/button'
import { Badge } from '@/components/ui/badge'
import { SearchInput } from '@/components/SearchInput'
import { StickySaveBar } from '@/components/StickySaveBar'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { PageHeader } from '@/components/PageHeader'
import type { FileEntry } from '@/types/api'

export function ClusterFilesPage() {
  const { id = '' } = useParams<{ id: string }>()
  const { t } = useTranslation()
  const qc = useQueryClient()

  const [curDir, setCurDir] = useState('')
  const [fileFilter, setFileFilter] = useState('')
  const [selectedFile, setSelectedFile] = useState<string | null>(null)
  const [editorText, setEditorText] = useState('')
  const [dirty, setDirty] = useState(false)
  const [savedNeedRestart, setSavedNeedRestart] = useState(false)
  const [restartConfirmOpen, setRestartConfirmOpen] = useState(false)
  const [deleteTarget, setDeleteTarget] = useState<FileEntry | null>(null)
  const [pendingSelectPath, setPendingSelectPath] = useState<string | null>(null)

  const { data: clusterData } = useQuery({
    queryKey: ['cluster', id],
    queryFn: () => api.getCluster(id),
    enabled: !!id,
  })
  const canOperate = clusterData?.effective_role === 'admin' || clusterData?.effective_role === 'operator'
  const isClusterRunning =
    clusterData?.cluster.master_status.state === 'running' ||
    (clusterData?.cluster.meta.caves_enabled && clusterData?.cluster.caves_status?.state === 'running')
  const onlinePlayersCount = clusterData?.cluster.world_info?.online_players?.length ?? 0

  const {
    data: dirData,
    isLoading,
    isFetching: dirFetching,
    isError: dirIsError,
    error: dirError,
    refetch,
  } = useQuery({
    queryKey: ['cluster-files', id, curDir],
    queryFn: () => api.listFiles(id, curDir),
    enabled: !!id,
  })

  useRegisterUnsaved('cluster-files', dirty)

  const { data: fileData, isFetching: loadingFile } = useQuery({
    queryKey: ['cluster-file-content', id, selectedFile],
    queryFn: () => api.readFile(id, selectedFile!),
    enabled: !!id && !!selectedFile,
  })

  useEffect(() => {
    setCurDir('')
    setSelectedFile(null)
    setEditorText('')
    setDirty(false)
  }, [id])

  useEffect(() => {
    if (fileData) {
      setEditorText(fileData.content ?? '')
      setDirty(false)
    }
  }, [fileData])

  const saveMut = useMutation({
    mutationFn: () => api.writeFile(id, selectedFile!, editorText),
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setDirty(false)
      setSavedNeedRestart(true)
      void qc.invalidateQueries({ queryKey: ['cluster-file-content', id, selectedFile] })
      void qc.invalidateQueries({ queryKey: ['cluster-files', id, curDir] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const restartClusterMut = useMutation({
    mutationFn: () => api.restartCluster(id),
    onSuccess: () => {
      toast.success(t('dashboard.toastRestarted', { name: clusterData?.cluster.cluster_ini.cluster_name || id }))
      setSavedNeedRestart(false)
      setRestartConfirmOpen(false)
      void qc.invalidateQueries({ queryKey: ['cluster', id] })
      void qc.invalidateQueries({ queryKey: ['clusters'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (relPath: string) => api.deleteFile(id, relPath),
    onSuccess: (_, relPath) => {
      toast.success(t('common.actionSuccess'))
      setDeleteTarget(null)
      if (selectedFile === relPath) {
        setSelectedFile(null)
        setEditorText('')
        setDirty(false)
      }
      void qc.invalidateQueries({ queryKey: ['cluster-files', id, curDir] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  // Ctrl/Cmd + S shortcut
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        if (!canOperate || !fileData?.editable || !dirty || !selectedFile || saveMut.isPending) return
        e.preventDefault()
        saveMut.mutate()
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [canOperate, fileData?.editable, dirty, selectedFile, saveMut])

  const breadcrumbs = curDir ? curDir.split('/') : []
  const entries = dirData?.entries ?? []
  const filteredEntries = fileFilter.trim()
    ? entries.filter((e) => e.name.toLowerCase().includes(fileFilter.trim().toLowerCase()))
    : entries

  const handleDiscardFile = () => {
    if (!fileData) return
    setEditorText(fileData.content ?? '')
    setDirty(false)
  }

  const requestSelectFile = (relPath: string) => {
    if (relPath === selectedFile) return
    if (dirty) {
      setPendingSelectPath(relPath)
    } else {
      setSelectedFile(relPath)
    }
  }

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={t('nav.groupCare')}
        title={t('files.title')}
        subtitle={t('files.subtitle')}
        actions={
          <Button variant="outline" size="sm" onClick={() => void refetch()}>
            <RefreshCw className={cn('h-3.5 w-3.5', dirFetching && 'animate-spin')} />
            {t('common.refresh')}
          </Button>
        }
      />

      {dirIsError && <QueryErrorCard error={dirError} onRetry={() => void refetch()} compact />}

      {savedNeedRestart && isClusterRunning && canOperate && (
        <div className="flex flex-wrap items-center justify-between gap-3 rounded-lg border border-border bg-card px-3.5 py-2.5 text-[13px]">
          <div className="flex items-center gap-2.5 min-w-0">
            <span className="h-[7px] w-[7px] rounded-full bg-amber-500 shrink-0" aria-hidden="true" />
            <span className="text-foreground/90">{t('files.savedRestartPrompt')}</span>
          </div>
          <Button
            size="sm"
            variant="warning"
            disabled={restartClusterMut.isPending}
            onClick={() => {
              if (onlinePlayersCount > 0) setRestartConfirmOpen(true)
              else restartClusterMut.mutate()
            }}
          >
            <RotateCw className={cn('h-3.5 w-3.5', restartClusterMut.isPending && 'animate-spin')} />
            {t('config.restartNow')}
          </Button>
        </div>
      )}

      {/* Breadcrumbs */}
      <nav
        aria-label={t('files.title')}
        className="flex items-center gap-1 text-xs flex-wrap rounded-lg border border-border bg-card px-2.5 py-1.5"
      >
        <button
          type="button"
          onClick={() => {
            setCurDir('')
            setFileFilter('')
          }}
          className="min-h-10 sm:min-h-7 px-1.5 font-mono text-primary hover:underline cursor-pointer flex items-center gap-1 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
        >
          <FolderOpen className="h-3.5 w-3.5" />
          {id} ({t('files.root')})
        </button>
        {breadcrumbs.map((seg, idx) => {
          const target = breadcrumbs.slice(0, idx + 1).join('/')
          return (
            <span key={target} className="inline-flex items-center gap-1">
              <ChevronRight className="h-3.5 w-3.5 text-muted-foreground" />
              <button
                type="button"
                onClick={() => {
                  setCurDir(target)
                  setFileFilter('')
                }}
                className="min-h-10 sm:min-h-7 px-1.5 font-mono hover:text-primary cursor-pointer rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
              >
                {seg}
              </button>
            </span>
          )
        })}
      </nav>

      {/* Split Layout: Directory List + File Viewer/Editor */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-4">
        {/* Directory Panel */}
        <Card className="lg:col-span-5">
          <CardHeader className="pb-2 space-y-2">
            <CardTitle className="text-sm font-mono">{curDir || '/'}</CardTitle>
            {entries.some((e) => e.protected) && (
              <p className="text-xs text-muted-foreground flex items-center gap-1.5">
                <Lock className="h-3 w-3 shrink-0" />
                <span>{t('files.protectedLegend')}</span>
              </p>
            )}
            {entries.length > 0 && (
              <SearchInput
                value={fileFilter}
                onChange={setFileFilter}
                placeholder={t('files.searchPlaceholder')}
                inputClassName="h-8"
              />
            )}
          </CardHeader>
          <CardContent>
            {isLoading ? (
              <div className="py-10 text-center text-xs text-muted-foreground">{t('common.loading')}</div>
            ) : filteredEntries.length === 0 ? (
              <div className="py-10 text-center text-xs text-muted-foreground">{t('common.noData')}</div>
            ) : (
              <div className="divide-y divide-border max-h-[540px] overflow-y-auto">
                {curDir && (
                  <button
                    type="button"
                    onClick={() => {
                      const parts = curDir.split('/')
                      parts.pop()
                      setCurDir(parts.join('/'))
                      setFileFilter('')
                    }}
                    className="w-full py-2.5 px-2 flex items-center gap-2 text-xs text-muted-foreground hover:bg-secondary/50 rounded-md cursor-pointer focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <Folder className="h-4 w-4 text-muted-foreground" />
                    <span className="font-mono">..</span>
                  </button>
                )}
                {filteredEntries.map((entry) => {
                  const isSel = selectedFile === entry.rel_path
                  return (
                    <div
                      key={entry.rel_path}
                      className={cn(
                        'py-2 px-2 flex items-center justify-between gap-2 rounded-md transition-colors',
                        isSel ? 'bg-[var(--surface-hover)]' : 'hover:bg-[var(--surface-hover)]/50',
                      )}
                    >
                      <button
                        type="button"
                        onClick={() => {
                          if (entry.is_dir) {
                            setCurDir(entry.rel_path)
                            setFileFilter('')
                          } else {
                            requestSelectFile(entry.rel_path)
                          }
                        }}
                        className="flex items-center gap-2 min-w-0 flex-1 text-left cursor-pointer py-1 rounded focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                      >
                        {entry.is_dir ? (
                          <Folder className="h-4 w-4 shrink-0 text-muted-foreground" />
                        ) : entry.editable ? (
                          <FileCode className="h-4 w-4 shrink-0 text-foreground/75" />
                        ) : (
                          <FileText className="h-4 w-4 shrink-0 text-muted-foreground" />
                        )}
                        <div className="min-w-0">
                          <div className="text-xs font-mono font-medium truncate flex items-center gap-1.5">
                            <span className="truncate">{entry.name}</span>
                            {entry.protected && (
                              <Lock
                                className="h-3.5 w-3.5 text-muted-foreground shrink-0"
                                aria-label={t('files.protected')}
                              />
                            )}
                          </div>
                          <div className="text-xs text-muted-foreground tabular-nums">
                            {entry.is_dir ? t('files.dirBadge') : formatBytes(entry.size_bytes)} ·{' '}
                            {formatDateTime(entry.mod_time)}
                          </div>
                        </div>
                      </button>

                      {!entry.is_dir && (
                        <div className="flex items-center gap-1 shrink-0">
                          <a
                            href={api.fileDownloadUrl(id, entry.rel_path)}
                            download={entry.name}
                            aria-label={`${t('backups.download')} ${entry.name}`}
                            title={t('backups.download')}
                            className={cn(buttonVariants({ size: 'icon', variant: 'ghost' }), 'h-11 w-11 sm:h-8 sm:w-8')}
                          >
                            <Download className="h-3.5 w-3.5" />
                          </a>
                          {canOperate && !entry.protected && (
                            <Button
                              size="icon"
                              variant="ghost"
                              aria-label={`${t('common.delete')} ${entry.name}`}
                              title={t('common.delete')}
                              className="h-11 w-11 sm:h-8 sm:w-8 text-red-700 hover:bg-red-600/12 dark:text-red-400 dark:hover:bg-red-500/12"
                              onClick={() => setDeleteTarget(entry)}
                            >
                              <Trash2 className="h-3.5 w-3.5" />
                            </Button>
                          )}
                        </div>
                      )}
                    </div>
                  )
                })}
              </div>
            )}
          </CardContent>
        </Card>

        {/* Viewer / Editor Panel */}
        <Card className="lg:col-span-7 flex flex-col">
          <CardHeader className="pb-3 flex flex-row items-center justify-between gap-2">
            <div className="min-w-0">
              <CardTitle className="text-sm font-mono truncate">
                {selectedFile || t('files.previewTitle')}
              </CardTitle>
              {fileData && (
                <div className="flex items-center gap-2 mt-1.5 flex-wrap">
                  <Badge variant="secondary" className="font-mono tabular-nums">
                    {formatBytes(fileData.size_bytes)}
                  </Badge>
                  {fileData.editable && canOperate && (
                    <Badge variant={dirty ? 'warning' : 'success'}>
                      {dirty ? t('files.unsaved') : t('files.editable')}
                    </Badge>
                  )}
                </div>
              )}
            </div>
            {selectedFile && fileData && (
              <div className="flex items-center gap-2 shrink-0">
                <a
                  href={api.fileDownloadUrl(id, selectedFile)}
                  download
                  aria-label={t('backups.download')}
                  title={t('backups.download')}
                  className={buttonVariants({ size: 'sm', variant: 'outline' })}
                >
                  <Download className="h-3.5 w-3.5" />
                  <span className="hidden sm:inline">{t('backups.download')}</span>
                </a>
                {canOperate && fileData.editable && (
                  <>
                    {dirty && (
                      <Button size="sm" variant="outline" disabled={saveMut.isPending} onClick={handleDiscardFile}>
                        {t('common.discard')}
                      </Button>
                    )}
                    <Button size="sm" disabled={!dirty || saveMut.isPending} onClick={() => saveMut.mutate()}>
                      <Save className="h-3.5 w-3.5" />
                      {t('common.save')}
                    </Button>
                  </>
                )}
              </div>
            )}
          </CardHeader>
          <CardContent className="flex-1 flex flex-col">
            {!selectedFile ? (
              <div className="flex-1 min-h-[360px] flex items-center justify-center text-xs text-muted-foreground rounded-lg bg-secondary/25 p-6 text-center text-balance">
                {t('files.selectFileHint')}
              </div>
            ) : loadingFile ? (
              <div className="flex-1 min-h-[360px] flex items-center justify-center text-xs text-muted-foreground">
                {t('common.loading')}
              </div>
            ) : fileData?.is_binary ? (
              <div className="flex-1 min-h-[360px] flex flex-col items-center justify-center gap-3 text-xs text-muted-foreground rounded-lg bg-secondary/25 p-6 text-center">
                <p>{t('files.binaryNotice')}</p>
                <a
                  href={api.fileDownloadUrl(id, selectedFile)}
                  download
                  className={buttonVariants({ size: 'sm', variant: 'secondary' })}
                >
                  <Download className="h-3.5 w-3.5" />
                  {t('backups.download')}
                </a>
              </div>
            ) : (
              <div className="space-y-2 flex-1 flex flex-col">
                {fileData?.truncated && (
                  <div className="rounded-lg border border-amber-600/35 bg-amber-500/12 px-3 py-1.5 text-xs text-amber-900 dark:border-amber-500/30 dark:text-amber-300">
                    {t('files.truncatedNotice')}
                  </div>
                )}
                <textarea
                  readOnly={!canOperate || !fileData?.editable}
                  aria-label={selectedFile}
                  value={editorText}
                  onChange={(e) => {
                    setEditorText(e.target.value)
                    setDirty(true)
                  }}
                  spellCheck={false}
                  className="w-full flex-1 min-h-[440px] rounded-xl border border-[var(--surface-terminal-border)] bg-[var(--surface-terminal)] p-4 font-mono text-xs leading-relaxed text-slate-200 focus:outline-none focus:ring-2 focus:ring-ring"
                />
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <StickySaveBar
        visible={canOperate && Boolean(fileData?.editable) && dirty}
        saving={saveMut.isPending}
        onDiscard={handleDiscardFile}
        onSave={() => saveMut.mutate()}
      />

      {/* Confirmations */}
      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => {
          if (deleteTarget) deleteMut.mutate(deleteTarget.rel_path)
        }}
        title={t('files.deleteConfirmTitle')}
        description={
          deleteTarget?.name === 'cluster_token.txt'
            ? t('files.deleteTokenConfirm', { name: deleteTarget.name })
            : t('files.deleteConfirm', { name: deleteTarget?.name ?? '' })
        }
        confirmLabel={t('common.delete')}
        tone="danger"
        loading={deleteMut.isPending}
      />

      <ConfirmDialog
        open={restartConfirmOpen}
        onClose={() => setRestartConfirmOpen(false)}
        onConfirm={() => restartClusterMut.mutate()}
        title={t('dashboard.restartOnlineConfirmTitle')}
        description={t('dashboard.restartOnlineConfirmDesc', {
          name: clusterData?.cluster.cluster_ini.cluster_name || id,
          count: onlinePlayersCount,
        })}
        confirmLabel={t('config.restartNow')}
        tone="warning"
        loading={restartClusterMut.isPending}
      />

      <ConfirmDialog
        open={pendingSelectPath !== null}
        onClose={() => setPendingSelectPath(null)}
        onConfirm={() => {
          if (pendingSelectPath) {
            setDirty(false)
            setSelectedFile(pendingSelectPath)
            setPendingSelectPath(null)
          }
        }}
        title={t('common.unsavedSwitchConfirmTitle')}
        description={t('common.unsavedSwitchConfirmDesc')}
        confirmLabel={t('common.discard')}
        tone="warning"
      />
    </div>
  )
}
