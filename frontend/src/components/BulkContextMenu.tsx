import { useEffect, useRef } from 'react'

interface Props {
  x: number
  y: number
  count: number
  onDownload: () => void
  onCut: () => void
  onCopy: () => void
  onCompress: () => void
  onDelete: () => void
  onClose: () => void
}

export default function BulkContextMenu({ x, y, count, onDownload, onCut, onCopy, onCompress, onDelete, onClose }: Props) {
  const ref = useRef<HTMLDivElement>(null)
  const isMobile = typeof window !== 'undefined' && window.innerWidth < 768

  useEffect(() => {
    const handler = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) {
        onClose()
      }
    }
    document.addEventListener('mousedown', handler)
    return () => document.removeEventListener('mousedown', handler)
  }, [onClose])

  const style: React.CSSProperties = isMobile ? {} : {
    position: 'fixed',
    left: Math.min(x, window.innerWidth - 180),
    top: Math.min(y, window.innerHeight - 120),
    zIndex: 50,
  }

  const btnClass = "w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"

  return (
    <>
    {isMobile && <div className="context-menu-backdrop" onClick={onClose} />}
    <div ref={ref} style={style} className={`bg-white dark:bg-gray-800 shadow-xl border border-gray-200 dark:border-gray-700 py-2 ${isMobile ? 'context-menu-mobile z-50' : 'rounded-lg w-48 fixed'}`}>
      <div className="px-4 md:px-3 py-1.5 text-xs text-gray-400 border-b border-gray-100">
        {count} items selected
      </div>
      <button onClick={onDownload} className={btnClass}>
        <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        Download All
      </button>
      <button onClick={onCut} className={btnClass}>
        <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.121 14.121L19 19m-7-7l7-7m-7 7l-2.879 2.879M12 12L9.121 9.121m0 5.758a3 3 0 10-4.243 4.243 3 3 0 004.243-4.243zm0-5.758a3 3 0 10-4.243-4.243 3 3 0 004.243 4.243z" />
        </svg>
        Cut
      </button>
      <button onClick={onCopy} className={btnClass}>
        <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
        </svg>
        Copy
      </button>
      <button onClick={onCompress} className={btnClass}>
        <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 16v1a3 3 0 003 3h10a3 3 0 003-3v-1m-4-8l-4-4m0 0L8 8m4-4v12" />
        </svg>
        Compress to Zip
      </button>
      <hr className="my-1 border-gray-100 dark:border-gray-700" />
      <button
        onClick={onDelete}
        className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-red-50 text-red-600 flex items-center gap-2"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
        Delete All
      </button>
    </div>
    </>
  )
}
