import React, { useEffect, useRef, useState } from 'react'
import { Link, NavLink, Outlet, useLocation, useNavigate, useParams } from 'react-router-dom'
import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import {
  Activity,
  Archive,
  Bell,
  ChevronRight,
  Download,
  FolderOpen,
  Globe,
  LayoutDashboard,
  LogOut,
  Menu,
  Moon,
  Package,
  ScrollText,
  Search,
  Settings,
  Sliders,
  Sun,
  Terminal,
  UserCheck,
  Users,
  X,
} from 'lucide-react'
import { api } from '@/lib/api'
import { cn, getClusterHealth } from '@/lib/utils'
import { useAuth } from '@/hooks/useAuth'
import { useTheme } from '@/hooks/useTheme'
import { useUnsavedGuard } from '@/hooks/useUnsavedGuard'
import { setLanguage } from '@/i18n'
import { CollapsibleContent } from '@/components/ui/collapsible'
import { SeasonBadge, ShardStateBadge } from '@/components/StatusBadge'
import { CommandPalette } from '@/components/CommandPalette'
import { WalrusLogo } from '@/components/WalrusLogo'

const FOCUSABLE_SELECTOR =
  'a[href], button:not([disabled]), textarea:not([disabled]), input:not([disabled]), select:not([disabled]), [tabindex]:not([tabindex="-1"])'

