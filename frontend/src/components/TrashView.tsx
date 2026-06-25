import { useState, useEffect } from 'react'
import { listTrash, restoreFromTrash, deleteFromTrash, emptyTrash } from '../api'
import type { TrashItem } from '../types'
import { confirm as confirmModal } from './ConfirmModal'
import { useDialog } from '../hooks/useDialog'

interface Props {
  onClose: () => void
  onNavigate: (path: string) => void
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

function formatDate(ms: number): string {
  return new Date(ms).toLocaleDateString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

export default function TrashView({ onClose, onNavigate }: Props) {
  const [items, setItems] = useState<TrashItem[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')

  const refresh = () => {
    setLoading(true)
    setError('')
    listTrash()
      .then(setItems)
      .catch(() => setError('Couldn’t load trash. Try again.'))
      .finally(() => setLoading(false))
  }

  useEffect(() => { refresh() }, [])

  const dialogRef = useDialog<HTMLDivElement>(onClose)

  const handleRestore = async (id: string) => {
    setError('')
    try {
      await restoreFromTrash(id)
      refresh()
    } catch {
      setError('Failed to restore item.')
    }
  }

  const handleDelete = async (id: string) => {
    const ok = await confirmModal({
      title: 'Delete permanently?',
      message: 'Permanently delete this item? This cannot be undone.',
      destructive: true,
      confirmLabel: 'Delete forever',
    })
    if (!ok) return
    setError('')
    try {
      await deleteFromTrash(id)
      refresh()
    } catch {
      setError('Failed to delete item.')
    }
  }

  const handleEmpty = async () => {
    const ok = await confirmModal({
      title: 'Empty trash?',
      message: 'All items in trash will be permanently deleted. This cannot be undone.',
      destructive: true,
      confirmLabel: 'Empty trash',
    })
    if (!ok) return
    setError('')
    try {
      await emptyTrash()
      refresh()
    } catch {
      setError('Failed to empty trash.')
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div ref={dialogRef} role="dialog" aria-modal="true" tabIndex={-1} className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-3xl mx-0 md:mx-4 max-h-full md:max-h-[80vh] h-full md:h-auto flex flex-col overflow-hidden focus:outline-none" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
          <div className="flex items-center gap-2">
            <svg className="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
            </svg>
            <h2 className="text-base font-semibold text-gray-800 dark:text-gray-200">Trash</h2>
            <span className="text-xs text-gray-400">{items.length} items</span>
          </div>
          <div className="flex items-center gap-2">
            {items.length > 0 && (
              <button onClick={handleEmpty} className="text-xs px-2.5 py-1 text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 rounded-md transition">
                Empty Trash
              </button>
            )}
            <button onClick={onClose} className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition">
              <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            </button>
          </div>
        </div>

        {error && (
          <div className="mx-5 mt-3 px-3 py-2 bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-400 text-sm rounded-md flex-shrink-0">
            {error}
          </div>
        )}

        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="text-gray-400 text-center py-12">Loading...</div>
          ) : items.length === 0 ? (
            <div className="text-gray-400 text-center py-12">Trash is empty</div>
          ) : (
            <table className="w-full text-sm">
              <thead className="sticky top-0 bg-gray-50 dark:bg-gray-900">
                <tr className="text-left text-xs text-gray-500 dark:text-gray-400 uppercase">
                  <th className="px-3 md:px-4 py-2 font-medium">Name</th>
                  <th className="px-4 py-2 font-medium hidden md:table-cell">Original Location</th>
                  <th className="px-4 py-2 font-medium hidden sm:table-cell">Size</th>
                  <th className="px-4 py-2 font-medium hidden md:table-cell">Deleted</th>
                  <th className="px-3 md:px-4 py-2 font-medium">Actions</th>
                </tr>
              </thead>
              <tbody>
                {items.map((item) => (
                  <tr key={item.id} className="border-t border-gray-100 dark:border-gray-700 hover:bg-gray-50 dark:hover:bg-gray-700/50">
                    <td className="px-3 md:px-4 py-2 text-gray-800 dark:text-gray-200 truncate max-w-[150px] md:max-w-none">{item.name}</td>
                    <td className="px-4 py-2 text-gray-500 dark:text-gray-400 font-mono text-xs truncate max-w-48 hidden md:table-cell">{item.originalPath}</td>
                    <td className="px-4 py-2 text-gray-500 dark:text-gray-400 whitespace-nowrap hidden sm:table-cell">{formatSize(item.size)}</td>
                    <td className="px-4 py-2 text-gray-500 dark:text-gray-400 whitespace-nowrap hidden md:table-cell">{formatDate(item.deletedAt)}</td>
                    <td className="px-3 md:px-4 py-2">
                      <div className="flex gap-1">
                        <button onClick={() => handleRestore(item.id)} className="text-xs px-2 py-1 text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded transition">
                          Restore
                        </button>
                        <button onClick={() => handleDelete(item.id)} className="text-xs px-2 py-1 text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 rounded transition">
                          Delete
                        </button>
                      </div>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        <div className="px-5 py-2 border-t border-gray-200 dark:border-gray-700 text-xs text-gray-400 flex-shrink-0">
          Items are automatically deleted after 30 days
        </div>
      </div>
    </div>
  )
}
