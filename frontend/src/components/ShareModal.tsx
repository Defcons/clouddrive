import { useState } from 'react'
import { createShare } from '../api'
import { useDialog } from '../hooks/useDialog'
import type { FileItem } from '../types'

interface Props {
  file: FileItem
  safe: boolean
  onClose: () => void
}

export default function ShareModal({ file, safe, onClose }: Props) {
  const [shareUrl, setShareUrl] = useState<string | null>(null)
  const [password, setPassword] = useState<string | null>(null)
  const [loading, setLoading] = useState(false)
  const [error, setError] = useState('')
  const [copied, setCopied] = useState(false)
  const [expiresIn, setExpiresIn] = useState(168) // 7 days in hours
  const [mode, setMode] = useState<'download' | 'collaborate'>('download')
  const [generated, setGenerated] = useState(false)

  const handleGenerate = () => {
    setLoading(true)
    setError('')
    createShare(file.path, safe, expiresIn, mode)
      .then((data) => {
        const fullUrl = `${window.location.origin}${data.url}`
        setShareUrl(fullUrl)
        if (data.password) {
          setPassword(data.password)
        }
        setGenerated(true)
      })
      .catch(() => setError('Failed to generate share link'))
      .finally(() => setLoading(false))
  }

  const dialogRef = useDialog<HTMLDivElement>(onClose)

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
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        tabIndex={-1}
        className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-md mx-4 overflow-hidden focus:outline-none"
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
          <p className="text-sm text-gray-600 dark:text-gray-400 mb-1">
            {file.isDir ? 'Folder' : 'File'}: <span className="font-medium text-gray-800 dark:text-gray-200">{file.name}</span>
          </p>

          {!generated && (
            <div className="space-y-3 mb-4">
              {file.isDir && (
                <div>
                  <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Access type</label>
                  <div className="flex gap-2">
                    <button
                      onClick={() => setMode('download')}
                      className={`flex-1 px-3 py-2 text-xs rounded-lg border transition ${mode === 'download' ? 'border-blue-500 bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400' : 'border-gray-200 dark:border-gray-600 text-gray-600 dark:text-gray-400'}`}
                    >
                      <div className="font-medium">Download only</div>
                      <div className="text-[10px] mt-0.5 opacity-70">Browse and download files</div>
                    </button>
                    <button
                      onClick={() => setMode('collaborate')}
                      className={`flex-1 px-3 py-2 text-xs rounded-lg border transition ${mode === 'collaborate' ? 'border-green-500 bg-green-50 dark:bg-green-900/30 text-green-700 dark:text-green-400' : 'border-gray-200 dark:border-gray-600 text-gray-600 dark:text-gray-400'}`}
                    >
                      <div className="font-medium">Collaborate</div>
                      <div className="text-[10px] mt-0.5 opacity-70">Browse, download and upload</div>
                    </button>
                  </div>
                </div>
              )}
              <div>
                <label className="block text-xs font-medium text-gray-500 dark:text-gray-400 mb-1">Expires in</label>
                <select
                  value={expiresIn}
                  onChange={(e) => setExpiresIn(Number(e.target.value))}
                  className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 rounded-lg"
                >
                  <option value={1}>1 hour</option>
                  <option value={24}>1 day</option>
                  <option value={168}>7 days</option>
                  <option value={720}>30 days</option>
                  <option value={8760}>1 year</option>
                </select>
              </div>
              <p className="text-xs text-gray-400">
                {file.isDir ? 'Recipients can browse and download individual files' : 'Direct download link'}
                {safe && ' — password protected'}
              </p>
              <button
                onClick={handleGenerate}
                disabled={loading}
                className={`w-full py-2 rounded-lg text-sm font-medium transition ${
                  safe ? 'bg-green-600 hover:bg-green-700 text-white' : 'bg-blue-600 hover:bg-blue-700 text-white'
                } disabled:opacity-50`}
              >
                {loading ? 'Generating...' : 'Generate Link'}
              </button>
            </div>
          )}

          {loading && generated && <div className="text-gray-400 text-sm py-4 text-center">Generating link...</div>}

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
