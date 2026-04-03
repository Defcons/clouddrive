import { useState, useCallback } from 'react'

interface Props {
  onUpload: (files: FileList) => void
  children: React.ReactNode
  uploadProgress: number | null
}

export default function UploadZone({ onUpload, children, uploadProgress }: Props) {
  const [dragOver, setDragOver] = useState(false)

  const handleDragOver = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setDragOver(true)
  }, [])

  const handleDragLeave = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setDragOver(false)
  }, [])

  const handleDrop = useCallback((e: React.DragEvent) => {
    e.preventDefault()
    e.stopPropagation()
    setDragOver(false)
    if (e.dataTransfer.files.length > 0) {
      onUpload(e.dataTransfer.files)
    }
  }, [onUpload])

  return (
    <div
      onDragOver={handleDragOver}
      onDragLeave={handleDragLeave}
      onDrop={handleDrop}
      className={`flex-1 relative ${dragOver ? 'ring-2 ring-blue-400 ring-inset bg-blue-50/50' : ''}`}
    >
      {dragOver && (
        <div className="absolute inset-0 flex items-center justify-center bg-blue-50/80 z-10 pointer-events-none">
          <div className="text-blue-600 font-medium text-lg">Drop files to upload</div>
        </div>
      )}
      {uploadProgress !== null && (
        <div className="absolute top-0 left-0 right-0 z-20">
          <div className="h-1 bg-gray-200">
            <div
              className="h-1 bg-blue-600 transition-all duration-300"
              style={{ width: `${uploadProgress}%` }}
            />
          </div>
          <div className="text-xs text-blue-600 text-center py-1 bg-blue-50">
            Uploading... {uploadProgress}%
          </div>
        </div>
      )}
      {children}
    </div>
  )
}
