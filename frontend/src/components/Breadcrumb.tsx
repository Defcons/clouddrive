interface Props {
  path: string
  homeFolder: string
  onNavigate: (path: string) => void
}

export default function Breadcrumb({ path, homeFolder, onNavigate }: Props) {
  const homeParts = (homeFolder || '/').split('/').filter(Boolean)
  const parts = path.split('/').filter(Boolean)

  // Only show parts at or below the home folder
  const visibleParts = parts.slice(homeParts.length)

  // Collapse long paths: show first, "...", last 2
  const MAX_VISIBLE = 4
  let displayParts = visibleParts
  let collapsed = false
  if (visibleParts.length > MAX_VISIBLE) {
    collapsed = true
    displayParts = [
      visibleParts[0],
      '...',
      ...visibleParts.slice(-2),
    ]
  }

  return (
    <nav className="flex items-center gap-1 text-sm overflow-x-auto whitespace-nowrap py-1 scrollbar-none">
      <button
        onClick={() => onNavigate(homeFolder || '/')}
        className="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:underline font-medium flex-shrink-0"
      >
        Home
      </button>
      {displayParts.map((part, i) => {
        if (part === '...') {
          return (
            <span key="ellipsis" className="flex items-center gap-1">
              <span className="text-gray-400 dark:text-gray-600">/</span>
              <span className="text-gray-400 dark:text-gray-600">...</span>
            </span>
          )
        }

        // Calculate the real index in visibleParts
        let realIndex: number
        if (!collapsed) {
          realIndex = i
        } else if (i === 0) {
          realIndex = 0
        } else {
          realIndex = visibleParts.length - (displayParts.length - i)
        }

        const partPath = '/' + parts.slice(0, homeParts.length + realIndex + 1).join('/')
        const isLast = realIndex === visibleParts.length - 1

        return (
          <span key={partPath} className="flex items-center gap-1">
            <span className="text-gray-400 dark:text-gray-600">/</span>
            {isLast ? (
              <span className="text-gray-800 dark:text-gray-200 font-medium">{part}</span>
            ) : (
              <button
                onClick={() => onNavigate(partPath)}
                className="text-blue-600 dark:text-blue-400 hover:text-blue-800 dark:hover:text-blue-300 hover:underline"
              >
                {part}
              </button>
            )}
          </span>
        )
      })}
    </nav>
  )
}
