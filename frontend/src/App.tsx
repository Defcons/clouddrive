import { useState, useEffect } from 'react'
import { checkAuth } from './api'
import LoginPage from './components/LoginPage'
import FileExplorer from './components/FileExplorer'

export default function App() {
  const [authenticated, setAuthenticated] = useState<boolean | null>(null)

  useEffect(() => {
    checkAuth().then(setAuthenticated)
  }, [])

  if (authenticated === null) {
    return (
      <div className="h-screen flex items-center justify-center">
        <div className="text-gray-400 text-lg">Loading...</div>
      </div>
    )
  }

  if (!authenticated) {
    return <LoginPage onLogin={() => setAuthenticated(true)} />
  }

  return <FileExplorer onLogout={() => setAuthenticated(false)} />
}
