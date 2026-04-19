import { useState, useEffect } from 'react'
import { checkAuth, getCurrentUser } from './api'
import LoginPage from './components/LoginPage'
import FileExplorer from './components/FileExplorer'
import ErrorBoundary from './components/ErrorBoundary'
import ConfirmModalHost from './components/ConfirmModal'

export default function App() {
  const [authenticated, setAuthenticated] = useState<boolean | null>(null)

  useEffect(() => {
    checkAuth().then(setAuthenticated)
  }, [])

  if (authenticated === null) {
    return (
      <div className="h-screen flex items-center justify-center bg-gray-50 dark:bg-gray-900">
        <div className="text-gray-400 text-lg">Loading...</div>
      </div>
    )
  }

  return (
    <ErrorBoundary>
      {!authenticated ? (
        <LoginPage onLogin={() => setAuthenticated(true)} />
      ) : (
        <FileExplorer
          initialPath={getCurrentUser().homeFolder}
          onLogout={() => setAuthenticated(false)}
        />
      )}
      <ConfirmModalHost />
    </ErrorBoundary>
  )
}
