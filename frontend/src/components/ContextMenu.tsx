import { useEffect, useRef } from 'react'

interface Props {
  x: number
  y: number
  onPreview?: () => void
  onShare: () => void
  onSafeShare: () => void
  onDownload: () => void
  onCut: () => void
  onCopy: () => void
  onRename: () => void
  onDelete: () => void
  onQuickAccess?: () => void
  onMakePrivate?: () => void
  onMakePublic?: () => void
  onExtract?: () => void
  onToggleOffsite?: () => void
  onClose: () => void
  isDir: boolean
  canPreview: boolean
  isPrivate?: boolean
  isZip?: boolean
  offsiteBackup?: boolean
}

export default function ContextMenu({ x, y, onPreview, onShare, onSafeShare, onDownload, onCut, onCopy, onRename, onDelete, onQuickAccess, onMakePrivate, onMakePublic, onExtract, onToggleOffsite, onClose, isDir, canPreview, isPrivate, isZip, offsiteBackup }: Props) {
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

  // Adjust position so menu doesn't overflow viewport
  const style: React.CSSProperties = isMobile ? {} : {
    position: 'fixed',
    left: Math.min(x, window.innerWidth - 180),
    top: Math.min(y, window.innerHeight - 200),
    zIndex: 50,
  }

  return (
    <>
    {isMobile && <div className="context-menu-backdrop" onClick={onClose} />}
    <div ref={ref} style={style} className={`bg-white dark:bg-gray-800 shadow-xl border border-gray-200 dark:border-gray-700 py-2 ${isMobile ? 'context-menu-mobile z-50' : 'rounded-lg w-44 fixed'}`}>
      {!isDir && canPreview && (
        <button
          onClick={onPreview}
          className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
        >
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 12a3 3 0 11-6 0 3 3 0 016 0z" />
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M2.458 12C3.732 7.943 7.523 5 12 5c4.478 0 8.268 2.943 9.542 7-1.274 4.057-5.064 7-9.542 7-4.477 0-8.268-2.943-9.542-7z" />
          </svg>
          Preview
        </button>
      )}
      <button
        onClick={onDownload}
        className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
      >
        <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 10v6m0 0l-3-3m3 3l3-3m2 8H7a2 2 0 01-2-2V5a2 2 0 012-2h5.586a1 1 0 01.707.293l5.414 5.414a1 1 0 01.293.707V19a2 2 0 01-2 2z" />
        </svg>
        Download{isDir ? ' (zip)' : ''}
      </button>
      <button
        onClick={onShare}
        className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
      >
        <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8.684 13.342C8.886 12.938 9 12.482 9 12c0-.482-.114-.938-.316-1.342m0 2.684a3 3 0 110-2.684m0 2.684l6.632 3.316m-6.632-6l6.632-3.316m0 0a3 3 0 105.367-2.684 3 3 0 00-5.367 2.684zm0 9.316a3 3 0 105.368 2.684 3 3 0 00-5.368-2.684z" />
        </svg>
        Share
      </button>
      <button
        onClick={onSafeShare}
        className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
      >
        <svg className="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
        </svg>
        Safe Share
      </button>
      {isDir && onQuickAccess && (
        <button
          onClick={onQuickAccess}
          className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
        >
          <svg className="w-4 h-4 text-yellow-400" fill="currentColor" viewBox="0 0 20 20">
            <path d="M9.049 2.927c.3-.921 1.603-.921 1.902 0l1.07 3.292a1 1 0 00.95.69h3.462c.969 0 1.371 1.24.588 1.81l-2.8 2.034a1 1 0 00-.364 1.118l1.07 3.292c.3.921-.755 1.688-1.54 1.118l-2.8-2.034a1 1 0 00-1.175 0l-2.8 2.034c-.784.57-1.838-.197-1.539-1.118l1.07-3.292a1 1 0 00-.364-1.118L2.98 8.72c-.783-.57-.38-1.81.588-1.81h3.461a1 1 0 00.951-.69l1.07-3.292z" />
          </svg>
          Add to Quick Access
        </button>
      )}
      {isDir && !isPrivate && onMakePrivate && (
        <button
          onClick={onMakePrivate}
          className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
        >
          <svg className="w-4 h-4 text-orange-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 15v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2zm10-10V7a4 4 0 00-8 0v4h8z" />
          </svg>
          Make Private
        </button>
      )}
      {isDir && isPrivate && onMakePublic && (
        <button
          onClick={onMakePublic}
          className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
        >
          <svg className="w-4 h-4 text-green-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 11V7a4 4 0 118 0m-4 8v2m-6 4h12a2 2 0 002-2v-6a2 2 0 00-2-2H6a2 2 0 00-2 2v6a2 2 0 002 2z" />
          </svg>
          Make Public
        </button>
      )}
      {isDir && onToggleOffsite && (
        <button
          onClick={onToggleOffsite}
          className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
        >
          <svg className={`w-4 h-4 ${offsiteBackup ? 'text-blue-500' : 'text-gray-500'}`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M3 15a4 4 0 004 4h9a5 5 0 10-.1-9.999 5.002 5.002 0 10-9.78 2.096A4.001 4.001 0 003 15z" />
          </svg>
          {offsiteBackup ? 'Remove from Offsite Backup' : 'Add to Offsite Backup'}
        </button>
      )}
      {isZip && onExtract && (
        <button
          onClick={onExtract}
          className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
        >
          <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 10v6m0 0l-3-3m3 3l3-3M3 17V7a2 2 0 012-2h6l2 2h6a2 2 0 012 2v8a2 2 0 01-2 2H5a2 2 0 01-2-2z" />
          </svg>
          Extract Here
        </button>
      )}
      <hr className="my-1 border-gray-100 dark:border-gray-700" />
      <button
        onClick={onCut}
        className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
      >
        <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M14.121 14.121L19 19m-7-7l7-7m-7 7l-2.879 2.879M12 12L9.121 9.121m0 5.758a3 3 0 10-4.243 4.243 3 3 0 004.243-4.243zm0-5.758a3 3 0 10-4.243-4.243 3 3 0 004.243 4.243z" />
        </svg>
        Cut
      </button>
      <button
        onClick={onCopy}
        className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
      >
        <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M8 16H6a2 2 0 01-2-2V6a2 2 0 012-2h8a2 2 0 012 2v2m-6 12h8a2 2 0 002-2v-8a2 2 0 00-2-2h-8a2 2 0 00-2 2v8a2 2 0 002 2z" />
        </svg>
        Copy
      </button>
      <button
        onClick={onRename}
        className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-gray-100 dark:hover:bg-gray-700 flex items-center gap-2"
      >
        <svg className="w-4 h-4 text-gray-500" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M11 5H6a2 2 0 00-2 2v11a2 2 0 002 2h11a2 2 0 002-2v-5m-1.414-9.414a2 2 0 112.828 2.828L11.828 15H9v-2.828l8.586-8.586z" />
        </svg>
        Rename
      </button>
      <hr className="my-1 border-gray-100 dark:border-gray-700" />
      <button
        onClick={onDelete}
        className="w-full text-left px-4 py-3 md:px-3 md:py-2 text-sm hover:bg-red-50 dark:hover:bg-red-900/30 text-red-600 flex items-center gap-2"
      >
        <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
          <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" />
        </svg>
        Move to Trash
      </button>
    </div>
    </>
  )
}
