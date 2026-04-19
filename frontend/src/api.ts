const API_BASE = '/api'

// Auth state is carried by an HttpOnly cookie set by the backend on /auth/login.
// We cannot read it from JS (that's the point) — instead we call /auth/check
// which returns the user profile if the cookie is valid, and store that in
// module state for the UI to read synchronously.

export type CurrentUser = {
  username: string
  role: string
  homeFolder: string
}

let currentUser: CurrentUser | null = null

export function getCurrentUser(): CurrentUser {
  return (
    currentUser ?? {
      username: '',
      role: '',
      homeFolder: '/',
    }
  )
}

// Shared request options so every call sends the auth cookie.
const FETCH_OPTS: RequestInit = { credentials: 'same-origin' }

let csrfToken: string | null = null

async function ensureCSRF(): Promise<string> {
  if (csrfToken) return csrfToken
  const res = await fetch(`${API_BASE}/csrf`, FETCH_OPTS)
  if (res.ok) {
    const data = await res.json()
    csrfToken = data.csrfToken
    return csrfToken!
  }
  return ''
}

async function writeHeaders(): Promise<Record<string, string>> {
  const csrf = await ensureCSRF()
  return { 'Content-Type': 'application/json', 'X-CSRF-Token': csrf }
}

async function writeHeadersNoContent(): Promise<Record<string, string>> {
  const csrf = await ensureCSRF()
  return { 'X-CSRF-Token': csrf }
}

export async function login(username: string, password: string) {
  const res = await fetch(`${API_BASE}/auth/login`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ username, password }),
  })
  if (res.status === 429) throw new Error('Too many login attempts. Try again later.')
  if (!res.ok) throw new Error('Invalid credentials')
  const data = await res.json()
  currentUser = {
    username: data.username,
    role: data.role,
    homeFolder: data.homeFolder,
  }
  csrfToken = null // force CSRF re-issue for the new session
  return data
}

export async function checkAuth(): Promise<boolean> {
  try {
    const res = await fetch(`${API_BASE}/auth/check`, FETCH_OPTS)
    if (!res.ok) return false
    const data = await res.json()
    if (data.valid) {
      currentUser = {
        username: data.username || '',
        role: data.role || '',
        homeFolder: data.homeFolder || '/',
      }
      return true
    }
    return false
  } catch {
    return false
  }
}

export async function logout() {
  try {
    await fetch(`${API_BASE}/auth/logout`, { ...FETCH_OPTS, method: 'POST' })
  } catch {
    // Network error during logout is fine — cookie may still be cleared on next check.
  }
  currentUser = null
  csrfToken = null
}

export async function changePassword(currentPassword: string, newPassword: string) {
  const res = await fetch(`${API_BASE}/auth/change-password`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ currentPassword, newPassword }),
  })
  if (!res.ok) {
    const text = await res.text()
    throw new Error(text || 'Failed to change password')
  }
  // PwVersion bump invalidates the session — caller should redirect to login.
  return res.json()
}

export async function listFiles(path: string) {
  const res = await fetch(`${API_BASE}/files?path=${encodeURIComponent(path)}`, FETCH_OPTS)
  if (!res.ok) throw new Error('Failed to list files')
  return res.json()
}

export async function downloadFile(path: string) {
  const res = await fetch(`${API_BASE}/files/download?path=${encodeURIComponent(path)}`, FETCH_OPTS)
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

export async function uploadFiles(
  path: string,
  files: File[],
  onProgress?: (pct: number) => void,
) {
  const formData = new FormData()
  for (const file of files) {
    formData.append('files', file)
  }

  const csrf = await ensureCSRF()
  return new Promise<void>((resolve, reject) => {
    const xhr = new XMLHttpRequest()
    xhr.open('POST', `${API_BASE}/files/upload?path=${encodeURIComponent(path)}`)
    xhr.withCredentials = true // send auth cookie
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
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ path, name }),
  })
  if (!res.ok) throw new Error('Failed to create folder')
  return res.json()
}

