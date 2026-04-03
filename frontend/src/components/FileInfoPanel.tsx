import type { FileItem } from '../types'
import { getPreviewUrl } from '../api'
import { TAG_COLORS } from '../types'
import FileIcon from './FileIcon'

interface Props {
  file: FileItem
  onClose: () => void
}

function formatSize(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

function formatDate(ms: number): string {
  if (!ms) return '—'
  return new Date(ms).toLocaleDateString(undefined, {
    year: 'numeric', month: 'long', day: 'numeric',
    hour: '2-digit', minute: '2-digit', second: '2-digit',
  })
}

function getExtension(name: string): string {
  const ext = name.split('.').pop()?.toLowerCase()
  return ext && ext !== name.toLowerCase() ? ext : ''
}

export default function FileInfoPanel({ file, onClose }: Props) {
  const ext = getExtension(file.name)
  const isImage = /\.(jpg|jpeg|png|gif|webp|svg|bmp)$/i.test(file.name)

  return (
    <div className="w-64 flex-shrink-0 bg-white dark:bg-gray-800 border-l border-gray-200 dark:border-gray-700 flex flex-col overflow-hidden">
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-100 dark:border-gray-700">
        <span className="text-xs font-semibold text-gray-500 dark:text-gray-400 uppercase tracking-wider">Details</span>
        <button
          onClick={onClose}
          className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition"
        >
          <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>

      <div className="flex-1 overflow-y-auto p-3 space-y-4">
        {/* Preview / Icon */}
        <div className="flex justify-center py-2">
          {isImage ? (
            <img
              src={getPreviewUrl(file.path)}
              alt={file.name}
              className="max-w-full max-h-40 rounded-lg object-contain"
            />
          ) : (
            <div className="w-16 h-16 flex items-center justify-center">
              {file.isDir ? (
                <svg className="w-14 h-14 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
                  <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
                </svg>
              ) : (
                <FileIcon name={file.name} isDir={false} />
              )}
            </div>
          )}
        </div>

        {/* Name */}
        <div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-0.5">Name</div>
          <div className="text-sm text-gray-800 dark:text-gray-200 font-medium break-all">{file.name}</div>
        </div>

        {/* Type */}
        <div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-0.5">Type</div>
          <div className="text-sm text-gray-800 dark:text-gray-200">
            {file.isDir ? 'Folder' : ext ? ext.toUpperCase() + ' File' : 'File'}
          </div>
        </div>

        {/* Size */}
        <div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-0.5">Size</div>
          <div className="text-sm text-gray-800 dark:text-gray-200">
            {file.isDir
              ? file.itemCount !== undefined ? `${file.itemCount} items` : '—'
              : formatSize(file.size)}
          </div>
        </div>

        {/* Path */}
        <div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-0.5">Location</div>
          <div className="text-sm text-gray-600 dark:text-gray-400 font-mono text-xs break-all">{file.path}</div>
        </div>

        {/* Created */}
        <div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-0.5">Created</div>
          <div className="text-sm text-gray-800 dark:text-gray-200">{formatDate(file.createdAt)}</div>
        </div>

        {/* Modified */}
        <div>
          <div className="text-xs text-gray-500 dark:text-gray-400 mb-0.5">Modified</div>
          <div className="text-sm text-gray-800 dark:text-gray-200">{formatDate(file.modTime)}</div>
        </div>

        {/* Tags */}
        {file.tags && file.tags.length > 0 && (
          <div>
            <div className="text-xs text-gray-500 dark:text-gray-400 mb-1">Tags</div>
            <div className="flex flex-wrap gap-1">
              {file.tags.map((tag) => (
                <span
                  key={tag}
                  className={`w-4 h-4 rounded-full ${TAG_COLORS[tag] || 'bg-gray-400'}`}
                  title={tag}
                />
              ))}
            </div>
          </div>
        )}

        {/* Private */}
        {file.isPrivate && (
          <div className="flex items-center gap-1.5 text-orange-500 text-xs">
            <svg className="w-3.5 h-3.5" fill="currentColor" viewBox="0 0 20 20">
              <path fillRule="evenodd" d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z" clipRule="evenodd" />
            </svg>
            Private folder
          </div>
        )}
      </div>
    </div>
  )
}
