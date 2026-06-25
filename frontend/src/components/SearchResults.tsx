import { useState, useEffect, useRef } from 'react'
import { searchFiles } from '../api'
import type { FileItem } from '../types'
import FileIcon from './FileIcon'

interface Props {
  query: string
  onNavigate: (path: string) => void
  onClose: () => void
}

export default function SearchResults({ query, onNavigate, onClose }: Props) {
  const [results, setResults] = useState<FileItem[]>([])
  const [loading, setLoading] = useState(false)
  const [inFiles, setInFiles] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!query || query.length < 2) {
      setResults([])
      return
    }
    setLoading(true)
    // Guard against a slow earlier query resolving after a newer one (and
    // against setState after unmount).
    let cancelled = false
    const timer = setTimeout(() => {
      searchFiles(query, inFiles)
        .then((r) => { if (!cancelled) setResults(r) })
        .catch(() => { if (!cancelled) setResults([]) })
        .finally(() => { if (!cancelled) setLoading(false) })
    }, 300)
    return () => {
      cancelled = true
      clearTimeout(timer)
    }
  }, [query, inFiles])

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose()
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [onClose])

  if (!query || query.length < 2) return null

  return (
    <div ref={ref} className="absolute top-full left-0 right-0 mt-1 bg-white dark:bg-gray-800 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700 max-h-80 overflow-y-auto z-50">
      <label className="flex items-center gap-2 px-3 py-2 border-b border-gray-100 dark:border-gray-700 text-xs text-gray-500 dark:text-gray-400 cursor-pointer select-none">
        <input type="checkbox" checked={inFiles} onChange={(e) => setInFiles(e.target.checked)} className="rounded border-gray-300 text-blue-600 focus:ring-blue-500" />
        Search inside file contents
      </label>
      {loading ? (
        <div className="p-4 text-center text-gray-400 text-sm">Searching...</div>
      ) : results.length === 0 ? (
        <div className="p-4 text-center text-gray-400 text-sm">No results found</div>
      ) : (
        results.map((file) => (
          <button
            key={file.path}
            onClick={() => {
              onNavigate(file.isDir ? file.path : file.path.split('/').slice(0, -1).join('/') || '/')
              onClose()
            }}
            className="w-full flex items-center gap-2.5 px-3 py-2 text-left hover:bg-gray-50 dark:hover:bg-gray-700 transition"
          >
            <FileIcon name={file.name} isDir={file.isDir} />
            <div className="flex-1 min-w-0">
              <div className="text-sm text-gray-800 dark:text-gray-200 truncate">{file.name}</div>
              <div className="text-xs text-gray-400 truncate">{file.path}</div>
              {(file as any).snippet && (
                <div className="text-xs text-gray-500 dark:text-gray-400 truncate italic mt-0.5">{(file as any).snippet}</div>
              )}
            </div>
          </button>
        ))
      )}
    </div>
  )
}
