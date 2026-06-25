import { useState, useEffect, useCallback, useRef } from 'react'
import type { FileItem as FileItemType, ViewMode, Clipboard, TAG_COLORS } from '../types'
import { listFiles, downloadFile, uploadFiles, createFolder, renameFile, deleteFile, logout, isPreviewable, addQuickAccess, setFolderPrivate, removeFolderPrivate, moveFiles, copyFiles, extractZip, compressFiles, setFileTags, getDiskUsage, getPreviewUrl, setBackupTier } from '../api'
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
import AuditLogModal from './AuditLogModal'
import TrashView from './TrashView'
import RecentFiles from './RecentFiles'
import NotificationBell from './NotificationBell'
import { APP_VERSION } from '../changelog'
import { getCurrentUser } from '../api'
import { useTheme } from '../hooks/useTheme'
import { useToast } from '../hooks/useToast'
import ToastContainer from './ToastContainer'
import LoadingSkeleton from './LoadingSkeleton'
import KeyboardHelp from './KeyboardHelp'
import FileInfoPanel from './FileInfoPanel'
import FileFilter, { getFilterExtensions } from './FileFilter'
import BatchRename from './BatchRename'
import SharesManager from './SharesManager'
import { confirm as confirmModal, prompt as promptModal } from './ConfirmModal'

