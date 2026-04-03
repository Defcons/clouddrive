export default function LoadingSkeleton() {
  return (
    <div className="space-y-1 p-2">
      {Array.from({ length: 8 }).map((_, i) => (
        <div key={i} className="flex items-center gap-3 py-2 px-2">
          <div className="skeleton w-5 h-5 rounded" />
          <div className="skeleton h-4 flex-1" style={{ maxWidth: `${200 + Math.random() * 200}px` }} />
          <div className="skeleton h-4 w-16" />
          <div className="skeleton h-4 w-28" />
          <div className="skeleton h-4 w-28" />
        </div>
      ))}
    </div>
  )
}
