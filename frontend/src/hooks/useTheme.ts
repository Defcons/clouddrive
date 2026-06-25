import { useState, useEffect } from 'react'

type Theme = 'light' | 'dark'

const STORAGE_KEY = 'clouddrive_theme'
// Sentinel stored for users who haven't explicitly picked; we keep following
// the OS preference for them.
const SYSTEM = 'system'

function readStored(): Theme | 'system' {
  try {
    const stored = localStorage.getItem(STORAGE_KEY)
    if (stored === 'dark' || stored === 'light') return stored
  } catch {
    // localStorage unavailable (private mode / disabled) — follow the OS.
  }
  return SYSTEM
}

function systemTheme(): Theme {
  return window.matchMedia?.('(prefers-color-scheme: dark)').matches ? 'dark' : 'light'
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(() => {
    const stored = readStored()
    return stored === SYSTEM ? systemTheme() : stored
  })

  // If the user is following the OS preference, react to OS changes.
  useEffect(() => {
    if (readStored() !== SYSTEM) return
    const mq = window.matchMedia?.('(prefers-color-scheme: dark)')
    if (!mq) return
    const handler = (e: MediaQueryListEvent) => setTheme(e.matches ? 'dark' : 'light')
    mq.addEventListener('change', handler)
    return () => mq.removeEventListener('change', handler)
  }, [])

  // Apply theme to <html>.
  useEffect(() => {
    const root = document.documentElement
    root.classList.toggle('dark', theme === 'dark')
  }, [theme])

  const toggle = () => {
    setTheme((t) => {
      const next: Theme = t === 'dark' ? 'light' : 'dark'
      try {
        localStorage.setItem(STORAGE_KEY, next)
      } catch {
        // Non-fatal: theme still applies for this session.
      }
      return next
    })
  }

  return { theme, toggle }
}
