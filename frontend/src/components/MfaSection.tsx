import { useEffect, useState } from 'react'
import {
  getMfaStatus,
  startMfaSetup,
  confirmMfaSetup,
  disableMfa,
  regenerateBackupCodes,
  type MfaStatus,
} from '../api'
import { prompt as promptModal, confirm as confirmModal } from './ConfirmModal'

type EnrollState =
  | { kind: 'idle' }
  | { kind: 'loading' }
  | {
      kind: 'scanning'
      secret: string
      qrCodeDataUrl: string
      otpAuthUrl: string
      code: string
      error: string
      verifying: boolean
    }
  | { kind: 'backup_codes'; codes: string[] }

export default function MfaSection() {
  const [status, setStatus] = useState<MfaStatus | null>(null)
  const [enroll, setEnroll] = useState<EnrollState>({ kind: 'idle' })
  const [flash, setFlash] = useState<{ kind: 'success' | 'error'; text: string } | null>(null)

  const refresh = () => getMfaStatus().then(setStatus).catch(() => {})
  useEffect(() => {
    refresh()
  }, [])

  useEffect(() => {
    if (!flash) return
    const t = setTimeout(() => setFlash(null), 4000)
    return () => clearTimeout(t)
  }, [flash])

  const handleStart = async () => {
    const pw = await promptModal({
      title: 'Enable two-factor auth',
      message: 'Confirm your current password to begin setup.',
      prompt: { inputType: 'password', placeholder: 'Current password' },
      confirmLabel: 'Continue',
    })
    if (!pw) return
    setEnroll({ kind: 'loading' })
    try {
      const data = await startMfaSetup(pw)
      setEnroll({
        kind: 'scanning',
        secret: data.secret,
        qrCodeDataUrl: data.qrCodeDataUrl,
        otpAuthUrl: data.otpAuthUrl,
        code: '',
        error: '',
        verifying: false,
      })
    } catch (err: any) {
      setEnroll({ kind: 'idle' })
      setFlash({ kind: 'error', text: err?.message || 'Failed to start setup' })
    }
  }

  const handleVerify = async () => {
    if (enroll.kind !== 'scanning') return
    setEnroll({ ...enroll, verifying: true, error: '' })
    try {
      const res = await confirmMfaSetup(enroll.secret, enroll.code.trim())
      setEnroll({ kind: 'backup_codes', codes: res.backupCodes })
      refresh()
    } catch (err: any) {
      setEnroll({ ...enroll, verifying: false, error: err?.message || 'Invalid code' })
    }
  }

  const handleDisable = async () => {
    const pw = await promptModal({
      title: 'Disable two-factor auth?',
      message: 'This removes MFA from your account. Confirm your password to continue.',
      prompt: { inputType: 'password', placeholder: 'Current password' },
      confirmLabel: 'Disable 2FA',
      destructive: true,
    })
    if (!pw) return
    try {
      await disableMfa(pw)
      setFlash({ kind: 'success', text: 'Two-factor auth disabled' })
      refresh()
    } catch (err: any) {
      setFlash({ kind: 'error', text: err?.message || 'Failed' })
    }
  }

  const handleRegen = async () => {
    const pw = await promptModal({
      title: 'Regenerate backup codes?',
      message: 'Old backup codes will be invalidated. Confirm your password.',
      prompt: { inputType: 'password', placeholder: 'Current password' },
      confirmLabel: 'Regenerate',
    })
    if (!pw) return
    try {
      const res = await regenerateBackupCodes(pw)
      setEnroll({ kind: 'backup_codes', codes: res.backupCodes })
      refresh()
    } catch (err: any) {
      setFlash({ kind: 'error', text: err?.message || 'Failed' })
    }
  }

  const handleCopyCodes = async (codes: string[]) => {
    try {
      await navigator.clipboard.writeText(codes.join('\n'))
      setFlash({ kind: 'success', text: 'Backup codes copied to clipboard' })
    } catch {
      setFlash({ kind: 'error', text: 'Copy failed — save them manually' })
    }
  }

  const handleCodesDone = async () => {
    const ok = await confirmModal({
      title: 'Saved your backup codes?',
      message:
        'Make sure you saved all backup codes somewhere safe (password manager, printout). You cannot view them again.',
      confirmLabel: 'I saved them',
    })
    if (!ok) return
    setEnroll({ kind: 'idle' })
  }

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300">Two-factor authentication</h3>

      {flash && (
        <div
          className={`p-2 rounded-lg text-sm ${
            flash.kind === 'success'
              ? 'bg-green-50 text-green-600 dark:bg-green-900/30 dark:text-green-300'
              : 'bg-red-50 text-red-600 dark:bg-red-900/30 dark:text-red-300'
          }`}
        >
          {flash.text}
        </div>
      )}

      {enroll.kind === 'idle' && (
        <>
          {status?.enabled ? (
            <>
              <div className="flex items-center gap-2 text-sm text-green-700 dark:text-green-400">
                <svg className="w-4 h-4" fill="currentColor" viewBox="0 0 20 20">
                  <path
                    fillRule="evenodd"
                    d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z"
                    clipRule="evenodd"
                  />
                </svg>
                Enabled. {status.backupCodesRemaining} backup code
                {status.backupCodesRemaining === 1 ? '' : 's'} remaining.
              </div>
              <div className="flex gap-2">
                <button
                  onClick={handleRegen}
                  className="min-h-11 flex-1 bg-gray-200 dark:bg-gray-700 text-gray-800 dark:text-gray-200 py-2 rounded-lg text-sm font-medium hover:bg-gray-300 dark:hover:bg-gray-600"
                >
                  Regenerate backup codes
                </button>
                <button
                  onClick={handleDisable}
                  className="min-h-11 flex-1 bg-red-600 text-white py-2 rounded-lg text-sm font-medium hover:bg-red-700"
                >
                  Disable 2FA
                </button>
              </div>
            </>
          ) : (
            <>
              <p className="text-sm text-gray-500 dark:text-gray-400">
                Add an authenticator app (Google Authenticator, Authy, 1Password, Bitwarden…) to
                require a 6-digit code on login.
              </p>
              <button
                onClick={handleStart}
                className="w-full min-h-11 bg-blue-600 text-white py-2 rounded-lg text-sm font-medium hover:bg-blue-700"
              >
                Enable 2FA
              </button>
            </>
          )}
        </>
      )}

      {enroll.kind === 'loading' && <div className="text-sm text-gray-500">Generating secret…</div>}

      {enroll.kind === 'scanning' && (
        <div className="space-y-3">
          <p className="text-sm text-gray-600 dark:text-gray-300">
            1. Scan this QR code in your authenticator app. 2. Type the 6-digit code it generates to
            confirm.
          </p>
          <div className="flex justify-center">
            <img src={enroll.qrCodeDataUrl} alt="Scan with your authenticator app" className="w-60 h-60 rounded" />
          </div>
          <details className="text-xs text-gray-500 dark:text-gray-400">
            <summary className="cursor-pointer">Can't scan? Enter this manually</summary>
            <div className="mt-2 font-mono break-all select-all bg-gray-100 dark:bg-gray-900 p-2 rounded">
              {enroll.secret}
            </div>
          </details>
          {enroll.error && (
            <div className="p-2 bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-300 rounded-lg text-sm">
              {enroll.error}
            </div>
          )}
          <input
            type="text"
            inputMode="numeric"
            pattern="[0-9]*"
            value={enroll.code}
            onChange={(e) =>
              setEnroll({ ...enroll, code: e.target.value.replace(/\D/g, '').slice(0, 6) })
            }
            placeholder="123456"
            className="w-full min-h-11 px-3 py-2 text-center tracking-widest text-lg border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-lg focus:outline-none focus:ring-2 focus:ring-blue-500"
            autoFocus
          />
          <div className="flex gap-2">
            <button
              onClick={() => setEnroll({ kind: 'idle' })}
              className="min-h-11 flex-1 bg-gray-200 dark:bg-gray-700 text-gray-800 dark:text-gray-200 py-2 rounded-lg text-sm"
            >
              Cancel
            </button>
            <button
              onClick={handleVerify}
              disabled={enroll.verifying || enroll.code.length !== 6}
              className="min-h-11 flex-1 bg-blue-600 text-white py-2 rounded-lg text-sm font-medium hover:bg-blue-700 disabled:opacity-50"
            >
              {enroll.verifying ? 'Verifying…' : 'Verify & enable'}
            </button>
          </div>
        </div>
      )}

      {enroll.kind === 'backup_codes' && (
        <div className="space-y-3">
          <div className="p-3 bg-yellow-50 dark:bg-yellow-900/30 text-yellow-700 dark:text-yellow-300 rounded-lg text-sm">
            Save these backup codes. Each works once if you lose access to your authenticator. You
            won't see them again.
          </div>
          <div className="grid grid-cols-2 gap-2 font-mono text-sm bg-gray-100 dark:bg-gray-900 p-3 rounded-lg">
            {enroll.codes.map((c) => (
              <div key={c} className="select-all">
                {c}
              </div>
            ))}
          </div>
          <div className="flex gap-2">
            <button
              onClick={() => handleCopyCodes(enroll.codes)}
              className="min-h-11 flex-1 bg-gray-200 dark:bg-gray-700 text-gray-800 dark:text-gray-200 py-2 rounded-lg text-sm"
            >
              Copy all
            </button>
            <button
              onClick={handleCodesDone}
              className="min-h-11 flex-1 bg-blue-600 text-white py-2 rounded-lg text-sm font-medium hover:bg-blue-700"
            >
              Done
            </button>
          </div>
        </div>
      )}
    </div>
  )
}
