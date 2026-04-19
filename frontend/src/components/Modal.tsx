import { useEffect, useRef } from 'react'

type Props = {
  open: boolean
  onClose: () => void
  title: string
  children: React.ReactNode
  labelledById?: string
}

/**
 * Modal — a minimal accessible dialog with:
 * - Focus trap (tab cycles inside, first focusable auto-focused)
 * - Esc to close
 * - Backdrop click to close
 * - aria-modal + role=dialog
 * - Restores focus to the previously-focused element on close
 */
export default function Modal({ open, onClose, title, children, labelledById }: Props) {
  const dialogRef = useRef<HTMLDivElement>(null)
  const previousActiveRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    if (!open) return
    previousActiveRef.current = document.activeElement as HTMLElement | null

    // Focus first focusable element inside dialog.
    const focusFirst = () => {
      const focusables = dialogRef.current?.querySelectorAll<HTMLElement>(
        'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
      )
      if (focusables && focusables.length > 0) {
        focusables[0].focus()
      } else {
        dialogRef.current?.focus()
      }
    }
    const t = setTimeout(focusFirst, 0)

    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onClose()
        return
      }
      if (e.key === 'Tab' && dialogRef.current) {
        const focusables = Array.from(
          dialogRef.current.querySelectorAll<HTMLElement>(
            'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
          ),
        ).filter((el) => el.offsetParent !== null || el === document.activeElement)

        if (focusables.length === 0) return
        const first = focusables[0]
        const last = focusables[focusables.length - 1]

        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault()
          last.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }

    document.addEventListener('keydown', onKeyDown)
    const prevOverflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'

    return () => {
      clearTimeout(t)
      document.removeEventListener('keydown', onKeyDown)
      document.body.style.overflow = prevOverflow
      // Restore focus to whatever was focused before the modal opened.
      previousActiveRef.current?.focus?.()
    }
  }, [open, onClose])

  if (!open) return null

  return (
    <div
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/50 p-4"
      onMouseDown={(e) => {
        if (e.target === e.currentTarget) onClose()
      }}
    >
      <div
        ref={dialogRef}
        role="dialog"
        aria-modal="true"
        aria-labelledby={labelledById || 'modal-title'}
        tabIndex={-1}
        className="bg-white dark:bg-gray-800 rounded-2xl shadow-xl max-w-md w-full max-h-[90vh] overflow-auto focus:outline-none"
      >
        <div className="px-6 pt-6 pb-2">
          <h2 id={labelledById || 'modal-title'} className="text-lg font-semibold text-gray-900 dark:text-gray-100">
            {title}
          </h2>
        </div>
        <div className="px-6 pb-6">{children}</div>
      </div>
    </div>
  )
}
