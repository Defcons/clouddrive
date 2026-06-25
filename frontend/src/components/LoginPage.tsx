import { useState } from 'react'
import { login, challengeMfa } from '../api'

type Step = 'password' | 'mfa'

export default function LoginPage({ onLogin }: { onLogin: () => void }) {
  const [step, setStep] = useState<Step>('password')
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [mfaToken, setMfaToken] = useState('')
  const [totpCode, setTotpCode] = useState('')
  const [backupCode, setBackupCode] = useState('')
  const [useBackup, setUseBackup] = useState(false)
  const [trustDevice, setTrustDevice] = useState(true)
  const [error, setError] = useState('')
  const [loading, setLoading] = useState(false)

  const handlePasswordSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    // Track auth success separately so an error thrown by onLogin() (a
    // post-auth navigation/refresh) isn't reported as bad credentials.
    let authed = false
    try {
      const result = await login(username, password)
      if (result.mfaRequired) {
        setMfaToken(result.mfaToken)
        setStep('mfa')
      } else {
        authed = true
      }
    } catch (err: any) {
      setError(err?.message || 'Invalid username or password')
    } finally {
      setLoading(false)
    }
    if (authed) onLogin()
  }

  const handleMfaSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setError('')
    setLoading(true)
    // Only auth failures should show "Invalid code" — the TOTP is consumed on
    // success, so a post-login error must not send the user back to re-enter it.
    let authed = false
    try {
      await challengeMfa({
        mfaToken,
        code: useBackup ? undefined : totpCode.trim(),
        backupCode: useBackup ? backupCode.trim() : undefined,
        trustDevice,
      })
      authed = true
    } catch (err: any) {
      setError(err?.message || 'Invalid code')
    } finally {
      setLoading(false)
    }
    if (authed) onLogin()
  }

  const handleBackToPassword = () => {
    setStep('password')
    setPassword('')
    setTotpCode('')
    setBackupCode('')
    setMfaToken('')
    setError('')
  }

  return (
    <div className="h-screen flex items-center justify-center bg-gray-100 dark:bg-gray-900 p-4">
      {step === 'password' ? (
        <form
          onSubmit={handlePasswordSubmit}
          className="bg-white dark:bg-gray-800 p-8 rounded-xl shadow-lg w-full max-w-sm"
        >
          <h1 className="text-2xl font-bold mb-1 text-gray-800 dark:text-gray-100">CloudDrive</h1>
          <p className="text-gray-500 dark:text-gray-400 mb-6 text-sm">Sign in to access your files</p>

          {error && (
            <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-300 rounded-lg text-sm">
              {error}
            </div>
          )}

          <label className="block mb-1 text-sm font-medium text-gray-700 dark:text-gray-300">
            Username
          </label>
          <input
            type="text"
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            className="w-full min-h-11 mb-4 px-3 py-2 border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            autoFocus
            autoComplete="username"
          />

          <label className="block mb-1 text-sm font-medium text-gray-700 dark:text-gray-300">
            Password
          </label>
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            className="w-full min-h-11 mb-6 px-3 py-2 border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500 focus:border-transparent"
            autoComplete="current-password"
          />

          <button
            type="submit"
            disabled={loading || !username || !password}
            className="w-full min-h-11 bg-blue-600 text-white py-2.5 rounded-lg font-medium hover:bg-blue-700 transition disabled:opacity-50"
          >
            {loading ? 'Signing in…' : 'Sign In'}
          </button>
        </form>
      ) : (
        <form
          onSubmit={handleMfaSubmit}
          className="bg-white dark:bg-gray-800 p-8 rounded-xl shadow-lg w-full max-w-sm"
        >
          <h1 className="text-lg font-semibold mb-1 text-gray-800 dark:text-gray-100">
            Two-factor code
          </h1>
          <p className="text-gray-500 dark:text-gray-400 mb-6 text-sm">
            {useBackup
              ? 'Enter one of your 8-character backup codes.'
              : 'Enter the 6-digit code from your authenticator app.'}
          </p>

          {error && (
            <div className="mb-4 p-3 bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-300 rounded-lg text-sm">
              {error}
            </div>
          )}

          {!useBackup ? (
            <input
              type="text"
              inputMode="numeric"
              pattern="[0-9]*"
              value={totpCode}
              onChange={(e) => setTotpCode(e.target.value.replace(/\D/g, '').slice(0, 6))}
              placeholder="123456"
              className="w-full min-h-11 mb-4 px-3 py-2 text-center tracking-widest text-lg border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              autoFocus
              autoComplete="one-time-code"
            />
          ) : (
            <input
              type="text"
              value={backupCode}
              onChange={(e) => setBackupCode(e.target.value)}
              placeholder="abcd-1234"
              className="w-full min-h-11 mb-4 px-3 py-2 font-mono border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
              autoFocus
            />
          )}

          <label className="flex items-center gap-2 mb-6 text-sm text-gray-700 dark:text-gray-300 cursor-pointer">
            <input
              type="checkbox"
              checked={trustDevice}
              onChange={(e) => setTrustDevice(e.target.checked)}
              className="w-4 h-4"
            />
            Trust this device for 30 days
          </label>

          <button
            type="submit"
            disabled={loading || (useBackup ? !backupCode : totpCode.length !== 6)}
            className="w-full min-h-11 bg-blue-600 text-white py-2.5 rounded-lg font-medium hover:bg-blue-700 transition disabled:opacity-50"
          >
            {loading ? 'Verifying…' : 'Verify'}
          </button>

          <div className="mt-4 flex items-center justify-between text-xs">
            <button
              type="button"
              onClick={() => {
                setUseBackup(!useBackup)
                setError('')
              }}
              className="text-blue-600 dark:text-blue-400 hover:underline"
            >
              {useBackup ? 'Use authenticator app instead' : 'Use backup code instead'}
            </button>
            <button
              type="button"
              onClick={handleBackToPassword}
              className="text-gray-500 dark:text-gray-400 hover:underline"
            >
              Back
            </button>
          </div>
        </form>
      )}
    </div>
  )
}
