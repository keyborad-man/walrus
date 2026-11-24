import React, { useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import {
  AlertTriangle,
  ChevronDown,
  Download,
  Globe,
  HelpCircle,
  Moon,
  ShieldCheck,
  Sun,
  Trees,
} from 'lucide-react'
import { api, formatError } from '@/lib/api'
import { cn } from '@/lib/utils'
import { useAuth } from '@/hooks/useAuth'
import { useTheme } from '@/hooks/useTheme'
import { setLanguage } from '@/i18n'
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '@/components/ui/card'
import { Button } from '@/components/ui/button'
import { Input, Label, SecretInput } from '@/components/ui/input'
import { CollapsibleContent } from '@/components/ui/collapsible'
import { DSTClockHeroDial } from '@/components/StatusBadge'
import { WalrusLogo } from '@/components/WalrusLogo'

export function SetupPage() {
  const { t, i18n } = useTranslation()
  const { refresh } = useAuth()
  const { theme, toggle: toggleTheme } = useTheme()
  const navigate = useNavigate()

  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [confirmPwd, setConfirmPwd] = useState('')
  const [token, setToken] = useState('')
  const [showTokenGuide, setShowTokenGuide] = useState(false)
  const [submitError, setSubmitError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  const passwordMismatch = confirmPwd.length > 0 && password !== confirmPwd

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitError(null)
    if (password !== confirmPwd) {
      setSubmitError({ kind: 'passwordMismatch' })
      return
    }
    setBusy(true)
    try {
      await api.setup({
        username: username.trim(),
        password,
        default_cluster_token: token.trim(),
        locale: i18n.language,
      })
      await refresh()
      toast.success(t('auth.setupWelcome'))
      navigate('/', { replace: true })
    } catch (err) {
      setSubmitError(err)
      toast.error(formatError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main className="min-h-screen flex items-center justify-center bg-background dst-campfire-bg p-4 sm:p-6 relative">
      {/* Top-right compact utility toggle */}
      <div className="absolute top-4 right-4 flex items-center gap-0.5 rounded-lg border border-border bg-card p-1">
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 rounded-md"
          aria-label={t('common.switchLang')}
          title={t('common.switchLang')}
          onClick={() => void setLanguage(i18n.language === 'en-US' ? 'zh-CN' : 'en-US')}
        >
          <Globe className="h-3.5 w-3.5" />
        </Button>
        <Button
          variant="ghost"
          size="icon"
          className="h-7 w-7 rounded-md"
          aria-label={t('common.toggleTheme')}
          title={t('common.toggleTheme')}
          onClick={toggleTheme}
        >
          {theme === 'dark' ? <Sun className="h-3.5 w-3.5" /> : <Moon className="h-3.5 w-3.5" />}
        </Button>
      </div>

      <div className="w-full max-w-4xl grid grid-cols-1 lg:grid-cols-12 gap-5 items-stretch">
        {/* Three-Step Launch Roadmap */}
        <section className="hidden lg:flex lg:col-span-5 flex-col justify-between space-y-6 rounded-[10px] surface-card p-6">
          <div className="space-y-6">
            <div className="flex items-center gap-2.5">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-foreground text-background">
                <WalrusLogo className="h-5 w-5" />
              </div>
              <div>
                <div className="text-[15px] font-semibold tracking-[-0.015em] text-foreground">
                  {t('common.appName')}
                </div>
                <div className="text-xs text-muted-foreground">{t('auth.campfireTagline')}</div>
              </div>
            </div>

            <div className="flex items-center gap-4 rounded-lg border border-border bg-[var(--surface-elevated)]/55 p-3.5">
              <DSTClockHeroDial day={1} phase="day" season="autumn" className="h-15 w-15 shrink-0" />
              <div className="space-y-0.5 min-w-0">
                <div className="text-[11px] font-mono text-muted-foreground">SETUP</div>
                <div className="text-[13.5px] font-semibold text-foreground text-pretty">{t('auth.setupStepsTitle')}</div>
              </div>
            </div>

            <ol className="space-y-3.5 pt-1">
              <li className="flex items-start gap-3 text-xs">
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md bg-foreground text-background font-mono text-[11px] font-semibold">
                  01
                </span>
                <span className="pt-0.5 font-medium text-foreground leading-relaxed text-pretty">
                  {t('auth.setupStep1')}
                </span>
              </li>
              <li className="flex items-start gap-3 text-xs">
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md border border-border bg-[var(--surface-elevated)] font-mono text-[11px] font-medium text-muted-foreground">
                  02
                </span>
                <span className="pt-0.5 text-muted-foreground leading-relaxed text-pretty">
                  <Download className="inline h-3.5 w-3.5 mr-1 -mt-0.5 text-foreground/75" />
                  {t('auth.setupStep2')}
                </span>
              </li>
              <li className="flex items-start gap-3 text-xs">
                <span className="flex h-6 w-6 shrink-0 items-center justify-center rounded-md border border-border bg-[var(--surface-elevated)] font-mono text-[11px] font-medium text-muted-foreground">
                  03
                </span>
                <span className="pt-0.5 text-muted-foreground leading-relaxed text-pretty">
                  <Trees className="inline h-3.5 w-3.5 mr-1 -mt-0.5 text-foreground/75" />
                  {t('auth.setupStep3')}
                </span>
              </li>
            </ol>
          </div>

          <div className="pt-4 border-t border-border text-xs text-muted-foreground leading-relaxed text-pretty">
            {t('common.appSubtitle')}
          </div>
        </section>

        {/* Initial Setup Form Card */}
        <Card className="lg:col-span-7 w-full max-w-md lg:max-w-none mx-auto">
          <CardHeader className="space-y-3 pb-3">
            <div className="flex items-center gap-2.5 lg:hidden">
              <div className="flex h-8 w-8 shrink-0 items-center justify-center rounded-lg bg-foreground text-background">
                <WalrusLogo className="h-5 w-5" />
              </div>
              <div>
                <div className="text-sm font-semibold tracking-[-0.015em] text-foreground">{t('common.appName')}</div>
                <div className="text-[11px] text-muted-foreground">{t('auth.setupStepsTitle')}</div>
              </div>
            </div>
            <div>
              <CardTitle as="h1" className="text-[19px] font-semibold tracking-[-0.02em]">
                {t('auth.setupTitle')}
              </CardTitle>
              <CardDescription className="mt-1 text-pretty">{t('auth.setupSubtitle')}</CardDescription>
            </div>
            {/* Compact Mobile Step Progress Strip */}
            <div className="grid grid-cols-3 gap-1.5 pt-1 lg:hidden">
              <div className="rounded-md border border-[var(--border-strong)] bg-[var(--surface-hover)] px-2 py-1.5 text-[11px] font-medium text-foreground truncate">
                01 · {t('auth.username')}
              </div>
              <div className="rounded-md border border-border bg-[var(--surface-elevated)]/60 px-2 py-1.5 text-[11px] text-muted-foreground truncate">
                02 · SteamCMD
              </div>
              <div className="rounded-md border border-border bg-[var(--surface-elevated)]/60 px-2 py-1.5 text-[11px] text-muted-foreground truncate">
                03 · DST
              </div>
            </div>
          </CardHeader>
          <CardContent className="pt-2">
            <form onSubmit={handleSubmit} className="space-y-4">
              {Boolean(submitError) && (
                <div
                  role="alert"
                  className="flex items-start gap-2 rounded-lg border border-red-500/30 bg-red-500/8 px-3 py-2.5 text-xs text-red-700 dark:text-red-300"
                >
                  <AlertTriangle className="h-4 w-4 text-red-600 dark:text-red-400 shrink-0 mt-0.5" />
                  <span className="break-words">
                    {typeof submitError === 'object' && submitError !== null && 'kind' in submitError
                      ? t('auth.passwordMismatch')
                      : formatError(submitError)}
                  </span>
                </div>
              )}

              <div className="space-y-1.5">
                <Label htmlFor="setup-user">{t('auth.username')}</Label>
                <Input
                  id="setup-user"
                  required
                  minLength={3}
                  autoComplete="username"
                  value={username}
                  onChange={(e) => setUsername(e.target.value)}
                />
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-4">
                <div className="space-y-1.5">
                  <div className="flex items-center justify-between gap-2">
                    <Label htmlFor="setup-pwd">{t('auth.password')}</Label>
                    <span className="text-[11px] text-muted-foreground">{t('auth.passwordMinHint')}</span>
                  </div>
                  <SecretInput
                    id="setup-pwd"
                    required
                    minLength={6}
                    autoComplete="new-password"
                    value={password}
                    onChange={(e) => setPassword(e.target.value)}
                  />
                </div>
                <div className="space-y-1.5">
                  <Label htmlFor="setup-pwd2">{t('auth.confirmPassword')}</Label>
                  <SecretInput
                    id="setup-pwd2"
                    required
                    minLength={6}
                    autoComplete="new-password"
                    aria-invalid={passwordMismatch}
                    className={cn(passwordMismatch && 'border-red-600 focus-visible:ring-red-600')}
                    value={confirmPwd}
                    onChange={(e) => setConfirmPwd(e.target.value)}
                  />
                </div>
              </div>
              {passwordMismatch && (
                <p role="alert" className="text-xs font-medium text-red-700 dark:text-red-400 -mt-2">
                  {t('auth.passwordMismatch')}
                </p>
              )}

              <div className="space-y-1.5">
                <Label htmlFor="setup-tok">{t('auth.defaultToken')}</Label>
                <SecretInput
                  id="setup-tok"
                  autoComplete="off"
                  placeholder="pds-g^KU_..."
                  value={token}
                  onChange={(e) => setToken(e.target.value)}
                />
                <p className="text-xs text-muted-foreground leading-relaxed text-pretty">
                  {t('auth.defaultTokenHint')}
                </p>
                <div>
                  <button
                    type="button"
                    aria-expanded={showTokenGuide}
                    onClick={() => setShowTokenGuide((v) => !v)}
                    className="inline-flex items-center gap-1 rounded text-xs font-medium text-foreground hover:underline transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring cursor-pointer"
                  >
                    <HelpCircle className="h-3.5 w-3.5 text-muted-foreground" />
                    <span>{t('auth.tokenGuideToggle')}</span>
                    <ChevronDown
                      className={cn(
                        'h-3.5 w-3.5 text-muted-foreground transition-transform duration-200 ease-out',
                        showTokenGuide && 'rotate-180',
                      )}
                    />
                  </button>
                  <CollapsibleContent open={showTokenGuide}>
                    <div className="pt-1.5">
                      <div className="rounded-lg border border-border bg-[var(--surface-elevated)] px-3 py-2.5 text-xs text-muted-foreground leading-relaxed text-pretty">
                        {t('auth.tokenGuideSteps')}
                      </div>
                    </div>
                  </CollapsibleContent>
                </div>
              </div>

              <Button type="submit" className="w-full h-9" disabled={busy || passwordMismatch}>
                <ShieldCheck className="h-3.5 w-3.5" />
                {busy ? t('common.saving') : t('auth.initBtn')}
              </Button>
            </form>
          </CardContent>
        </Card>
      </div>
    </main>
  )
}
