const API_BASE = '/api'

function getToken(): string | null {
  return localStorage.getItem('token')
}

function authHeaders(): Record<string, string> {
  const token = getToken()
  return token ? { Authorization: `Bearer ${token}` } : {}
}

let csrfToken: string | null = null

async function ensureCSRF(): Promise<string> {
  if (csrfToken) return csrfToken
  const res = await fetch(`${API_BASE}/csrf`, { headers: authHeaders() })
  if (res.ok) {
    const data = await res.json()
    csrfToken = data.csrfToken
    return csrfToken!
  }
  return ''
}

async function writeHeaders(): Promise<Record<string, string>> {
  const csrf = await ensureCSRF()
  return { ...authHeaders(), 'Content-Type': 'application/json', 'X-CSRF-Token': csrf }
}

async function writeHeadersNoContent(): Promise<Record<string, string>> {
  const csrf = await ensureCSRF()
  return { ...authHeaders(), 'X-CSRF-Token': csrf }
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
  csrfToken = null
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
    headers: await writeHeaders(),
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

  const csrf = await ensureCSRF()
  return new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}/files/upload?path=${encodeURIComponent(path)}`)
    const token = getToken()
    if (token) xhr.setRequestHeader('Authorization', `Bearer ${token}`)
    if (csrf) xhr.setRequestHeader('X-CSRF-Token', csrf)

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
    headers: await writeHeaders(),
    body: JSON.stringify({ path, name }),
  })
  if (!res.ok) throw new Error('Failed to create folder')
  return res.json()
}

export async function renameFile(oldPath: string, newName: string) {
  const res = await fetch(`${API_BASE}/files/rename`, {
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ oldPath, newName }),
  })
  if (!res.ok) throw new Error('Failed to rename')
  return res.json()
}

export async function deleteFile(path: string) {
  const res = await fetch(`${API_BASE}/files?path=${encodeURIComponent(path)}`, {
    method: 'DELETE',
    headers: await writeHeadersNoContent(),
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
    headers: await writeHeaders(),
    body: JSON.stringify({ path, allowedUsers: allowedUsers || [] }),
  })
  if (!res.ok) throw new Error('Failed to set permissions')
  return res.json()
}

export async function removeFolderPrivate(path: string) {
  const res = await fetch(`${API_BASE}/files/permissions?path=${encodeURIComponent(path)}`, {
    method: 'DELETE',
    headers: await writeHeadersNoContent(),
  })
  if (!res.ok) throw new Error('Failed to remove permissions')
  return res.json()
}

export async function getBackupTier(path: string): Promise<{ path: string; tier: number; exact: number; inherited: boolean }> {
  const res = await fetch(`${API_BASE}/files/backup-tier?path=${encodeURIComponent(path)}`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to get backup tier')
  return res.json()
}

export async function setBackupTier(path: string, tier: number) {
  const res = await fetch(`${API_BASE}/files/backup-tier`, {
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ path, tier }),
  })
  if (!res.ok) throw new Error('Failed to set backup tier')
  return res.json()
}

export async function listBackupTiers(): Promise<Record<string, number>> {
  const res = await fetch(`${API_BASE}/backup-tiers`, { headers: authHeaders() })
  if (!res.ok) throw new Error('Failed to list backup tiers')
  return res.json()
}

export async function createShare(path: string, safe = false, expiresIn = 168, mode = 'download'): Promise<{ token: string; url: string; password?: string }> {
  const res = await fetch(`${API_BASE}/shares`, {
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ path, safe, expiresIn, mode }),
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

// Search
export async function searchFiles(query: string): Promise<any[]> {
  const res = await fetch(`${API_BASE}/files/search?q=${encodeURIComponent(query)}`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Search failed')
  return res.json()
}

// Move / Copy
export async function moveFiles(paths: string[], destination: string) {
  const res = await fetch(`${API_BASE}/files/move`, {
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ paths, destination }),
  })
  if (!res.ok) throw new Error('Move failed')
  return res.json()
}

export async function copyFiles(paths: string[], destination: string) {
  const res = await fetch(`${API_BASE}/files/copy`, {
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ paths, destination }),
  })
  if (!res.ok) throw new Error('Copy failed')
  return res.json()
}

// Recent files
export async function getRecentFiles(): Promise<any[]> {
  const res = await fetch(`${API_BASE}/files/recent`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to get recent files')
  return res.json()
}

// Trash
export async function listTrash(): Promise<any[]> {
  const res = await fetch(`${API_BASE}/trash`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to list trash')
  return res.json()
}

export async function restoreFromTrash(id: string) {
  const res = await fetch(`${API_BASE}/trash/restore`, {
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ id }),
  })
  if (!res.ok) throw new Error('Failed to restore')
  return res.json()
}

export async function deleteFromTrash(id: string) {
  const res = await fetch(`${API_BASE}/trash?id=${encodeURIComponent(id)}`, {
    method: 'DELETE',
    headers: await writeHeadersNoContent(),
  })
  if (!res.ok) throw new Error('Failed to delete permanently')
  return res.json()
}

export async function emptyTrash() {
  const res = await fetch(`${API_BASE}/trash/empty`, {
    method: 'DELETE',
    headers: await writeHeadersNoContent(),
  })
  if (!res.ok) throw new Error('Failed to empty trash')
  return res.json()
}

// Extract / Compress
export async function extractZip(path: string) {
  const res = await fetch(`${API_BASE}/files/extract`, {
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ path }),
  })
  if (!res.ok) throw new Error('Failed to extract')
  return res.json()
}

export async function compressFiles(paths: string[], name: string) {
  const res = await fetch(`${API_BASE}/files/compress`, {
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ paths, name }),
  })
  if (!res.ok) throw new Error('Failed to compress')
  return res.json()
}

// Tags
export async function setFileTags(path: string, tags: string[]) {
  const res = await fetch(`${API_BASE}/files/tags`, {
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ path, tags }),
  })
  if (!res.ok) throw new Error('Failed to set tags')
  return res.json()
}

// Notifications
export async function getNotifications(): Promise<any[]> {
  const res = await fetch(`${API_BASE}/notifications`, {
    headers: authHeaders(),
  })
  if (!res.ok) return []
  return res.json()
}

export async function getUnreadCount(): Promise<number> {
  const res = await fetch(`${API_BASE}/notifications/unread`, {
    headers: authHeaders(),
  })
  if (!res.ok) return 0
  const data = await res.json()
  return data.count
}

export async function markNotificationsRead(ids?: string[]) {
  const res = await fetch(`${API_BASE}/notifications/read`, {
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify(ids ? { ids } : { all: true }),
  })
  if (!res.ok) throw new Error('Failed to mark read')
  return res.json()
}

export async function getAuditLog(limit = 200): Promise<{ timestamp: string; action: string; username: string; ip: string; detail: string }[]> {
  const res = await fetch(`${API_BASE}/audit?limit=${limit}`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to fetch audit log')
  return res.json()
}

export async function getDiskUsage() {
  const res = await fetch(`${API_BASE}/disk`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to get disk usage')
  return res.json()
}
