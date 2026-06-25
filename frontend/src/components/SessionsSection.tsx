import { useState, useEffect, useCallback } from 'react'
import { listSessions, revokeSession, type ActiveSession } from '../api'
import { useToast } from '../hooks/useToast'

function deviceLabel(ua: string): string {
  if (!ua) return 'Unknown device'
  let browser = 'Browser'
  if (/Edg\//.test(ua)) browser = 'Edge'
  else if (/Chrome\//.test(ua)) browser = 'Chrome'
  else if (/Firefox\//.test(ua)) browser = 'Firefox'
  else if (/Safari\//.test(ua)) browser = 'Safari'
  let os = ''
  if (/Windows/.test(ua)) os = 'Windows'
  else if (/Android/.test(ua)) os = 'Android'
  else if (/iPhone|iPad|iOS/.test(ua)) os = 'iOS'
  else if (/Mac OS X|Macintosh/.test(ua)) os = 'macOS'
  else if (/Linux/.test(ua)) os = 'Linux'
  return os ? `${browser} · ${os}` : browser
}

function timeAgo(ms: number): string {
  const mins = Math.floor((Date.now() - ms) / 60000)
  if (mins < 1) return 'just now'
  if (mins < 60) return `${mins}m ago`
  const hours = Math.floor(mins / 60)
  if (hours < 24) return `${hours}h ago`
  return `${Math.floor(hours / 24)}d ago`
}

export default function SessionsSection() {
  const [sessions, setSessions] = useState<ActiveSession[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const toast = useToast()

  const refresh = useCallback(() => {
    setLoading(true)
    setError(false)
    listSessions()
      .then(setSessions)
      .catch(() => setError(true))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { refresh() }, [refresh])

  const handleRevoke = async (id: string) => {
    try {
      await revokeSession(id)
      toast.success('Session signed out')
      refresh()
    } catch (e: any) {
      toast.error(e?.message || 'Failed to revoke session')
    }
  }

  return (
    <div>
      <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">Active sessions</h3>
      {loading ? (
        <div className="text-gray-400 text-sm py-3 text-center">Loading…</div>
      ) : error ? (
        <div className="text-red-500 text-sm py-3 text-center">Couldn’t load sessions.</div>
      ) : sessions.length === 0 ? (
        <div className="text-gray-400 text-sm py-3 text-center">No active sessions.</div>
      ) : (
        <div className="space-y-2">
          {sessions.map((s) => (
            <div key={s.id} className="flex items-center justify-between gap-2 border border-gray-200 dark:border-gray-700 rounded-lg px-3 py-2">
              <div className="min-w-0">
                <div className="text-sm text-gray-800 dark:text-gray-200">
                  {deviceLabel(s.userAgent)}
                  {s.current && <span className="ml-2 text-[10px] px-1.5 py-0.5 bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400 rounded-full">this device</span>}
                </div>
                <div className="text-xs text-gray-400">{s.ip || 'unknown IP'} · active {timeAgo(s.lastSeen)}</div>
              </div>
              {!s.current && (
                <button
                  onClick={() => handleRevoke(s.id)}
                  className="text-xs px-2 py-1 text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 rounded transition flex-shrink-0"
                >
                  Sign out
                </button>
              )}
            </div>
          ))}
        </div>
      )}
    </div>
  )
}
