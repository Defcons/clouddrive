import { useEffect, useState } from 'react'
import { getSettings, updateSettings, type InstanceSettings as Settings } from '../api'

// Admin-only instance configuration: display name + feature toggles. Saving
// calls onSaved() so the app re-reads settings (header/context-menu update).
export default function InstanceSettings({ onSaved }: { onSaved: () => void }) {
  const [s, setS] = useState<Settings | null>(null)
  const [saving, setSaving] = useState(false)
  const [message, setMessage] = useState('')
  const [error, setError] = useState('')

  useEffect(() => {
    getSettings().then(setS)
  }, [])

  const save = async () => {
    if (!s) return
    setSaving(true)
    setError('')
    setMessage('')
    try {
      const saved = await updateSettings({ ...s, instanceName: s.instanceName.trim() || 'CloudDrive' })
      setS(saved)
      setMessage('Configuration saved')
      onSaved()
    } catch (err: any) {
      setError(err?.message || 'Failed to save')
    } finally {
      setSaving(false)
    }
  }

  if (!s) return null

  return (
    <div className="space-y-3">
      <h3 className="text-sm font-medium text-gray-700 dark:text-gray-300">Configuration</h3>

      {message && <div className="p-2 bg-green-50 dark:bg-green-900/30 text-green-600 dark:text-green-300 rounded-lg text-sm">{message}</div>}
      {error && <div className="p-2 bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-300 rounded-lg text-sm">{error}</div>}

      <div>
        <label className="block text-xs text-gray-500 mb-1">Instance name</label>
        <input
          type="text"
          value={s.instanceName}
          onChange={(e) => setS({ ...s, instanceName: e.target.value })}
          className="w-full px-3 py-2 border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-100 rounded-lg text-sm focus:outline-none focus:ring-2 focus:ring-blue-500"
        />
      </div>

      <label className="flex items-start justify-between gap-3 cursor-pointer py-1">
        <span>
          <span className="block text-sm text-gray-700 dark:text-gray-200">Sharing</span>
          <span className="block text-xs text-gray-400">Public and password-protected share links</span>
        </span>
        <input
          type="checkbox"
          checked={s.sharingEnabled}
          onChange={(e) => setS({ ...s, sharingEnabled: e.target.checked })}
          className="mt-1 w-4 h-4 shrink-0"
        />
      </label>

      <label className="flex items-start justify-between gap-3 cursor-pointer py-1">
        <span>
          <span className="block text-sm text-gray-700 dark:text-gray-200">Offsite-backup flags</span>
          <span className="block text-xs text-gray-400">Show “Add to Offsite Backup” on folders</span>
        </span>
        <input
          type="checkbox"
          checked={s.offsiteBackupEnabled}
          onChange={(e) => setS({ ...s, offsiteBackupEnabled: e.target.checked })}
          className="mt-1 w-4 h-4 shrink-0"
        />
      </label>

      <button
        onClick={save}
        disabled={saving}
        className="w-full bg-blue-600 text-white py-2 rounded-lg text-sm font-medium hover:bg-blue-700 transition disabled:opacity-50"
      >
        {saving ? 'Saving…' : 'Save configuration'}
      </button>
    </div>
  )
}
