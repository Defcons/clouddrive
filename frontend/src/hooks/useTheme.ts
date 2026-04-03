import { useState, useEffect } from 'react'

type Theme = 'light' | 'dark'

function getStoredTheme(): Theme {
  const stored = localStorage.getItem('clouddrive_theme')
  if (stored === 'dark' || stored === 'light') return stored
  // Check system preference
  if (window.matchMedia('(prefers-color-scheme: dark)').matches) return 'dark'
  return 'light'
}

export function useTheme() {
  const [theme, setTheme] = useState<Theme>(getStoredTheme)

  useEffect(() => {
    const root = document.documentElement
    if (theme === 'dark') {
      root.classList.add('dark')
    } else {
      root.classList.remove('dark')
    }
    localStorage.setItem('clouddrive_theme', theme)
  }, [theme])

  const toggle = () => setTheme((t) => (t === 'dark' ? 'light' : 'dark'))

  return { theme, toggle }
}
