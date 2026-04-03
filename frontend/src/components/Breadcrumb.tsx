interface Props {
  path: string
  onNavigate: (path: string) => void
}

export default function Breadcrumb({ path, onNavigate }: Props) {
  const parts = path.split('/').filter(Boolean)

  return (
    <nav className="flex items-center gap-1 text-sm overflow-x-auto whitespace-nowrap py-1">
      <button
        onClick={() => onNavigate('/')}
        className="text-blue-600 hover:text-blue-800 hover:underline font-medium flex-shrink-0"
      >
        Home
      </button>
      {parts.map((part, i) => {
        const partPath = '/' + parts.slice(0, i + 1).join('/')
        const isLast = i === parts.length - 1
        return (
          <span key={partPath} className="flex items-center gap-1">
            <span className="text-gray-400">/</span>
            {isLast ? (
              <span className="text-gray-800 font-medium">{part}</span>
            ) : (
              <button
                onClick={() => onNavigate(partPath)}
                className="text-blue-600 hover:text-blue-800 hover:underline"
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
