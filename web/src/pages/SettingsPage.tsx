import React, { useEffect, useMemo, useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { ChevronDown, HelpCircle, KeyRound, Network, Save, ServerCog } from 'lucide-react'
import { api } from '@/lib/api'
import { cn } from '@/lib/utils'
import { useRegisterUnsaved } from '@/hooks/useUnsavedGuard'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input, Label, SecretInput, Select } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { CopyButton } from '@/components/ui/copy-button'
import { CollapsibleContent } from '@/components/ui/collapsible'
import { StickySaveBar } from '@/components/StickySaveBar'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { PageHeader } from '@/components/PageHeader'
import { PageCardsSkeleton } from '@/components/ui/skeleton'

export function SettingsPage() {
  const { t } = useTranslation()
  const qc = useQueryClient()

  const { data, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['system-settings'],
    queryFn: api.getSettings,
  })

  const [portStart, setPortStart] = useState(10990)
  const [portEnd, setPortEnd] = useState(11060)
  const [defaultToken, setDefaultToken] = useState('')
  const [steamKey, setSteamKey] = useState('')
  const [branch, setBranch] = useState('public')
  const [arch, setArch] = useState('64')
  const [autoUpdate, setAutoUpdate] = useState('manual')
  const [backupRetain, setBackupRetain] = useState(20)
  const [showTokenGuide, setShowTokenGuide] = useState(false)

  const resetFromData = () => {
    if (!data?.settings) return
    setPortStart(data.settings.port_pool_start)
    setPortEnd(data.settings.port_pool_end)
    setBranch(data.settings.dst_branch)
    setArch(data.settings.dst_arch)
    setAutoUpdate(data.settings.auto_update_mode)
    setBackupRetain(data.settings.backup_max_retain)
    setDefaultToken('')
    setSteamKey('')
  }

  useEffect(() => {
    resetFromData()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [data])

  const isDirty = useMemo(() => {
    if (!data?.settings) return false
    const s = data.settings
    return (
      Number(portStart) !== s.port_pool_start ||
      Number(portEnd) !== s.port_pool_end ||
      branch !== s.dst_branch ||
      arch !== s.dst_arch ||
      autoUpdate !== s.auto_update_mode ||
      Number(backupRetain) !== s.backup_max_retain ||
      defaultToken.trim().length > 0 ||
      steamKey.trim().length > 0
    )
  }, [data, portStart, portEnd, branch, arch, autoUpdate, backupRetain, defaultToken, steamKey])

  const startNum = Number(portStart)
  const endNum = Number(portEnd)
  const portPoolValid =
    Number.isInteger(startNum) &&
    Number.isInteger(endNum) &&
    startNum >= 1024 &&
    endNum <= 65535 &&
    endNum - startNum >= 6
  const portPoolCapacity = portPoolValid ? Math.floor((endNum - startNum + 1) / 7) : 0

  useRegisterUnsaved('system-settings', isDirty)

  const saveMut = useMutation({
    mutationFn: () => {
      const payload: Record<string, unknown> = {
        port_pool_start: Number(portStart),
        port_pool_end: Number(portEnd),
        dst_branch: branch,
        dst_arch: arch,
        auto_update_mode: autoUpdate,
        backup_max_retain: Number(backupRetain),
      }
      if (defaultToken.trim()) payload.default_cluster_token = defaultToken.trim()
      if (steamKey.trim()) payload.steam_api_key = steamKey.trim()
      return api.updateSettings(payload)
    },
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      setDefaultToken('')
      setSteamKey('')
      void qc.invalidateQueries({ queryKey: ['system-settings'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        if (!isDirty || !portPoolValid || saveMut.isPending) return
        e.preventDefault()
        saveMut.mutate()
      }
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [isDirty, portPoolValid, saveMut])

  if (isError) {
    return <QueryErrorCard error={error} onRetry={() => void refetch()} />
  }

  if (isLoading || !data) {
    return <PageCardsSkeleton count={3} />
  }

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (!portPoolValid) return
    saveMut.mutate()
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <PageHeader
        eyebrow={t('nav.system')}
        title={t('settings.title')}
        subtitle={t('settings.subtitle')}
        badge={isDirty ? <Badge variant="warning">{t('files.unsaved')}</Badge> : undefined}
      />

      {/* Runtime Info */}
      <Card>
        <CardHeader>
          <CardTitle>{t('settings.runtimeInfo')}</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-wrap items-center gap-3 text-xs">
          <Badge variant={data.runtime_mode === 'mock' ? 'warning' : 'success'}>
            {data.runtime_mode === 'mock' ? t('common.mockBadge') : t('common.linuxBadge')}
          </Badge>
          <div className="flex items-start gap-1.5 min-w-0 text-muted-foreground">
            <span className="shrink-0 pt-0.5">{t('settings.dataDir')}:</span>
            <code className="font-mono text-foreground break-all pt-0.5">{data.data_dir}</code>
            <CopyButton value={data.data_dir} />
          </div>
        </CardContent>
      </Card>

      {/* Port Pool */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <Network className="h-4 w-4 text-primary" />
            {t('settings.portPool')}
          </CardTitle>
          <CardDescription>{t('settings.portPoolHint')}</CardDescription>
        </CardHeader>
        <CardContent className="space-y-3">
          <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
            <div className="space-y-1.5">
              <Label htmlFor="set-pstart">{t('settings.portStart')}</Label>
              <Input
                id="set-pstart"
                type="number"
                min={1024}
                max={65000}
                aria-invalid={!portPoolValid}
                className={cn(!portPoolValid && 'border-red-600 focus-visible:ring-red-600')}
                value={portStart}
                onChange={(e) => setPortStart(Number(e.target.value))}
              />
            </div>
            <div className="space-y-1.5">
              <Label htmlFor="set-pend">{t('settings.portEnd')}</Label>
              <Input
                id="set-pend"
                type="number"
                min={1030}
                max={65535}
                aria-invalid={!portPoolValid}
                className={cn(!portPoolValid && 'border-red-600 focus-visible:ring-red-600')}
                value={portEnd}
                onChange={(e) => setPortEnd(Number(e.target.value))}
              />
            </div>
          </div>
          {!portPoolValid ? (
            <p role="alert" className="text-xs font-medium text-red-700 dark:text-red-400">
              {t('settings.portPoolInvalid')}
            </p>
          ) : (
            <p className="text-xs text-muted-foreground tabular-nums">
              {t('settings.portPoolCapacity', { count: portPoolCapacity })}
            </p>
          )}
        </CardContent>
      </Card>

      {/* Tokens */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <KeyRound className="h-4 w-4 text-primary" />
            {t('settings.tokensSection')}
          </CardTitle>
        </CardHeader>
        <CardContent className="space-y-4">
          <div className="space-y-1.5">
            <div className="flex items-center justify-between gap-2 flex-wrap">
              <Label htmlFor="set-token">{t('settings.defaultToken')}</Label>
              {data.settings.has_default_token && (
                <Badge variant="success" className="font-mono">
                  {data.settings.default_cluster_token}
                </Badge>
              )}
            </div>
            <SecretInput
              id="set-token"
              placeholder={
                data.settings.has_default_token ? t('settings.secretPlaceholder') : t('settings.tokenPlaceholderUnset')
              }
              value={defaultToken}
              onChange={(e) => setDefaultToken(e.target.value)}
            />
            <div>
              <button
                type="button"
                aria-expanded={showTokenGuide}
                onClick={() => setShowTokenGuide((v) => !v)}
                className="inline-flex items-center gap-1 rounded text-xs font-medium text-primary hover:underline transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer"
              >
                <HelpCircle className="h-3.5 w-3.5" />
                <span>{t('auth.tokenGuideToggle')}</span>
                <ChevronDown className={`h-3.5 w-3.5 transition-transform duration-200 ease-out ${showTokenGuide ? 'rotate-180' : ''}`} />
              </button>
              <CollapsibleContent open={showTokenGuide}>
                <div className="pt-1.5">
                  <div className="rounded-lg bg-secondary/45 px-3 py-2 text-xs text-muted-foreground leading-relaxed">
                    {t('auth.tokenGuideSteps')}
                  </div>
                </div>
              </CollapsibleContent>
            </div>
          </div>
          <div className="space-y-1.5">
            <div className="flex items-center justify-between gap-2 flex-wrap">
              <Label htmlFor="set-steamkey">{t('settings.steamApiKey')}</Label>
              {data.settings.has_steam_api_key && (
                <Badge variant="success" className="font-mono">
                  {data.settings.steam_api_key}
                </Badge>
              )}
            </div>
            <SecretInput
              id="set-steamkey"
              placeholder={
                data.settings.has_steam_api_key
                  ? t('settings.secretPlaceholder')
                  : t('settings.steamKeyPlaceholderUnset')
              }
              value={steamKey}
              onChange={(e) => setSteamKey(e.target.value)}
            />
          </div>
        </CardContent>
      </Card>

      {/* DST Server Params */}
      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            <ServerCog className="h-4 w-4 text-primary" />
            {t('settings.dstSection')}
          </CardTitle>
        </CardHeader>
        <CardContent className="grid grid-cols-1 sm:grid-cols-2 gap-4">
          <div className="space-y-1.5">
            <Label htmlFor="set-branch">{t('settings.branch')}</Label>
            <Select id="set-branch" value={branch} onChange={(e) => setBranch(e.target.value)}>
              <option value="public">{t('settings.branches.public')}</option>
              <option value="updatebeta">{t('settings.branches.updatebeta')}</option>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="set-arch">{t('settings.arch')}</Label>
            <Select id="set-arch" value={arch} onChange={(e) => setArch(e.target.value)}>
              <option value="64">{t('settings.arches.64')}</option>
              <option value="32">{t('settings.arches.32')}</option>
            </Select>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="set-autoupd">{t('settings.autoUpdate')}</Label>
            <Select id="set-autoupd" value={autoUpdate} onChange={(e) => setAutoUpdate(e.target.value)}>
              <option value="manual">{t('settings.autoUpdateModes.manual')}</option>
              <option value="when_empty">{t('settings.autoUpdateModes.when_empty')}</option>
              <option value="scheduled">{t('settings.autoUpdateModes.scheduled')}</option>
            </Select>
            <p className="text-xs text-muted-foreground leading-relaxed">{t('settings.autoUpdateHint')}</p>
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="set-retain">{t('settings.backupRetain')}</Label>
            <Input
              id="set-retain"
              type="number"
              min={1}
              max={200}
              value={backupRetain}
              onChange={(e) => setBackupRetain(Number(e.target.value))}
            />
          </div>
        </CardContent>
      </Card>

      <div className="flex justify-end gap-2">
        {isDirty && (
          <Button type="button" variant="outline" disabled={saveMut.isPending} onClick={resetFromData}>
            {t('common.discard')}
          </Button>
        )}
        <Button type="submit" disabled={saveMut.isPending || !isDirty || !portPoolValid}>
          <Save className="h-4 w-4" />
          {saveMut.isPending ? t('common.saving') : t('common.save')}
        </Button>
      </div>

      <StickySaveBar
        visible={isDirty}
        saving={saveMut.isPending}
        disabled={!portPoolValid}
        onDiscard={resetFromData}
        onSave={() => {
          if (portPoolValid) saveMut.mutate()
        }}
      />
    </form>
  )
}
