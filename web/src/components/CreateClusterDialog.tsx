import React, { useEffect, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { AlertTriangle, ChevronDown, Network } from 'lucide-react'
import { api } from '@/lib/api'
import { CLUSTER_ID_RE } from '@/lib/dst'
import { cn } from '@/lib/utils'
import { Dialog } from '@/components/ui/dialog'
import { Button } from '@/components/ui/button'
import { Input, Label, SecretInput, Select } from '@/components/ui/input'
import { Switch } from '@/components/ui/switch'
import { CopyButton } from '@/components/ui/copy-button'
import { CollapsibleContent } from '@/components/ui/collapsible'

interface Props {
  open: boolean
  onClose: () => void
  onCreated?: (clusterId: string) => void
}

export function CreateClusterDialog({ open, onClose, onCreated }: Props) {
  const { t, i18n } = useTranslation()
  const qc = useQueryClient()

  const [clusterId, setClusterId] = useState('')
  const [clusterName, setClusterName] = useState('')
  const [description, setDescription] = useState('')
  const [password, setPassword] = useState('')
  const [gameMode, setGameMode] = useState('endless')
  const [intention, setIntention] = useState('cooperative')
  const [maxPlayers, setMaxPlayers] = useState(6)
  const [masterPreset, setMasterPreset] = useState('endless')
  const [cavesEnabled, setCavesEnabled] = useState(true)
  const [pvp, setPvp] = useState(false)
  const [pauseWhenEmpty, setPauseWhenEmpty] = useState(true)
  const [autoRestart, setAutoRestart] = useState(true)
  const [clusterToken, setClusterToken] = useState('')
  const [showAdvanced, setShowAdvanced] = useState(false)

  const { data: portsData } = useQuery({
    queryKey: ['next-ports', open],
    queryFn: api.previewNextPorts,
    enabled: open,
  })

  const { data: presetsData } = useQuery({
    queryKey: ['presets'],
    queryFn: api.presets,
    enabled: open,
  })

  const masterPresets = (presetsData?.presets ?? []).filter((p) => p.shard === 'master')

  const clusterIdInvalid = clusterId.trim().length > 0 && !CLUSTER_ID_RE.test(clusterId.trim())

  const createMut = useMutation({
    mutationFn: api.createCluster,
    onSuccess: (res) => {
      toast.success(t('common.actionSuccess'))
      void qc.invalidateQueries({ queryKey: ['clusters'] })
      onClose()
      onCreated?.(res.cluster.cluster_id)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  useEffect(() => {
    if (!open) {
      setClusterId('')
      setClusterName('')
      setDescription('')
      setPassword('')
      setClusterToken('')
      setShowAdvanced(false)
      createMut.reset()
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open])

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (clusterIdInvalid || !clusterId.trim() || !clusterName.trim()) return
    createMut.mutate({
      cluster_id: clusterId.trim(),
      cluster_name: clusterName.trim(),
      cluster_description: description.trim(),
      cluster_password: password,
      game_mode: gameMode,
      max_players: Number(maxPlayers) || 6,
      pvp,
      pause_when_empty: pauseWhenEmpty,
      cluster_intention: intention,
      caves_enabled: cavesEnabled,
      master_preset: masterPreset,
      cluster_token: clusterToken.trim(),
      auto_restart: autoRestart,
    })
  }

  return (
    <Dialog
      open={open}
      onClose={onClose}
      title={t('createCluster.title')}
      subtitle={t('createCluster.subtitle')}
      maxWidthClass="max-w-2xl"
      footer={
        <div className="flex justify-end gap-2">
          <Button type="button" variant="outline" onClick={onClose}>
            {t('common.cancel')}
          </Button>
          <Button
            type="submit"
            form="create-cluster-form"
            disabled={createMut.isPending || clusterIdInvalid || !clusterId.trim() || !clusterName.trim()}
          >
            {createMut.isPending ? t('common.saving') : t('createCluster.submit')}
          </Button>
        </div>
      }
    >
      <form id="create-cluster-form" onSubmit={handleSubmit} className="space-y-5">
        {createMut.error && (
          <div
            role="alert"
            className="flex items-start gap-2 rounded-lg border border-red-600/35 bg-red-600/10 px-3.5 py-2.5 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
          >
            <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
            <span className="break-words">{createMut.error.message}</span>
          </div>
        )}

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <Label htmlFor="cc-cid">{t('createCluster.clusterId')}</Label>
            <Input
              id="cc-cid"
              required
              aria-invalid={clusterIdInvalid}
              className={cn(clusterIdInvalid && 'border-red-600 focus-visible:ring-red-600')}
              placeholder="friends_s1"
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
            <Label htmlFor="cc-cname">{t('createCluster.clusterName')}</Label>
            <Input
              id="cc-cname"
              required
              placeholder={t('createCluster.clusterNamePlaceholder')}
              value={clusterName}
              onChange={(e) => setClusterName(e.target.value)}
            />
          </div>
        </div>

        <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <Label htmlFor="cc-mode">{t('createCluster.gameMode')}</Label>
            <Select
              id="cc-mode"
              value={gameMode}
              onChange={(e) => {
                setGameMode(e.target.value)
                setMasterPreset(e.target.value)
              }}
            >
              <option value="endless">{t('dashboard.modes.endless')}</option>
              <option value="survival">{t('dashboard.modes.survival')}</option>
              <option value="relaxed">{t('dashboard.modes.relaxed')}</option>
              <option value="wilderness">{t('dashboard.modes.wilderness')}</option>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cc-preset">{t('createCluster.masterPreset')}</Label>
            <Select id="cc-preset" value={masterPreset} onChange={(e) => setMasterPreset(e.target.value)}>
              {masterPresets.map((p) => (
                <option key={p.id} value={p.id}>
                  {i18n.language === 'en-US' ? p.name_en : p.name_zh}
                </option>
              ))}
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cc-intention">{t('createCluster.intention')}</Label>
            <Select id="cc-intention" value={intention} onChange={(e) => setIntention(e.target.value)}>
              <option value="cooperative">{t('config.intentions.cooperative')}</option>
              <option value="social">{t('config.intentions.social')}</option>
              <option value="competitive">{t('config.intentions.competitive')}</option>
              <option value="madness">{t('config.intentions.madness')}</option>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="cc-max">{t('createCluster.maxPlayers')}</Label>
            <Input
              id="cc-max"
              type="number"
              min={1}
              max={64}
              value={maxPlayers}
              onChange={(e) => setMaxPlayers(Number(e.target.value))}
            />
          </div>
        </div>

        {/* Caves primary toggle */}
        <label
          htmlFor="cc-caves"
          className="flex items-center justify-between gap-3 rounded-xl border border-border bg-secondary/35 px-4 py-3 cursor-pointer"
        >
          <div className="min-w-0">
            <div className="text-sm font-medium">{t('createCluster.cavesEnabled')}</div>
            <div className="text-xs text-muted-foreground">{t('createCluster.cavesHint')}</div>
          </div>
          <Switch
            id="cc-caves"
            checked={cavesEnabled}
            onCheckedChange={setCavesEnabled}
            ariaLabel={t('createCluster.cavesEnabled')}
          />
        </label>

        {/* Compact Port Preview */}
        {portsData?.ports && (
          <div className="rounded-xl border border-border bg-secondary/45 px-3.5 py-2.5 text-xs space-y-1.5">
            <div className="flex items-center gap-1.5 font-medium text-foreground">
              <Network className="h-3.5 w-3.5 text-primary shrink-0" />
              <span>{t('createCluster.allocatedPorts')}</span>
            </div>
            <div className="flex flex-wrap items-center gap-x-4 gap-y-1 font-mono text-xs text-muted-foreground tabular-nums">
              <span className="inline-flex items-center gap-0.5">
                Master :{portsData.ports.master_server_port}
                <CopyButton value={String(portsData.ports.master_server_port)} />
              </span>
              {cavesEnabled && (
                <span className="inline-flex items-center gap-0.5">
                  Caves :{portsData.ports.caves_server_port}
                  <CopyButton value={String(portsData.ports.caves_server_port)} />
                </span>
              )}
              <span>ShardBus :{portsData.ports.cluster_master_port}</span>
            </div>
          </div>
        )}

        {/* Progressive Disclosure: Advanced Options */}
        <div className="border-t border-border pt-3">
          <button
            type="button"
            aria-expanded={showAdvanced}
            onClick={() => setShowAdvanced((v) => !v)}
            className="flex w-full items-center justify-between text-xs font-medium text-muted-foreground hover:text-foreground transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded cursor-pointer py-1"
          >
            <span>{t('createCluster.advancedOptions')}</span>
            <ChevronDown className={`h-4 w-4 transition-transform duration-200 ease-out ${showAdvanced ? 'rotate-180' : ''}`} />
          </button>

          <CollapsibleContent open={showAdvanced}>
            <div className="pt-3 px-0.5 pb-0.5 space-y-4">
              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <Label htmlFor="cc-desc">{t('createCluster.description')}</Label>
                  <Input id="cc-desc" value={description} onChange={(e) => setDescription(e.target.value)} />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="cc-pwd">{t('createCluster.password')}</Label>
                  <Input id="cc-pwd" type="text" value={password} onChange={(e) => setPassword(e.target.value)} />
                </div>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 rounded-xl border border-border bg-secondary/30 p-3.5">
                <label htmlFor="cc-pause" className="flex items-center justify-between gap-2 cursor-pointer">
                  <span className="text-xs font-medium">{t('createCluster.pauseWhenEmpty')}</span>
                  <Switch
                    id="cc-pause"
                    checked={pauseWhenEmpty}
                    onCheckedChange={setPauseWhenEmpty}
                    ariaLabel={t('createCluster.pauseWhenEmpty')}
                  />
                </label>
                <label htmlFor="cc-autorestart" className="flex items-center justify-between gap-2 cursor-pointer">
                  <span className="text-xs font-medium">{t('createCluster.autoRestart')}</span>
                  <Switch
                    id="cc-autorestart"
                    checked={autoRestart}
                    onCheckedChange={setAutoRestart}
                    ariaLabel={t('createCluster.autoRestart')}
                  />
                </label>
                <label htmlFor="cc-pvp" className="flex items-center justify-between gap-2 cursor-pointer">
                  <span className="text-xs font-medium">{t('createCluster.pvp')}</span>
                  <Switch
                    id="cc-pvp"
                    checked={pvp}
                    onCheckedChange={setPvp}
                    ariaLabel={t('createCluster.pvp')}
                  />
                </label>
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="cc-token">{t('createCluster.clusterToken')}</Label>
                <SecretInput
                  id="cc-token"
                  placeholder="pds-g^KU_..."
                  value={clusterToken}
                  onChange={(e) => setClusterToken(e.target.value)}
                />
              </div>

              {portsData?.ports && (
                <div className="text-xs font-mono text-muted-foreground tabular-nums flex flex-wrap gap-x-4 gap-y-1">
                  <span>
                    Auth: {portsData.ports.master_auth_port}
                    {cavesEnabled ? ` / ${portsData.ports.caves_auth_port}` : ''}
                  </span>
                  <span>
                    Steam: {portsData.ports.master_steam_port}
                    {cavesEnabled ? ` / ${portsData.ports.caves_steam_port}` : ''}
                  </span>
                </div>
              )}
            </div>
          </CollapsibleContent>
        </div>
      </form>
    </Dialog>
  )
}
