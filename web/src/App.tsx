import React from 'react'
import { createBrowserRouter, Navigate, Outlet, RouterProvider } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { Toaster } from 'sonner'
import { ApiError } from '@/lib/api'
import { AuthProvider, useAuth } from '@/hooks/useAuth'
import { ThemeProvider, useTheme } from '@/hooks/useTheme'
import { UnsavedGuardProvider } from '@/hooks/useUnsavedGuard'
import { AppShell } from '@/components/AppShell'
import { SetupPage } from '@/pages/SetupPage'
import { LoginPage } from '@/pages/LoginPage'
import { DashboardPage } from '@/pages/DashboardPage'
import { CreateClusterPage } from '@/pages/CreateClusterPage'
import { ClusterOverviewPage } from '@/pages/ClusterOverviewPage'
import { ClusterConsolePage } from '@/pages/ClusterConsolePage'
import { ClusterConfigPage } from '@/pages/ClusterConfigPage'
import { ClusterModsPage } from '@/pages/ClusterModsPage'
import { ClusterPlayersPage } from '@/pages/ClusterPlayersPage'
import { ClusterBackupsPage } from '@/pages/ClusterBackupsPage'
import { ClusterFilesPage } from '@/pages/ClusterFilesPage'
import { SteamPage } from '@/pages/SteamPage'
import { WebhooksPage } from '@/pages/WebhooksPage'
import { UsersPage } from '@/pages/UsersPage'
import { SettingsPage } from '@/pages/SettingsPage'
import { AuditLogsPage } from '@/pages/AuditLogsPage'

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      retry: (failureCount, error) => {
        if (error instanceof ApiError && (error.status === 401 || error.status === 403)) {
          return false
        }
        return failureCount < 1
      },
      refetchOnWindowFocus: false,
    },
  },
})

function RequireAuth({ children }: { children: React.ReactNode }) {
  const { t } = useTranslation()
  const { loading, initialized, user } = useAuth()
  if (loading) {
    return (
      <div className="min-h-screen flex items-center justify-center bg-background text-sm text-muted-foreground">
        {t('common.loading')}
      </div>
    )
  }
  if (!initialized) return <Navigate to="/setup" replace />
  if (!user) return <Navigate to="/login" replace />
  return <>{children}</>
}

function RequireAdmin({ children }: { children: React.ReactNode }) {
  const { user } = useAuth()
  if (user?.role !== 'admin') return <Navigate to="/" replace />
  return <>{children}</>
}

function SetupRoute() {
  const { loading, initialized } = useAuth()
  if (loading) return null
  if (initialized) return <Navigate to="/login" replace />
  return <SetupPage />
}

function LoginRoute() {
  const { loading, initialized, user } = useAuth()
  if (loading) return null
  if (!initialized) return <Navigate to="/setup" replace />
  if (user) return <Navigate to="/" replace />
  return <LoginPage />
}

function RootLayout() {
  return (
    <UnsavedGuardProvider>
      <Outlet />
    </UnsavedGuardProvider>
  )
}

function ThemedToaster() {
  const { theme } = useTheme()
  return (
    <Toaster
      richColors
      position="bottom-right"
      offset={{ bottom: 76, right: 20 }}
      mobileOffset={{ bottom: 76, right: 16 }}
      theme={theme}
    />
  )
}

const router = createBrowserRouter([
  {
    element: <RootLayout />,
    children: [
      { path: '/setup', element: <SetupRoute /> },
      { path: '/login', element: <LoginRoute /> },
      {
        element: (
          <RequireAuth>
            <AppShell />
          </RequireAuth>
        ),
        children: [
          { path: '/', element: <DashboardPage /> },
          {
            path: '/clusters/new',
            element: (
              <RequireAdmin>
                <CreateClusterPage />
              </RequireAdmin>
            ),
          },
          { path: '/clusters/:id/overview', element: <ClusterOverviewPage /> },
          { path: '/clusters/:id/console', element: <ClusterConsolePage /> },
          { path: '/clusters/:id/config', element: <ClusterConfigPage /> },
          { path: '/clusters/:id/mods', element: <ClusterModsPage /> },
          { path: '/clusters/:id/players', element: <ClusterPlayersPage /> },
          { path: '/clusters/:id/backups', element: <ClusterBackupsPage /> },
          { path: '/clusters/:id/files', element: <ClusterFilesPage /> },
          {
            path: '/steam',
            element: (
              <RequireAdmin>
                <SteamPage />
              </RequireAdmin>
            ),
          },
          {
            path: '/webhooks',
            element: (
              <RequireAdmin>
                <WebhooksPage />
              </RequireAdmin>
            ),
          },
          {
            path: '/users',
            element: (
              <RequireAdmin>
                <UsersPage />
              </RequireAdmin>
            ),
          },
          {
            path: '/settings',
            element: (
              <RequireAdmin>
                <SettingsPage />
              </RequireAdmin>
            ),
          },
          {
            path: '/audit',
            element: (
              <RequireAdmin>
                <AuditLogsPage />
              </RequireAdmin>
            ),
          },
        ],
      },
      { path: '*', element: <Navigate to="/" replace /> },
    ],
  },
])

export function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <ThemeProvider>
        <AuthProvider>
          <RouterProvider router={router} />
          <ThemedToaster />
        </AuthProvider>
      </ThemeProvider>
    </QueryClientProvider>
  )
}

export default App
