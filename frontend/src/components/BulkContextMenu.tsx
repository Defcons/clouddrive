import { useEffect, useRef } from 'react'

interface Props {
  x: number
  y: number
  count: number
  onDownload: () => void
  onDelete: () => void
  onClose: () => void
}

export default function BulkContextMenu({ x, y, count, onDownload, onDelete, onClose }: Props) {
  const ref = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose()
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [onClose])

  const style: React.CSSProperties = {
    position: 'fixed',
    left: Math.min(x, window.innerWidth - 180),
    top: Math.min(y, window.innerHeight - 120),
    zIndex: 50,
  }

  return (
    <div ref={ref} style={style} className="bg-white rounded-lg shadow-xl border border-gray-200 py-1 w-48">
      <div className="px-3 py-1.5 text-xs text-gray-400 border-b border-gray-100">
        {count} items selected
      </div>
      <button
        onClick={onDownload}
        className="w-full text-left px-3 py-2 text-sm hover:bg-gray-100 flex items-center gap-2"
      >
        <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        Download All
      </button>
      <hr className="my-1 border-gray-100" />
      <button
        onClick={onDelete}
        className="w-full text-left px-3 py-2 text-sm hover:bg-red-50 text-red-600 flex items-center gap-2"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
        Delete All
      </button>
    </div>
  )
}
