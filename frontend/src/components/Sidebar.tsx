import { useState, useEffect, useCallback } from 'react'
import { listFiles, getQuickAccess, removeQuickAccess } from '../api'
import type { FileItem } from '../types'

interface TreeNode {
  name: string
  path: string
  children: TreeNode[] | null // null = not loaded yet
  loading: boolean
}

interface Props {
  currentPath: string
  homeFolder: string
  onNavigate: (path: string) => void
  onContextMenu: (e: React.MouseEvent, file: FileItem) => void
  onShowTrash: () => void
  diskUsage: { totalSize: number; totalSpace: number; freeSpace: number; perUser?: { username: string; size: number }[] } | null
  onDrop?: (paths: string[], destination: string) => void
  mobileOpen?: boolean
  onMobileClose?: () => void
}

function SidebarItem({
  node,
  depth,
  currentPath,
  onNavigate,
  onToggle,
  onContextMenu,
  expanded,
}: {
  node: TreeNode
  depth: number
  currentPath: string
  onNavigate: (path: string) => void
  onToggle: (path: string) => void
  onContextMenu: (e: React.MouseEvent, file: FileItem) => void
  expanded: boolean
}) {
  const isActive = currentPath === node.path
  const hasChildren = node.children && node.children.length > 0

  return (
    <div>
      <button
        onClick={() => {
          if (isActive) {
            onToggle(node.path)
          } else {
            onNavigate(node.path)
          }
        }}
        onContextMenu={(e) => {
          e.preventDefault()
          e.stopPropagation()
          onContextMenu(e, { name: node.name, path: node.path, isDir: true, size: 0, createdAt: 0, modTime: 0 })
        }}
        onDragOver={(e) => {
          e.preventDefault()
          e.stopPropagation()
          e.currentTarget.style.backgroundColor = 'rgba(59,130,246,0.15)'
        }}
        onDragLeave={(e) => {
          e.currentTarget.style.backgroundColor = ''
        }}
        onDrop={(e) => {
          e.preventDefault()
          e.stopPropagation()
          e.currentTarget.style.backgroundColor = ''
          const data = e.dataTransfer.getData('application/clouddrive-paths')
          if (data) {
            const event = new CustomEvent('clouddrive-drop', { detail: { paths: JSON.parse(data), destination: node.path } })
            window.dispatchEvent(event)
          }
        }}
        className={`w-full flex items-center gap-1.5 py-1 px-2 text-left text-sm rounded-md transition group ${
          isActive
            ? 'bg-blue-50 dark:bg-blue-900/30 text-blue-700 dark:text-blue-400 font-medium'
            : 'text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700'
        }`}
        style={{ paddingLeft: `${8 + depth * 16}px` }}
      >
        {/* Chevron */}
        <span className="w-4 h-4 flex items-center justify-center flex-shrink-0">
          {node.loading ? (
            <svg className="w-3 h-3 text-gray-400 animate-spin" fill="none" viewBox="0 0 24 24">
              <circle className="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" strokeWidth="4" />
              <path className="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z" />
            </svg>
          ) : hasChildren || node.children === null ? (
            <svg
              className={`w-3 h-3 text-gray-400 transition-transform ${expanded ? 'rotate-90' : ''}`}
              fill="none"
              stroke="currentColor"
              viewBox="0 0 24 24"
            >
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
            </svg>
          ) : null}
        </span>

        {/* Folder icon */}
        <svg
          className={`w-4 h-4 flex-shrink-0 ${isActive ? 'text-blue-500' : 'text-blue-400'}`}
          fill="currentColor"
          viewBox="0 0 20 20"
        >
          {expanded ? (
            <path
              fillRule="evenodd"
              d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v1H8a3 3 0 00-3 3v1.5a1.5 1.5 0 01-3 0V6z"
              clipRule="evenodd"
            />
          ) : (
            <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
          )}
        </svg>

        {/* Name */}
        <span className="truncate">{node.name}</span>
      </button>

      {/* Children */}
      {expanded && node.children && node.children.length > 0 && (
        <div>
          {node.children.map((child) => (
            <SidebarItemWrapper
              key={child.path}
              node={child}
              depth={depth + 1}
              currentPath={currentPath}
              onNavigate={onNavigate}
              onContextMenu={onContextMenu}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function SidebarItemWrapper({
  node,
  depth,
  currentPath,
  onNavigate,
  onContextMenu,
}: {
  node: TreeNode
  depth: number
  currentPath: string
  onNavigate: (path: string) => void
  onContextMenu: (e: React.MouseEvent, file: FileItem) => void
}) {
  const [expanded, setExpanded] = useState(false)
  const [manuallyExpanded, setManuallyExpanded] = useState(false)
  const [autoExpanded, setAutoExpanded] = useState(false)
  const [localNode, setLocalNode] = useState(node)

  // Keep in sync with parent
  useEffect(() => {
    setLocalNode((prev) => ({
      ...node,
      children: prev.children ?? node.children,
      loading: prev.loading,
    }))
  }, [node])

  // Auto-expand if current path is inside this node
  useEffect(() => {
    const isInside = currentPath.startsWith(node.path + '/') || currentPath === node.path
    if (isInside) {
      if (!expanded) {
        setExpanded(true)
        setAutoExpanded(true)
        loadChildren()
      }
    } else if (autoExpanded && !manuallyExpanded) {
      // Auto-collapse when navigating away, unless user manually expanded
      setExpanded(false)
      setAutoExpanded(false)
    }
  }, [currentPath])

  // Listen for expand-all / collapse-all
  useEffect(() => {
    const handleExpand = () => {
      setExpanded(true)
      setManuallyExpanded(true)
      loadChildren()
    }
    const handleCollapse = () => {
      setExpanded(false)
      setManuallyExpanded(false)
      setAutoExpanded(false)
    }
    window.addEventListener('sidebar-expand-all', handleExpand)
    window.addEventListener('sidebar-collapse-all', handleCollapse)
    return () => {
      window.removeEventListener('sidebar-expand-all', handleExpand)
      window.removeEventListener('sidebar-collapse-all', handleCollapse)
    }
  }, [])

  const loadChildren = useCallback(async () => {
    if (localNode.children !== null) return
    setLocalNode((prev) => ({ ...prev, loading: true }))
    try {
      const items: FileItem[] = await listFiles(node.path)
      const dirs = items
        .filter((f) => f.isDir)
        .map((f) => ({
          name: f.name,
          path: f.path,
          children: null,
          loading: false,
        }))
      setLocalNode((prev) => ({ ...prev, children: dirs, loading: false }))
    } catch {
      setLocalNode((prev) => ({ ...prev, children: [], loading: false }))
    }
  }, [node.path, localNode.children])

  const handleToggle = (path: string) => {
    if (path === node.path) {
      const newExpanded = !expanded
      setExpanded(newExpanded)
      if (newExpanded) {
        setManuallyExpanded(true)
        if (localNode.children === null) loadChildren()
      } else {
        setManuallyExpanded(false)
        setAutoExpanded(false)
      }
    }
  }

  return (
    <SidebarItem
      node={localNode}
      depth={depth}
      currentPath={currentPath}
      onNavigate={onNavigate}
      onToggle={handleToggle}
      onContextMenu={onContextMenu}
      expanded={expanded}
    />
  )
}

function formatBytes(bytes: number): string {
  if (bytes === 0) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  const i = Math.floor(Math.log(bytes) / Math.log(1024))
  return `${(bytes / Math.pow(1024, i)).toFixed(1)} ${units[i]}`
}

export default function Sidebar({ currentPath, homeFolder, onNavigate, onContextMenu, onShowTrash, diskUsage, onDrop, mobileOpen, onMobileClose }: Props) {
  const [rootFolders, setRootFolders] = useState<TreeNode[]>([])
  const [loading, setLoading] = useState(true)
  const [collapsed, setCollapsed] = useState(false)
  const [quickAccess, setQuickAccess] = useState(getQuickAccess())

  // Refresh quick access when it might have changed
  useEffect(() => {
    const handler = () => setQuickAccess(getQuickAccess())
    window.addEventListener('quickaccess-updated', handler)
    return () => window.removeEventListener('quickaccess-updated', handler)
  }, [])

  const rootPath = homeFolder || '/'
  const [refreshKey, setRefreshKey] = useState(0)

  useEffect(() => {
    const handler = () => setRefreshKey((k) => k + 1)
    window.addEventListener('sidebar-refresh', handler)
    return () => window.removeEventListener('sidebar-refresh', handler)
  }, [])

  useEffect(() => {
    setLoading(true)
    listFiles(rootPath)
      .then((items: FileItem[]) => {
        const dirs = items
          .filter((f: FileItem) => f.isDir)
          .map((f: FileItem) => ({
            name: f.name,
            path: f.path,
            children: null,
            loading: false,
          }))
        setRootFolders(dirs)
      })
      .catch(() => {})
      .finally(() => setLoading(false))
  }, [rootPath, refreshKey])

  const handleRemoveQuickAccess = (e: React.MouseEvent, path: string) => {
    e.stopPropagation()
    removeQuickAccess(path)
    setQuickAccess(getQuickAccess())
  }

  // Close mobile sidebar on navigate
  const handleMobileNavigate = (p: string) => {
    onNavigate(p)
    onMobileClose?.()
  }

  if (collapsed) {
    return (
      <div className="w-10 flex-shrink-0 bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 hidden md:flex flex-col items-center pt-2">
        <button
          onClick={() => setCollapsed(false)}
          className="p-1.5 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-md transition"
          title="Expand sidebar"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 5l7 7-7 7" />
          </svg>
        </button>
      </div>
    )
  }

  const sidebarContent = (
    <div className="w-full h-full bg-white dark:bg-gray-800 border-r border-gray-200 dark:border-gray-700 flex flex-col overflow-hidden">
      {/* Sidebar header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-gray-100 dark:border-gray-700">
        <span className="text-xs font-semibold text-gray-500 uppercase tracking-wider">Folders</span>
        <div className="flex items-center gap-0.5">
          <button
            onClick={() => window.dispatchEvent(new CustomEvent('sidebar-expand-all'))}
            className="p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-md transition"
            title="Expand all"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 9l-7 7-7-7" />
            </svg>
          </button>
          <button
            onClick={() => window.dispatchEvent(new CustomEvent('sidebar-collapse-all'))}
            className="p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-md transition"
            title="Collapse all"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M5 15l7-7 7 7" />
            </svg>
          </button>
          <button
            onClick={() => setCollapsed(true)}
            className="p-1 text-gray-400 hover:text-gray-600 hover:bg-gray-100 rounded-md transition"
            title="Hide sidebar"
          >
            <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
            </svg>
          </button>
        </div>
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto py-1 px-1">
        {/* Quick Access */}
        {quickAccess.length > 0 && (
          <div className="mb-2">
            <div className="px-2 py-1 text-xs font-semibold text-gray-400 uppercase tracking-wider">Quick Access</div>
            {quickAccess.map((item) => (
              <button
                key={item.path}
                onClick={() => handleMobileNavigate(item.path)}
                onContextMenu={(e) => {
                  e.preventDefault()
                  e.stopPropagation()
                  onContextMenu(e, { name: item.name, path: item.path, isDir: true, size: 0, createdAt: 0, modTime: 0 })
                }}
                className={`w-full flex items-center gap-1.5 py-1 px-2 text-left text-sm rounded-md transition group ${
                  currentPath === item.path
                    ? 'bg-blue-50 text-blue-700 font-medium'
                    : 'text-gray-700 hover:bg-gray-100'
                }`}
              >
                <svg className="w-4 h-4 text-yellow-400 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
                  <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
                </svg>
                <span className="truncate flex-1">{item.name}</span>
                <span
                  onClick={(e) => handleRemoveQuickAccess(e, item.path)}
                  className="hidden group-hover:block p-0.5 text-gray-400 hover:text-red-500 rounded"
                >
                  <svg className="w-3 h-3" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
                  </svg>
                </span>
              </button>
            ))}
            <hr className="my-1.5 border-gray-100 mx-2" />
          </div>
        )}

        {/* Home / root */}
        <button
          onClick={() => handleMobileNavigate(rootPath)}
          className={`w-full flex items-center gap-1.5 py-1 px-2 text-left text-sm rounded-md transition mb-0.5 ${
            currentPath === rootPath
              ? 'bg-blue-50 text-blue-700 font-medium'
              : 'text-gray-700 hover:bg-gray-100'
          }`}
        >
          <svg className="w-4 h-4 text-gray-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 12l2-2m0 0l7-7 7 7M5 10v10a1 1 0 001 1h3m10-11l2 2m-2-2v10a1 1 0 01-1 1h-3m-4 0h4" />
          </svg>
          <span className="truncate">Home</span>
        </button>

        {loading ? (
          <div className="text-gray-400 text-xs text-center py-4">Loading...</div>
        ) : rootFolders.length === 0 ? (
          <div className="text-gray-400 text-xs text-center py-4">No folders</div>
        ) : (
          rootFolders.map((folder) => (
            <SidebarItemWrapper
              key={folder.path}
              node={folder}
              depth={0}
              currentPath={currentPath}
              onNavigate={onNavigate}
              onContextMenu={onContextMenu}
            />
          ))
        )}
      </div>

      {/* Trash */}
      <div className="border-t border-gray-100 dark:border-gray-700 px-1 py-1">
        <button
          onClick={() => { onShowTrash(); onMobileClose?.() }}
          className="w-full flex items-center gap-1.5 py-1.5 md:py-1 px-2 text-left text-sm rounded-md text-gray-700 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 transition"
        >
          <svg className="w-4 h-4 text-gray-400 flex-shrink-0" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
          </svg>
          <span className="truncate">Trash</span>
        </button>
      </div>

      {/* Storage usage */}
      {diskUsage && (
        <div className="border-t border-gray-100 dark:border-gray-700 px-3 py-2 group relative">
          <div className="text-xs text-gray-400 mb-1">
            {formatBytes(diskUsage.totalSize)} used{diskUsage.totalSpace > 0 ? ` of ${formatBytes(diskUsage.totalSpace)}` : ''}
          </div>
          <div className="h-1.5 bg-gray-200 dark:bg-gray-700 rounded-full overflow-hidden">
            <div
              className="h-full bg-blue-500 rounded-full transition-all"
              style={{ width: diskUsage.totalSpace > 0 ? `${Math.min(100, (diskUsage.totalSize / diskUsage.totalSpace) * 100)}%` : '0%' }}
            />
          </div>
          {/* Per-user tooltip */}
          {diskUsage.perUser && diskUsage.perUser.length > 0 && (
            <div className="absolute bottom-full left-0 right-0 mb-2 bg-gray-800 dark:bg-gray-700 text-white rounded-lg shadow-xl p-3 hidden group-hover:block z-50">
              <div className="text-xs font-medium mb-2">Storage by folder</div>
              {diskUsage.perUser.map((u) => (
                <div key={u.username} className="flex items-center justify-between text-xs py-0.5">
                  <span className="text-gray-300">{u.username}</span>
                  <span className="text-gray-400 font-mono">{formatBytes(u.size)}</span>
                </div>
              ))}
              {diskUsage.freeSpace > 0 && (
                <div className="flex items-center justify-between text-xs py-0.5 mt-1 pt-1 border-t border-gray-600">
                  <span className="text-gray-300">Free</span>
                  <span className="text-gray-400 font-mono">{formatBytes(diskUsage.freeSpace)}</span>
                </div>
              )}
            </div>
          )}
        </div>
      )}
    </div>
  )

  return (
    <>
      {/* Desktop sidebar */}
      <div className="hidden md:flex w-56 flex-shrink-0">
        {sidebarContent}
      </div>
      {/* Mobile drawer */}
      <div className={`sidebar-backdrop md:hidden ${mobileOpen ? 'open' : ''}`} onClick={onMobileClose} />
      <div className={`sidebar-mobile md:hidden ${mobileOpen ? 'open' : ''}`}>
        {sidebarContent}
      </div>
    </>
  )
}
