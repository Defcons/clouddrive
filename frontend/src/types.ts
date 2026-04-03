export interface FileItem {
  name: string
  path: string
  isDir: boolean
  size: number
  createdAt: number
  modTime: number
  itemCount?: number
  isPrivate?: boolean
}

export interface DiskUsage {
  totalFiles: number
  totalDirs: number
  totalSize: number
  storageRoot: string
}

export type ViewMode = 'list' | 'grid'
