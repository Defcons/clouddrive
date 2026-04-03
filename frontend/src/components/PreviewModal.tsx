import { useState, useEffect } from 'react'
import type { FileItem } from '../types'
import { getPreviewUrl, getPreviewType, fetchTextPreview } from '../api'

interface Props {
  file: FileItem
  onClose: () => void
}

export default function PreviewModal({ file, onClose }: Props) {
  const [textContent, setTextContent] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const type = getPreviewType(file.name)
  const url = getPreviewUrl(file.path)

  useEffect(() => {
    if (type === 'text') {
      setLoading(true)
      fetchTextPreview(file.path)
        .then(setTextContent)
        .catch(() => setTextContent('Failed to load file.'))
        .finally(() => setLoading(false))
    }
  }, [file.path, type])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70" onClick={onClose}>
      <div
        className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl max-w-5xl w-full mx-4 max-h-[90vh] flex flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
          <h2 className="text-sm font-medium text-gray-800 dark:text-gray-200 truncate">{file.name}</h2>
          <button
            onClick={onClose}
            className="p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-lg transition"
          >
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        {/* Content */}
        <div className="flex-1 overflow-auto p-4 flex items-center justify-center min-h-0">
          {type === 'image' && (
            <img
              src={url}
              alt={file.name}
              className="max-w-full max-h-[75vh] object-contain rounded"
            />
          )}

          {type === 'pdf' && (
            <iframe
              src={url}
              className="w-full h-[75vh] rounded border border-gray-200"
              title={file.name}
            />
          )}

          {type === 'video' && (
            <video
              src={url}
              controls
              className="max-w-full max-h-[75vh] rounded"
            >
              Your browser does not support video playback.
            </video>
          )}

          {type === 'audio' && (
            <div className="flex flex-col items-center gap-4 py-8">
              <svg className="w-20 h-20 text-indigo-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M9 19V6l12-3v13M9 19c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zm12-3c0 1.105-1.343 2-3 2s-3-.895-3-2 1.343-2 3-2 3 .895 3 2zM9 10l12-3" />
              </svg>
              <audio src={url} controls className="w-full max-w-md" />
            </div>
          )}

          {type === 'text' && (
            <pre className="w-full h-[75vh] overflow-auto bg-gray-50 rounded-lg border border-gray-200 p-4 text-sm text-gray-800 font-mono whitespace-pre-wrap">
              {loading ? 'Loading...' : textContent}
            </pre>
          )}

          {!type && (
            <div className="text-gray-400 text-center py-8">
              <p>Preview not available for this file type.</p>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
