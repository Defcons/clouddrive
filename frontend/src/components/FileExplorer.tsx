import { useState, useEffect, useCallback } from 'react'
import type { FileItem as FileItemType, ViewMode } from '../types'
import { listFiles, downloadFile, uploadFiles, createFolder, renameFile, deleteFile, logout, isPreviewable, addQuickAccess, setFolderPrivate, removeFolderPrivate } from '../api'
import Breadcrumb from './Breadcrumb'
import Toolbar from './Toolbar'
import FileIcon from './FileIcon'
import ContextMenu from './ContextMenu'
import BulkContextMenu from './BulkContextMenu'
import UploadZone from './UploadZone'
import PreviewModal from './PreviewModal'
import ShareModal from './ShareModal'
import Sidebar from './Sidebar'
import UpdateToast from './UpdateToast'
import ChangelogModal from './ChangelogModal'
import SettingsModal from './SettingsModal'
import { APP_VERSION } from '../changelog'
import { getCurrentUser } from '../api'
import { useTheme } from '../hooks/useTheme'

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

function SortHeader({ label, column, sortBy, sortDir, onClick, className }: {
  label: string
  column: 'name' | 'size' | 'createdAt' | 'modTime'
  sortBy: string
  sortDir: 'asc' | 'desc'
  onClick: (col: 'name' | 'size' | 'createdAt' | 'modTime') => void
  className?: string
}) {
  const active = sortBy === column
  return (
    <th
      className={`font-medium cursor-pointer select-none hover:text-gray-700 transition ${className || ''}`}
      onClick={() => onClick(column)}
    >
      <span className="inline-flex items-center gap-1">
        {label}
        {active && (
          <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d={sortDir === 'asc' ? 'M5 15l7-7 7 7' : 'M19 9l-7 7-7-7'} />
          </svg>
        )}
      </span>
    </th>
  )
}