function formatSize(bytes: number): string {
  if (bytes === 0) return '—'
  const units = ['B', 'KB', 'MB', 'GB', 'TB', 'PB']
  // Clamp the index so absurdly large sizes don't index past the array
  // (which would render "undefined").
  const i = Math.min(Math.floor(Math.log(bytes) / Math.log(1024)), units.length - 1)
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
  const [viewMode, setViewMode] = useState<ViewMode>(() => (localStorage.getItem('clouddrive_viewMode') as ViewMode) || 'list')
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
  const [showAuditLog, setShowAuditLog] = useState(false)
  const [showTrash, setShowTrash] = useState(false)
  const [showRecent, setShowRecent] = useState(false)
  const [clipboard, setClipboard] = useState<Clipboard | null>(null)
  const [showKeyboardHelp, setShowKeyboardHelp] = useState(false)
  const [showInfoPanel, setShowInfoPanel] = useState(false)
  const [fileFilter, setFileFilter] = useState('')
  const [showBatchRename, setShowBatchRename] = useState(false)
  const [showSharesManager, setShowSharesManager] = useState(false)
  const [mobileSidebar, setMobileSidebar] = useState(false)
  const [diskUsage, setDiskUsage] = useState<{ totalSize: number; totalSpace: number; freeSpace: number; perUser?: { username: string; size: number }[] } | null>(null)
  const searchRef = useRef<HTMLInputElement>(null)
  const [sortBy, setSortBy] = useState<'name' | 'size' | 'createdAt' | 'modTime'>(() => (localStorage.getItem('clouddrive_sortBy') as any) || 'name')
  const [sortDir, setSortDir] = useState<'asc' | 'desc'>(() => (localStorage.getItem('clouddrive_sortDir') as any) || 'asc')
  const [visibleCount, setVisibleCount] = useState(100)
  const [selectedFiles, setSelectedFiles] = useState<Set<string>>(new Set())
  const [lastClickedPath, setLastClickedPath] = useState<string | null>(null)
  const user = getCurrentUser()
  const { theme, toggle: toggleTheme } = useTheme()
  const toast = useToast()

  // Bookmarkable URLs via hash
  useEffect(() => {
    window.location.hash = encodeURIComponent(path)
  }, [path])

  useEffect(() => {
    const handler = () => {
      const hash = decodeURIComponent(window.location.hash.slice(1))
      if (hash && hash !== path) setPath(hash)
    }
    window.addEventListener('hashchange', handler)
    return () => window.removeEventListener('hashchange', handler)
  }, [path])

  // Save preferences
  useEffect(() => { localStorage.setItem('clouddrive_viewMode', viewMode) }, [viewMode])
  useEffect(() => { localStorage.setItem('clouddrive_sortBy', sortBy) }, [sortBy])
  useEffect(() => { localStorage.setItem('clouddrive_sortDir', sortDir) }, [sortDir])

  // Reset pagination when path changes
  useEffect(() => { setVisibleCount(100) }, [path])

  // Listen for sidebar drop events
  useEffect(() => {
    const handler = (e: Event) => {
      const { paths, destination } = (e as CustomEvent).detail
      moveFiles(paths, destination).then(() => {
        refresh()
        window.dispatchEvent(new Event('sidebar-refresh'))
        toast.success(`Moved ${paths.length} item(s)`)
      }).catch(() => toast.error('Move failed'))
    }
    window.addEventListener('clouddrive-drop', handler)
    return () => window.removeEventListener('clouddrive-drop', handler)
  }, [path])

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

  // Apply file type filter
  const filteredFiles = fileFilter
    ? sortedFiles.filter((f) => {
        if (f.isDir) return true // always show directories
        const exts = getFilterExtensions(fileFilter)
        if (!exts) return true
        const ext = f.name.split('.').pop()?.toLowerCase() || ''
        return exts.includes(ext)
      })
    : sortedFiles

  const refreshSeq = useRef(0)
  const refresh = useCallback(async () => {
    // Guard against a slow response for a previous folder overwriting a newer
    // one when the user navigates quickly: only the latest request may apply.
    const seq = ++refreshSeq.current
    setLoading(true)
    setError('')
    try {
      const data = await listFiles(path)
      if (seq !== refreshSeq.current) return
      setFiles(data)
    } catch (e: any) {
      if (seq !== refreshSeq.current) return
      setError(e.message)
    } finally {
      if (seq === refreshSeq.current) setLoading(false)
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

  // Global keyboard shortcuts
  useEffect(() => {
    const handler = (e: KeyboardEvent) => {
      const isInput = e.target instanceof HTMLInputElement || e.target instanceof HTMLTextAreaElement
      const mod = e.ctrlKey || e.metaKey

      if (e.key === 'Escape') {
        if (previewFile || shareFile || showChangelog || showSettings || showTrash || showRecent || showAuditLog) return
        if (contextMenu) { setContextMenu(null); return }
        if (clipboard) { setClipboard(null); return }
        if (renaming) { setRenaming(null); return }
        if (selectedFiles.size > 0) { setSelectedFiles(new Set()); setLastClickedPath(null); return }
        return
      }

      if (isInput && !mod) return

      if (mod && e.key === 'a') { e.preventDefault(); setSelectedFiles(new Set(files.map((f) => f.path))); return }
      if (mod && e.key === 'c' && !isInput && selectedFiles.size > 0) { setClipboard({ paths: Array.from(selectedFiles), mode: 'copy' }); return }
      if (mod && e.key === 'x' && !isInput && selectedFiles.size > 0) { setClipboard({ paths: Array.from(selectedFiles), mode: 'cut' }); return }
      if (mod && e.key === 'v' && !isInput) { handlePaste(); return }
      if (mod && e.key === 'f') { e.preventDefault(); searchRef.current?.focus(); return }
      if (mod && e.shiftKey && e.key === 'N') { e.preventDefault(); handleNewFolder(); return }

      if ((e.key === 'Delete' || e.key === 'Backspace') && !isInput && selectedFiles.size > 0) {
        const selected = getSelectedFileObjects()
        confirmModal({
          title: 'Move to trash?',
          message: `Move ${selected.length} item(s) to trash?`,
          destructive: true,
          confirmLabel: 'Move to trash',
        }).then((ok) => {
          if (!ok) return
          Promise.all(selected.map((f) => deleteFile(f.path))).then(() => {
            setSelectedFiles(new Set()); refresh(); window.dispatchEvent(new Event('sidebar-refresh'))
          })
        })
        return
      }

      if (e.key === 'F2' && !isInput && selectedFiles.size === 1) {
        const file = files.find((f) => f.path === Array.from(selectedFiles)[0])
        if (file) { setRenaming(file.path); setRenameValue(file.name) }
        return
      }

      // ? — keyboard help
      if (e.key === '?' && !isInput) { setShowKeyboardHelp(true); return }

      // i — toggle info panel
      if (e.key === 'i' && !isInput && !mod && selectedFiles.size === 1) { setShowInfoPanel((v) => !v); return }

      if (e.key === 'Enter' && !isInput && selectedFiles.size === 1) {
        const file = files.find((f) => f.path === Array.from(selectedFiles)[0])
        if (file) { file.isDir ? navigate(file.path) : handlePreview(file) }
        return
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [previewFile, shareFile, showChangelog, showSettings, showTrash, showRecent, showAuditLog, contextMenu, renaming, selectedFiles, clipboard, files, path])

  const handleUpload = async (fileList: FileList) => {
    setUploadProgress(0)
    try {
      await uploadFiles(path, Array.from(fileList), setUploadProgress)
      await refresh()
      toast.success(`Uploaded ${fileList.length} file${fileList.length !== 1 ? 's' : ''}`)
    } catch {
      toast.error('Upload failed')
    } finally {
      setUploadProgress(null)
    }
  }

  const handleNewFolder = async () => {
    const name = await promptModal({
      title: 'New folder',
      message: 'Enter a name for the new folder:',
      prompt: { placeholder: 'Folder name', initialValue: '' },
      confirmLabel: 'Create',
    })
    if (!name) return
    try {
      await createFolder(path, name)
      await refresh()
      window.dispatchEvent(new Event('sidebar-refresh'))
      toast.success(`Created folder "${name}"`)
    } catch {
      toast.error('Failed to create folder')
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
      toast.success(`Renamed to "${renameValue}"`)
    } catch {
      toast.error('Rename failed')
    }
    setRenaming(null)
  }

  const handleDelete = async (file: FileItemType) => {
    setContextMenu(null)
    try {
      await deleteFile(file.path)
      await refresh()
      window.dispatchEvent(new Event('sidebar-refresh'))
      toast.info(`"${file.name}" moved to trash`, {
        label: 'Undo',
        onClick: async () => {
          try {
            const { restoreFromTrash, listTrash } = await import('../api')
            const trashItems = await listTrash()
            const match = trashItems.find((t: any) => t.name === file.name)
            if (match) {
              await restoreFromTrash(match.id)
              await refresh()
              window.dispatchEvent(new Event('sidebar-refresh'))
              toast.success(`Restored "${file.name}"`)
            }
          } catch {
            toast.error('Failed to undo')
          }
        },
      })
    } catch {
      toast.error('Delete failed')
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

  const handleToggleOffsite = async (file: FileItemType) => {
    setContextMenu(null)
    const currentlyOffsite = file.backupTier === 2
    const newTier = currentlyOffsite ? 0 : 2
    try {
      await setBackupTier(file.path, newTier)
      toast.success(currentlyOffsite ? `Removed "${file.name}" from offsite backup` : `Added "${file.name}" to offsite backup`)
      await refresh()
    } catch {
      toast.error('Failed to update backup settings')
    }
  }

  // Disk usage
  useEffect(() => {
    getDiskUsage().then(setDiskUsage).catch(() => {})
    const interval = setInterval(() => getDiskUsage().then(setDiskUsage).catch(() => {}), 5 * 60 * 1000)
    return () => clearInterval(interval)
  }, [])

  // Clipboard operations
  const handleCut = () => {
    const paths = Array.from(selectedFiles)
    if (paths.length === 0) return
    setClipboard({ paths, mode: 'cut' })
    setContextMenu(null)
  }

  const handleCopy = () => {
    const paths = Array.from(selectedFiles)
    if (paths.length === 0) return
    setClipboard({ paths, mode: 'copy' })
    setContextMenu(null)
  }

  const handlePaste = async () => {
    if (!clipboard) return
    try {
      if (clipboard.mode === 'cut') {
        await moveFiles(clipboard.paths, path)
        setClipboard(null)
        toast.success(`Moved ${clipboard.paths.length} item(s)`)
      } else {
        await copyFiles(clipboard.paths, path)
        toast.success(`Copied ${clipboard.paths.length} item(s)`)
      }
      await refresh()
      window.dispatchEvent(new Event('sidebar-refresh'))
    } catch {
      toast.error('Paste failed')
    }
  }

  // Extract / Compress
  const handleExtract = async (file: FileItemType) => {
    setContextMenu(null)
    try {
      const result = await extractZip(file.path)
      await refresh()
      toast.success(`Extracted ${result.extracted} files`)
    } catch {
      toast.error('Extract failed')
    }
  }

  const handleCompress = async () => {
    setContextMenu(null)
    const selected = getSelectedFileObjects()
    const name = await promptModal({
      title: 'Create archive',
      message: `Compress ${selected.length} item(s) into a zip file:`,
      prompt: { placeholder: 'archive.zip', initialValue: 'archive.zip' },
      confirmLabel: 'Create archive',
    })
    if (!name) return
    try {
      await compressFiles(selected.map((f) => f.path), name)
      await refresh()
      toast.success(`Created ${name}`)
    } catch {
      toast.error('Compress failed')
    }
  }

  // Tags
  const handleSetTags = async (file: FileItemType, tags: string[]) => {
    try {
      await setFileTags(file.path, tags)
      await refresh()
    } catch {
      setError('Failed to set tags')
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
  const selectRange = (fromPath: string, toPath: string) => {
    const fromIdx = filteredFiles.findIndex((f) => f.path === fromPath)
    const toIdx = filteredFiles.findIndex((f) => f.path === toPath)
    if (fromIdx >= 0 && toIdx >= 0) {
      const start = Math.min(fromIdx, toIdx)
      const end = Math.max(fromIdx, toIdx)
      const newSelection = new Set(selectedFiles)
      for (let i = start; i <= end; i++) {
        newSelection.add(filteredFiles[i].path)
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
    return filteredFiles.filter((f) => selectedFiles.has(f.path))
  }

  // summarizeFailures builds a short "name1, name2, …" tail for a toast.
  const summarizeFailures = (names: string[]): string =>
    names.slice(0, 3).join(', ') + (names.length > 3 ? `, +${names.length - 3} more` : '')

  const handleBulkDownload = async () => {
    setContextMenu(null)
    const selected = getSelectedFileObjects()
    const failed: string[] = []
    for (const file of selected) {
      try {
        await downloadFile(file.path)
      } catch {
        failed.push(file.name)
      }
    }
    setSelectedFiles(new Set())
    if (failed.length === 0) {
      toast.success(`Downloaded ${selected.length} item${selected.length !== 1 ? 's' : ''}`)
    } else {
      toast.error(`Failed to download ${failed.length} of ${selected.length}: ${summarizeFailures(failed)}`)
    }
  }

  const handleBulkDelete = async () => {
    setContextMenu(null)
    const selected = getSelectedFileObjects()
    const ok = await confirmModal({
      title: 'Move to trash?',
      message: `Delete ${selected.length} item${selected.length !== 1 ? 's' : ''}? They can be restored from Trash.`,
      destructive: true,
      confirmLabel: 'Move to trash',
    })
    if (!ok) return
    const failed: string[] = []
    for (const file of selected) {
      try {
        await deleteFile(file.path)
      } catch {
        failed.push(file.name)
      }
    }
    setSelectedFiles(new Set())
    await refresh()
    window.dispatchEvent(new Event('sidebar-refresh'))
    const okCount = selected.length - failed.length
    if (failed.length === 0) {
      toast.success(`Moved ${okCount} item${okCount !== 1 ? 's' : ''} to trash`)
    } else {
      toast.error(`Deleted ${okCount} of ${selected.length}; failed: ${summarizeFailures(failed)}`)
    }
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
      <header className="bg-white dark:bg-gray-800 border-b border-gray-200 dark:border-gray-700 px-3 md:px-4 py-2 flex-shrink-0">
        <div className="space-y-1.5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-1.5 md:gap-2">
              {/* Hamburger menu for mobile */}
              <button
                onClick={() => setMobileSidebar(true)}
                className="md:hidden p-1.5 -ml-1 text-gray-500 hover:text-gray-700 dark:text-gray-400 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition"
                title="Open sidebar"
              >
                <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
                </svg>
              </button>
              <h1 className="text-lg font-bold text-gray-800 dark:text-gray-100">CloudDrive</h1>
              <button
                onClick={() => setShowChangelog(true)}
                className="px-1.5 sm:px-2 py-0.5 border border-gray-300 text-gray-500 text-xs font-mono rounded-md hover:border-blue-400 hover:text-blue-600 transition"
                title="View changelog"
              >
                v{APP_VERSION}
              </button>
              <span className="hidden sm:inline text-xs text-gray-400 dark:text-gray-600">|</span>
              <span className="hidden sm:inline text-xs text-gray-500 dark:text-gray-400">{user.username}</span>
            </div>
            <div className="flex items-center gap-0.5 md:gap-1">
              <button
                onClick={toggleTheme}
                className="p-1.5 md:p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition"
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
              <NotificationBell onNavigate={navigate} />
              <button
                onClick={() => setShowSharesManager(true)}
                className="p-1.5 md:p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition"
                title="Active Shares"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13.828 10.172a4 4 0 00-5.656 0l-4 4a4 4 0 105.656 5.656l1.102-1.101m-.758-4.899a4 4 0 005.656 0l4-4a4 4 0 00-5.656-5.656l-1.1 1.1" />
                </svg>
              </button>
              {user.role === 'admin' && (
                <button
                  onClick={() => setShowAuditLog(true)}
                  className="hidden sm:inline-flex p-1.5 md:p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition"
                  title="Audit Log"
                >
                  <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
                  </svg>
                </button>
              )}
              <button
                onClick={() => setShowSettings(true)}
                className="p-1.5 md:p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition"
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
            onPaste={handlePaste}
            onNavigate={navigate}
            onShowRecent={() => setShowRecent(true)}
            clipboard={clipboard}
            searchRef={searchRef}
          />
          <div className="flex items-center gap-2 flex-wrap">
            <FileFilter value={fileFilter} onChange={setFileFilter} />
            {selectedFiles.size >= 2 && (
              <button
                onClick={() => setShowBatchRename(true)}
                className="flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-lg hover:bg-gray-200 dark:hover:bg-gray-600 transition"
              >
                Batch Rename
              </button>
            )}
            {selectedFiles.size === 1 && (
              <button
                onClick={() => setShowInfoPanel((v) => !v)}
                className={`p-1.5 rounded-lg transition ${showInfoPanel ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-600' : 'text-gray-500 dark:text-gray-400 hover:bg-gray-200 dark:hover:bg-gray-700'}`}
                title="File info (i)"
              >
                <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                  <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M13 16h-1v-4h-1m1-4h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z" />
                </svg>
              </button>
            )}
          </div>
          <div className="flex items-center gap-1 md:gap-1.5 min-w-0">
            <button
              onClick={goBack}
              disabled={history.length === 0}
              className="flex items-center gap-0.5 md:gap-1 px-1.5 md:px-2 py-1 text-xs font-medium text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600 transition disabled:opacity-30 flex-shrink-0"
            >
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
              </svg>
              <span className="hidden sm:inline">Back</span>
            </button>
            <button
              onClick={goUp}
              disabled={path === (user.homeFolder || '/')}
              className="flex items-center gap-0.5 md:gap-1 px-1.5 md:px-2 py-1 text-xs font-medium text-gray-600 dark:text-gray-300 bg-gray-100 dark:bg-gray-700 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600 transition disabled:opacity-30 flex-shrink-0"
            >
              <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
              </svg>
              <span className="hidden sm:inline">Up</span>
            </button>
            <span className="text-gray-300 flex-shrink-0">|</span>
            <div className="min-w-0 overflow-hidden flex-1">
              <Breadcrumb path={path} homeFolder={user.homeFolder} onNavigate={navigate} />
            </div>
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
        <Sidebar
          currentPath={path}
          homeFolder={user.homeFolder}
          onNavigate={navigate}
          onContextMenu={handleContextMenu}
          onShowTrash={() => setShowTrash(true)}
          diskUsage={diskUsage}
          onDrop={(paths, dest) => moveFiles(paths, dest).then(() => { refresh(); window.dispatchEvent(new Event('sidebar-refresh')) })}
          mobileOpen={mobileSidebar}
          onMobileClose={() => setMobileSidebar(false)}
        />

        {/* File list */}
        <UploadZone onUpload={handleUpload} uploadProgress={uploadProgress}>
          <div className="w-full p-2 md:p-4 overflow-auto flex-1 folder-transition">
          {selectedFiles.size > 0 && (
            <div className="flex items-center gap-2 md:gap-3 mb-3 px-2 py-2 bg-blue-50 dark:bg-blue-900/30 rounded-lg border border-blue-200 dark:border-blue-800 overflow-x-auto scrollbar-none">
              <span className="text-sm text-blue-700 dark:text-blue-300 font-medium whitespace-nowrap flex-shrink-0">
                {selectedFiles.size} item{selectedFiles.size !== 1 ? 's' : ''}
                <span className="hidden sm:inline">
                {' '}selected
                {(() => {
                  const totalSize = getSelectedFileObjects().reduce((sum, f) => sum + (f.isDir ? 0 : f.size), 0)
                  return totalSize > 0 ? ` (${formatSize(totalSize)})` : ''
                })()}
                </span>
              </span>
              <button onClick={handleCut} className="text-xs px-2.5 py-1.5 bg-gray-600 text-white rounded-md hover:bg-gray-700 transition flex-shrink-0">Cut</button>
              <button onClick={handleCopy} className="text-xs px-2.5 py-1.5 bg-gray-600 text-white rounded-md hover:bg-gray-700 transition flex-shrink-0">Copy</button>
              <button onClick={handleBulkDownload} className="text-xs px-2.5 py-1.5 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition flex-shrink-0 hidden sm:block">Download</button>
              <button onClick={handleBulkDelete} className="text-xs px-2.5 py-1.5 bg-red-500 text-white rounded-md hover:bg-red-600 transition flex-shrink-0">Delete</button>
              <button
                onClick={() => setSelectedFiles(new Set())}
                className="text-xs text-blue-600 hover:text-blue-800 ml-auto flex-shrink-0"
              >
                Clear
              </button>
            </div>
          )}
          {loading ? (
            <LoadingSkeleton />
          ) : filteredFiles.length === 0 ? (
            <div className="text-gray-400 text-center py-12">
              <svg className="w-16 h-16 mx-auto mb-3 text-gray-300" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M3 7v10a2 2 0 002 2h14a2 2 0 002-2V9a2 2 0 00-2-2h-6l-2-2H5a2 2 0 00-2 2z" />
              </svg>
              {files.length === 0 ? (
                <>
                  <p>This folder is empty</p>
                  <p className="text-sm mt-1">Drop files here or click Upload</p>
                </>
              ) : (
                <>
                  <p>No files match this filter</p>
                  <p className="text-sm mt-1">Clear the filter to see all {files.length} item(s)</p>
                </>
              )}
            </div>
          ) : viewMode === 'list' ? (
            <table className="w-full" style={{ tableLayout: 'fixed' }}>
              <colgroup>
                <col style={{ width: '36px' }} />
                <col />
                <col style={{ width: '100px' }} />
                <col className="hidden md:table-column" style={{ width: '160px' }} />
                <col className="hidden md:table-column" style={{ width: '160px' }} />
              </colgroup>
              <thead>
                <tr className="text-left text-xs text-gray-500 dark:text-gray-400 uppercase tracking-wider border-b border-gray-200 dark:border-gray-700">
                  <th
                    className="pb-2 text-center cursor-pointer"
                    onClick={() => {
                      // Operate on the visible (filtered) rows, not the full set.
                      if (selectedFiles.size === filteredFiles.length) {
                        setSelectedFiles(new Set())
                      } else {
                        setSelectedFiles(new Set(filteredFiles.map((f) => f.path)))
                      }
                    }}
                  >
                    <input
                      type="checkbox"
                      ref={(el) => {
                        if (el) el.indeterminate = selectedFiles.size > 0 && selectedFiles.size < filteredFiles.length
                      }}
                      checked={filteredFiles.length > 0 && selectedFiles.size === filteredFiles.length}
                      onChange={() => {}}
                      className="w-3.5 h-3.5 rounded border-gray-300 text-blue-600 focus:ring-blue-500 cursor-pointer pointer-events-none"
                    />
                  </th>
                  <SortHeader label="Name" column="name" sortBy={sortBy} sortDir={sortDir} onClick={handleSort} className="pb-2 pl-2" />
                  <SortHeader label="Size" column="size" sortBy={sortBy} sortDir={sortDir} onClick={handleSort} className="pb-2 text-right" />
                  <SortHeader label="Created" column="createdAt" sortBy={sortBy} sortDir={sortDir} onClick={handleSort} className="pb-2 text-right hidden md:table-cell" />
                  <SortHeader label="Modified" column="modTime" sortBy={sortBy} sortDir={sortDir} onClick={handleSort} className="pb-2 text-right pr-2 hidden md:table-cell" />
                </tr>
              </thead>
              <tbody>
                {filteredFiles.slice(0, visibleCount).map((file) => (
                  <tr
                    key={file.path}
                    className={`cursor-pointer group border-b border-gray-100 dark:border-gray-800 last:border-0 ${
                      selectedFiles.has(file.path) ? 'bg-blue-50 dark:bg-blue-900/30' : 'hover:bg-gray-50 dark:hover:bg-gray-800'
                    } ${clipboard?.mode === 'cut' && clipboard.paths.includes(file.path) ? 'opacity-50' : ''}`}
                    draggable
                    onDragStart={(e) => {
                      const paths = selectedFiles.has(file.path) ? Array.from(selectedFiles) : [file.path]
                      e.dataTransfer.setData('application/clouddrive-paths', JSON.stringify(paths))
                      e.dataTransfer.effectAllowed = 'move'
                    }}
                    onDragOver={(e) => {
                      if (file.isDir) { e.preventDefault(); e.currentTarget.style.backgroundColor = 'rgba(59,130,246,0.15)' }
                    }}
                    onDragLeave={(e) => {
                      e.currentTarget.style.backgroundColor = ''
                    }}
                    onDrop={(e) => {
                      e.preventDefault()
                      e.currentTarget.style.backgroundColor = ''
                      if (!file.isDir) return
                      const data = e.dataTransfer.getData('application/clouddrive-paths')
                      if (data) {
                        const paths = JSON.parse(data) as string[]
                        moveFiles(paths, file.path).then(() => { refresh(); window.dispatchEvent(new Event('sidebar-refresh')) })
                      }
                    }}
                    onClick={(e) => handleClick(e, file)}
                    onDoubleClick={() => handleDoubleClick(file)}
                    onTouchStart={(e) => {
                      const timer = setTimeout(() => {
                        const touch = e.touches[0]
                        if (touch) {
                          setContextMenu({ x: touch.clientX, y: touch.clientY, file })
                        }
                      }, 500)
                      ;(e.currentTarget as any)._longPressTimer = timer
                    }}
                    onTouchEnd={(e) => {
                      clearTimeout((e.currentTarget as any)._longPressTimer)
                    }}
                    onTouchMove={(e) => {
                      clearTimeout((e.currentTarget as any)._longPressTimer)
                    }}
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
                    <td
                      className="py-3 md:py-2 cursor-pointer text-center"
                      onClick={(e) => { e.stopPropagation(); handleCheckboxToggle(e, file) }}
                    >
                      <input
                        type="checkbox"
                        checked={selectedFiles.has(file.path)}
                        onChange={() => {}}
                        className="w-4 h-4 md:w-3.5 md:h-3.5 rounded border-gray-300 text-blue-600 focus:ring-blue-500 cursor-pointer pointer-events-none"
                      />
                    </td>
                    <td className="py-3 md:py-2 pl-2">
                      <div className="flex items-center gap-2.5">
                        <div className="relative flex-shrink-0">
                          {!file.isDir && /\.(jpg|jpeg|png|gif|webp|svg|bmp)$/i.test(file.name) ? (
                            <>
                              <img
                                src={getPreviewUrl(file.path)}
                                alt=""
                                className="w-5 h-5 rounded object-cover"
                                loading="lazy"
                                onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; (e.target as HTMLImageElement).nextElementSibling?.classList.remove('hidden') }}
                              />
                              <span className="hidden"><FileIcon name={file.name} isDir={file.isDir} /></span>
                            </>
                          ) : (
                            <FileIcon name={file.name} isDir={file.isDir} />
                          )}
                          {file.isPrivate && (
                            <svg className="w-2.5 h-2.5 text-orange-500 absolute -bottom-0.5 -right-0.5" fill="currentColor" viewBox="0 0 20 20">
                              <path fillRule="evenodd" d="M5 9V7a5 5 0 0110 0v2a2 2 0 012 2v5a2 2 0 01-2 2H5a2 2 0 01-2-2v-5a2 2 0 012-2zm8-2v2H7V7a3 3 0 016 0z" clipRule="evenodd" />
                            </svg>
                          )}
                          {file.backupTier === 2 && !file.isPrivate && (
                            <span title="Offsite backup" className="absolute -bottom-0.5 -right-0.5">
                              <svg className="w-2.5 h-2.5 text-blue-500" fill="currentColor" viewBox="0 0 20 20">
                                <path d="M5.5 16a3.5 3.5 0 01-.369-6.98 4 4 0 117.753-1.977A4.5 4.5 0 1113.5 16h-8z" />
                              </svg>
                            </span>
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
                            className="px-1.5 py-0.5 border border-blue-400 rounded text-sm focus:outline-none focus:ring-1 focus:ring-blue-500 bg-white dark:bg-gray-700 text-gray-800 dark:text-gray-200"
                            onClick={(e) => e.stopPropagation()}
                          />
                        ) : (
                          <span className="text-sm text-gray-800 dark:text-gray-200 group-hover:text-blue-600 dark:group-hover:text-blue-400 transition">
                            {file.name}
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="py-2 md:py-2 text-right text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap">
                      {file.isDir
                        ? file.itemCount !== undefined
                          ? `${file.itemCount} item${file.itemCount !== 1 ? 's' : ''}`
                          : '—'
                        : formatSize(file.size)}
                    </td>
                    <td className="py-2 text-right text-sm text-gray-500 dark:text-gray-400 whitespace-nowrap hidden md:table-cell">
                      {formatDate(file.createdAt)}
                    </td>
                    <td className="py-2 text-right text-sm text-gray-500 pr-2 whitespace-nowrap hidden md:table-cell">
                      {formatDate(file.modTime)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          ) : (
            <div className="grid grid-cols-3 sm:grid-cols-3 md:grid-cols-4 lg:grid-cols-6 gap-2 md:gap-3">
              {filteredFiles.slice(0, visibleCount).map((file) => (
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
                  <div className="w-20 h-20 flex items-center justify-center mb-2 rounded-lg overflow-hidden">
                    {file.isDir ? (
                      <svg className="w-12 h-12 text-blue-400" fill="currentColor" viewBox="0 0 20 20">
                        <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
                      </svg>
                    ) : /\.(jpg|jpeg|png|gif|webp|svg|bmp)$/i.test(file.name) ? (
                      <img
                        src={getPreviewUrl(file.path)}
                        alt={file.name}
                        className="w-full h-full object-cover"
                        loading="lazy"
                        onError={(e) => { (e.target as HTMLImageElement).style.display = 'none'; (e.target as HTMLImageElement).nextElementSibling?.classList.remove('hidden') }}
                      />
                    ) : (
                      <svg className="w-12 h-12 text-gray-300 dark:text-gray-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1} d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
                      </svg>
                    )}
                    {/* Fallback icon if image fails to load */}
                    {/\.(jpg|jpeg|png|gif|webp|svg|bmp)$/i.test(file.name) && (
                      <svg className="w-12 h-12 text-gray-300 dark:text-gray-600 hidden" fill="none" stroke="currentColor" viewBox="0 0 24 24">
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
                      className="w-full px-1 py-0.5 border border-blue-400 rounded text-xs text-center focus:outline-none focus:ring-1 focus:ring-blue-500 bg-white dark:bg-gray-700 text-gray-800 dark:text-gray-200"
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
          {/* Empty space — right-click to paste */}
          <div
            className={`min-h-[120px] flex-1 rounded-lg mt-2 transition ${
              clipboard
                ? 'border-2 border-dashed border-gray-300 dark:border-gray-600 cursor-pointer hover:border-blue-400 dark:hover:border-blue-500 hover:bg-blue-50/30 dark:hover:bg-blue-900/10'
                : ''
            }`}
            onContextMenu={(e) => {
              if (clipboard) {
                e.preventDefault()
                setContextMenu({ x: e.clientX, y: e.clientY, file: { name: '__paste__', path: '', isDir: false, size: 0, createdAt: 0, modTime: 0 } as FileItemType })
              }
            }}
            onClick={() => {
              if (clipboard) handlePaste()
            }}
          >
            {clipboard && (
              <div className="flex items-center justify-center h-full min-h-[120px] text-gray-400 dark:text-gray-600 text-sm">
                <span>Click or right-click to paste {clipboard.paths.length} {clipboard.mode === 'cut' ? 'cut' : 'copied'} item{clipboard.paths.length !== 1 ? 's' : ''} here</span>
              </div>
            )}
          </div>
          {filteredFiles.length > visibleCount && (
            <div className="text-center py-4">
              <button
                onClick={() => setVisibleCount((c) => c + 100)}
                className="px-4 py-2 text-sm text-blue-600 dark:text-blue-400 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded-lg transition"
              >
                Load more ({filteredFiles.length - visibleCount} remaining)
              </button>
            </div>
          )}
          </div>
        </UploadZone>

        {/* File info panel */}
        {showInfoPanel && selectedFiles.size === 1 && (() => {
          const file = files.find((f) => f.path === Array.from(selectedFiles)[0])
          return file ? <FileInfoPanel file={file} onClose={() => setShowInfoPanel(false)} /> : null
        })()}
      </div>

      {/* Context menu */}
      {contextMenu && selectedFiles.size > 1 && (
        <BulkContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          count={selectedFiles.size}
          onDownload={handleBulkDownload}
          onCut={handleCut}
          onCopy={handleCopy}
          onCompress={handleCompress}
          onDelete={handleBulkDelete}
          onClose={() => setContextMenu(null)}
        />
      )}
      {contextMenu && selectedFiles.size <= 1 && contextMenu.file.name === '__paste__' && clipboard && (
        <div
          className="fixed z-50 bg-white dark:bg-gray-800 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700 py-1 w-44"
          style={{ left: Math.min(contextMenu.x, window.innerWidth - 180), top: Math.min(contextMenu.y, window.innerHeight - 60) }}
        >
          <button
            onClick={() => { handlePaste(); setContextMenu(null) }}
            className="w-full text-left px-3 py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
          >
            <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5H7a2 2 0 00-2 2v12a2 2 0 002 2h10a2 2 0 002-2V7a2 2 0 00-2-2h-2M9 5a2 2 0 002 2h2a2 2 0 002-2M9 5a2 2 0 012-2h2a2 2 0 012 2" />
            </svg>
            Paste ({clipboard.paths.length} {clipboard.mode === 'cut' ? 'cut' : 'copied'})
          </button>
        </div>
      )}
      {contextMenu && selectedFiles.size <= 1 && contextMenu.file.name !== '__paste__' && (
        <ContextMenu
          x={contextMenu.x}
          y={contextMenu.y}
          isDir={contextMenu.file.isDir}
          canPreview={isPreviewable(contextMenu.file.name)}
          onPreview={() => handlePreview(contextMenu.file)}
          onShare={() => handleShare(contextMenu.file, false)}
          onSafeShare={() => handleShare(contextMenu.file, true)}
          onDownload={() => handleDownload(contextMenu.file)}
          onCut={() => { setSelectedFiles(new Set([contextMenu.file.path])); handleCut() }}
          onCopy={() => { setSelectedFiles(new Set([contextMenu.file.path])); handleCopy() }}
          onRename={() => handleRenameStart(contextMenu.file)}
          onDelete={() => handleDelete(contextMenu.file)}
          onExtract={() => handleExtract(contextMenu.file)}
          isZip={contextMenu.file.name.toLowerCase().endsWith('.zip')}
          onQuickAccess={() => handleQuickAccess(contextMenu.file)}
          onMakePrivate={() => handleMakePrivate(contextMenu.file)}
          onMakePublic={() => handleMakePublic(contextMenu.file)}
          onToggleOffsite={() => handleToggleOffsite(contextMenu.file)}
          offsiteBackup={contextMenu.file.backupTier === 2}
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

      {showAuditLog && (
        <AuditLogModal onClose={() => setShowAuditLog(false)} />
      )}

      {showTrash && (
        <TrashView onClose={() => setShowTrash(false)} onNavigate={navigate} />
      )}

      {showRecent && (
        <RecentFiles onClose={() => setShowRecent(false)} onNavigate={navigate} />
      )}

      {showSharesManager && (
        <SharesManager onClose={() => setShowSharesManager(false)} />
      )}

      {showKeyboardHelp && (
        <KeyboardHelp onClose={() => setShowKeyboardHelp(false)} />
      )}

      {showBatchRename && (
        <BatchRename
          files={getSelectedFileObjects().map((f) => ({ name: f.name, path: f.path }))}
          onRename={async (renames) => {
            for (const r of renames) {
              try { await renameFile(r.oldPath, r.newName) } catch {}
            }
            toast.success(`Renamed ${renames.length} files`)
            setSelectedFiles(new Set())
            refresh()
            window.dispatchEvent(new Event('sidebar-refresh'))
          }}
          onClose={() => setShowBatchRename(false)}
        />
      )}

      <UpdateToast />
      <ToastContainer toasts={toast.toasts} onDismiss={toast.removeToast} />
    </div>
  )
}
