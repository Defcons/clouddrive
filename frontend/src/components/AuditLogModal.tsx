import { useState, useEffect } from 'react'
import { getAuditLog } from '../api'
import { useDialog } from '../hooks/useDialog'

interface Props {
  onClose: () => void
}

const ACTION_COLORS: Record<string, string> = {
  LOGIN_OK: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400',
  LOGIN_FAIL: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400',
  UPLOAD: 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400',
  DELETE: 'bg-red-100 text-red-700 dark:bg-red-900/40 dark:text-red-400',
  RENAME: 'bg-yellow-100 text-yellow-700 dark:bg-yellow-900/40 dark:text-yellow-400',
  MKDIR: 'bg-purple-100 text-purple-700 dark:bg-purple-900/40 dark:text-purple-400',
  SHARE: 'bg-indigo-100 text-indigo-700 dark:bg-indigo-900/40 dark:text-indigo-400',
  PW_CHANGE: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-400',
  PRIVATE: 'bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-400',
  PUBLIC: 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400',
}

export default function AuditLogModal({ onClose }: Props) {
  const [entries, setEntries] = useState<{ timestamp: string; action: string; username: string; ip: string; detail: string }[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const [filter, setFilter] = useState('')

  useEffect(() => {
    let mounted = true
    getAuditLog(500)
      .then((e) => { if (mounted) setEntries(e) })
      .catch(() => { if (mounted) setError(true) })
      .finally(() => { if (mounted) setLoading(false) })
    return () => { mounted = false }
  }, [])

  const dialogRef = useDialog<HTMLDivElement>(onClose)

  const filtered = filter
    ? entries.filter((e) =>
        e.action.toLowerCase().includes(filter.toLowerCase()) ||
        e.username.toLowerCase().includes(filter.toLowerCase()) ||
        e.detail.toLowerCase().includes(filter.toLowerCase())
      )
    : entries

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        tabIndex={-1}
        className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-4xl mx-4 max-h-[85vh] flex flex-col overflow-hidden focus:outline-none"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
          <div className="flex items-center gap-3">
            <h2 className="text-base font-semibold text-gray-800 dark:text-gray-200">Audit Log</h2>
            <span className="text-xs text-gray-400">{filtered.length} entries</span>
          </div>
          <div className="flex items-center gap-2">
            <input
              type="text"
              placeholder="Filter..."
              value={filter}
              onChange={(e) => setFilter(e.target.value)}
              className="px-2.5 py-1 text-xs border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 rounded-md focus:outline-none focus:ring-1 focus:ring-blue-500"
            />
            <button
              onClick={onClose}
              className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition"
            >
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        {/* Log entries */}
        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="text-gray-400 text-center py-12">Loading...</div>
          ) : error ? (
            <div className="text-gray-400 text-center py-12">Couldn’t load the audit log. Try again.</div>
          ) : filtered.length === 0 ? (
            <div className="text-gray-400 text-center py-12">No audit entries</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-gray-50 dark:bg-gray-900">
                <tr className="text-left text-xs text-gray-500 dark:text-gray-400 uppercase">
                  <th className="px-4 py-2 font-medium">Time</th>
                  <th className="px-4 py-2 font-medium">Action</th>
                  <th className="px-4 py-2 font-medium">User</th>
                  <th className="px-4 py-2 font-medium">Detail</th>
                  <th className="px-4 py-2 font-medium">IP</th>
                </tr>
              </thead>
              <tbody>
                {filtered.map((entry, i) => (
                  <tr key={i} className="border-t border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <td className="px-4 py-2 whitespace-nowrap text-gray-500 dark:text-gray-400 font-mono text-xs">
                      {entry.timestamp}
                    </td>
                    <td className="px-4 py-2">
                      <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${ACTION_COLORS[entry.action] || 'bg-gray-100 text-gray-600 dark:bg-gray-700 dark:text-gray-300'}`}>
                        {entry.action}
                      </span>
                    </td>
                    <td className="px-4 py-2 text-gray-700 dark:text-gray-300 font-medium">
                      {entry.username}
                    </td>
                    <td className="px-4 py-2 text-gray-600 dark:text-gray-400 max-w-md truncate" title={entry.detail}>
                      {entry.detail}
                    </td>
                    <td className="px-4 py-2 text-gray-400 font-mono text-xs">
                      {entry.ip}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>
      </div>
    </div>
  )
}