export default function FileExplorer({ initialPath, onLogout }: { initialPath: string; onLogout: () => void }) {
  const [path, setPath] = useState(initialPath || '/')
  const [history, setHistory] = useState<string[]>([])
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
  const [showSettings, setShowSettings] = useState(false)
  const [sortBy, setSortBy] = useState<'name' | 'size' | 'createdAt' | 'modTime'>('name')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>('asc')
  const user = getCurrentUser()
  const { theme, toggle: toggleTheme } = useTheme()

  const handleSort = (column: 'name' | 'size' | 'createdAt' | 'modTime') => {
    if (sortBy === column) {
      setSortDir((d) => (d === 'asc' ? 'desc' : 'asc'))
    } else {
      setSortBy(column)
      setSortDir(column === 'name' ? 'asc' : 'desc')
    }
  }

  const sortedFiles = [...files].sort((a, b) => {
    // Directories always first
    if (a.isDir !== b.isDir) return a.isDir ? -1 : 1

    let cmp = 0
    switch (sortBy) {
      case 'name':
        cmp = a.name.toLowerCase().localeCompare(b.name.toLowerCase())
        break
      case 'size':
        cmp = (a.size || 0) - (b.size || 0)
        break
      case 'createdAt':
        cmp = (a.createdAt || 0) - (b.createdAt || 0)
        break
      case 'modTime':
        cmp = (a.modTime || 0) - (b.modTime || 0)
        break
    }
    return sortDir === 'asc' ? cmp : -cmp
  })

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
    if (newPath !== path) {
      setHistory((prev) => [...prev, path])
    }
    setPath(newPath)
    setContextMenu(null)
  }

  const goBack = () => {
    if (history.length === 0) return
    const prev = history[history.length - 1]
    setHistory((h) => h.slice(0, -1))
    setPath(prev)
    setContextMenu(null)
  }

  const goUp = () => {
    const homeFolder = user.homeFolder || '/'
    if (path === homeFolder || path === '/') return
    const parent = path.split('/').slice(0, -1).join('/') || '/'
    // Don't go above home folder
    if (homeFolder !== '/' && !parent.startsWith(homeFolder) && parent !== homeFolder) return
    navigate(parent)
  }

  // Handle mouse back button
  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (e.button === 3) { // mouse back button
        e.preventDefault()
        goBack()
      }
    }
    window.addEventListener('mouseup', handler)
    return () => window.removeEventListener('mouseup', handler)
  }, [history, path])

  // Global Escape key — cascading close: modals > context menu > selection > rename
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      if (e.key !== 'Escape') return
      // Modals handle their own Escape, but if none are open, handle here
      if (previewFile || shareFile || showChangelog || showSettings) return
      if (contextMenu) {
        setContextMenu(null)
        return
      }
      if (renaming) {
        setRenaming(null)
        return
      }
      if (selectedFiles.size > 0) {
        setSelectedFiles(new Set())
        setLastClickedPath(null)
        return
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [previewFile, shareFile, showChangelog, showSettings, contextMenu, renaming, selectedFiles])

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
      window.dispatchEvent(new Event('sidebar-refresh'))
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
      window.dispatchEvent(new Event('sidebar-refresh'))
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
      window.dispatchEvent(new Event('sidebar-refresh'))
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

  const handleMakePrivate = async (file: FileItemType) => {
    setContextMenu(null)
    try {
      await setFolderPrivate(file.path)
      await refresh()
    } catch {
      setError('Failed to make folder private')
    }
  }

  const handleMakePublic = async (file: FileItemType) => {
    setContextMenu(null)
    try {
      await removeFolderPrivate(file.path)
      await refresh()
    } catch {
      setError('Failed to make folder public')
    }
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

  const handleClick = (e: React.MouseEvent, file: FileItemType) => {
    // If shift/ctrl held, handle multi-select instead
    if (e.shiftKey || e.ctrlKey || e.metaKey) {
      handleSelectionClick(e, file)
      return
    }
    if (file.isDir) {
      navigate(file.path)
    } else {
      handlePreview(file)
    }
  }

  const handleDoubleClick = (file: FileItemType) => {
    if (!file.isDir) {
      handleDownload(file)
    }
  }

  // Multi-select
  const [selectedFiles, setSelectedFiles] = useState<Set<string>>(new Set())
  const [lastClickedPath, setLastClickedPath] = useState<string | null>(null)

  const selectRange = (fromPath: string, toPath: string) => {
    const fromIdx = sortedFiles.findIndex((f) => f.path === fromPath)
    const toIdx = sortedFiles.findIndex((f) => f.path === toPath)
    if (fromIdx >= 0 && toIdx >= 0) {
      const start = Math.min(fromIdx, toIdx)
      const end = Math.max(fromIdx, toIdx)
      const newSelection = new Set(selectedFiles)
      for (let i = start; i <= end; i++) {
        newSelection.add(sortedFiles[i].path)
      }
      setSelectedFiles(newSelection)
    }
  }

  const handleSelectionClick = (e: React.MouseEvent, file: FileItemType) => {
    if (e.shiftKey && lastClickedPath) {
      // Shift-click: select range from last clicked to current
      selectRange(lastClickedPath, file.path)
    } else {
      // Ctrl/Cmd click: toggle individual
      const newSelection = new Set(selectedFiles)
      if (newSelection.has(file.path)) {
        newSelection.delete(file.path)
      } else {
        newSelection.add(file.path)
      }
      setSelectedFiles(newSelection)
      setLastClickedPath(file.path)
    }
  }

  const handleCheckboxToggle = (e: React.MouseEvent, file: FileItemType) => {
    e.stopPropagation()
    const newSelection = new Set(selectedFiles)
    if (newSelection.has(file.path)) {
      newSelection.delete(file.path)
    } else {
      newSelection.add(file.path)
    }
    setSelectedFiles(newSelection)
    setLastClickedPath(file.path)
  }

  // Shift+mouseenter drag selection
  const handleMouseEnter = (e: React.MouseEvent, file: FileItemType) => {
    if (e.shiftKey && e.buttons === 1 && lastClickedPath) {
      selectRange(lastClickedPath, file.path)
    }
  }

  const getSelectedFileObjects = (): FileItemType[] => {
    return sortedFiles.filter((f) => selectedFiles.has(f.path))
  }

  const handleBulkDownload = async () => {
    setContextMenu(null)
    const selected = getSelectedFileObjects()
    for (const file of selected) {
      try {
        await downloadFile(file.path)
      } catch {
        setError(`Failed to download ${file.name}`)
      }
    }
    setSelectedFiles(new Set())
  }

  const handleBulkDelete = async () => {
    setContextMenu(null)
    const selected = getSelectedFileObjects()
    if (!confirm(`Delete ${selected.length} item${selected.length !== 1 ? 's' : ''}?`)) return
    for (const file of selected) {
      try {
        await deleteFile(file.path)
      } catch {
        setError(`Failed to delete ${file.name}`)
      }
    }
    setSelectedFiles(new Set())
    await refresh()
    window.dispatchEvent(new Event('sidebar-refresh'))
  }

  // Clear selection when path changes
  useEffect(() => {
    setSelectedFiles(new Set())
  }, [path])

  // Right-click on multi-selection
  const handleMultiContextMenu = (e: React.MouseEvent) => {
    if (selectedFiles.size > 1) {
      e.preventDefault()
      setContextMenu({ x: e.clientX, y: e.clientY, file: { name: `${selectedFiles.size} items`, path: '', isDir: false, size: 0, createdAt: 0, modTime: 0 } as FileItemType })
    }
  }

  return (
    <div className="h-screen flex flex-col bg-gray-50 dark:bg-gray-900">
      {/* Header */}
      <header className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-4 py-3 flex-shrink-0">
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
              <span className="text-xs text-gray-400 dark:text-gray-600">|</span>
              <span className="text-xs text-gray-500 dark:text-gray-400">{user.username}</span>
              <button
                onClick={toggleTheme}
                className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition"
                title={theme === 'dark' ? 'Switch to light mode' : 'Switch to dark mode'}
              >
                {theme === 'dark' ? (
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 3v1m0 16v1m9-9h-1M4 12H3m15.364 6.364l-.707-.707M6.343 6.343l-.707-.707m12.728 0l-.707.707M6.343 17.657l-.707.707M16 12a4 4 0 11-8 0 4 4 0 018 0z" />
                  </svg>
                ) : (
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M20.354 15.354A9 9 0 018.646 3.646 9.003 9.003 0 0012 21a9.003 9.003 0 008.354-5.646z" />
                  </svg>
                )}
              </button>
              <button
                onClick={() => setShowSettings(true)}
                className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition"
                title="Settings"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10.325 4.317c.426-1.756 2.924-1.756 3.35 0a1.724 1.724 0 002.573 1.066c1.543-.94 3.31.826 2.37 2.37a1.724 1.724 0 001.066 2.573c1.756.426 1.756 2.924 0 3.35a1.724 1.724 0 00-1.066 2.573c.94 1.543-.826 3.31-2.37 2.37a1.724 1.724 0 00-2.573 1.066c-.426 1.756-2.924 1.756-3.35 0a1.724 1.724 0 00-2.573-1.066c-1.543.94-3.31-.826-2.37-2.37a1.724 1.724 0 00-1.066-2.573c-1.756-.426-1.756-2.924 0-3.35a1.724 1.724 0 001.066-2.573c-.94-1.543.826-3.31 2.37-2.37.996.608 2.296.07 2.572-1.065z" />
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
                </svg>
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
          <div className="flex items-center gap-1.5">
            <button
              onClick={goBack}
              disabled={history.length === 0}
              className="flex items-center gap-1 px-2 py-1 text-xs font-medium text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600 transition disabled:opacity-30"
            >
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
              </svg>
              Back
            </button>
            <button
              onClick={goUp}
              disabled={path === (user.homeFolder || '/')}
              className="flex items-center gap-1 px-2 py-1 text-xs font-medium text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600 transition disabled:opacity-30"
            >
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
              </svg>
              Up
            </button>
            <span className="text-gray-300">|</span>
            <Breadcrumb path={path} homeFolder={user.homeFolder} onNavigate={navigate} />
          </div>
        </div>
      </header>

      {/* Error banner */}
      {error && (
        <div className="bg-red-50 dark:bg-red-900/30 text-red-600 dark:text-red-400 px-4 py-2 text-sm flex items-center justify-between flex-shrink-0">
          <span>{error}</span>
          <button onClick={() => setError('')} className="text-red-400 hover:text-red-600">&times;</button>
        </div>
      )}

      {/* Main area with sidebar */}
      <div className="flex flex-1 min-h-0">
        <Sidebar currentPath={path} homeFolder={user.homeFolder} onNavigate={navigate} onContextMenu={handleContextMenu} />

        {/* File list */}
        <UploadZone onUpload={handleUpload} uploadProgress={uploadProgress}>
          <div className="w-full p-4 overflow-auto flex-1">
          {selectedFiles.size > 0 && (
            <div className="flex items-center gap-3 mb-3 px-2 py-2 bg-blue-50 dark:bg-blue-900/30 rounded-lg border border-blue-200 dark:border-blue-800">
              <span className="text-sm text-blue-700 font-medium">
                {selectedFiles.size} item{selectedFiles.size !== 1 ? 's' : ''} selected
              </span>
              <button
                onClick={handleBulkDownload}
                className="text-xs px-2.5 py-1 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition"
              >
                Download
              </button>
              <button
                onClick={handleBulkDelete}
                className="text-xs px-2.5 py-1 bg-red-500 text-white rounded-md hover:bg-red-600 transition"
              >
                Delete
              </button>
              <button
                onClick={() => setSelectedFiles(new Set())}
                className="text-xs text-blue-600 hover:text-blue-800 ml-auto"
              >
                Clear selection
              </button>
            </div>
          )}
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
            <table className="w-full" style={{ tableLayout: 'fixed' }}>
              <colgroup>
                <col style={{ width: '28px' }} />
                <col />
                <col style={{ width: '100px' }} />
                <col style={{ width: '160px' }} />
                <col style={{ width: '160px' }} />
              </colgroup>
              <thead>
                <tr className="text-left text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider border-b border-gray-200 dark:border-gray-700">
                  <th className="pb-2 pl-1">
                    <input
                      type="checkbox"
                      checked={selectedFiles.size > 0 && selectedFiles.size === files.length}
                      onChange={() => {
                        if (selectedFiles.size === files.length) {
                          setSelectedFiles(new Set())
                        } else {
                          setSelectedFiles(new Set(files.map((f) => f.path)))
                        }
                      }}
                      className="w-3.5 h-3.5 rounded border-gray-300 text-blue-600 focus:ring-blue-500 cursor-pointer"
                    />
                  </th>
                  <SortHeader label="Name" column="name" sortBy={sortBy} sortDir={sortDir} onClick={handleSort} className="pb-2 pl-2" />
                  <SortHeader label="Size" column="size" sortBy={sortBy} sortDir={sortDir} onClick={handleSort} className="pb-2 text-right" />
                  <SortHeader label="Created" column="createdAt" sortBy={sortBy} sortDir={sortDir} onClick={handleSort} className="pb-2 text-right" />
                  <SortHeader label="Modified" column="modTime" sortBy={sortBy} sortDir={sortDir} onClick={handleSort} className="pb-2 text-right pr-2" />
                </tr>
              </thead>
              <tbody>
                {sortedFiles.map((file) => (
                  <tr
                    key={file.path}
                    className={`cursor-pointer group border-b border-gray-100 dark:border-gray-800 last:border-0 ${
                      selectedFiles.has(file.path) ? 'bg-blue-50 dark:bg-blue-900/30' : 'hover:bg-gray-50 dark:hover:bg-gray-800'
                    }`}
                    onClick={(e) => handleClick(e, file)}
                    onDoubleClick={() => handleDoubleClick(file)}
                    onMouseDown={(e) => {
                      if (e.shiftKey) {
                        e.preventDefault() // prevent text selection during drag
                        if (!lastClickedPath) setLastClickedPath(file.path)
                      }
                    }}
                    onMouseEnter={(e) => handleMouseEnter(e, file)}
                    onContextMenu={(e) => {
                      if (selectedFiles.size > 1 && selectedFiles.has(file.path)) {
                        handleMultiContextMenu(e)
                      } else {
                        handleContextMenu(e, file)
                      }
                    }}
                  >
                    <td className="py-2 pl-1">
                      <input
                        type="checkbox"
                        checked={selectedFiles.has(file.path)}
                        onClick={(e) => handleCheckboxToggle(e, file)}
                        onChange={() => {}}
                        className="w-3.5 h-3.5 rounded border-gray-300 text-blue-600 focus:ring-blue-500 cursor-pointer"
                      />
                    </td>
                    <td className="py-2 pl-2">
                      <div className="flex items-center gap-2.5">
                        <div className="relative flex-shrink-0">
                          <FileIcon name={file.name} isDir={file.isDir} />
                          {file.isPrivate && (
                            <svg className="w-2.5 h-2.5 text-orange-500 absolute -bottom-0.5 -right-0.5" fill="currentColor" viewBox="0 0 20 20">
                              <path fillRule="evenodd" d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z" clipRule="evenodd" />
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
                            className="px-1.5 py-0.5 border border-blue-400 rounded text-sm focus:outline-none focus:ring-1 focus:ring-blue-500"
                            onClick={(e) => e.stopPropagation()}
                          />
                        ) : (
                          <span className="text-sm text-gray-800 dark:text-gray-200 group-hover:text-blue-600 dark:group-hover:text-blue-400 transition">
                            {file.name}
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="py-2 text-right text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">
                      {file.isDir
                        ? file.itemCount !== undefined
                          ? `${file.itemCount} item${file.itemCount !== 1 ? 's' : ''}`
                          : '—'
                        : formatSize(file.size)}
                    </td>
                    <td className="py-2 text-right text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">
                      {formatDate(file.createdAt)}
                    </td>
                    <td className="py-2 text-right text-sm text-gray-500 pr-2 whitespace-nowrap">
                      {formatDate(file.modTime)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-3">
              {sortedFiles.map((file) => (
                <div
                  key={file.path}
                  className={`flex flex-col items-center p-3 rounded-lg cursor-pointer group ${
                    selectedFiles.has(file.path) ? 'bg-blue-50 ring-1 ring-blue-200' : 'hover:bg-gray-100'
                  }`}
                  onClick={(e) => handleClick(e, file)}
                  onDoubleClick={() => handleDoubleClick(file)}
                  onContextMenu={(e) => {
                    if (selectedFiles.size > 1 && selectedFiles.has(file.path)) {
                      handleMultiContextMenu(e)
                    } else {
                      handleContextMenu(e, file)
                    }
                  }}
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
      {contextMenu && selectedFiles.size > 1 && (
        <BulkContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          count={selectedFiles.size}
          onDownload={handleBulkDownload}
          onDelete={handleBulkDelete}
          onClose={() => setContextMenu(null)}
        />
      )}
      {contextMenu && selectedFiles.size <= 1 && (
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
          onMakePrivate={() => handleMakePrivate(contextMenu.file)}
          onMakePublic={() => handleMakePublic(contextMenu.file)}
          isPrivate={contextMenu.file.isPrivate}
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

      {showSettings && (
        <SettingsModal onClose={() => setShowSettings(false)} />
      )}

      <UpdateToast />
    </div>
  )
}
