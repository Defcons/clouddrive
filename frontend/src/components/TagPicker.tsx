import { useState } from 'react'
import { TAG_COLORS } from '../types'

interface Props {
  currentTags: string[]
  onSave: (tags: string[]) => void
  onClose: () => void
}

const PRESET_TAGS = Object.keys(TAG_COLORS)

export default function TagPicker({ currentTags, onSave, onClose }: Props) {
  const [selected, setSelected] = useState<Set<string>>(new Set(currentTags))

  const toggle = (tag: string) => {
    const next = new Set(selected)
    if (next.has(tag)) {
      next.delete(tag)
    } else {
      next.add(tag)
    }
    setSelected(next)
  }

  return (
    <div className="absolute z-50 mt-1 bg-white dark:bg-gray-800 rounded-lg shadow-xl border border-gray-200 dark:border-gray-700 p-3 w-48">
      <div className="text-xs font-medium text-gray-500 dark:text-gray-400 mb-2">Tags</div>
      <div className="flex flex-wrap gap-2 mb-3">
        {PRESET_TAGS.map((tag) => (
          <button
            key={tag}
            onClick={() => toggle(tag)}
            className={`w-6 h-6 rounded-full border-2 transition ${TAG_COLORS[tag]} ${
              selected.has(tag) ? 'border-white dark:border-gray-200 ring-2 ring-blue-500' : 'border-transparent'
            }`}
            title={tag}
          />
        ))}
      </div>
      <div className="flex gap-2">
        <button
          onClick={() => { onSave(Array.from(selected)); onClose() }}
          className="flex-1 text-xs px-2 py-1 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition"
        >
          Save
        </button>
        <button
          onClick={onClose}
          className="flex-1 text-xs px-2 py-1 bg-gray-100 dark:bg-gray-700 text-gray-600 dark:text-gray-300 rounded-md hover:bg-gray-200 dark:hover:bg-gray-600 transition"
        >
          Cancel
        </button>
      </div>
    </div>
  )
}
