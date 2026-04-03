export interface FileItem {
  name: string
  path: string
  isDir: boolean
  size: number
  modTime: number
}

export interface DiskUsage {
  totalFiles: number
  totalDirs: number
  totalSize: number
  storageRoot: string
}

export type ViewMode = 'list' | 'grid'
