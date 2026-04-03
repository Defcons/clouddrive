import { useState, useEffect, useRef } from 'react'

export default function UpdateToast() {
  const [visible, setVisible] = useState(false)
  const startTimeRef = useRef<number | null>(null)

  useEffect(() => {
    const poll = async () => {
      try {
        const res = await fetch('/api/version')
        if (!res.ok) return
        const data = await res.json()

        if (startTimeRef.current === null) {
          startTimeRef.current = data.startTime
        } else if (data.startTime !== startTimeRef.current) {
          setVisible(true)
          startTimeRef.current = data.startTime
        }
      } catch {
        // ignore
      }
    }

    poll()
    const interval = setInterval(poll, 30000)
    return () => clearInterval(interval)
  }, [])

  useEffect(() => {
    if (!visible) return
    const timer = setTimeout(() => setVisible(false), 12000)
    return () => clearTimeout(timer)
  }, [visible])

  return (
    <div
      className={`fixed left-1/2 -translate-x-1/2 z-[100] transition-all duration-500 ease-[cubic-bezier(0.34,1.56,0.64,1)] ${
        visible ? 'top-20 opacity-100' : '-top-20 opacity-0 pointer-events-none'
      }`}
    >
      <div className="bg-white border border-blue-200 rounded-xl shadow-lg px-5 py-3 flex items-center gap-3">
        <div className="w-2 h-2 bg-blue-500 rounded-full animate-pulse" />
        <span className="text-sm text-gray-700">
          A new update has been deployed
        </span>
        <button
          onClick={() => window.location.reload()}
          className="px-3 py-1 bg-blue-600 text-white text-xs font-medium rounded-lg hover:bg-blue-700 transition"
        >
          Refresh
        </button>
        <button
          onClick={() => setVisible(false)}
          className="text-gray-400 hover:text-gray-600 ml-1"
        >
          <svg className="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
          </svg>
        </button>
      </div>
    </div>
  )
}
