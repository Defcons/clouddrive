import { useState, useCallback } from 'react'

export interface Toast {
  id: number
  message: string
  type: 'success' | 'error' | 'info'
  action?: { label: string; onClick: () => void }
}

let toastId = 0

export function useToast() {
  const [toasts, setToasts] = useState<Toast[]>([])

  const addToast = useCallback((message: string, type: Toast['type'] = 'info', action?: Toast['action'], duration = 4000) => {
    const id = ++toastId
    setToasts((prev) => [...prev, { id, message, type, action }])
    // Auto-dismiss every toast after its duration — including ones with an
    // action (e.g. Undo), which otherwise lived forever and piled up.
    setTimeout(() => {
      setToasts((prev) => prev.filter((t) => t.id !== id))
    }, duration)
    return id
  }, [])

  const removeToast = useCallback((id: number) => {
    setToasts((prev) => prev.filter((t) => t.id !== id))
  }, [])

  const success = useCallback((msg: string) => addToast(msg, 'success'), [addToast])
  const error = useCallback((msg: string) => addToast(msg, 'error', undefined, 6000), [addToast])
  const info = useCallback((msg: string, action?: Toast['action']) => addToast(msg, 'info', action, action ? 8000 : 4000), [addToast])

  return { toasts, addToast, removeToast, success, error, info }
}
