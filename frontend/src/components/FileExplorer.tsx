import { useState, useEffect, useCallback } from 'react'
import type { FileItem as FileItemType, ViewMode } from '../types'
import { listFiles, downloadFile, uploadFiles, createFolder, renameFile, deleteFile, logout, isPreviewable, addQuickAccess } from '../api'
import Breadcrumb from './Breadcrumb'
import Toolbar from './Toolbar'
import FileIcon from './FileIcon'
import ContextMenu from './ContextMenu'
import UploadZone from './UploadZone'
import PreviewModal from './PreviewModal'
import ShareModal from './ShareModal'
import Sidebar from './Sidebar'
import UpdateToast from './UpdateToast'
import ChangelogModal from './ChangelogModal'
import { APP_VERSION } from '../changelog'

function formatSize(bytes: number): string {
  if (bytes === 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(i > 0 ? 1 : 0)} ${units[i]}`
}

function formatDate(ms: number): string {
  return new Date(ms).toLocaleDateString(undefined, {
    year: 'numeric',
    month: 'short',
    day: 'numeric',
    hour: '2-digit',
    minute: '2-digit',
  })
}

export default function FileExplorer({ onLogout }: { onLogout: () => void }) {
  const [path, setPath] = useState('/')
  const [files, setFiles] = useState<FileItemType[]>([])
  const [viewMode, setViewMode] = useState<ViewMode>('list')
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [uploadProgress, setUploadProgress] = useState<number | null>(null)
  const [contextMenu, setContextMenu] = useState<{ x: number; y: number; file: FileItemType } | null>(null)
  const [renaming, setRenaming] = useState<string | null>(null)
  const [renameValue, setRenameValue] = useState('')
  const [previewFile, setPreviewFile] = useState<FileItemType | null>(null)
  const [shareFile, setShareFile] = useState<FileItemType | null>(null)
  const [shareSafe, setShareSafe] = useState(false)
  const [showChangelog, setShowChangelog] = useState(false)

  const refresh = useCallback(async () => {
    setLoading(true)
    setError('')
    try {
      const data = await listFiles(path)
      setFiles(data)
    } catch (e: any) {
      setError(e.message)
    } finally {
      setLoading(false)
    }
  }, [path])

  useEffect(() => {
    refresh()
  }, [refresh])

  const navigate = (newPath: string) => {
    setPath(newPath)
    setContextMenu(null)
  }

  const handleUpload = async (fileList: FileList) => {
    setUploadProgress(0)
    try {
      await uploadFiles(path, Array.from(fileList), setUploadProgress)
      await refresh()
    } catch {
      setError('Upload failed')
    } finally {
      setUploadProgress(null)
    }
  }

  const handleNewFolder = async () => {
    const name = prompt('Folder name:')
    if (!name) return
    try {
      await createFolder(path, name)
      await refresh()
    } catch {
      setError('Failed to create folder')
    }
  }

  const handleDownload = async (file: FileItemType) => {
    setContextMenu(null)
    try {
      await downloadFile(file.path)
    } catch {
      setError('Download failed')
    }
  }

  const handleRenameStart = (file: FileItemType) => {
    setContextMenu(null)
    setRenaming(file.path)
    setRenameValue(file.name)
  }

  const handleRenameSubmit = async (file: FileItemType) => {
    if (!renameValue || renameValue === file.name) {
      setRenaming(null)
      return
    }
    try {
      await renameFile(file.path, renameValue)
      await refresh()
    } catch {
      setError('Rename failed')
    }
    setRenaming(null)
  }

  const handleDelete = async (file: FileItemType) => {
    setContextMenu(null)
    if (!confirm(`Delete "${file.name}"?`)) return
    try {
      await deleteFile(file.path)
      await refresh()
    } catch {
      setError('Delete failed')
    }
  }

  const handleContextMenu = (e: React.MouseEvent, file: FileItemType) => {
    e.preventDefault()
    setContextMenu({ x: e.clientX, y: e.clientY, file })
  }

  const handlePreview = (file: FileItemType) => {
    setContextMenu(null)
    setPreviewFile(file)
  }

  const handleQuickAccess = (file: FileItemType) => {
    setContextMenu(null)
    addQuickAccess(file.name, file.path)
    window.dispatchEvent(new Event('quickaccess-updated'))
  }

  const handleShare = (file: FileItemType, safe = false) => {
    setContextMenu(null)
    setShareSafe(safe)
    setShareFile(file)
  }

  const handleLogout = () => {
    logout()
    onLogout()
  }

  const handleClick = (file: FileItemType) => {
    if (file.isDir) {
      navigate(file.path)
    }
  }

  const handleDoubleClick = (file: FileItemType) => {
    if (!file.isDir) {
      handleDownload(file)
    }
  }

  return (
    <div className="h-screen flex flex-col">
      {/* Header */}
      <header className="bg-white border-b border-gray-200 px-4 py-3 flex-shrink-0">
        <div className="max-w-7xl mx-auto space-y-2">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <h1 className="text-lg font-bold text-gray-800">CloudDrive</h1>
              <button
                onClick={() => setShowChangelog(true)}
                className="px-2 py-0.5 border border-gray-300 text-gray-500 text-xs font-mono rounded-md hover:border-blue-400 hover:text-blue-600 transition"
                title="View changelog"
              >
                v{APP_VERSION}
              </button>
            </div>
          </div>
          <Toolbar
            viewMode={viewMode}
            onViewModeChange={setViewMode}
            onUpload={handleUpload}
            onNewFolder={handleNewFolder}
            onRefresh={refresh}
            onLogout={handleLogout}
          />
          <Breadcrumb path={path} onNavigate={navigate} />
        </div>
      </header>

      {/* Error banner */}
      {error && (
        <div className="bg-red-50 text-red-600 px-4 py-2 text-sm flex items-center justify-between flex-shrink-0">
          <span>{error}</span>
          <button onClick={() => setError('')} className="text-red-400 hover:text-red-600">&times;</button>
        </div>
      )}

      {/* Main area with sidebar */}
      <div className="flex flex-1 min-h-0">
        <Sidebar currentPath={path} onNavigate={navigate} />

        {/* File list */}
        <UploadZone onUpload={handleUpload} uploadProgress={uploadProgress}>
          <div className="w-full p-4 overflow-auto flex-1">
          {loading ? (
            <div className="text-gray-400 text-center py-12">Loading...</div>
          ) : files.length === 0 ? (
            <div className="text-gray-400 text-center py-12">
              <svg className="w-16 h-16 mx-auto mb-3 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
              </svg>
              <p>This folder is empty</p>
              <p className="text-sm mt-1">Drop files here or click Upload</p>
            </div>
          ) : viewMode === 'list' ? (
            <table className="w-full">
              <thead>
                <tr className="text-left text-xs text-gray-500 uppercase tracking-wider border-b border-gray-200">
                  <th className="pb-2 pl-2 font-medium">Name</th>
                  <th className="pb-2 font-medium w-24 text-right">Size</th>
                  <th className="pb-2 font-medium w-44 text-right pr-2">Modified</th>
                </tr>
              </thead>
              <tbody>
                {files.map((file) => (
                  <tr
                    key={file.path}
                    className="hover:bg-gray-50 cursor-pointer group border-b border-gray-100 last:border-0"
                    onClick={() => handleClick(file)}
                    onDoubleClick={() => handleDoubleClick(file)}
                    onContextMenu={(e) => handleContextMenu(e, file)}
                  >
                    <td className="py-2 pl-2">
                      <div className="flex items-center gap-2.5">
                        <FileIcon name={file.name} isDir={file.isDir} />
                        {renaming === file.path ? (
                          <input
                            autoFocus
                            value={renameValue}
                            onChange={(e) => setRenameValue(e.target.value)}
                            onBlur={() => handleRenameSubmit(file)}
                            onKeyDown={(e) => {
                              if (e.key === 'Enter') handleRenameSubmit(file)
                              if (e.key === 'Escape') setRenaming(null)
                            }}
                            className="px-1.5 py-0.5 border border-blue-400 rounded text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
                            onClick={(e) => e.stopPropagation()}
                          />
                        ) : (
                          <span className="text-sm text-gray-800 group-hover:text-blue-600 transition">
                            {file.name}
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="py-2 text-right text-sm text-gray-500">
                      {file.isDir ? '—' : formatSize(file.size)}
                    </td>
                    <td className="py-2 text-right text-sm text-gray-500 pr-2">
                      {formatDate(file.modTime)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
              {files.map((file) => (
                <div
                  key={file.path}
                  className="flex flex-col items-center p-3 rounded-lg hover:bg-gray-100 cursor-pointer group"
                  onDoubleClick={() => handleDoubleClick(file)}
                  onContextMenu={(e) => handleContextMenu(e, file)}
                >
                  <div className="w-12 h-12 flex items-center justify-center mb-2">
                    {file.isDir ? (
                      <svg className="w-10 h-10 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
                        <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
                      </svg>
                    ) : (
                      <svg className="w-10 h-10 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
                      </svg>
                    )}
                  </div>
                  {renaming === file.path ? (
                    <input
                      autoFocus
                      value={renameValue}
                      onChange={(e) => setRenameValue(e.target.value)}
                      onBlur={() => handleRenameSubmit(file)}
                      onKeyDown={(e) => {
                        if (e.key === 'Enter') handleRenameSubmit(file)
                        if (e.key === 'Escape') setRenaming(null)
                      }}
                      className="w-full px-1 py-0.5 border border-blue-400 rounded text-xs text-center focus:outline-none focus:ring-1 focus:ring-blue-500"
                      onClick={(e) => e.stopPropagation()}
                    />
                  ) : (
                    <span className="text-xs text-gray-700 text-center truncate w-full group-hover:text-blue-600">
                      {file.name}
                    </span>
                  )}
                </div>
              ))}
            </div>
          )}
          </div>
        </UploadZone>
      </div>

      {/* Context menu */}
      {contextMenu && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          isDir={contextMenu.file.isDir}
          canPreview={isPreviewable(contextMenu.file.name)}
          onPreview={() => handlePreview(contextMenu.file)}
          onShare={() => handleShare(contextMenu.file, false)}
          onSafeShare={() => handleShare(contextMenu.file, true)}
          onDownload={() => handleDownload(contextMenu.file)}
          onRename={() => handleRenameStart(contextMenu.file)}
          onDelete={() => handleDelete(contextMenu.file)}
          onQuickAccess={() => handleQuickAccess(contextMenu.file)}
          onClose={() => setContextMenu(null)}
        />
      )}

      {previewFile && (
        <PreviewModal file={previewFile} onClose={() => setPreviewFile(null)} />
      )}

      {shareFile && (
        <ShareModal file={shareFile} safe={shareSafe} onClose={() => setShareFile(null)} />
      )}

      {showChangelog && (
        <ChangelogModal onClose={() => setShowChangelog(false)} />
      )}

      <UpdateToast />
    </div>
  )
}
