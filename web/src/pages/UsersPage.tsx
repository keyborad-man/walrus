import React, { useState } from 'react'
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { AlertTriangle, Pencil, Plus, Shield, Trash2 } from 'lucide-react'
import { api } from '@/lib/api'
import { formatDateTime } from '@/lib/utils'
import { useAuth } from '@/hooks/useAuth'
import { Card, CardContent, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input, Label, SecretInput, Select } from '@/components/ui/input'
import { Badge } from '@/components/ui/badge'
import { Switch } from '@/components/ui/switch'
import { QueryErrorCard } from '@/components/QueryErrorCard'
import { Dialog } from '@/components/ui/dialog'
import { ConfirmDialog } from '@/components/ConfirmDialog'
import { PageHeader } from '@/components/PageHeader'
import { TableRowsSkeleton } from '@/components/ui/skeleton'
import type { Role, UserClusterPermission, UserWithPermissions } from '@/types/api'

export function UsersPage() {
  const { t } = useTranslation()
  const { user: currentUser } = useAuth()
  const qc = useQueryClient()

  const { data: usersData, isLoading, isError, error, refetch } = useQuery({
    queryKey: ['users'],
    queryFn: api.listUsers,
  })
  const { data: clustersData } = useQuery({
    queryKey: ['clusters'],
    queryFn: api.listClusters,
  })

  const users = usersData?.users ?? []
  const clusters = clustersData?.clusters ?? []

  const [dialogOpen, setDialogOpen] = useState(false)
  const [editing, setEditing] = useState<UserWithPermissions | null>(null)
  const [deleteTarget, setDeleteTarget] = useState<UserWithPermissions | null>(null)
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<Role>('operator')
  const [disabled, setDisabled] = useState(false)
  const [scopedPerms, setScopedPerms] = useState<Record<string, Role | ''>>({})

  const openCreate = () => {
    setEditing(null)
    setUsername('')
    setPassword('')
    setRole('operator')
    setDisabled(false)
    setScopedPerms({})
    setDialogOpen(true)
  }

  const openEdit = (u: UserWithPermissions) => {
    setEditing(u)
    setUsername(u.username)
    setPassword('')
    setRole(u.role)
    setDisabled(u.disabled)
    const map: Record<string, Role | ''> = {}
    for (const p of u.permissions ?? []) {
      map[p.cluster_id] = p.role
    }
    setScopedPerms(map)
    setDialogOpen(true)
  }

  const buildPermList = (): UserClusterPermission[] => {
    if (role === 'admin') return []
    return Object.entries(scopedPerms)
      .filter(([, r]) => r === 'operator' || r === 'viewer')
      .map(([cluster_id, r]) => ({ cluster_id, role: r as Role }))
  }

  const createMut = useMutation({
    mutationFn: () =>
      api.createUser({
        username: username.trim(),
        password,
        role,
        permissions: buildPermList(),
      }),
    onSuccess: () => {
      toast.success(t('common.actionSuccess'))
      void qc.invalidateQueries({ queryKey: ['users'] })
      setDialogOpen(false)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const updateMut = useMutation({
    mutationFn: () =>
      api.updateUser(editing!.id, {
        role,
        disabled,
        password: password || undefined,
        permissions: buildPermList(),
      }),
    onSuccess: () => {
      toast.success(t('common.savedSuccess'))
      void qc.invalidateQueries({ queryKey: ['users'] })
      setDialogOpen(false)
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const deleteMut = useMutation({
    mutationFn: (uid: number) => api.deleteUser(uid),
    onSuccess: () => {
      toast.success(t('common.actionSuccess'))
      setDeleteTarget(null)
      void qc.invalidateQueries({ queryKey: ['users'] })
    },
    onError: (err: Error) => toast.error(err.message),
  })

  const handleSubmit = (e: React.FormEvent) => {
    e.preventDefault()
    if (editing) updateMut.mutate()
    else createMut.mutate()
  }

  return (
    <div className="space-y-6">
      <PageHeader
        eyebrow={t('nav.system')}
        title={t('users.title')}
        subtitle={t('users.subtitle')}
        actions={
          <Button onClick={openCreate}>
            <Plus className="h-4 w-4" />
            {t('users.addUser')}
          </Button>
        }
      />

      {isError && <QueryErrorCard error={error} onRetry={() => void refetch()} compact />}

      <Card>
        <CardHeader>
          <CardTitle className="flex items-center gap-2 tabular-nums">
            <Shield className="h-4 w-4 text-primary" />
            {t('users.title')} ({users.length})
          </CardTitle>
        </CardHeader>
        <CardContent>
          {isLoading ? (
            <TableRowsSkeleton rows={4} />
          ) : (
            <>
              {/* Mobile stacked cards (< md) */}
              <div className="divide-y divide-border md:hidden">
                {users.map((u) => (
                  <div key={u.id} className="py-3.5 first:pt-0 last:pb-0 space-y-2.5">
                    <div className="flex items-start justify-between gap-2">
                      <div className="min-w-0">
                        <div className="font-semibold text-sm truncate">{u.username}</div>
                        <div className="mt-1 flex flex-wrap items-center gap-1.5">
                          <Badge variant={u.role === 'admin' ? 'default' : u.role === 'operator' ? 'info' : 'secondary'}>
                            {t(`users.roles.${u.role}`, u.role)}
                          </Badge>
                          <Badge variant={u.disabled ? 'destructive' : 'success'}>
                            {u.disabled ? t('common.disabled') : t('common.enabled')}
                          </Badge>
                        </div>
                      </div>
                      <div className="flex items-center gap-1 shrink-0">
                        <Button
                          size="icon"
                          variant="ghost"
                          aria-label={`${t('common.edit')} ${u.username}`}
                          title={t('common.edit')}
                          onClick={() => openEdit(u)}
                        >
                          <Pencil className="h-4 w-4" />
                        </Button>
                        {currentUser?.id !== u.id && (
                          <Button
                            size="icon"
                            variant="ghost"
                            aria-label={`${t('common.delete')} ${u.username}`}
                            title={t('common.delete')}
                            className="text-red-700 hover:bg-red-600/12 dark:text-red-400 dark:hover:bg-red-500/12"
                            onClick={() => setDeleteTarget(u)}
                          >
                            <Trash2 className="h-4 w-4" />
                          </Button>
                        )}
                      </div>
                    </div>
                    <div className="text-xs text-muted-foreground space-y-1 tabular-nums">
                      <div className="flex flex-wrap items-center gap-1.5">
                        <span>{t('users.clusterScope')}:</span>
                        {u.role === 'admin' || !u.permissions?.length ? (
                          <span>{t('users.allClusters')}</span>
                        ) : (
                          u.permissions.map((p) => (
                            <Badge key={p.cluster_id} variant="secondary" className="font-mono">
                              {p.cluster_id} · {t(`users.roleShort.${p.role}`, p.role)}
                            </Badge>
                          ))
                        )}
                      </div>
                      <div>{formatDateTime(u.created_at)}</div>
                    </div>
                  </div>
                ))}
              </div>

              {/* Desktop table (>= md) */}
              <div className="hidden md:block overflow-x-auto">
                <table className="w-full text-sm">
                  <thead>
                    <tr className="border-b border-border text-left text-xs text-muted-foreground">
                      <th className="pb-3 pr-4 font-medium">{t('auth.username')}</th>
                      <th className="pb-3 pr-4 font-medium">{t('users.role')}</th>
                      <th className="pb-3 pr-4 font-medium">{t('users.clusterScope')}</th>
                      <th className="pb-3 pr-4 font-medium">{t('common.status')}</th>
                      <th className="pb-3 pr-4 font-medium">{t('backups.createdAt')}</th>
                      <th className="pb-3 text-right font-medium">{t('common.actions')}</th>
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border">
                    {users.map((u) => (
                      <tr key={u.id} className="hover:bg-secondary/30">
                        <td className="py-3 pr-4 font-medium">{u.username}</td>
                        <td className="py-3 pr-4">
                          <Badge variant={u.role === 'admin' ? 'default' : u.role === 'operator' ? 'info' : 'secondary'}>
                            {t(`users.roles.${u.role}`, u.role)}
                          </Badge>
                        </td>
                        <td className="py-3 pr-4 text-xs text-muted-foreground">
                          {u.role === 'admin' || !u.permissions?.length ? (
                            t('users.allClusters')
                          ) : (
                            <div className="flex flex-wrap gap-1.5">
                              {u.permissions.map((p) => (
                                <Badge key={p.cluster_id} variant="secondary" className="font-mono">
                                  {p.cluster_id} · {t(`users.roleShort.${p.role}`, p.role)}
                                </Badge>
                              ))}
                            </div>
                          )}
                        </td>
                        <td className="py-3 pr-4">
                          <Badge variant={u.disabled ? 'destructive' : 'success'}>
                            {u.disabled ? t('common.disabled') : t('common.enabled')}
                          </Badge>
                        </td>
                        <td className="py-3 pr-4 text-xs text-muted-foreground tabular-nums">
                          {formatDateTime(u.created_at)}
                        </td>
                        <td className="py-3 text-right whitespace-nowrap">
                          <div className="inline-flex items-center gap-1">
                            <Button
                              size="icon"
                              variant="ghost"
                              aria-label={`${t('common.edit')} ${u.username}`}
                              title={t('common.edit')}
                              onClick={() => openEdit(u)}
                            >
                              <Pencil className="h-4 w-4" />
                            </Button>
                            {currentUser?.id !== u.id && (
                              <Button
                                size="icon"
                                variant="ghost"
                                aria-label={`${t('common.delete')} ${u.username}`}
                                title={t('common.delete')}
                                className="text-red-700 hover:bg-red-600/12 dark:text-red-400 dark:hover:bg-red-500/12"
                                onClick={() => setDeleteTarget(u)}
                              >
                                <Trash2 className="h-4 w-4" />
                              </Button>
                            )}
                          </div>
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

      <Dialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        title={editing ? `${t('common.edit')}: ${editing.username}` : t('users.addUser')}
        footer={
          <div className="flex justify-end gap-2">
            <Button type="button" variant="outline" onClick={() => setDialogOpen(false)}>
              {t('common.cancel')}
            </Button>
            <Button type="submit" form="user-form" disabled={createMut.isPending || updateMut.isPending}>
              {t('common.save')}
            </Button>
          </div>
        }
      >
        <form id="user-form" onSubmit={handleSubmit} className="space-y-4">
          {(createMut.error || updateMut.error) && (
            <div
              role="alert"
              className="flex items-start gap-2 rounded-lg border border-red-600/35 bg-red-600/10 px-3.5 py-2.5 text-xs text-red-900 dark:border-red-500/35 dark:text-red-200"
            >
              <AlertTriangle className="h-4 w-4 text-red-700 dark:text-red-400 shrink-0 mt-0.5" />
              <span className="break-words">{(createMut.error || updateMut.error)?.message}</span>
            </div>
          )}
          {!editing && (
            <div className="space-y-1.5">
              <Label htmlFor="usr-name">{t('auth.username')}</Label>
              <Input
                id="usr-name"
                required
                minLength={3}
                value={username}
                onChange={(e) => setUsername(e.target.value)}
              />
            </div>
          )}
          <div className="space-y-1.5">
            <Label htmlFor="usr-pwd">{editing ? t('users.newPasswordOptional') : t('auth.password')}</Label>
            <SecretInput
              id="usr-pwd"
              required={!editing}
              minLength={6}
              value={password}
              onChange={(e) => setPassword(e.target.value)}
            />
          </div>
          <div className="space-y-1.5">
            <Label htmlFor="usr-role">{t('users.role')}</Label>
            <Select id="usr-role" value={role} onChange={(e) => setRole(e.target.value as Role)}>
              <option value="admin">{t('users.roles.admin')}</option>
              <option value="operator">{t('users.roles.operator')}</option>
              <option value="viewer">{t('users.roles.viewer')}</option>
            </Select>
          </div>

          {editing && currentUser?.id !== editing.id && (
            <label
              htmlFor="usr-disabled"
              className="flex items-center justify-between rounded-lg border border-border p-3 cursor-pointer"
            >
              <span className="text-sm">{t('users.disableAccount')}</span>
              <Switch
                id="usr-disabled"
                checked={disabled}
                onCheckedChange={setDisabled}
                ariaLabel={t('users.disableAccount')}
              />
            </label>
          )}

          {role !== 'admin' && clusters.length > 0 && (
            <div className="space-y-2 border-t border-border pt-4">
              <div className="text-xs font-semibold">{t('users.clusterScope')}</div>
              <p className="text-xs text-muted-foreground leading-relaxed">{t('users.clusterScopeHint')}</p>
              <div className="space-y-2 pt-1">
                {clusters.map((c) => (
                  <div key={c.cluster_id} className="flex items-center justify-between gap-2 text-xs">
                    <span className="font-mono truncate">{c.cluster_id}</span>
                    <Select
                      aria-label={`${c.cluster_id} ${t('users.role')}`}
                      className="w-36 h-10 sm:h-8 text-xs"
                      value={scopedPerms[c.cluster_id] ?? ''}
                      onChange={(e) =>
                        setScopedPerms((prev) => ({
                          ...prev,
                          [c.cluster_id]: e.target.value as Role | '',
                        }))
                      }
                    >
                      <option value="">—</option>
                      <option value="operator">{t('users.roles.operator')}</option>
                      <option value="viewer">{t('users.roles.viewer')}</option>
                    </Select>
                  </div>
                ))}
              </div>
            </div>
          )}
        </form>
      </Dialog>

      <ConfirmDialog
        open={!!deleteTarget}
        onClose={() => setDeleteTarget(null)}
        onConfirm={() => {
          if (deleteTarget) deleteMut.mutate(deleteTarget.id)
        }}
        title={t('users.deleteConfirmTitle')}
        description={t('users.deleteConfirm', { name: deleteTarget?.username ?? '' })}
        confirmLabel={t('common.delete')}
        tone="danger"
        loading={deleteMut.isPending}
      />
    </div>
  )
}
