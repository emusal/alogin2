import { useState, useEffect } from 'react'
import type { Server, AppServer } from '../types'
import { AppServerFormModal } from './AppServerFormModal'

interface Props {
  servers: Server[]
  onConnect: (serverId: number, autoGW: boolean, app: string) => void
}

export function AppServerList({ servers, onConnect }: Props) {
  const [appServers, setAppServers] = useState<AppServer[]>([])
  const [loading, setLoading] = useState(true)
  const [modalMode, setModalMode] = useState<'add' | 'edit' | null>(null)
  const [editingAS, setEditingAS] = useState<AppServer | null>(null)

  useEffect(() => {
    fetch('/api/app-servers')
      .then(r => r.json())
      .then(data => { setAppServers(Array.isArray(data) ? data : []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  const serverLabel = (serverId: number) => {
    const s = servers.find(x => x.id === serverId)
    return s ? `${s.user}@${s.host}` : `id=${serverId}`
  }

  const handleDelete = async (as: AppServer) => {
    if (!window.confirm(`Delete app-server "${as.name}"?`)) return
    try {
      const res = await fetch(`/api/app-servers/${as.id}`, { method: 'DELETE' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setAppServers(list => list.filter(x => x.id !== as.id))
    } catch (err: unknown) {
      alert('Delete failed: ' + (err instanceof Error ? err.message : String(err)))
    }
  }

  const handleSave = (saved: AppServer) => {
    if (modalMode === 'add') {
      setAppServers(list => [...list, saved])
    } else {
      setAppServers(list => list.map(x => x.id === saved.id ? saved : x))
    }
    setModalMode(null)
    setEditingAS(null)
  }

  if (loading) return <p className="dim">Loading app-servers…</p>

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', overflow: 'hidden' }}>
      <div className="toolbar">
        <span className="count">{appServers.length} app-server{appServers.length !== 1 ? 's' : ''}</span>
        <button className="add-btn" onClick={() => { setEditingAS(null); setModalMode('add') }}>
          + Add App-Server
        </button>
      </div>

      {appServers.length === 0 ? (
        <p className="dim" style={{ padding: '1rem 1.5rem' }}>No app-server bindings configured.</p>
      ) : (
        <table className="data-table">
          <thead>
            <tr>
              <th>Name</th>
              <th>Server</th>
              <th>Plugin</th>
              <th>Auto-GW</th>
              <th>Description</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {appServers.map(as => (
              <tr key={as.id}>
                <td><strong>{as.name}</strong></td>
                <td>{serverLabel(as.server_id)}</td>
                <td><code>{as.plugin_name}</code></td>
                <td>{as.auto_gw ? 'yes' : 'no'}</td>
                <td className="dim">{as.description || '—'}</td>
                <td className="actions-cell">
                  <button
                    className="connect-btn"
                    onClick={() => onConnect(as.server_id, as.auto_gw, as.plugin_name)}
                  >
                    Connect
                  </button>
                  <button className="action-btn" onClick={() => { setEditingAS(as); setModalMode('edit') }}>
                    Edit
                  </button>
                  <button className="action-btn danger" onClick={() => handleDelete(as)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {modalMode !== null && (
        <AppServerFormModal
          initial={editingAS}
          servers={servers}
          onSave={handleSave}
          onClose={() => { setModalMode(null); setEditingAS(null) }}
        />
      )}
    </div>
  )
}
