import { useState, useEffect } from 'react'

interface ShareItem {
  token: string
  filePath: string
  fileName: string
  isDir: boolean
  password: string
  createdBy: string
  createdAt: number
  expiresAt: number
  downloads: number
  lastAccess: number
}

interface Props {
  onClose: () => void
}

function formatDate(ms: number): string {
  if (!ms) return '—'
  return new Date(ms).toLocaleDateString(undefined, {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

function timeRemaining(expiresAt: number): string {
  const diff = expiresAt - Date.now()
  if (diff <= 0) return 'Expired'
  const hours = Math.floor(diff / 3600000)
  if (hours < 24) return `${hours}h left`
  const days = Math.floor(hours / 24)
  return `${days}d left`
}

export default function SharesManager({ onClose }: Props) {
  const [shares, setShares] = useState<ShareItem[]>([])
  const [loading, setLoading] = useState(true)

  const fetchShares = async () => {
    try {
      const res = await fetch('/api/shares', { credentials: 'same-origin' })
      if (res.ok) {
        const data = await res.json()
        setShares(data || [])
      }
    } catch {}
    setLoading(false)
  }

  useEffect(() => { fetchShares() }, [])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  const handleRevoke = async (token: string) => {
    try {
      const csrfRes = await fetch('/api/csrf', { credentials: 'same-origin' })
      const csrfData = await csrfRes.json()
      await fetch('/api/shares/revoke', {
        method: 'POST',
        credentials: 'same-origin',
        headers: {
          'Content-Type': 'application/json',
          'X-CSRF-Token': csrfData.csrfToken,
        },
        body: JSON.stringify({ token }),
      })
      fetchShares()
    } catch {}
  }

  const copyLink = (token: string) => {
    navigator.clipboard.writeText(`${window.location.origin}/share/${token}`)
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-3xl mx-4 max-h-[80vh] flex flex-col overflow-hidden" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-2">
            <h2 className="text-base font-semibold text-gray-800 dark:text-gray-200">Active Shares</h2>
            <span className="text-xs text-gray-400">{shares.length} links</span>
          </div>
          <button onClick={onClose} className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg transition">
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="text-gray-400 text-center py-12">Loading...</div>
          ) : shares.length === 0 ? (
            <div className="text-gray-400 text-center py-12">No active share links</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-gray-50 dark:bg-gray-900">
                <tr className="text-left text-xs text-gray-500 dark:text-gray-400 uppercase">
                  <th className="px-4 py-2">File</th>
                  <th className="px-4 py-2">Type</th>
                  <th className="px-4 py-2">Created</th>
                  <th className="px-4 py-2">Expires</th>
                  <th className="px-4 py-2">Downloads</th>
                  <th className="px-4 py-2">Actions</th>
                </tr>
              </thead>
              <tbody>
                {shares.map((share) => (
                  <tr key={share.token} className="border-t border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <td className="px-4 py-2">
                      <div className="text-gray-800 dark:text-gray-200 font-medium">{share.fileName}</div>
                      <div className="text-xs text-gray-400 font-mono truncate max-w-48">{share.filePath}</div>
                    </td>
                    <td className="px-4 py-2">
                      <span className={`px-2 py-0.5 rounded-full text-xs font-medium ${
                        share.password
                          ? 'bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400'
                          : 'bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400'
                      }`}>
                        {share.password ? 'Safe' : 'Open'}
                      </span>
                    </td>
                    <td className="px-4 py-2 text-gray-500 dark:text-gray-400 whitespace-nowrap">{formatDate(share.createdAt)}</td>
                    <td className="px-4 py-2 text-gray-500 dark:text-gray-400 whitespace-nowrap">{timeRemaining(share.expiresAt)}</td>
                    <td className="px-4 py-2 text-gray-500 dark:text-gray-400 text-center">{share.downloads}</td>
                    <td className="px-4 py-2">
                      <div className="flex gap-1">
                        <button onClick={() => copyLink(share.token)} className="text-xs px-2 py-1 text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded transition">
                          Copy Link
                        </button>
                        <button onClick={() => handleRevoke(share.token)} className="text-xs px-2 py-1 text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 rounded transition">
                          Revoke
                        </button>
                      </div>
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
