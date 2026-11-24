import React, { useRef, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { AlertTriangle, ArrowRight, Globe, Loader2, Moon, Sun } from 'lucide-react'
import { api, formatError } from '@/lib/api'
import { useAuth } from '@/hooks/useAuth'
import { useTheme } from '@/hooks/useTheme'
import { setLanguage } from '@/i18n'
import { Button } from '@/components/ui/button'
import { Input, Label, SecretInput } from '@/components/ui/input'
import { AuthCampfireBackdrop } from '@/components/AuthCampfireBackdrop'
import { WalrusLogo } from '@/components/WalrusLogo'

export function LoginPage() {
  const { t, i18n } = useTranslation()
  const { refresh } = useAuth()
  const { theme, toggle: toggleTheme } = useTheme()
  const navigate = useNavigate()

  const stageRef = useRef<HTMLElement | null>(null)
  const formRef = useRef<HTMLFormElement | null>(null)

  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [submitError, setSubmitError] = useState<unknown>(null)
  const [busy, setBusy] = useState(false)

  const triggerShake = () => {
    const form = formRef.current
    if (!form) return
    form.classList.remove('auth-form-shake')
    void form.offsetWidth
    form.classList.add('auth-form-shake')
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSubmitError(null)
    setBusy(true)
    try {
      await api.login({ username: username.trim(), password })
      await refresh()
      toast.success(t('auth.welcomeBack'))
      navigate('/', { replace: true })
    } catch (err) {
      setSubmitError(err)
      triggerShake()
      toast.error(formatError(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <main
      ref={stageRef}
      className="min-h-screen flex flex-col bg-background text-foreground relative overflow-hidden"
    >
      <AuthCampfireBackdrop containerRef={stageRef} />

      {/* Top-right bare utility controls */}
      <header className="relative z-10 flex items-center justify-end px-5 sm:px-6 pt-5">
        <div className="flex items-center gap-1">
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8 rounded-md text-muted-foreground hover:text-foreground"
            aria-label={t('common.switchLang')}
            title={t('common.switchLang')}
            onClick={() => void setLanguage(i18n.language === 'en-US' ? 'zh-CN' : 'en-US')}
          >
            <Globe className="h-4 w-4" />
          </Button>
          <Button
            variant="ghost"
            size="icon"
            className="h-8 w-8 rounded-md text-muted-foreground hover:text-foreground"
            aria-label={t('common.toggleTheme')}
            title={t('common.toggleTheme')}
            onClick={toggleTheme}
          >
            {theme === 'dark' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
          </Button>
        </div>
      </header>

      {/* Focused cardless single column */}
      <section className="relative z-10 flex-1 flex items-center justify-center px-5 sm:px-6 py-10">
        <div className="w-full max-w-[348px] space-y-7">
          {/* Lead: emblem + heading */}
          <div className="flex flex-col items-center text-center space-y-4 auth-enter-lead">
            <div className="auth-logo-badge flex h-11 w-11 items-center justify-center rounded-xl border border-[var(--border-strong)] bg-foreground text-background dark:bg-[#16161a] dark:text-foreground shadow-[inset_0_1px_0_rgba(255,255,255,0.12)]">
              <WalrusLogo className="h-6 w-6" />
            </div>
            <div className="space-y-1.5">
              <h1 className="text-[24px] font-semibold tracking-[-0.03em] leading-tight text-foreground text-balance">
                {t('auth.loginTitle')}
              </h1>
              <p className="text-[13.5px] text-muted-foreground leading-relaxed text-pretty">
                {t('auth.loginSubtitle')}
              </p>
            </div>
          </div>

          {/* Login form */}
          <form ref={formRef} onSubmit={handleSubmit} className="space-y-4">
            {Boolean(submitError) && (
              <div
                role="alert"
                className="auth-alert-enter flex items-start gap-2 rounded-lg border border-red-500/30 bg-red-500/10 backdrop-blur-xs px-3 py-2.5 text-xs text-red-700 dark:text-red-300"
              >
                <AlertTriangle className="h-4 w-4 text-red-600 dark:text-red-400 shrink-0 mt-0.5" />
                <span className="break-words">{formatError(submitError)}</span>
              </div>
            )}

            <div className="space-y-3.5 auth-enter-support">
              <div className="space-y-1.5">
                <Label htmlFor="login-user" className="text-foreground/90">
                  {t('auth.username')}
                </Label>
                <Input
                  id="login-user"
                  required
                  autoFocus
                  autoComplete="username"
                  className="h-10 sm:h-10 rounded-[9px] bg-[var(--surface-input)]/88 backdrop-blur-xs text-[13.5px]"
                  value={username}
                  onChange={(e) => {
                    setUsername(e.target.value)
                    if (submitError) setSubmitError(null)
                  }}
                />
              </div>

              <div className="space-y-1.5">
                <Label htmlFor="login-pwd" className="text-foreground/90">
                  {t('auth.password')}
                </Label>
                <SecretInput
                  id="login-pwd"
                  required
                  autoComplete="current-password"
                  className="h-10 sm:h-10 rounded-[9px] bg-[var(--surface-input)]/88 backdrop-blur-xs text-[13.5px]"
                  value={password}
                  onChange={(e) => {
                    setPassword(e.target.value)
                    if (submitError) setSubmitError(null)
                  }}
                />
              </div>
            </div>

            <div className="pt-1 auth-enter-cta">
              <Button
                type="submit"
                className="auth-submit-btn relative overflow-hidden w-full h-10 sm:h-10 rounded-[9px] text-[13.5px]"
                disabled={busy}
              >
                <span aria-hidden="true" className="auth-btn-sheen" />
                {busy ? (
                  <>
                    <Loader2 className="h-4 w-4 animate-spin" />
                    <span>{t('common.loading')}</span>
                  </>
                ) : (
                  <>
                    <span>{t('auth.loginBtn')}</span>
                    <ArrowRight className="auth-btn-arrow h-3.5 w-3.5" />
                  </>
                )}
              </Button>
            </div>
          </form>
        </div>
      </section>

      {/* Quiet bottom colophon */}
      <footer className="relative z-10 pb-6 px-6 text-center auth-enter-fade">
        <span className="text-xs text-muted-foreground/75">
          {t('common.appName')} · {t('common.appSubtitle')}
        </span>
      </footer>
    </main>
  )
}
