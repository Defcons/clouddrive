import { useState, useEffect } from 'react'
import { createShare } from '../api'
import type { FileItem } from '../types'

interface Props {
  file: FileItem
  safe: boolean
  onClose: () => void
}

export default function ShareModal({ file, safe, onClose }: Props) {
  const [shareUrl, setShareUrl] = useState<string | null>(null)
  const [password, setPassword] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState(false)

  useEffect(() => {
    setLoading(true)
    createShare(file.path, safe)
      .then((data) => {
        const fullUrl = `${window.location.origin}${data.url}`
        setShareUrl(fullUrl)
        if (data.password) {
          setPassword(data.password)
        }
      })
      .catch(() => setError('Failed to generate share link'))
      .finally(() => setLoading(false))
  }, [file.path, safe])

  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose()
    }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  const handleCopy = async () => {
    if (!shareUrl) return
    let text = shareUrl
    if (password) {
      text = `${shareUrl}\nPassword: ${password}`
    }
    try {
      await navigator.clipboard.writeText(text)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    } catch {
      const input = document.createElement('textarea')
      input.value = text
      document.body.appendChild(input)
      input.select()
      document.execCommand('copy')
      document.body.removeChild(input)
      setCopied(true)
      setTimeout(() => setCopied(false), 2000)
    }
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-md mx-4 overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
          <div className="flex items-center gap-2">
            {safe ? (
              <svg className="w-5 h-5 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
              </svg>
            ) : (
              <svg className="w-5 h-5 text-blue-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z" />
              </svg>
            )}
            <h2 className="text-base font-semibold text-gray-800">
              {safe ? 'Safe Share' : 'Share'}
            </h2>
          </div>
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
        <div className="px-5 py-4">
          <p className="text-sm text-gray-600 mb-1">
            {file.isDir ? 'Folder' : 'File'}: <span className="font-medium text-gray-800">{file.name}</span>
          </p>
          <p className="text-xs text-gray-400 mb-4">
            {file.isDir ? 'Will be downloaded as a .zip' : 'Direct download link'} — expires in 7 days
            {safe && ' — password protected'}
          </p>

          {loading && <div className="text-gray-400 text-sm py-4 text-center">Generating link...</div>}

          {error && <div className="text-red-500 text-sm py-4 text-center">{error}</div>}

          {shareUrl && (
            <div className="space-y-3">
              <div>
                <label className="block text-xs font-medium text-gray-500 mb-1">Link</label>
                <input
                  readOnly
                  value={shareUrl}
                  className="w-full px-3 py-2 bg-gray-50 border border-gray-200 rounded-lg text-sm text-gray-700 font-mono truncate focus:outline-none"
                  onFocus={(e) => e.target.select()}
                />
              </div>

              {password && (
                <div>
                  <label className="block text-xs font-medium text-gray-500 mb-1">Password</label>
                  <input
                    readOnly
                    value={password}
                    className="w-full px-3 py-2 bg-gray-50 border border-gray-200 rounded-lg text-sm text-gray-700 font-mono focus:outline-none"
                    onFocus={(e) => e.target.select()}
                  />
                </div>
              )}

              <button
                onClick={handleCopy}
                className={`w-full px-4 py-2.5 rounded-lg text-sm font-medium transition ${
                  copied
                    ? 'bg-green-100 text-green-700'
                    : safe
                      ? 'bg-green-600 text-white hover:bg-green-700'
                      : 'bg-blue-600 text-white hover:bg-blue-700'
                }`}
              >
                {copied
                  ? 'Copied!'
                  : password
                    ? 'Copy Link + Password'
                    : 'Copy Link'
                }
              </button>
            </div>
          )}
        </div>
      </div>
    </div>
  )
}
