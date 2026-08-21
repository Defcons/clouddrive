import { useState } from 'react'
import { setup } from '../api'

// First-run wizard: shown only when the server reports no accounts exist yet
// (GET /api/setup/status). Creates the initial admin and logs straight in.
export default function SetupPage({ onDone }: { onDone: () => void }) {
  const [username, setUsername] = useState('admin')
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const tooShort = password.length > 0 && password.length < 8
  const mismatch = confirm.length > 0 && confirm !== password
  const canSubmit = username.trim() !== '' && password.length >= 8 && confirm === password

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    // Track success separately so an error thrown by onDone() (a post-setup
    // navigation) isn't misreported as a setup failure.
    let done = false
    try {
      await setup(username.trim(), password)
      done = true
    } catch (err: any) {
      setError(err?.message || 'Setup failed')
    } finally {
      setLoading(false)
    }
    if (done) onDone()
  }

  return (
    <div className="h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900 p-4">
      <form
        onSubmit={handleSubmit}
        className="bg-white dark:bg-gray-800 p-8 rounded-xl shadow-lg w-full max-w-sm"
      >
        <h1 className="text-2xl font-bold mb-1 text-gray-800 dark:text-gray-100">Welcome to CloudDrive</h1>
        <p className="text-gray-500 dark:text-gray-400 mb-6 text-sm">
          Create your admin account to get started.
        </p>

        {error && (
          <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-300 rounded-lg text-sm">
            {error}
          </div>
        )}

        <label className="block mb-1 text-sm font-medium text-gray-700 dark:text-gray-300">Username</label>
        <input
          type="text"
          value={username}
          onChange={(e) => setUsername(e.target.value)}
          className="w-full min-h-11 mb-4 px-3 py-2 border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          autoFocus
          autoComplete="username"
        />

        <label className="block mb-1 text-sm font-medium text-gray-700 dark:text-gray-300">Password</label>
        <input
          type="password"
          value={password}
          onChange={(e) => setPassword(e.target.value)}
          className="w-full min-h-11 mb-1 px-3 py-2 border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          autoComplete="new-password"
        />
        <p className={`mb-4 text-xs ${tooShort ? 'text-red-500' : 'text-gray-400 dark:text-gray-500'}`}>
          At least 8 characters.
        </p>

        <label className="block mb-1 text-sm font-medium text-gray-700 dark:text-gray-300">Confirm password</label>
        <input
          type="password"
          value={confirm}
          onChange={(e) => setConfirm(e.target.value)}
          className="w-full min-h-11 mb-1 px-3 py-2 border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
          autoComplete="new-password"
        />
        <p className={`mb-6 text-xs ${mismatch ? 'text-red-500' : 'text-gray-400 dark:text-gray-500'}`}>
          {mismatch ? "Passwords don't match." : 'Re-enter the same password.'}
        </p>

        <button
          type="submit"
          disabled={loading || !canSubmit}
          className="w-full min-h-11 bg-blue-600 text-white py-2.5 rounded-lg font-medium hover:bg-blue-700 transition disabled:opacity-50"
        >
          {loading ? 'Creating…' : 'Create account'}
        </button>
      </form>
    </div>
  )
}
