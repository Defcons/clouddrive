const API_BASE = '/api'

function getToken(): string | null {
  return localStorage.getItem('token')
}

function authHeaders(): Record<string, string> {
  const token = getToken()
  return token ? { Authorization: `Bearer ${token}` } : {}
}

export async function login(username: string, password: string) {
  const res = await fetch(`${API_BASE}/auth/login`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (res.status === 429) throw new Error('Too many login attempts. Try again later.')
  if (!res.ok) throw new Error('Invalid credentials')
  const data = await res.json()
  localStorage.setItem('token', data.token)
  localStorage.setItem('username', data.username)
  localStorage.setItem('role', data.role)
  localStorage.setItem('homeFolder', data.homeFolder)
  return data
}

export async function checkAuth(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/auth/check`, {
      headers: authHeaders(),
    })
    return res.ok
  } catch {
    return false
  }
}

export function logout() {
  localStorage.removeItem('token')
  localStorage.removeItem('username')
  localStorage.removeItem('role')
  localStorage.removeItem('homeFolder')
}

export function getCurrentUser() {
  return {
    username: localStorage.getItem('username') || '',
    role: localStorage.getItem('role') || '',
    homeFolder: localStorage.getItem('homeFolder') || '/',
  }
}

export async function changePassword(currentPassword: string, newPassword: string) {
  const res = await fetch(`${API_BASE}/auth/change-password`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ currentPassword, newPassword }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Failed to change password')
  }
  return res.json()
}

export async function listFiles(path: string) {
  const res = await fetch(`${API_BASE}/files?path=${encodeURIComponent(path)}`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to list files')
  return res.json()
}

export async function downloadFile(path: string) {
  const res = await fetch(`${API_BASE}/files/download?path=${encodeURIComponent(path)}`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to download')
  const blob = await res.blob()
  const url = URL.createObjectURL(blob)
  const a = document.createElement('a')
  a.href = url
  a.download = path.split('/').pop() || 'download'
  document.body.appendChild(a)
  a.click()
  document.body.removeChild(a)
  URL.revokeObjectURL(url)
}

export async function uploadFiles(path: string, files: File[], onProgress?: (pct: number) => void) {
  const formData = new FormData()
  for (const file of files) {
    formData.append('files', file)
  }

  return new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}/files/upload?path=${encodeURIComponent(path)}`)
    const token = getToken()
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)

    xhr.upload.onprogress = (e) => {
      if (e.lengthComputable && onProgress) {
        onProgress(Math.round((e.loaded / e.total) * 100))
      }
    }
    xhr.onload = () => (xhr.status < 300 ? resolve() : reject(new Error('Upload failed')))
    xhr.onerror = () => reject(new Error('Upload failed'))
    xhr.send(formData)
  })
}

export async function createFolder(path: string, name: string) {
  const res = await fetch(`${API_BASE}/files/mkdir`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ path, name }),
  })
  if (!res.ok) throw new Error('Failed to create folder')
  return res.json()
}

export async function renameFile(oldPath: string, newName: string) {
  const res = await fetch(`${API_BASE}/files/rename`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ oldPath, newName }),
  })
  if (!res.ok) throw new Error('Failed to rename')
  return res.json()
}

export async function deleteFile(path: string) {
  const res = await fetch(`${API_BASE}/files?path=${encodeURIComponent(path)}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to delete')
  return res.json()
}

export function getPreviewUrl(path: string): string {
  const token = getToken()
  return `${API_BASE}/files/preview?path=${encodeURIComponent(path)}&token=${encodeURIComponent(token || '')}`
}

const PREVIEWABLE_EXTENSIONS = new Set([
  'jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico',
  'pdf',
  'mp4', 'webm', 'ogg',
  'mp3', 'wav', 'flac', 'aac',
  'txt', 'md', 'json', 'yml', 'yaml', 'xml', 'csv', 'log',
  'js', 'ts', 'jsx', 'tsx', 'css', 'html', 'go', 'py', 'sh', 'bat',
])

export function isPreviewable(name: string): boolean {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  return PREVIEWABLE_EXTENSIONS.has(ext)
}

export function getPreviewType(name: string): 'image' | 'pdf' | 'video' | 'audio' | 'text' | null {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico'].includes(ext)) return 'image'
  if (ext === 'pdf') return 'pdf'
  if (['mp4', 'webm', 'ogg'].includes(ext)) return 'video'
  if (['mp3', 'wav', 'flac', 'aac'].includes(ext)) return 'audio'
  if (['txt', 'md', 'json', 'yml', 'yaml', 'xml', 'csv', 'log', 'js', 'ts', 'jsx', 'tsx', 'css', 'html', 'go', 'py', 'sh', 'bat'].includes(ext)) return 'text'
  return null
}

export async function fetchTextPreview(path: string): Promise<string> {
  const res = await fetch(`${API_BASE}/files/preview?path=${encodeURIComponent(path)}`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to load preview')
  return res.text()
}

export async function setFolderPrivate(path: string, allowedUsers?: string[]) {
  const res = await fetch(`${API_BASE}/files/permissions`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ path, allowedUsers: allowedUsers || [] }),
  })
  if (!res.ok) throw new Error('Failed to set permissions')
  return res.json()
}

export async function removeFolderPrivate(path: string) {
  const res = await fetch(`${API_BASE}/files/permissions?path=${encodeURIComponent(path)}`, {
    method: 'DELETE',
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to remove permissions')
  return res.json()
}

export async function createShare(path: string, safe = false): Promise<{ token: string; url: string; password?: string }> {
  const res = await fetch(`${API_BASE}/shares`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json', ...authHeaders() },
    body: JSON.stringify({ path, safe }),
  })
  if (!res.ok) throw new Error('Failed to create share')
  return res.json()
}

// Quick access (stored in localStorage, per-user)
function quickAccessKey(): string {
  const username = localStorage.getItem('username') || 'default'
  return `clouddrive_quick_access_${username}`
}

export function getQuickAccess(): { name: string; path: string }[] {
  try {
    return JSON.parse(localStorage.getItem(quickAccessKey()) || '[]')
  } catch {
    return []
  }
}

export function addQuickAccess(name: string, path: string) {
  const items = getQuickAccess()
  if (!items.find((i) => i.path === path)) {
    items.push({ name, path })
    localStorage.setItem(quickAccessKey(), JSON.stringify(items))
  }
}

export function removeQuickAccess(path: string) {
  const items = getQuickAccess().filter((i) => i.path !== path)
  localStorage.setItem(quickAccessKey(), JSON.stringify(items))
}

export async function getDiskUsage() {
  const res = await fetch(`${API_BASE}/disk`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to get disk usage')
  return res.json()
}
