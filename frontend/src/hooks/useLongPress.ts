import { useCallback, useRef } from 'react'

export function useLongPress(callback: (e: React.TouchEvent) => void, delay = 500) {
  const timerRef = useRef<ReturnType<typeof setTimeout> | null>(null)
  const targetRef = useRef<EventTarget | null>(null)

  const start = useCallback((e: React.TouchEvent) => {
    targetRef.current = e.target
    timerRef.current = setTimeout(() => {
      callback(e)
    }, delay)
  }, [callback, delay])

  const clear = useCallback(() => {
    if (timerRef.current) {
      clearTimeout(timerRef.current)
      timerRef.current = null
    }
  }, [])

  return {
    onTouchStart: start,
    onTouchEnd: clear,
    onTouchMove: clear,
  }
}