export async function renameFile(oldPath: string, newName: string) {
  const res = await fetch(`${API_BASE}/files/rename`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ oldPath, newName }),
  })
  if (!res.ok) throw new Error('Failed to rename')
  return res.json()
}

export async function deleteFile(path: string) {
  const res = await fetch(`${API_BASE}/files?path=${encodeURIComponent(path)}`, {
    ...FETCH_OPTS,
    method: 'DELETE',
    headers: await writeHeadersNoContent(),
  })
  if (!res.ok) throw new Error('Failed to delete')
  return res.json()
}

// Preview URL: the auth cookie is sent automatically for same-origin requests,
// so no token in the query string. This used to be the single biggest auth
// leak — tokens in `?token=` landed in CF logs, NPM logs, browser history,
// Referer headers, etc.
export function getPreviewUrl(path: string): string {
  return `${API_BASE}/files/preview?path=${encodeURIComponent(path)}`
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

export function getPreviewType(
  name: string,
): 'image' | 'pdf' | 'video' | 'audio' | 'text' | null {
  const ext = name.split('.').pop()?.toLowerCase() || ''
  if (['jpg', 'jpeg', 'png', 'gif', 'webp', 'svg', 'bmp', 'ico'].includes(ext)) return 'image'
  if (ext === 'pdf') return 'pdf'
  if (['mp4', 'webm', 'ogg'].includes(ext)) return 'video'
  if (['mp3', 'wav', 'flac', 'aac'].includes(ext)) return 'audio'
  if (
    [
      'txt', 'md', 'json', 'yml', 'yaml', 'xml', 'csv', 'log',
      'js', 'ts', 'jsx', 'tsx', 'css', 'html', 'go', 'py', 'sh', 'bat',
    ].includes(ext)
  )
    return 'text'
  return null
}

export async function fetchTextPreview(path: string): Promise<string> {
  const res = await fetch(`${API_BASE}/files/preview?path=${encodeURIComponent(path)}`, FETCH_OPTS)
  if (!res.ok) throw new Error('Failed to load preview')
  return res.text()
}

export async function setFolderPrivate(path: string, allowedUsers?: string[]) {
  const res = await fetch(`${API_BASE}/files/permissions`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ path, allowedUsers: allowedUsers || [] }),
  })
  if (!res.ok) throw new Error('Failed to set permissions')
  return res.json()
}

export async function removeFolderPrivate(path: string) {
  const res = await fetch(`${API_BASE}/files/permissions?path=${encodeURIComponent(path)}`, {
    ...FETCH_OPTS,
    method: 'DELETE',
    headers: await writeHeadersNoContent(),
  })
  if (!res.ok) throw new Error('Failed to remove permissions')
  return res.json()
}

export async function getBackupTier(
  path: string,
): Promise<{ path: string; tier: number; exact: number; inherited: boolean }> {
  const res = await fetch(`${API_BASE}/files/backup-tier?path=${encodeURIComponent(path)}`, FETCH_OPTS)
  if (!res.ok) throw new Error('Failed to get backup tier')
  return res.json()
}

export async function setBackupTier(path: string, tier: number) {
  const res = await fetch(`${API_BASE}/files/backup-tier`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ path, tier }),
  })
  if (!res.ok) throw new Error('Failed to set backup tier')
  return res.json()
}

export async function listBackupTiers(): Promise<Record<string, number>> {
  const res = await fetch(`${API_BASE}/backup-tiers`, FETCH_OPTS)
  if (!res.ok) throw new Error('Failed to list backup tiers')
  return res.json()
}

export async function createShare(
  path: string,
  safe = false,
  expiresIn = 168,
  mode = 'download',
): Promise<{ token: string; url: string; password?: string }> {
  const res = await fetch(`${API_BASE}/shares`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ path, safe, expiresIn, mode }),
  })
  if (!res.ok) throw new Error('Failed to create share')
  return res.json()
}

