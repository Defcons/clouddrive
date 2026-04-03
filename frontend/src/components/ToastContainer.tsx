import type { Toast } from '../hooks/useToast'

interface Props {
  toasts: Toast[]
  onDismiss: (id: number) => void
}

const TYPE_STYLES = {
  success: 'bg-green-600 text-white',
  error: 'bg-red-600 text-white',
  info: 'bg-gray-800 dark:bg-gray-700 text-white',
}

export default function ToastContainer({ toasts, onDismiss }: Props) {
  if (toasts.length === 0) return null

  return (
    <div className="fixed bottom-4 right-4 z-[100] flex flex-col gap-2 max-w-sm">
      {toasts.map((toast) => (
        <div
          key={toast.id}
          className={`${TYPE_STYLES[toast.type]} rounded-lg shadow-lg px-4 py-3 flex items-center gap-3 animate-slide-up`}
        >
          <span className="text-sm flex-1">{toast.message}</span>
          {toast.action && (
            <button
              onClick={() => { toast.action!.onClick(); onDismiss(toast.id) }}
              className="text-sm font-semibold underline hover:no-underline whitespace-nowrap"
            >
              {toast.action.label}
            </button>
          )}
          <button
            onClick={() => onDismiss(toast.id)}
            className="text-white/70 hover:text-white"
          >
            <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
            </svg>
          </button>
        </div>
      ))}
    </div>
  )
}
