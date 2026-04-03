import { useEffect } from 'react'

interface Props {
  onClose: () => void
}

const isMac = typeof navigator !== 'undefined' && navigator.platform.includes('Mac')
const mod = isMac ? '\u2318' : 'Ctrl'

const SHORTCUTS = [
  { section: 'Navigation', items: [
    { keys: 'Click folder', desc: 'Open folder' },
    { keys: 'Click file', desc: 'Preview file' },
    { keys: 'Double-click file', desc: 'Download file' },
    { keys: 'Back / Up buttons', desc: 'Navigate history' },
    { keys: 'Mouse back button', desc: 'Go back' },
  ]},
  { section: 'Selection', items: [
    { keys: `${mod}+A`, desc: 'Select all' },
    { keys: 'Click checkbox', desc: 'Toggle selection' },
    { keys: `${mod}+Click`, desc: 'Toggle individual' },
    { keys: 'Shift+Click', desc: 'Select range' },
    { keys: 'Shift+Drag', desc: 'Drag select range' },
    { keys: 'Esc', desc: 'Clear selection / Close' },
  ]},
  { section: 'File Operations', items: [
    { keys: `${mod}+C`, desc: 'Copy selected' },
    { keys: `${mod}+X`, desc: 'Cut selected' },
    { keys: `${mod}+V`, desc: 'Paste' },
    { keys: 'Delete', desc: 'Move to trash' },
    { keys: 'F2', desc: 'Rename selected' },
    { keys: 'Enter', desc: 'Open selected' },
    { keys: `${mod}+Shift+N`, desc: 'New folder' },
  ]},
  { section: 'General', items: [
    { keys: `${mod}+F`, desc: 'Search' },
    { keys: '?', desc: 'This help' },
    { keys: 'Esc', desc: 'Close modal / menu' },
  ]},
]

export default function KeyboardHelp({ onClose }: Props) {
  useEffect(() => {
    const handler = (e: KeyboardEvent) => { if (e.key === 'Escape') onClose() }
    document.addEventListener('keydown', handler)
    return () => document.removeEventListener('keydown', handler)
  }, [onClose])

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/50" onClick={onClose}>
      <div
        className="bg-white dark:bg-gray-800 rounded-xl shadow-2xl w-full max-w-2xl mx-4 max-h-[80vh] flex flex-col overflow-hidden"
        onClick={(e) => e.stopPropagation()}
      >
        <div className="flex items-center justify-between px-5 py-4 border-b border-gray-200 dark:border-gray-700">
          <h2 className="text-base font-semibold text-gray-800 dark:text-gray-200">Keyboard Shortcuts</h2>
          <button onClick={onClose} className="p-1 text-gray-400 hover:text-gray-600 dark:hover:text-gray-300 hover:bg-gray-100 dark:hover:bg-gray-700 rounded-lg transition">
            <svg className="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
        <div className="flex-1 overflow-y-auto p-5">
          <div className="grid grid-cols-2 gap-6">
            {SHORTCUTS.map((section) => (
              <div key={section.section}>
                <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300 mb-2">{section.section}</h3>
                <div className="space-y-1.5">
                  {section.items.map((item) => (
                    <div key={item.desc} className="flex items-center justify-between text-sm">
                      <span className="text-gray-600 dark:text-gray-400">{item.desc}</span>
                      <kbd className="px-2 py-0.5 bg-gray-100 dark:bg-gray-700 text-gray-700 dark:text-gray-300 text-xs font-mono rounded border border-gray-200 dark:border-gray-600 whitespace-nowrap">
                        {item.keys}
                      </kbd>
                    </div>
                  ))}
                </div>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  )
}
