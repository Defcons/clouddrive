import { useState, useEffect, useCallback } from 'react'
import { listVersions, getVersionDownloadUrl, restoreVersion, type FileVersion } from '../api'
import { useToast } from '../hooks/useToast'
import { useDialog } from '../hooks/useDialog'
import { confirm as confirmModal } from './ConfirmModal'

interface Props {
  path: string
  onClose: () => void
  onRestored: () => void
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

function formatWhen(ms: number): string {
  return new Date(ms).toLocaleString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit',
  })
}

export default function VersionsModal({ path, onClose, onRestored }: Props) {
  const [versions, setVersions] = useState<FileVersion[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState(false)
  const toast = useToast()
  const dialogRef = useDialog<HTMLDivElement>(onClose)
  const name = path.split('/').pop() || path

  const refresh = useCallback(() => {
    setLoading(true)
    setError(false)
    listVersions(path)
      .then(setVersions)
      .catch(() => setError(true))
      .finally(() => setLoading(false))
  }, [path])

  useEffect(() => { refresh() }, [refresh])

  const handleRestore = async (id: string) => {
    const ok = await confirmModal({
      title: 'Restore this version?',
      message: 'The current file is saved as a new version first, so you can undo this.',
      confirmLabel: 'Restore',
    })
    if (!ok) return
    try {
      await restoreVersion(path, id)
      toast.success('Version restored')
      onRestored()
      refresh()
    } catch (e: any) {
      toast.error(e?.message || 'Failed to restore version')
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div ref={dialogRef} role="dialog" aria-modal="true" tabIndex={-1} className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-lg mx-4 max-h-[80vh] flex flex-col overflow-hidden focus:outline-none">
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
          <div className="min-w-0">
            <h2 className="text-base font-semibold text-gray-800 dark:text-gray-200">Version history</h2>
            <div className="text-xs text-gray-400 truncate">{name}</div>
          </div>
          <button onClick={onClose} className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition">
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-3">
          {loading ? (
            <div className="text-gray-400 text-center py-8 text-sm">Loading…</div>
          ) : error ? (
            <div className="text-red-500 text-center py-8 text-sm">Couldn’t load version history.</div>
          ) : versions.length === 0 ? (
            <div className="text-gray-400 text-center py-8 text-sm">
              No previous versions yet. A version is saved automatically each time this file is overwritten.
            </div>
          ) : (
            <div className="space-y-2">
              {versions.map((v, i) => (
                <div key={v.id} className="flex items-center justify-between gap-2 border border-gray-200 dark:border-gray-700 rounded-lg px-3 py-2">
                  <div className="min-w-0">
                    <div className="text-sm text-gray-800 dark:text-gray-200">
                      {formatWhen(v.savedAt)}{i === 0 && <span className="ml-2 text-[10px] px-1.5 py-0.5 bg-blue-100 text-blue-700 dark:bg-blue-900/40 dark:text-blue-400 rounded-full">latest</span>}
                    </div>
                    <div className="text-xs text-gray-400">{formatSize(v.size)}</div>
                  </div>
                  <div className="flex gap-1 flex-shrink-0">
                    <a
                      href={getVersionDownloadUrl(path, v.id)}
                      className="text-xs px-2 py-1 text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded transition"
                    >
                      Download
                    </a>
                    <button
                      onClick={() => handleRestore(v.id)}
                      className="text-xs px-2 py-1 text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded transition"
                    >
                      Restore
                    </button>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
