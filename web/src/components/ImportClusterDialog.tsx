import React, { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { AlertTriangle, FolderArchive, Upload } from 'lucide-react'
import { api } from '@/lib/api'
import { CLUSTER_ID_RE } from '@/lib/dst'
import { cn } from '@/lib/utils'
import { Dialog } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input, Label, SecretInput } from '@/components/ui/input'

interface Props {
  open: boolean
  onClose: () => void
  onImported?: (clusterId: string) => void
}

export function ImportClusterDialog({ open, onClose, onImported }: Props) {
  const { t } = useTranslation()
  const qc = useQueryClient()

  const [file, setFile] = useState<File | null>(null)
  const [clusterId, setClusterId] = useState('')
  const [clusterName, setClusterName] = useState('')
  const [clusterToken, setClusterToken] = useState('')

  const importMut = useMutation({
    mutationFn: () =>
      api.importClusterZip({
        file: file!,
        cluster_id: clusterId.trim(),
        cluster_name: clusterName.trim() || undefined,
        cluster_token: clusterToken.trim() || undefined,
      }),
    onSuccess: (res) => {
      toast.success(t('common.actionSuccess'))
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      setFile(null)
      setClusterId('')
      setClusterName('')
      setClusterToken('')
      onClose()
      onImported?.(res.cluster.cluster_id)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const clusterIdInvalid = clusterId.trim().length > 0 && !CLUSTER_ID_RE.test(clusterId.trim())

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!file || !clusterId.trim() || clusterIdInvalid) return
    importMut.mutate()
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={t('importCluster.title')}
      subtitle={t('importCluster.subtitle')}
      maxWidthClass="max-w-lg"
      footer={
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            form="import-cluster-form"
            disabled={!file || !clusterId.trim() || clusterIdInvalid || importMut.isPending}
          >
            <Upload className="h-4 w-4" />
            {importMut.isPending ? t('importCluster.importing') : t('importCluster.submit')}
          </Button>
        </div>
      }
    >
      <form id="import-cluster-form" onSubmit={handleSubmit} className="space-y-4">
        {importMut.error && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-lg border border-red-600/35 bg-red-600/10 px-3.5 py-2.5 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
          >
            <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
            <span className="break-words">{importMut.error.message}</span>
          </div>
        )}

        <div className="space-y-1.5">
          <Label htmlFor="ic-file">{t('importCluster.selectZip')}</Label>
          <label
            htmlFor="ic-file"
            className="flex flex-col items-center justify-center gap-2 rounded-xl border-2 border-dashed border-border hover:border-primary/60 focus-within:ring-2 focus-within:ring-ring bg-secondary/25 p-6 text-center cursor-pointer transition-colors"
          >
            {file ? (
              <>
                <FolderArchive className="h-7 w-7 text-primary" />
                <div className="text-sm font-medium">{file.name}</div>
                <div className="text-xs text-muted-foreground tabular-nums">
                  {(file.size / 1024 / 1024).toFixed(2)} MB
                </div>
              </>
            ) : (
              <>
                <Upload className="h-7 w-7 text-muted-foreground" />
                <div className="text-xs text-muted-foreground">{t('importCluster.selectZip')}</div>
              </>
            )}
            <input
              id="ic-file"
              type="file"
              accept=".zip"
              className="sr-only"
              onChange={(e) => {
                const f = e.target.files?.[0] ?? null
                setFile(f)
                if (f && !clusterId) {
                  const base = f.name
                    .replace(/\.zip$/i, '')
                    .replace(/[^a-zA-Z0-9_-]+/g, '_')
                    .slice(0, 32)
                  if (base.length >= 2) setClusterId(base)
                }
              }}
            />
          </label>
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="ic-cid">{t('importCluster.clusterId')}</Label>
          <Input
            id="ic-cid"
            required
            aria-invalid={clusterIdInvalid}
            className={cn(clusterIdInvalid && 'border-red-600 focus-visible:ring-red-600')}
            placeholder="imported_world_1"
            value={clusterId}
            onChange={(e) => setClusterId(e.target.value)}
          />
          {clusterIdInvalid ? (
            <p role="alert" className="text-xs font-medium text-red-700 dark:text-red-400">
              {t('createCluster.clusterIdInvalid')}
            </p>
          ) : (
            <p className="text-xs text-muted-foreground">{t('createCluster.clusterIdHint')}</p>
          )}
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="ic-cname">{t('importCluster.clusterNameOptional')}</Label>
          <Input id="ic-cname" value={clusterName} onChange={(e) => setClusterName(e.target.value)} />
        </div>

        <div className="space-y-1.5">
          <Label htmlFor="ic-token">{t('importCluster.tokenOptional')}</Label>
          <SecretInput
            id="ic-token"
            placeholder="pds-g^KU_..."
            value={clusterToken}
            onChange={(e) => setClusterToken(e.target.value)}
          />
        </div>
      </form>
    </Dialog>
  )
}