// Quick access — purely a UI preference, not auth-related, so localStorage is fine.
function quickAccessKey(): string {
  const username = getCurrentUser().username || 'default'
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

// Return types kept loose here — concrete shapes live in types.ts and are
// imported by the components that consume these functions.
// eslint-disable-next-line @typescript-eslint/no-explicit-any
export async function searchFiles(query: string): Promise<any[]> {
  const res = await fetch(`${API_BASE}/files/search?q=${encodeURIComponent(query)}`, FETCH_OPTS)
  if (!res.ok) throw new Error('Search failed')
  return res.json()
}

export async function moveFiles(paths: string[], destination: string) {
  const res = await fetch(`${API_BASE}/files/move`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ paths, destination }),
  })
  if (!res.ok) throw new Error('Move failed')
  return res.json()
}

export async function copyFiles(paths: string[], destination: string) {
  const res = await fetch(`${API_BASE}/files/copy`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ paths, destination }),
  })
  if (!res.ok) throw new Error('Copy failed')
  return res.json()
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export async function getRecentFiles(): Promise<any[]> {
  const res = await fetch(`${API_BASE}/files/recent`, FETCH_OPTS)
  if (!res.ok) throw new Error('Failed to get recent files')
  return res.json()
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export async function listTrash(): Promise<any[]> {
  const res = await fetch(`${API_BASE}/trash`, FETCH_OPTS)
  if (!res.ok) throw new Error('Failed to list trash')
  return res.json()
}

export async function restoreFromTrash(id: string) {
  const res = await fetch(`${API_BASE}/trash/restore`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ id }),
  })
  if (!res.ok) throw new Error('Failed to restore')
  return res.json()
}

export async function deleteFromTrash(id: string) {
  const res = await fetch(`${API_BASE}/trash?id=${encodeURIComponent(id)}`, {
    ...FETCH_OPTS,
    method: 'DELETE',
    headers: await writeHeadersNoContent(),
  })
  if (!res.ok) throw new Error('Failed to delete permanently')
  return res.json()
}

export async function emptyTrash() {
  const res = await fetch(`${API_BASE}/trash/empty`, {
    ...FETCH_OPTS,
    method: 'DELETE',
    headers: await writeHeadersNoContent(),
  })
  if (!res.ok) throw new Error('Failed to empty trash')
  return res.json()
}

export async function extractZip(path: string) {
  const res = await fetch(`${API_BASE}/files/extract`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ path }),
  })
  if (!res.ok) throw new Error('Failed to extract')
  return res.json()
}

export async function compressFiles(paths: string[], name: string) {
  const res = await fetch(`${API_BASE}/files/compress`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ paths, name }),
  })
  if (!res.ok) throw new Error('Failed to compress')
  return res.json()
}

export async function setFileTags(path: string, tags: string[]) {
  const res = await fetch(`${API_BASE}/files/tags`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify({ path, tags }),
  })
  if (!res.ok) throw new Error('Failed to set tags')
  return res.json()
}

// eslint-disable-next-line @typescript-eslint/no-explicit-any
export async function getNotifications(): Promise<any[]> {
  const res = await fetch(`${API_BASE}/notifications`, FETCH_OPTS)
  if (!res.ok) return []
  return res.json()
}

export async function getUnreadCount(): Promise<number> {
  const res = await fetch(`${API_BASE}/notifications/unread`, FETCH_OPTS)
  if (!res.ok) return 0
  const data = await res.json()
  return data.count
}

export async function markNotificationsRead(ids?: string[]) {
  const res = await fetch(`${API_BASE}/notifications/read`, {
    ...FETCH_OPTS,
    method: 'POST',
    headers: await writeHeaders(),
    body: JSON.stringify(ids ? { ids } : { all: true }),
  })
  if (!res.ok) throw new Error('Failed to mark read')
  return res.json()
}

export async function getAuditLog(limit = 200): Promise<{ timestamp: string; action: string; username: string; ip: string; detail: string }[]> {
  const res = await fetch(`${API_BASE}/audit?limit=${limit}`, FETCH_OPTS)
  if (!res.ok) throw new Error('Failed to fetch audit log')
  return res.json()
}

export async function getDiskUsage() {
  const res = await fetch(`${API_BASE}/disk`, FETCH_OPTS)
  if (!res.ok) throw new Error('Failed to get disk usage')
  return res.json()
}
