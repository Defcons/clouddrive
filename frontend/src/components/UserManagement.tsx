import { useState, useEffect, useCallback } from 'react'
import { listUsers, createUser, updateUser, deleteUser, getCurrentUser, type AdminUser } from '../api'
import { useToast } from '../hooks/useToast'
import { confirm as confirmModal } from './ConfirmModal'

const MB = 1024 * 1024

function quotaLabel(bytes: number): string {
  if (!bytes) return 'Unlimited'
  if (bytes >= 1024 * MB) return `${(bytes / (1024 * MB)).toFixed(1)} GB`
  return `${Math.round(bytes / MB)} MB`
}

export default function UserManagement() {
  const [users, setUsers] = useState<AdminUser[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [showAdd, setShowAdd] = useState(false)
  const toast = useToast()
  const me = getCurrentUser().username

  const refresh = useCallback(() => {
    setLoading(true)
    setError('')
    listUsers()
      .then(setUsers)
      .catch(() => setError('Couldn’t load users.'))
      .finally(() => setLoading(false))
  }, [])

  useEffect(() => { refresh() }, [refresh])

  const handleDelete = async (username: string) => {
    const ok = await confirmModal({
      title: 'Delete user?',
      message: `Delete "${username}"? Their files are NOT removed, but they can no longer sign in.`,
      destructive: true,
      confirmLabel: 'Delete user',
    })
    if (!ok) return
    try {
      await deleteUser(username)
      toast.success(`Deleted ${username}`)
      refresh()
    } catch (e: any) {
      toast.error(e?.message || 'Failed to delete user')
    }
  }

  return (
    <div>
      <div className="flex items-center justify-between mb-2">
        <h3 className="text-sm font-semibold text-gray-700 dark:text-gray-300">Users</h3>
        <button
          onClick={() => setShowAdd((s) => !s)}
          className="text-xs px-2.5 py-1 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition"
        >
          {showAdd ? 'Cancel' : 'Add user'}
        </button>
      </div>

      {showAdd && (
        <AddUserForm
          onDone={() => { setShowAdd(false); refresh() }}
          onError={(m) => toast.error(m)}
          onSuccess={(m) => toast.success(m)}
        />
      )}

      {loading ? (
        <div className="text-gray-400 text-sm py-4 text-center">Loading…</div>
      ) : error ? (
        <div className="text-red-500 text-sm py-4 text-center">{error}</div>
      ) : (
        <div className="space-y-2 mt-2">
          {users.map((u) => (
            <UserRow
              key={u.username}
              user={u}
              isSelf={u.username === me}
              onSaved={(m) => { toast.success(m); refresh() }}
              onError={(m) => toast.error(m)}
              onDelete={() => handleDelete(u.username)}
            />
          ))}
        </div>
      )}
    </div>
  )
}

function AddUserForm({
  onDone,
  onError,
  onSuccess,
}: {
  onDone: () => void
  onError: (m: string) => void
  onSuccess: (m: string) => void
}) {
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [homeFolder, setHomeFolder] = useState('')
  const [role, setRole] = useState('user')
  const [quotaMB, setQuotaMB] = useState('')
  const [busy, setBusy] = useState(false)

  const submit = async () => {
    setBusy(true)
    try {
      await createUser({
        username: username.trim(),
        password,
        homeFolder: homeFolder.trim() || `/${username.trim()}`,
        role,
        quota: quotaMB ? Math.round(Number(quotaMB) * MB) : 0,
      })
      onSuccess(`Created ${username}`)
      onDone()
    } catch (e: any) {
      onError(e?.message || 'Failed to create user')
    } finally {
      setBusy(false)
    }
  }

  const inputCls =
    'w-full px-2 py-1 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 rounded-md focus:outline-none focus:ring-1 focus:ring-blue-500'

  return (
    <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-3 space-y-2 mb-2">
      <input className={inputCls} placeholder="Username" value={username} onChange={(e) => setUsername(e.target.value)} />
      <input className={inputCls} type="password" placeholder="Password (min 8 chars)" value={password} onChange={(e) => setPassword(e.target.value)} />
      <input className={inputCls} placeholder="Home folder (default /username)" value={homeFolder} onChange={(e) => setHomeFolder(e.target.value)} />
      <div className="flex gap-2">
        <select className={inputCls} value={role} onChange={(e) => setRole(e.target.value)}>
          <option value="user">User</option>
          <option value="admin">Admin</option>
        </select>
        <input className={inputCls} type="number" min="0" placeholder="Quota MB (0 = ∞)" value={quotaMB} onChange={(e) => setQuotaMB(e.target.value)} />
      </div>
      <button
        onClick={submit}
        disabled={busy || !username.trim() || password.length < 8}
        className="w-full text-sm py-1.5 bg-green-600 text-white rounded-md hover:bg-green-700 transition disabled:opacity-50"
      >
        {busy ? 'Creating…' : 'Create user'}
      </button>
    </div>
  )
}

function UserRow({
  user,
  isSelf,
  onSaved,
  onError,
  onDelete,
}: {
  user: AdminUser
  isSelf: boolean
  onSaved: (m: string) => void
  onError: (m: string) => void
  onDelete: () => void
}) {
  const [editing, setEditing] = useState(false)
  const [role, setRole] = useState(user.role)
  const [homeFolder, setHomeFolder] = useState(user.homeFolder)
  const [quotaMB, setQuotaMB] = useState(user.quota ? String(Math.round(user.quota / MB)) : '')
  const [newPassword, setNewPassword] = useState('')
  const [busy, setBusy] = useState(false)

  const save = async () => {
    setBusy(true)
    try {
      await updateUser({
        username: user.username,
        homeFolder: homeFolder.trim() || '/',
        role,
        quota: quotaMB ? Math.round(Number(quotaMB) * MB) : 0,
        newPassword: newPassword || undefined,
      })
      onSaved(`Updated ${user.username}`)
      setEditing(false)
      setNewPassword('')
    } catch (e: any) {
      onError(e?.message || 'Failed to update user')
    } finally {
      setBusy(false)
    }
  }

  const inputCls =
    'w-full px-2 py-1 text-sm border border-gray-300 dark:border-gray-600 dark:bg-gray-700 dark:text-gray-200 rounded-md focus:outline-none focus:ring-1 focus:ring-blue-500'

  return (
    <div className="border border-gray-200 dark:border-gray-700 rounded-lg p-2.5">
      <div className="flex items-center justify-between gap-2">
        <div className="min-w-0">
          <div className="text-sm font-medium text-gray-800 dark:text-gray-200 truncate">
            {user.username}
            {user.role === 'admin' && <span className="ml-2 text-[10px] px-1.5 py-0.5 bg-orange-100 text-orange-700 dark:bg-orange-900/40 dark:text-orange-400 rounded-full">admin</span>}
            {user.mfaEnabled && <span className="ml-1 text-[10px] px-1.5 py-0.5 bg-green-100 text-green-700 dark:bg-green-900/40 dark:text-green-400 rounded-full">MFA</span>}
          </div>
          <div className="text-xs text-gray-400 truncate">{user.homeFolder} · {quotaLabel(user.quota)}</div>
        </div>
        <div className="flex gap-1 flex-shrink-0">
          <button onClick={() => setEditing((e) => !e)} className="text-xs px-2 py-1 text-blue-600 hover:bg-blue-50 dark:hover:bg-blue-900/30 rounded transition">
            {editing ? 'Close' : 'Edit'}
          </button>
          {!isSelf && (
            <button onClick={onDelete} className="text-xs px-2 py-1 text-red-600 hover:bg-red-50 dark:hover:bg-red-900/30 rounded transition">
              Delete
            </button>
          )}
        </div>
      </div>

      {editing && (
        <div className="mt-2 space-y-2">
          <input className={inputCls} value={homeFolder} onChange={(e) => setHomeFolder(e.target.value)} placeholder="Home folder" />
          <div className="flex gap-2">
            <select className={inputCls} value={role} onChange={(e) => setRole(e.target.value)} disabled={isSelf}>
              <option value="user">User</option>
              <option value="admin">Admin</option>
            </select>
            <input className={inputCls} type="number" min="0" value={quotaMB} onChange={(e) => setQuotaMB(e.target.value)} placeholder="Quota MB (0 = ∞)" />
          </div>
          <input className={inputCls} type="password" value={newPassword} onChange={(e) => setNewPassword(e.target.value)} placeholder="New password (leave blank to keep)" />
          <button onClick={save} disabled={busy} className="w-full text-sm py-1.5 bg-blue-600 text-white rounded-md hover:bg-blue-700 transition disabled:opacity-50">
            {busy ? 'Saving…' : 'Save changes'}
          </button>
        </div>
      )}
    </div>
  )
}
