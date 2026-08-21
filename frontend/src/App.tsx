import { useState, useEffect } from 'react'
import { checkAuth, getCurrentUser, getSetupStatus, setOnAuthExpired, getSettings, DEFAULT_SETTINGS, type InstanceSettings } from './api'
import LoginPage from './components/LoginPage'
import SetupPage from './components/SetupPage'
import FileExplorer from './components/FileExplorer'
import ErrorBoundary from './components/ErrorBoundary'
import ConfirmModalHost from './components/ConfirmModal'

export default function App() {
  const [authenticated, setAuthenticated] = useState<boolean | null>(null)
  const [needsSetup, setNeedsSetup] = useState<boolean | null>(null)
  const [settings, setSettings] = useState<InstanceSettings>(DEFAULT_SETTINGS)

  const refreshSettings = () =>
    getSettings().then((s) => {
      setSettings(s)
      document.title = s.instanceName
    })

  useEffect(() => {
    // On first run (no accounts yet) show the setup wizard; otherwise fall
    // through to the normal auth check.
    getSetupStatus().then((needed) => {
      setNeedsSetup(needed)
      if (needed) {
        setAuthenticated(false)
      } else {
        checkAuth().then(setAuthenticated)
      }
    })
  }, [])

  // When any API call hits a 401 (session expired/invalidated server-side),
  // drop back to the login screen instead of stranding the user.
  useEffect(() => {
    setOnAuthExpired(() => setAuthenticated(false))
    return () => setOnAuthExpired(null)
  }, [])

  // Once signed in, load the instance settings (name + feature flags).
  useEffect(() => {
    if (authenticated) refreshSettings()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [authenticated])

  if (needsSetup === null || authenticated === null) {
    return (
      <div className="h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div className="text-gray-400 text-lg">Loading...</div>
      </div>
    )
  }

  return (
    <ErrorBoundary>
      {needsSetup ? (
        <SetupPage
          onDone={() => {
            setNeedsSetup(false)
            setAuthenticated(true)
          }}
        />
      ) : !authenticated ? (
        <LoginPage onLogin={() => setAuthenticated(true)} />
      ) : (
        <FileExplorer
          initialPath={getCurrentUser().homeFolder}
          onLogout={() => setAuthenticated(false)}
          settings={settings}
          onSettingsChanged={refreshSettings}
        />
      )}
      <ConfirmModalHost />
    </ErrorBoundary>
  )
}
