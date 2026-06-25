import { useEffect, useRef } from 'react'

/**
 * useDialog wires modal accessibility onto a container element without changing
 * its markup/layout:
 *  - Escape closes
 *  - focus moves into the dialog on open and is restored to the trigger on close
 *  - Tab is trapped within the dialog
 *
 * Returns a ref to attach to the dialog container. Also give that element
 * role="dialog" aria-modal="true" and tabIndex={-1}.
 */
export function useDialog<T extends HTMLElement>(onClose: () => void) {
  const ref = useRef<T>(null)

  useEffect(() => {
    const previous = document.activeElement as HTMLElement | null
    const el = ref.current

    const focusables = () =>
      Array.from(
        el?.querySelectorAll<HTMLElement>(
          'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
        ) ?? [],
      ).filter((n) => n.offsetParent !== null)

    // Focus the first focusable element, or the container itself.
    ;(focusables()[0] ?? el)?.focus()

    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault()
        onClose()
        return
      }
      if (e.key === 'Tab' && el) {
        const f = focusables()
        if (f.length === 0) return
        const first = f[0]
        const last = f[f.length - 1]
        if (e.shiftKey && document.activeElement === first) {
          e.preventDefault()
          last.focus()
        } else if (!e.shiftKey && document.activeElement === last) {
          e.preventDefault()
          first.focus()
        }
      }
    }

    document.addEventListener('keydown', onKey)
    return () => {
      document.removeEventListener('keydown', onKey)
      previous?.focus?.()
    }
  }, [onClose])

  return ref
}
