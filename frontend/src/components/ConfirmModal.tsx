import { useEffect, useState } from 'react'
import Modal from './Modal'

type ConfirmOptions = {
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  destructive?: boolean
  /** If provided, renders a text input; the value is passed to resolve(). */
  prompt?: {
    placeholder?: string
    initialValue?: string
    inputType?: 'text' | 'password'
  }
}

type PendingConfirm = ConfirmOptions & {
  resolve: (value: boolean | string | null) => void
}

let setPendingGlobal: ((c: PendingConfirm | null) => void) | null = null

/**
 * Drop-in replacement for window.confirm() / window.prompt() that uses an
 * accessible modal. Usage:
 *
 *   await confirm({ title: 'Delete?', message: '...' })        // boolean
 *   await prompt({ title: 'Name?', message: '...', prompt:{} }) // string|null
 */
export function confirm(opts: Omit<ConfirmOptions, 'prompt'>): Promise<boolean> {
  return new Promise((resolve) => {
    setPendingGlobal?.({ ...opts, resolve: (v) => resolve(v === true) })
  })
}

export function prompt(opts: ConfirmOptions): Promise<string | null> {
  if (!opts.prompt) opts = { ...opts, prompt: {} }
  return new Promise((resolve) => {
    setPendingGlobal?.({
      ...opts,
      resolve: (v) => {
        if (v === false || v === null) resolve(null)
        else resolve(typeof v === 'string' ? v : null)
      },
    })
  })
}

export default function ConfirmModalHost() {
  const [pending, setPending] = useState<PendingConfirm | null>(null)
  const [inputValue, setInputValue] = useState('')

  useEffect(() => {
    setPendingGlobal = setPending
    return () => {
      setPendingGlobal = null
    }
  }, [])

  useEffect(() => {
    if (pending?.prompt) {
      setInputValue(pending.prompt.initialValue || '')
    } else {
      setInputValue('')
    }
  }, [pending])

  if (!pending) return null

  const close = (result: boolean | string | null) => {
    pending.resolve(result)
    setPending(null)
  }

  const isPrompt = !!pending.prompt

  return (
    <Modal open onClose={() => close(isPrompt ? null : false)} title={pending.title}>
      <p className="text-sm text-gray-600 dark:text-gray-300 mb-4">{pending.message}</p>
      {isPrompt && (
        <input
          type={pending.prompt!.inputType || 'text'}
          value={inputValue}
          onChange={(e) => setInputValue(e.target.value)}
          placeholder={pending.prompt!.placeholder || ''}
          autoFocus
          onKeyDown={(e) => {
            if (e.key === 'Enter') {
              e.preventDefault()
              close(inputValue)
            }
          }}
          className="w-full min-h-11 px-3 py-2 border border-gray-300 dark:border-gray-600 rounded-lg text-sm bg-white dark:bg-gray-700 text-gray-900 dark:text-gray-100 focus:ring-2 focus:ring-blue-500 focus:outline-none mb-4"
        />
      )}
      <div className="flex gap-2 justify-end">
        <button
          onClick={() => close(isPrompt ? null : false)}
          className="min-h-11 px-4 py-2 rounded-lg text-sm font-medium bg-gray-200 dark:bg-gray-700 text-gray-800 dark:text-gray-200 hover:bg-gray-300 dark:hover:bg-gray-600"
        >
          {pending.cancelLabel || 'Cancel'}
        </button>
        <button
          onClick={() => close(isPrompt ? inputValue : true)}
          className={`min-h-11 px-4 py-2 rounded-lg text-sm font-medium text-white ${
            pending.destructive
              ? 'bg-red-600 hover:bg-red-700'
              : 'bg-blue-600 hover:bg-blue-700'
          }`}
        >
          {pending.confirmLabel || (pending.destructive ? 'Delete' : 'Confirm')}
        </button>
      </div>
    </Modal>
  )
}
