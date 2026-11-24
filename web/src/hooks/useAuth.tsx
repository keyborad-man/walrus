import React, { createContext, useContext, useEffect, useState, useCallback, useRef } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { toast } from 'sonner'
import { api, AUTH_UNAUTHORIZED_EVENT } from '@/lib/api'
import type { User, UserClusterPermission } from '@/types/api'

interface AuthState {
  loading: boolean
  initialized: boolean
  runtimeMode: string
  user: User | null
  permissions: UserClusterPermission[]
  refresh: () => Promise<void>
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const qc = useQueryClient()
  const [loading, setLoading] = useState(true)
  const [initialized, setInitialized] = useState(true)
  const [runtimeMode, setRuntimeMode] = useState('mock')
  const [user, setUser] = useState<User | null>(null)
  const [permissions, setPermissions] = useState<UserClusterPermission[]>([])

  const userRef = useRef<User | null>(null)
  const expiredNotifiedRef = useRef(false)

  useEffect(() => {
    userRef.current = user
    if (user) {
      expiredNotifiedRef.current = false
    }
  }, [user])

  const refresh = useCallback(async () => {
    try {
      const st = await api.authStatus()
      setInitialized(st.initialized)
      setRuntimeMode(st.runtime_mode)
      if (!st.initialized) {
        setUser(null)
        setPermissions([])
        return
      }
      try {
        const me = await api.me()
        setUser(me.user)
        setPermissions(me.permissions ?? [])
      } catch {
        setUser(null)
        setPermissions([])
      }
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  useEffect(() => {
    const onUnauthorized = () => {
      if (!userRef.current) return
      userRef.current = null
      setUser(null)
      setPermissions([])
      qc.clear()
      if (!expiredNotifiedRef.current) {
        expiredNotifiedRef.current = true
        toast.error(t('auth.sessionExpired'))
      }
    }
    window.addEventListener(AUTH_UNAUTHORIZED_EVENT, onUnauthorized)
    return () => window.removeEventListener(AUTH_UNAUTHORIZED_EVENT, onUnauthorized)
  }, [qc, t])

  const logout = useCallback(async () => {
    await api.logout().catch(() => undefined)
    userRef.current = null
    setUser(null)
    setPermissions([])
    qc.clear()
  }, [qc])

  return (
    <AuthContext.Provider value={{ loading, initialized, runtimeMode, user, permissions, refresh, logout }}>
      {children}
    </AuthContext.Provider>
  )
}

export function useAuth() {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('useAuth must be used inside AuthProvider')
  return ctx
}
