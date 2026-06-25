import { useState, useEffect, useRef } from 'react'
import type { FileItem } from '../types'
import { getPreviewUrl, getPreviewType, fetchTextPreview, downloadFile } from '../api'

interface Props {
  file: FileItem
  onClose: () => void
}

// Cap rendered text so a multi-MB log doesn't freeze the tab in a <pre>.
const MAX_TEXT_PREVIEW = 200_000

export default function PreviewModal({ file, onClose }: Props) {
  const [textContent, setTextContent] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const type = getPreviewType(file.name)
  const url = getPreviewUrl(file.path)
  const dialogRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (type !== 'text') return
    // Guard against a slow fetch for a previous file overwriting a newer one.
    let cancelled = false
    setLoading(true)
    setTextContent(null)
    fetchTextPreview(file.path)
      .then((t) => { if (!cancelled) setTextContent(t) })
      .catch(() => { if (!cancelled) setTextContent('Failed to load file.') })
      .finally(() => { if (!cancelled) setLoading(false) })
    return () => { cancelled = true }
  }, [file.path, type])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  // Move focus into the dialog on open and restore it to the triggering
  // element on close, so keyboard/screen-reader users aren't stranded.
  useEffect(() => {
    const prevFocus = document.activeElement as HTMLElement | null
    dialogRef.current?.focus()
    return () => prevFocus?.focus?.()
  }, [])

  // Minimal focus trap: keep Tab cycling within the dialog.
  const trapTab = (e: React.KeyboardEvent) => {
    if (e.key !== 'Tab') return
    const focusables = dialogRef.current?.querySelectorAll<HTMLElement>(
      'button, [href], input, textarea, select, [tabindex]:not([tabindex="-1"])',
    )
    if (!focusables || focusables.length === 0) return
    const first = focusables[0]
    const last = focusables[focusables.length - 1]
    if (e.shiftKey && document.activeElement === first) {
      e.preventDefault()
      last.focus()
    } else if (!e.shiftKey && document.activeElement === last) {
      e.preventDefault()
      first.focus()
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70" onClick={onClose}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-label={file.name}
        tabIndex={-1}
        onKeyDown={trapTab}
        className="bg-white dark:bg-gray-800 rounded-none md:rounded-xl shadow-2xl max-w-5xl w-full mx-0 md:mx-4 max-h-full md:max-h-[90vh] h-full md:h-auto flex flex-col overflow-hidden outline-none"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-4 py-3 border-b border-gray-200 dark:border-gray-700 flex-shrink-0">
          <h2 className="text-sm font-medium text-gray-800 dark:text-gray-200 truncate">{file.name}</h2>
          <div className="flex items-center gap-1">
            <button
              onClick={() => downloadFile(file.path)}
              className="flex items-center gap-1 px-2.5 py-1 text-xs font-medium text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600 transition"
            >
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
              </svg>
              Download
            </button>
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
              {loading
                ? 'Loading...'
                : textContent && textContent.length > MAX_TEXT_PREVIEW
                  ? textContent.slice(0, MAX_TEXT_PREVIEW) +
                    '\n\n— preview truncated; download the file to see the rest —'
                  : textContent}
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
