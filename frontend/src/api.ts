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
  if (!res.ok) throw new Error('Invalid credentials')
  const data = await res.json()
  localStorage.setItem('token', data.token)
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

export async function getDiskUsage() {
  const res = await fetch(`${API_BASE}/disk`, {
    headers: authHeaders(),
  })
  if (!res.ok) throw new Error('Failed to get disk usage')
  return res.json()
}
