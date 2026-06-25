import { useState, useEffect } from 'react'

interface Props {
  files: { name: string; path: string }[]
  onRename: (renames: { oldPath: string; newName: string }[]) => void
  onClose: () => void
}

export default function BatchRename({ files, onRename, onClose }: Props) {
  const [find, setFind] = useState('')
  const [replace, setReplace] = useState('')
  const [prefix, setPrefix] = useState('')
  const [suffix, setSuffix] = useState('')
  const [mode, setMode] = useState<'findReplace' | 'addFix'>('findReplace')

  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  const getNewName = (name: string): string => {
    if (mode === 'findReplace' && find) {
      return name.split(find).join(replace)
    }
    if (mode === 'addFix') {
      const ext = name.includes('.') ? '.' + name.split('.').pop() : ''
      const base = name.includes('.') ? name.slice(0, name.lastIndexOf('.')) : name
      return `${prefix}${base}${suffix}${ext}`
    }
    return name
  }

  const previews = files.map((f) => ({
    original: f.name,
    newName: getNewName(f.name),
    path: f.path,
    changed: f.name !== getNewName(f.name),
  }))

  const changedCount = previews.filter((p) => p.changed).length

  const handleApply = () => {
    const renames = previews
      .filter((p) => p.changed)
      .map((p) => ({ oldPath: p.path, newName: p.newName }))
    if (renames.length > 0) onRename(renames)
    onClose()
  }

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onMouseDown={(e) => { if (e.target === e.currentTarget) onClose() }}>
      <div className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-lg mx-4 max-h-[80vh] flex flex-col overflow-hidden" onClick={(e) => e.stopPropagation()}>
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-base font-semibold text-gray-800 dark:text-gray-200">Batch Rename ({files.length} files)</h2>
          <button onClick={onClose} className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 rounded-lg transition">
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>

        <div className="px-5 py-4 space-y-4">
          {/* Mode toggle */}
          <div className="flex gap-2">
            <button
              onClick={() => setMode('findReplace')}
              className={`px-3 py-1.5 text-xs rounded-md transition ${mode === 'findReplace' ? 'bg-blue-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'}`}
            >
              Find & Replace
            </button>
            <button
              onClick={() => setMode('addFix')}
              className={`px-3 py-1.5 text-xs rounded-md transition ${mode === 'addFix' ? 'bg-blue-600 text-white' : 'bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300'}`}
            >
              Add Prefix/Suffix
            </button>
          </div>

          {mode === 'findReplace' ? (
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-gray-500 dark:text-gray-400 mb-1">Find</label>
                <input value={find} onChange={(e) => setFind(e.target.value)} className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 rounded-md" placeholder="Text to find" />
              </div>
              <div>
                <label className="block text-xs text-gray-500 dark:text-gray-400 mb-1">Replace with</label>
                <input value={replace} onChange={(e) => setReplace(e.target.value)} className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 rounded-md" placeholder="Replacement" />
              </div>
            </div>
          ) : (
            <div className="grid grid-cols-2 gap-2">
              <div>
                <label className="block text-xs text-gray-500 dark:text-gray-400 mb-1">Prefix</label>
                <input value={prefix} onChange={(e) => setPrefix(e.target.value)} className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 rounded-md" placeholder="Add before name" />
              </div>
              <div>
                <label className="block text-xs text-gray-500 dark:text-gray-400 mb-1">Suffix</label>
                <input value={suffix} onChange={(e) => setSuffix(e.target.value)} className="w-full px-3 py-1.5 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 rounded-md" placeholder="Add after name" />
              </div>
            </div>
          )}
        </div>

        {/* Preview */}
        <div className="flex-1 overflow-y-auto border-t border-gray-200 dark:border-gray-700">
          <div className="px-5 py-2 text-xs text-gray-500 dark:text-gray-400 font-medium">Preview ({changedCount} changes)</div>
          {previews.map((p) => (
            <div key={p.path} className={`px-5 py-1 text-xs flex gap-2 ${p.changed ? '' : 'opacity-40'}`}>
              <span className="text-gray-500 dark:text-gray-400 line-through flex-1 truncate">{p.original}</span>
              <span className="text-gray-400 dark:text-gray-600">&rarr;</span>
              <span className={`flex-1 truncate ${p.changed ? 'text-green-600 dark:text-green-400 font-medium' : 'text-gray-500 dark:text-gray-400'}`}>{p.newName}</span>
            </div>
          ))}
        </div>

        <div className="px-5 py-3 border-t border-gray-200 dark:border-gray-700 flex justify-end gap-2">
          <button onClick={onClose} className="px-4 py-1.5 text-sm text-gray-600 dark:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-md transition">Cancel</button>
          <button onClick={handleApply} disabled={changedCount === 0} className="px-4 py-1.5 text-sm bg-blue-600 text-white rounded-md hover:bg-blue-700 transition disabled:opacity-50">
            Rename {changedCount} file{changedCount !== 1 ? 's' : ''}
          </button>
        </div>
      </div>
    </div>
  )
}