export function AppShell() {
  const { t, i18n } = useTranslation()
  const { user, logout } = useAuth()
  const { theme, toggle: toggleTheme } = useTheme()
  const { guardAction } = useUnsavedGuard()
  const { id: activeClusterId } = useParams<{ id?: string }>()
  const location = useLocation()
  const navigate = useNavigate()

  const [mobileOpen, setMobileOpen] = useState(false)
  const [drawerMounted, setDrawerMounted] = useState(false)
  const [drawerEntered, setDrawerEntered] = useState(false)
  const [commandOpen, setCommandOpen] = useState(false)
  const [expandedClusterId, setExpandedClusterId] = useState<string | null>(activeClusterId ?? null)

  const drawerRef = useRef<HTMLElement | null>(null)
  const menuBtnRef = useRef<HTMLButtonElement | null>(null)

  useEffect(() => {
    if (mobileOpen) {
      setDrawerMounted(true)
      let raf2 = 0
      const raf1 = window.requestAnimationFrame(() => {
        raf2 = window.requestAnimationFrame(() => setDrawerEntered(true))
      })
      return () => {
        window.cancelAnimationFrame(raf1)
        window.cancelAnimationFrame(raf2)
      }
    }
    setEnteredSafe(false)
    const timer = window.setTimeout(() => setDrawerMounted(false), 260)
    return () => window.clearTimeout(timer)
  }, [mobileOpen])

  const setEnteredSafe = (v: boolean) => setDrawerEntered(v)

  useEffect(() => {
    if (activeClusterId) setExpandedClusterId(activeClusterId)
  }, [activeClusterId])

  // Global Ctrl/Cmd + K (and bare "/" outside text inputs) opens Command Palette
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault()
        setCommandOpen((v) => !v)
        return
      }
      if (e.key === '/' && !e.ctrlKey && !e.metaKey && !e.altKey) {
        const tag = (document.activeElement?.tagName ?? '').toLowerCase()
        const isEditable =
          tag === 'input' ||
          tag === 'textarea' ||
          tag === 'select' ||
          (document.activeElement as HTMLElement | null)?.isContentEditable
        if (!isEditable) {
          e.preventDefault()
          setCommandOpen(true)
        }
      }
    }
    window.addEventListener('keydown', onKey)
    return () => window.removeEventListener('keydown', onKey)
  }, [])

  // Accessible mobile drawer: scroll lock & restore focus
  useEffect(() => {
    if (!mobileOpen) return
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      document.body.style.overflow = prevOverflow
      menuBtnRef.current?.focus()
    }
  }, [mobileOpen])

  // Accessible mobile drawer: Escape close & focus trap
  useEffect(() => {
    if (!mobileOpen || !drawerMounted) return
    const timer = window.setTimeout(() => {
      const first = drawerRef.current?.querySelector<HTMLElement>(FOCUSABLE_SELECTOR)
      first?.focus()
    }, 0)

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        setMobileOpen(false)
        return
      }
      if (e.key === 'Tab' && drawerRef.current) {
        const focusables = Array.from(drawerRef.current.querySelectorAll<HTMLElement>(FOCUSABLE_SELECTOR)).filter(
          (el) => el.offsetParent !== null,
        )
        if (focusables.length === 0) return
        const first = focusables[0]
        const last = focusables[focusables.length - 1]
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault()
          last.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }

    window.addEventListener('keydown', onKey)
    return () => {
      window.clearTimeout(timer)
      window.removeEventListener('keydown', onKey)
    }
  }, [mobileOpen, drawerMounted])

  const { data: clustersData } = useQuery({
    queryKey: ['clusters'],
    queryFn: api.listClusters,
    refetchInterval: 5000,
  })
  const { data: steamData } = useQuery({
    queryKey: ['steam-status'],
    queryFn: api.steamStatus,
    refetchInterval: 30000,
  })
  const { data: versionData } = useQuery({
    queryKey: ['system-version'],
    queryFn: api.systemVersion,
    refetchInterval: 60000,
    retry: false,
  })
  const clusters = clustersData?.clusters ?? []
  const updateNeeded = steamData?.status?.update_needed ?? false
  const panelVersion = versionData?.info?.current_version
  const panelUpdateAvailable = versionData?.info?.update_available ?? false
  const activeCluster = clusters.find((c) => c.cluster_id === activeClusterId)

  const currentSubPage = (() => {
    const p = location.pathname
    if (p.endsWith('/console')) return 'console'
    if (p.endsWith('/config')) return 'config'
    if (p.endsWith('/mods')) return 'mods'
    if (p.endsWith('/players')) return 'players'
    if (p.endsWith('/backups')) return 'backups'
    if (p.endsWith('/files')) return 'files'
    return 'overview'
  })()

  const currentPageLabel = (() => {
    const p = location.pathname
    if (p === '/') return t('nav.dashboard')
    if (p.endsWith('/overview')) return t('nav.overview')
    if (p.endsWith('/console')) return t('nav.console')
    if (p.endsWith('/config')) return t('nav.config')
    if (p.endsWith('/mods')) return t('nav.mods')
    if (p.endsWith('/players')) return t('nav.players')
    if (p.endsWith('/backups')) return t('nav.backups')
    if (p.endsWith('/files')) return t('nav.files')
    if (p === '/steam') return t('nav.steam')
    if (p === '/webhooks') return t('nav.webhooks')
    if (p === '/users') return t('nav.users')
    if (p === '/settings') return t('nav.settings')
    if (p === '/audit') return t('nav.audit')
    return t('common.appName')
  })()

  const guardedNavigate = (to: string, e?: React.MouseEvent) => {
    if (e) {
      if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return
      e.preventDefault()
    }
    if (location.pathname === to) {
      setMobileOpen(false)
      return
    }
    guardAction(() => {
      setMobileOpen(false)
      navigate(to)
    })
  }

  const handleLogout = async () => {
    guardAction(async () => {
      await logout()
      navigate('/login', { replace: true })
    })
  }

  const toggleLang = () => {
    const next = i18n.language === 'en-US' ? 'zh-CN' : 'en-US'
    void setLanguage(next)
  }

  const focusRing =
    'focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-1 focus-visible:ring-offset-background'

  const navLinkClass = ({ isActive }: { isActive: boolean }) =>
    cn(
      'flex items-center gap-2.5 h-8 rounded-md px-2.5 text-[13px] transition-colors duration-150',
      focusRing,
      isActive
        ? 'bg-[var(--surface-hover)] text-foreground font-medium [&_svg]:text-foreground'
        : 'text-muted-foreground hover:bg-[var(--surface-hover)]/65 hover:text-foreground [&_svg]:text-muted-foreground',
    )

  const healthDot = (health: ReturnType<typeof getClusterHealth>) => {
    if (health === 'running') {
      return (
        <span className="relative inline-flex h-2 w-2 shrink-0 items-center justify-center" aria-hidden="true">
          <span className="absolute inline-flex h-full w-full rounded-full bg-emerald-500/45 dst-live-ring" />
          <span className="relative inline-flex h-[7px] w-[7px] rounded-full bg-emerald-600 dark:bg-emerald-500" />
        </span>
      )
    }
    const cls =
      health === 'crashed'
        ? 'bg-red-600 dark:bg-red-500'
        : health === 'degraded' || health === 'starting' || health === 'stopping'
          ? 'bg-amber-500'
          : 'bg-muted-foreground/45'
    return <span className={cn('h-[7px] w-[7px] shrink-0 rounded-full', cls)} aria-hidden="true" />
  }

  const sidebarContent = (
    <div className="flex h-full flex-col px-3 py-4 gap-3.5">
      {/* Brand Header */}
      <div className="flex items-center justify-between px-2 shrink-0">
        <Link
          to="/"
          onClick={(e) => guardedNavigate('/', e)}
          className={cn('flex items-center gap-2.5 min-w-0 rounded-md group', focusRing)}
        >
          <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-[7px] bg-foreground text-background">
            <WalrusLogo className="h-[18px] w-[18px]" />
          </div>
          <span className="text-[13.5px] font-semibold tracking-[-0.015em] text-foreground truncate leading-tight">
            {t('common.appName')}
          </span>
          {panelVersion && (
            <span
              title={versionData?.info?.commit && versionData.info.commit !== 'none' ? `${panelVersion} (${versionData.info.commit})` : panelVersion}
              className="rounded bg-[var(--surface-hover)] px-1.5 py-0.5 font-mono text-[10.5px] leading-none text-muted-foreground shrink-0"
            >
              {panelVersion}
            </span>
          )}
        </Link>
        <button
          type="button"
          onClick={() => setMobileOpen(false)}
          aria-label={t('common.closeNav')}
          title={t('common.closeNav')}
          className={cn(
            'flex h-8 w-8 items-center justify-center rounded-md text-muted-foreground hover:bg-[var(--surface-hover)] hover:text-foreground lg:hidden cursor-pointer pressable',
            focusRing,
          )}
        >
          <X className="h-4 w-4" />
        </button>
      </div>

      {/* Command Search Trigger */}
      <button
        type="button"
        onClick={() => {
          setMobileOpen(false)
          setCommandOpen(true)
        }}
        className={cn(
          'flex w-full h-[34px] items-center justify-between gap-2 rounded-lg border border-border bg-card px-2.5 text-[12.5px] text-muted-foreground hover:border-[var(--border-strong)] hover:text-foreground transition-colors cursor-pointer pressable',
          focusRing,
        )}
      >
        <span className="flex items-center gap-2 min-w-0">
          <Search className="h-3.5 w-3.5 shrink-0 text-muted-foreground" />
          <span className="truncate">{t('nav.openCommand')}</span>
        </span>
        <kbd className="inline-flex items-center rounded border border-[var(--border-strong)] px-1.5 py-0.5 font-mono text-[10.5px] leading-none text-muted-foreground shrink-0">
          Ctrl K
        </kbd>
      </button>

      {/* Main Nav */}
      <nav aria-label={t('common.appName')} className="flex-1 overflow-y-auto -mx-1 px-1 space-y-4">
        <div className="space-y-0.5">
          <NavLink to="/" end className={navLinkClass} onClick={(e) => guardedNavigate('/', e)}>
            <LayoutDashboard className="h-4 w-4 shrink-0" />
            <span>{t('nav.dashboard')}</span>
          </NavLink>
        </div>

        {/* Clusters Switcher + Grouped Sub-Nav */}
        <div className="space-y-1">
          <div className="flex items-center justify-between px-2.5 pt-1 pb-0.5">
            <span className="text-[11.5px] font-medium text-muted-foreground">{t('nav.clusters')}</span>
            <span className="font-mono text-[11px] tabular-nums text-muted-foreground/80">{clusters.length}</span>
          </div>
          {clusters.length === 0 ? (
            <p className="px-2.5 py-1 text-xs text-muted-foreground/80">{t('common.noData')}</p>
          ) : (
            <div className="space-y-0.5">
              {clusters.map((c) => {
                const health = getClusterHealth(c.master_status, c.caves_enabled, c.caves_status)
                const isSelected = activeClusterId === c.cluster_id
                const isExpanded = expandedClusterId === c.cluster_id
                const players = c.world_info?.online_players?.length ?? 0
                const base = `/clusters/${encodeURIComponent(c.cluster_id)}`
                return (
                  <div key={c.cluster_id}>
                    <div
                      className={cn(
                        'flex items-center justify-between rounded-md text-[13px] transition-colors',
                        isSelected
                          ? 'bg-[var(--surface-hover)] text-foreground font-medium'
                          : 'text-muted-foreground hover:bg-[var(--surface-hover)]/65 hover:text-foreground',
                      )}
                    >
                      <Link
                        to={`${base}/overview`}
                        onClick={(e) => {
                          setExpandedClusterId(c.cluster_id)
                          guardedNavigate(`${base}/overview`, e)
                        }}
                        className={cn('flex flex-1 items-center gap-2.5 min-w-0 h-8 px-2.5 rounded-l-md', focusRing)}
                      >
                        <span className="inline-flex w-4 items-center justify-center shrink-0">
                          {healthDot(health)}
                        </span>
                        <span className="sr-only">{t(`common.${health}`)}</span>
                        <span className="truncate flex-1">{c.cluster_name || c.cluster_id}</span>
                        <span className="font-mono text-[11.5px] tabular-nums text-muted-foreground shrink-0">
                          {health === 'running' ? `${players}/${c.max_players}` : t(`common.${health}`)}
                        </span>
                      </Link>
                      <button
                        type="button"
                        aria-expanded={isExpanded}
                        aria-label={t('common.expandClusterNamed', { name: c.cluster_name || c.cluster_id })}
                        onClick={() => setExpandedClusterId(isExpanded ? null : c.cluster_id)}
                        className={cn(
                          'flex h-8 w-7 shrink-0 items-center justify-center rounded-r-md text-muted-foreground hover:text-foreground cursor-pointer pressable',
                          focusRing,
                        )}
                      >
                        <ChevronRight
                          className={cn(
                            'h-3.5 w-3.5 [transition:transform_180ms_var(--ease-out),rotate_180ms_var(--ease-out),color_150ms_ease] motion-reduce:[transition:color_150ms_ease]',
                            isExpanded && 'rotate-90 text-foreground',
                          )}
                        />
                      </button>
                    </div>

                    <CollapsibleContent open={isExpanded}>
                      <div className="mt-0.5 ml-4 space-y-2 border-l border-border pl-2.5 py-1">
                        <div className="space-y-0.5">
                          <NavLink
                            to={`${base}/overview`}
                            className={navLinkClass}
                            onClick={(e) => guardedNavigate(`${base}/overview`, e)}
                          >
                            <Activity className="h-3.5 w-3.5 shrink-0" />
                            <span>{t('nav.overview')}</span>
                          </NavLink>
                          <NavLink
                            to={`${base}/console`}
                            className={navLinkClass}
                            onClick={(e) => guardedNavigate(`${base}/console`, e)}
                          >
                            <Terminal className="h-3.5 w-3.5 shrink-0" />
                            <span>{t('nav.console')}</span>
                          </NavLink>
                          <NavLink
                            to={`${base}/players`}
                            className={navLinkClass}
                            onClick={(e) => guardedNavigate(`${base}/players`, e)}
                          >
                            <UserCheck className="h-3.5 w-3.5 shrink-0" />
                            <span>{t('nav.players')}</span>
                          </NavLink>
                        </div>

                        <div className="space-y-0.5 pt-1 border-t border-border/60">
                          <NavLink
                            to={`${base}/config`}
                            className={navLinkClass}
                            onClick={(e) => guardedNavigate(`${base}/config`, e)}
                          >
                            <Sliders className="h-3.5 w-3.5 shrink-0" />
                            <span>{t('nav.config')}</span>
                          </NavLink>
                          <NavLink
                            to={`${base}/mods`}
                            className={navLinkClass}
                            onClick={(e) => guardedNavigate(`${base}/mods`, e)}
                          >
                            <Package className="h-3.5 w-3.5 shrink-0" />
                            <span>{t('nav.mods')}</span>
                          </NavLink>
                          <NavLink
                            to={`${base}/backups`}
                            className={navLinkClass}
                            onClick={(e) => guardedNavigate(`${base}/backups`, e)}
                          >
                            <Archive className="h-3.5 w-3.5 shrink-0" />
                            <span>{t('nav.backups')}</span>
                          </NavLink>
                          <NavLink
                            to={`${base}/files`}
                            className={navLinkClass}
                            onClick={(e) => guardedNavigate(`${base}/files`, e)}
                          >
                            <FolderOpen className="h-3.5 w-3.5 shrink-0" />
                            <span>{t('nav.files')}</span>
                          </NavLink>
                        </div>
                      </div>
                    </CollapsibleContent>
                  </div>
                )
              })}
            </div>
          )}
        </div>

        {/* Admin System Nav */}
        {user?.role === 'admin' && (
          <div className="space-y-0.5">
            <div className="px-2.5 pt-1 pb-1 text-[11.5px] font-medium text-muted-foreground">{t('nav.system')}</div>
            <NavLink to="/steam" className={navLinkClass} onClick={(e) => guardedNavigate('/steam', e)}>
              <Download className="h-4 w-4 shrink-0" />
              <span className="flex-1 truncate">{t('nav.steam')}</span>
              {updateNeeded && (
                <span className="text-[11px] font-medium text-amber-700 dark:text-amber-400 shrink-0">
                  {t('nav.steamUpdateBadge')}
                </span>
              )}
            </NavLink>
            <NavLink to="/webhooks" className={navLinkClass} onClick={(e) => guardedNavigate('/webhooks', e)}>
              <Bell className="h-4 w-4 shrink-0" />
              <span>{t('nav.webhooks')}</span>
            </NavLink>
            <NavLink to="/users" className={navLinkClass} onClick={(e) => guardedNavigate('/users', e)}>
              <Users className="h-4 w-4 shrink-0" />
              <span>{t('nav.users')}</span>
            </NavLink>
            <NavLink to="/settings" className={navLinkClass} onClick={(e) => guardedNavigate('/settings', e)}>
              <Settings className="h-4 w-4 shrink-0" />
              <span className="flex-1 truncate">{t('nav.settings')}</span>
              {panelUpdateAvailable && (
                <span className="text-[11px] font-medium text-amber-700 dark:text-amber-400 shrink-0">
                  {t('nav.panelUpdateBadge')}
                </span>
              )}
            </NavLink>
            <NavLink to="/audit" className={navLinkClass} onClick={(e) => guardedNavigate('/audit', e)}>
              <ScrollText className="h-4 w-4 shrink-0" />
              <span>{t('nav.audit')}</span>
            </NavLink>
          </div>
        )}
      </nav>

      {/* Compact Footer User & Utilities */}
      <div className="border-t border-border pt-3 px-1 flex items-center gap-2 shrink-0">
        <div className="flex h-7 w-7 shrink-0 items-center justify-center rounded-full bg-[var(--surface-hover)] text-[11px] font-semibold text-foreground uppercase">
          {user?.username?.slice(0, 1) || 'U'}
        </div>
        <div className="min-w-0 flex-1">
          <div className="truncate text-[12.5px] font-medium text-foreground leading-tight">{user?.username}</div>
          <div className="text-[11px] text-muted-foreground truncate leading-tight mt-0.5">
            {user?.role ? t(`users.roles.${user.role}`, user.role) : ''}
          </div>
        </div>
        <div className="flex items-center gap-0.5 shrink-0">
          <button
            type="button"
            onClick={toggleLang}
            aria-label={t('common.switchLang')}
            title={i18n.language === 'en-US' ? 'Switch to 中文' : 'Switch to English'}
            className={cn(
              'flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-[var(--surface-hover)] hover:text-foreground cursor-pointer pressable',
              focusRing,
            )}
          >
            <Globe className="h-3.5 w-3.5" />
          </button>
          <button
            type="button"
            onClick={toggleTheme}
            aria-label={t('common.toggleTheme')}
            title={t('common.toggleTheme')}
            className={cn(
              'flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-[var(--surface-hover)] hover:text-foreground cursor-pointer pressable',
              focusRing,
            )}
          >
            {theme === 'dark' ? <Sun className="h-3.5 w-3.5" /> : <Moon className="h-3.5 w-3.5" />}
          </button>
          <span className="h-3.5 w-px bg-border mx-0.5" aria-hidden="true" />
          <button
            type="button"
            onClick={() => void handleLogout()}
            aria-label={t('nav.logout')}
            title={t('nav.logout')}
            className={cn(
              'flex h-7 w-7 items-center justify-center rounded-md text-muted-foreground hover:bg-red-500/12 hover:text-red-700 dark:hover:text-red-400 cursor-pointer pressable',
              focusRing,
            )}
          >
            <LogOut className="h-3.5 w-3.5" />
          </button>
        </div>
      </div>
    </div>
  )

  return (
    <div className="min-h-screen bg-background text-foreground flex">
      {/* Skip-to-content accessibility link */}
      <a
        href="#main-content"
        className="sr-only focus:not-sr-only focus:fixed focus:top-3 focus:left-3 focus:z-50 focus:rounded-lg focus:bg-primary focus:px-3.5 py-2 focus:text-xs focus:font-semibold focus:text-primary-foreground focus:shadow-lg"
      >
        {t('common.skipToContent')}
      </a>

      {/* Desktop Sidebar */}
      <aside className="hidden lg:flex lg:w-[244px] lg:flex-col lg:fixed lg:inset-y-0 border-r border-border bg-[var(--surface-sidebar)]">
        {sidebarContent}
      </aside>

      {/* Accessible Mobile Drawer */}
      {drawerMounted && (
        <div className={cn('fixed inset-0 z-50 lg:hidden flex', !drawerEntered && 'pointer-events-none')}>
          <div
            className={cn(
              'fixed inset-0 bg-black/60 backdrop-blur-[2px] transition-opacity duration-[220ms]',
              drawerEntered ? 'opacity-100' : 'opacity-0',
            )}
            onClick={() => setMobileOpen(false)}
            aria-hidden="true"
          />
          <aside
            ref={drawerRef}
            role="dialog"
            aria-modal="true"
            aria-label={t('common.appName')}
            className={cn(
              'relative z-10 w-72 max-w-[85vw] bg-[var(--surface-sidebar)] border-r border-border shadow-2xl [transition:translate_240ms_var(--ease-drawer),transform_240ms_var(--ease-drawer),opacity_180ms_ease] motion-reduce:[transition:opacity_180ms_ease]',
              drawerEntered ? 'translate-x-0 opacity-100' : 'motion-safe:-translate-x-full motion-reduce:opacity-0',
            )}
          >
            {sidebarContent}
          </aside>
        </div>
      )}

      {/* Main Content Area */}
      <div className="flex-1 flex flex-col lg:pl-[244px] min-w-0">
        {/* Mobile Top Bar */}
        <header className="sticky top-0 z-30 flex h-13 items-center justify-between gap-2 border-b border-border bg-background/95 backdrop-blur-sm px-2.5 lg:hidden">
          <div className="flex items-center gap-1.5 min-w-0">
            <button
              ref={menuBtnRef}
              type="button"
              onClick={() => setMobileOpen(true)}
              aria-label={t('common.openNav')}
              title={t('common.openNav')}
              className={cn(
                'flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-[var(--surface-hover)] hover:text-foreground cursor-pointer pressable',
                focusRing,
              )}
            >
              <Menu className="h-4.5 w-4.5" />
            </button>
            <div className="flex items-center gap-2 min-w-0 text-[13.5px] font-semibold tracking-[-0.015em]">
              <span className="truncate">{currentPageLabel}</span>
            </div>
          </div>
          <div className="flex items-center gap-1 shrink-0">
            <button
              type="button"
              onClick={() => setCommandOpen(true)}
              aria-label={t('nav.openCommand')}
              title={t('nav.openCommand')}
              className={cn(
                'flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-[var(--surface-hover)] hover:text-foreground cursor-pointer pressable',
                focusRing,
              )}
            >
              <Search className="h-4 w-4" />
            </button>
            {updateNeeded && user?.role === 'admin' && (
              <Link
                to="/steam"
                onClick={(e) => guardedNavigate('/steam', e)}
                className={cn(
                  'inline-flex h-8 items-center rounded-md px-2 text-xs font-medium text-amber-700 dark:text-amber-400 hover:bg-amber-500/10',
                  focusRing,
                )}
              >
                {t('nav.steamUpdateBadge')}
              </Link>
            )}
            <button
              type="button"
              onClick={toggleTheme}
              aria-label={t('common.toggleTheme')}
              title={t('common.toggleTheme')}
              className={cn(
                'flex h-9 w-9 shrink-0 items-center justify-center rounded-md text-muted-foreground hover:bg-[var(--surface-hover)] hover:text-foreground cursor-pointer pressable',
                focusRing,
              )}
            >
              {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
            </button>
          </div>
        </header>

        {/* Mobile Compact Cluster Context Strip (< lg) */}
        {activeCluster && (
          <div className="flex lg:hidden items-center justify-between gap-2 border-b border-border bg-card px-3 py-1.5 text-xs">
            <div className="flex items-center gap-1.5 min-w-0 flex-1">
              {clusters.length > 1 ? (
                <select
                  aria-label={t('nav.selectCluster')}
                  value={activeCluster.cluster_id}
                  onChange={(e) => {
                    const nextId = e.target.value
                    if (nextId && nextId !== activeCluster.cluster_id) {
                      guardedNavigate(`/clusters/${encodeURIComponent(nextId)}/${currentSubPage}`)
                    }
                  }}
                  className={cn(
                    'h-7 max-w-[180px] truncate rounded-md border border-border bg-background px-2 py-0.5 text-xs font-medium text-foreground cursor-pointer',
                    focusRing,
                  )}
                >
                  {clusters.map((c) => (
                    <option key={c.cluster_id} value={c.cluster_id}>
                      {c.cluster_name || c.cluster_id}
                    </option>
                  ))}
                </select>
              ) : (
                <span className="font-medium text-foreground truncate">
                  {activeCluster.cluster_name || activeCluster.cluster_id}
                </span>
              )}
            </div>
            <div className="flex items-center gap-3 shrink-0" aria-live="polite">
              <SeasonBadge
                season={activeCluster.world_info?.season}
                day={activeCluster.world_info?.day}
                phase={activeCluster.world_info?.phase}
                size="sm"
              />
              <ShardStateBadge
                state={getClusterHealth(
                  activeCluster.master_status,
                  activeCluster.caves_enabled,
                  activeCluster.caves_status,
                )}
              />
            </div>
          </div>
        )}

        {/* Desktop Cluster Context Bar (>= lg, only shown inside a cluster to avoid duplicating top-level page headers) */}
        {activeCluster && (
          <header className="hidden lg:flex sticky top-0 z-30 h-12 items-center justify-between gap-4 border-b border-border bg-background/90 backdrop-blur-sm px-10 text-xs">
            <div className="flex items-center gap-2 min-w-0">
              <Link
                to="/"
                onClick={(e) => guardedNavigate('/', e)}
                className={cn('text-muted-foreground hover:text-foreground transition-colors rounded px-1 py-0.5', focusRing)}
              >
                {t('nav.dashboard')}
              </Link>
              <ChevronRight className="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" aria-hidden="true" />
              {clusters.length > 1 ? (
                <select
                  aria-label={t('nav.selectCluster')}
                  title={t('nav.switchClusterHint')}
                  value={activeCluster.cluster_id}
                  onChange={(e) => {
                    const nextId = e.target.value
                    if (nextId && nextId !== activeCluster.cluster_id) {
                      guardedNavigate(`/clusters/${encodeURIComponent(nextId)}/${currentSubPage}`)
                    }
                  }}
                  className={cn(
                    'h-7 rounded-md border border-border bg-card px-2 py-0.5 text-xs font-medium text-foreground hover:border-[var(--border-strong)] cursor-pointer transition-colors',
                    focusRing,
                  )}
                >
                  {clusters.map((c) => (
                    <option key={c.cluster_id} value={c.cluster_id}>
                      {c.cluster_name || c.cluster_id}
                    </option>
                  ))}
                </select>
              ) : (
                <span className="font-medium text-foreground truncate">
                  {activeCluster.cluster_name || activeCluster.cluster_id}
                </span>
              )}
              <ChevronRight className="h-3.5 w-3.5 text-muted-foreground/60 shrink-0" aria-hidden="true" />
              <span className="text-muted-foreground truncate">{currentPageLabel}</span>
            </div>

            <div className="flex items-center gap-4 shrink-0" aria-live="polite">
              <SeasonBadge
                season={activeCluster.world_info?.season}
                day={activeCluster.world_info?.day}
                phase={activeCluster.world_info?.phase}
                size="md"
              />
              <ShardStateBadge
                state={getClusterHealth(
                  activeCluster.master_status,
                  activeCluster.caves_enabled,
                  activeCluster.caves_status,
                )}
              />
              {currentSubPage !== 'console' && (
                <Link
                  to={`/clusters/${encodeURIComponent(activeCluster.cluster_id)}/console`}
                  onClick={(e) =>
                    guardedNavigate(`/clusters/${encodeURIComponent(activeCluster.cluster_id)}/console`, e)
                  }
                  title={t('nav.console')}
                  className={cn(
                    'inline-flex h-7 items-center gap-1.5 rounded-md border border-border bg-card px-2.5 text-xs font-medium text-muted-foreground hover:border-[var(--border-strong)] hover:text-foreground transition-colors',
                    focusRing,
                  )}
                >
                  <Terminal className="h-3.5 w-3.5" />
                  <span>{t('nav.console')}</span>
                </Link>
              )}
            </div>
          </header>
        )}

        <main
          id="main-content"
          tabIndex={-1}
          className="flex-1 px-4 py-5 sm:px-7 sm:py-6 lg:px-10 lg:py-7 max-w-[1236px] w-full mx-auto focus:outline-none"
        >
          <div key={location.pathname} className="dst-page-enter">
            <Outlet />
          </div>
        </main>
      </div>

      {/* Global Command Palette */}
      <CommandPalette
        open={commandOpen}
        onClose={() => setCommandOpen(false)}
        clusters={clusters}
        activeClusterId={activeClusterId}
        currentSubPage={currentSubPage}
        updateNeeded={updateNeeded}
        onNavigate={(to) => guardedNavigate(to)}
      />
    </div>
  )
}
