import { useState, useEffect } from 'react'
import { getRecentFiles } from '../api'
import type { FileItem } from '../types'
import FileIcon from './FileIcon'

interface Props {
  onNavigate: (path: string) => void
  onClose: () => void
}

function formatDate(ms: number): string {
  return new Date(ms).toLocaleDateString(undefined, {
    year: 'numeric', month: 'short', day: 'numeric',
    hour: '2-digit', minute: '2-digit',
  })
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

export default function RecentFiles({ onNavigate, onClose }: Props) {
  const [files, setFiles] = useState<FileItem[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    getRecentFiles()
      .then(setFiles)
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-2xl mx-4 max-h-[80vh] flex flex-col overflow-hidden" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
          <h2 className="text-base font-semibold text-gray-800 dark:text-gray-200">Recent Files</h2>
          <button onClick={onClose} className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition">
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div className="flex-1 overflow-y-auto">
          {loading ? (
            <div className="text-gray-400 text-center py-12">Loading...</div>
          ) : files.length === 0 ? (
            <div className="text-gray-400 text-center py-12">No recent files</div>
          ) : (
            files.map((file) => (
              <button
                key={file.path}
                onClick={() => {
                  onNavigate(file.path.split('/').slice(0, -1).join('/') || '/')
                  onClose()
                }}
                className="w-full flex items-center gap-3 px-4 py-2.5 hover:bg-gray-50 dark:hover:bg-gray-700 transition text-left"
              >
                <FileIcon name={file.name} isDir={false} />
                <div className="flex-1 min-w-0">
                  <div className="text-sm text-gray-800 dark:text-gray-200 truncate">{file.name}</div>
                  <div className="text-xs text-gray-400 truncate">{file.path}</div>
                </div>
                <div className="text-xs text-gray-400 whitespace-nowrap">{formatSize(file.size)}</div>
                <div className="text-xs text-gray-400 whitespace-nowrap">{formatDate(file.modTime)}</div>
              </button>
            ))
          )}
        </div>
      </div>
    </div>
  )
}
