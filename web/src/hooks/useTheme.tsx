import React, { createContext, useCallback, useContext, useEffect, useState } from 'react'

export type ThemeMode = 'dark' | 'light'

interface ThemeContextValue {
  theme: ThemeMode
  setTheme: (next: ThemeMode) => void
  toggle: () => void
}

const ThemeContext = createContext<ThemeContextValue | null>(null)

function readInitialTheme(): ThemeMode {
  try {
    return localStorage.getItem('walrus-theme') === 'light' ? 'light' : 'dark'
  } catch {
    return 'dark'
  }
}

export function ThemeProvider({ children }: { children: React.ReactNode }) {
  const [theme, setThemeState] = useState<ThemeMode>(readInitialTheme)

  useEffect(() => {
    const root = document.documentElement
    if (theme === 'dark') {
      root.classList.add('dark')
    } else {
      root.classList.remove('dark')
    }
    const meta = document.querySelector('meta[name="theme-color"]')
    if (meta) {
      meta.setAttribute('content', theme === 'dark' ? '#0a0a0b' : '#fafafa')
    }
    try {
      localStorage.setItem('walrus-theme', theme)
    } catch {
      // ignore storage errors
    }
  }, [theme])

  const setTheme = useCallback((next: ThemeMode) => setThemeState(next), [])
  const toggle = useCallback(() => setThemeState((prev) => (prev === 'dark' ? 'light' : 'dark')), [])

  return <ThemeContext.Provider value={{ theme, setTheme, toggle }}>{children}</ThemeContext.Provider>
}

export function useTheme(): ThemeContextValue {
  const ctx = useContext(ThemeContext)
  if (!ctx) {
    throw new Error('useTheme must be used inside ThemeProvider')
  }
  return ctx
}
