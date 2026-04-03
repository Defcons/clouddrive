export interface FileItem {
  name: string
  path: string
  isDir: boolean
  size: number
  createdAt: number
  modTime: number
  itemCount?: number
  isPrivate?: boolean
  tags?: string[]
}

export interface DiskUsage {
  totalFiles: number
  totalDirs: number
  totalSize: number
  storageRoot: string
}

export type ViewMode = 'list' | 'grid'

export interface Clipboard {
  paths: string[]
  mode: 'cut' | 'copy'
}

export interface TrashItem {
  id: string
  originalPath: string
  name: string
  isDir: boolean
  size: number
  deletedBy: string
  deletedAt: number
}

export interface NotificationItem {
  id: string
  username: string
  message: string
  link: string
  read: boolean
  createdAt: number
}

export const TAG_COLORS: Record<string, string> = {
  red: 'bg-red-500',
  orange: 'bg-orange-500',
  yellow: 'bg-yellow-500',
  green: 'bg-green-500',
  blue: 'bg-blue-500',
  purple: 'bg-purple-500',
  pink: 'bg-pink-500',
  gray: 'bg-gray-500',
}
