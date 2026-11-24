import React, { createContext, useCallback, useContext, useEffect, useRef, useState } from 'react'
import { useBlocker } from 'react-router-dom'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/components/ConfirmDialog'

interface UnsavedGuardContextValue {
  hasUnsaved: boolean
  setDirty: (key: string, dirty: boolean) => void
  guardAction: (action: () => void) => void
}

const UnsavedGuardContext = createContext<UnsavedGuardContextValue>({
  hasUnsaved: false,
  setDirty: () => {},
  guardAction: (action) => action(),
})

export function UnsavedGuardProvider({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const dirtyKeysRef = useRef<Set<string>>(new Set())
  const [hasUnsaved, setHasUnsaved] = useState(false)
  const [pendingAction, setPendingAction] = useState<(() => void) | null>(null)

  const setDirty = useCallback((key: string, dirty: boolean) => {
    if (dirty) dirtyKeysRef.current.add(key)
    else dirtyKeysRef.current.delete(key)
    setHasUnsaved(dirtyKeysRef.current.size > 0)
  }, [])

  const guardAction = useCallback((action: () => void) => {
    if (dirtyKeysRef.current.size > 0) {
      setPendingAction(() => action)
    } else {
      action()
    }
  }, [])

  // Intercept React Router navigations including browser Back / Forward (popstate)
  const blocker = useBlocker(
    ({ currentLocation, nextLocation }) =>
      dirtyKeysRef.current.size > 0 && currentLocation.pathname !== nextLocation.pathname,
  )

  useEffect(() => {
    const onBeforeUnload = (e: BeforeUnloadEvent) => {
      if (dirtyKeysRef.current.size > 0) {
        e.preventDefault()
        e.returnValue = ''
      }
    }
    window.addEventListener('beforeunload', onBeforeUnload)
    return () => window.removeEventListener('beforeunload', onBeforeUnload)
  }, [])

  const dialogOpen = pendingAction !== null || blocker.state === 'blocked'

  const handleCancel = () => {
    setPendingAction(null)
    if (blocker.state === 'blocked') {
      blocker.reset()
    }
  }

  const handleConfirm = () => {
    const fn = pendingAction
    dirtyKeysRef.current.clear()
    setHasUnsaved(false)
    setPendingAction(null)
    if (blocker.state === 'blocked') {
      blocker.proceed()
    }
    fn?.()
  }

  return (
    <UnsavedGuardContext.Provider value={{ hasUnsaved, setDirty, guardAction }}>
      {children}
      <ConfirmDialog
        open={dialogOpen}
        onClose={handleCancel}
        onConfirm={handleConfirm}
        title={t('common.unsavedSwitchConfirmTitle')}
        description={t('common.unsavedSwitchConfirmDesc')}
        confirmLabel={t('common.discard')}
        tone="warning"
      />
    </UnsavedGuardContext.Provider>
  )
}

export function useUnsavedGuard() {
  return useContext(UnsavedGuardContext)
}

export function useRegisterUnsaved(key: string, dirty: boolean) {
  const { setDirty } = useUnsavedGuard()
  useEffect(() => {
    setDirty(key, dirty)
    return () => setDirty(key, false)
  }, [key, dirty, setDirty])
}
