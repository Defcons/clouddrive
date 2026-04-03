const EXT_COLORS: Record<string, string> = {
  pdf: 'text-red-500',
  doc: 'text-blue-600', docx: 'text-blue-600',
  xls: 'text-green-600', xlsx: 'text-green-600', csv: 'text-green-600',
  ppt: 'text-orange-500', pptx: 'text-orange-500',
  zip: 'text-yellow-600', rar: 'text-yellow-600', '7z': 'text-yellow-600', tar: 'text-yellow-600', gz: 'text-yellow-600',
  png: 'text-purple-500', jpg: 'text-purple-500', jpeg: 'text-purple-500', gif: 'text-purple-500', svg: 'text-purple-500', webp: 'text-purple-500',
  mp4: 'text-pink-500', mkv: 'text-pink-500', avi: 'text-pink-500', mov: 'text-pink-500',
  mp3: 'text-indigo-500', wav: 'text-indigo-500', flac: 'text-indigo-500', ogg: 'text-indigo-500',
  js: 'text-yellow-500', ts: 'text-blue-500', py: 'text-green-500', go: 'text-cyan-500',
  txt: 'text-gray-500', md: 'text-gray-600', json: 'text-gray-600', yml: 'text-gray-600', yaml: 'text-gray-600',
}

export default function FileIcon({ name, isDir }: { name: string; isDir: boolean }) {
  if (isDir) {
    return (
      <svg className="w-5 h-5 text-blue-400 flex-shrink-0" fill="currentColor" viewBox="0 0 20 20">
        <path d="M2 6a2 2 0 012-2h5l2 2h5a2 2 0 012 2v6a2 2 0 01-2 2H4a2 2 0 01-2-2V6z" />
      </svg>
    )
  }

  const ext = name.split('.').pop()?.toLowerCase() || ''
  const color = EXT_COLORS[ext] || 'text-gray-400'

  return (
    <svg className={`w-5 h-5 ${color} flex-shrink-0`} fill="none" stroke="currentColor" viewBox="0 0 24 24">
      <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={1.5} d="M7 21h10a2 2 0 002-2V9.414a1 1 0 00-.293-.707l-5.414-5.414A1 1 0 0012.586 3H7a2 2 0 00-2 2v14a2 2 0 002 2z" />
    </svg>
  )
}
