import { useRef } from 'react'
import type { ViewMode } from '../types'

interface Props {
  viewMode: ViewMode
  onViewModeChange: (mode: ViewMode) => void
  onUpload: (files: FileList) => void
  onNewFolder: () => void
  onRefresh: () => void
  onLogout: () => void
}

export default function Toolbar({ viewMode, onViewModeChange, onUpload, onNewFolder, onRefresh, onLogout }: Props) {
  const fileInputRef = useRef<HTMLInputElement>(null)

  return (
    <div className="flex items-center gap-2 flex-wrap">
      <button
        onClick={() => fileInputRef.current?.click()}
        className="flex items-center gap-1.5 px-3 py-1.5 bg-blue-600 text-white rounded-lg text-sm font-medium hover:bg-blue-700 transition"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M7 16a4 4 0 01-.88-7.903A5 5 0 1115.9 6L16 6a5 5 0 011 9.9M15 13l-3-3m0 0l-3 3m3-3v12" />
        </svg>
        Upload
      </button>
      <input
        ref={fileInputRef}
        type="file"
        multiple
        className="hidden"
        onChange={(e) => e.target.files && onUpload(e.target.files)}
      />

      <button
        onClick={onNewFolder}
        className="flex items-center gap-1.5 px-3 py-1.5 bg-gray-200 dark:bg-gray-700 text-gray-700 dark:text-gray-300 rounded-lg text-sm font-medium hover:bg-gray-300 dark:hover:bg-gray-600 transition"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M9 13h6m-3-3v6m-9 1V7a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z" />
        </svg>
        New Folder
      </button>

      <button
        onClick={onRefresh}
        className="p-1.5 text-gray-500 hover:text-gray-700 hover:bg-gray-200 rounded-lg transition"
        title="Refresh"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15" />
        </svg>
      </button>

      <div className="ml-auto flex items-center gap-2">
        <div className="flex bg-gray-200 dark:bg-gray-700 rounded-lg p-0.5">
          <button
            onClick={() => onViewModeChange('list')}
            className={`p-1.5 rounded-md transition ${viewMode === 'list' ? 'bg-white dark:bg-gray-600 shadow-sm text-gray-800 dark:text-gray-200' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}`}
            title="List view"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6h16M4 12h16M4 18h16" />
            </svg>
          </button>
          <button
            onClick={() => onViewModeChange('grid')}
            className={`p-1.5 rounded-md transition ${viewMode === 'grid' ? 'bg-white dark:bg-gray-600 shadow-sm text-gray-800 dark:text-gray-200' : 'text-gray-500 dark:text-gray-400 hover:text-gray-700 dark:hover:text-gray-300'}`}
            title="Grid view"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2V6zM14 6a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2V6zM4 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2H6a2 2 0 01-2-2v-2zM14 16a2 2 0 012-2h2a2 2 0 012 2v2a2 2 0 01-2 2h-2a2 2 0 01-2-2v-2z" />
            </svg>
          </button>
        </div>

        <button
          onClick={onLogout}
          className="px-3 py-1.5 text-gray-500 hover:text-gray-700 text-sm hover:bg-gray-200 rounded-lg transition"
        >
          Logout
        </button>
      </div>
    </div>
  )
}
