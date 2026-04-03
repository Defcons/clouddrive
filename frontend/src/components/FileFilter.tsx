import { useState, useRef, useEffect } from 'react'

interface Props {
  value: string
  onChange: (filter: string) => void
}

const FILTERS = [
  { label: 'All Files', value: '' },
  { label: 'Images', value: 'images', ext: ['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico'] },
  { label: 'Documents', value: 'documents', ext: ['pdf', 'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'txt', 'md', 'csv'] },
  { label: 'Videos', value: 'videos', ext: ['mp4', 'mkv', 'avi', 'mov', 'webm'] },
  { label: 'Audio', value: 'audio', ext: ['mp3', 'wav', 'flac', 'ogg', 'aac'] },
  { label: 'Archives', value: 'archives', ext: ['zip', 'rar', '7z', 'tar', 'gz'] },
  { label: 'Code', value: 'code', ext: ['js', 'ts', 'jsx', 'tsx', 'py', 'go', 'html', 'css', 'json', 'yml', 'yaml', 'sh'] },
]

export function getFilterExtensions(filter: string): string[] | null {
  if (!filter) return null
  const f = FILTERS.find((f) => f.value === filter)
  return f?.ext || null
}

export default function FileFilter({ value, onChange }: Props) {
  const [open, setOpen] = useState(false)
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) setOpen(false)
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [])

  const current = FILTERS.find((f) => f.value === value) || FILTERS[0]

  return (
    <div className="relative" ref={ref}>
      <button
        onClick={() => setOpen(!open)}
        className={`flex items-center gap-1 px-2.5 py-1.5 text-xs font-medium rounded-lg transition ${
          value
            ? 'bg-blue-100 dark:bg-blue-900/40 text-blue-700 dark:text-blue-400'
            : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 hover:bg-gray-200 dark:hover:bg-gray-600'
        }`}
      >
        <svg className="w-3.5 h-3.5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 4a1 1 0 011-1h16a1 1 0 011 1v2.586a1 1 0 01-.293.707l-6.414 6.414a1 1 0 00-.293.707V17l-4 4v-6.586a1 1 0 00-.293-.707L3.293 7.293A1 1 0 013 6.586V4z" />
        </svg>
        {current.label}
      </button>

      {open && (
        <div className="absolute top-full left-0 mt-1 bg-white dark:bg-gray-800 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700 py-1 w-36 z-50">
          {FILTERS.map((filter) => (
            <button
              key={filter.value}
              onClick={() => { onChange(filter.value); setOpen(false) }}
              className={`w-full text-left px-3 py-1.5 text-xs hover:bg-gray-100 dark:hover:bg-gray-700 transition ${
                value === filter.value ? 'text-blue-600 dark:text-blue-400 font-medium' : 'text-gray-700 dark:text-gray-300'
              }`}
            >
              {filter.label}
            </button>
          ))}
        </div>
      )}
    </div>
  )
}
