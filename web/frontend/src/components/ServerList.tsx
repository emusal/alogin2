import { useState, useEffect } from 'react'
import type { Server, Gateway } from '../types'
import { ServerFormModal } from './ServerFormModal'
import './ServerList.css'

interface Props {
  onConnect: (server: Server) => void
}

export function ServerList({ onConnect }: Props) {
  const [servers, setServers] = useState<Server[]>([])
  const [gateways, setGateways] = useState<Gateway[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [modalMode, setModalMode] = useState<'add' | 'edit' | null>(null)
  const [editingServer, setEditingServer] = useState<Server | null>(null)

  useEffect(() => {
    const getJSON = (url: string) =>
      fetch(url).then(r => {
        if (!r.ok) throw new Error(`${r.status} ${r.statusText} (${url})`)
        return r.json()
      })

    Promise.all([getJSON('/api/servers'), getJSON('/api/gateways')])
      .then(([srvData, gwData]) => {
        setServers(Array.isArray(srvData) ? srvData : [])
        setGateways(Array.isArray(gwData) ? gwData : [])
        setLoading(false)
      })
      .catch(err => {
        setError(err.message)
        setLoading(false)
      })
  }, [])

  const filtered = servers.filter(s =>
    query === '' ||
    s.host.toLowerCase().includes(query.toLowerCase()) ||
    s.user.toLowerCase().includes(query.toLowerCase()) ||
    s.protocol.toLowerCase().includes(query.toLowerCase())
  )

  const gwLabel = (server: Server) => {
    if (server.gateway_id) {
      const gw = gateways.find(g => g.id === server.gateway_id)
      return gw ? `[gw] ${gw.name}` : null
    }
    if (server.gateway_server_id) {
      const s = servers.find(x => x.id === server.gateway_server_id)
      return s ? `[srv] ${s.user}@${s.host}` : null
    }
    return null
  }

  const handleDelete = async (server: Server) => {
    if (!window.confirm(`Delete ${server.user}@${server.host}?`)) return
    try {
      const res = await fetch(`/api/servers/${server.id}`, { method: 'DELETE' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setServers(s => s.filter(x => x.id !== server.id))
    } catch (err: unknown) {
      alert('Delete failed: ' + (err instanceof Error ? err.message : String(err)))
    }
  }

  const handleSave = (saved: Server) => {
    if (modalMode === 'add') {
      setServers(s => [...s, saved])
    } else {
      setServers(s => s.map(x => x.id === saved.id ? saved : x))
    }
    setModalMode(null)
    setEditingServer(null)
  }

  return (
    <div className="server-list-container">
      <div className="toolbar">
        <input
          className="search"
          type="text"
          placeholder="Search hosts, users, protocols..."
          value={query}
          onChange={e => setQuery(e.target.value)}
          autoFocus
        />
        <span className="count">{filtered.length} / {servers.length} servers</span>
        <button className="add-btn" onClick={() => { setEditingServer(null); setModalMode('add') }}>
          + Add Server
        </button>
      </div>

      {loading && <div className="status">Loading servers...</div>}
      {error && <div className="status error">Error: {error}</div>}
      {!loading && !error && (
        <table className="server-table">
          <thead>
            <tr>
              <th>HOST</th>
              <th>USER</th>
              <th>PROTO</th>
              <th>PORT</th>
              <th>GATEWAY</th>
              <th>LOCALE</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {filtered.map(server => (
              <tr key={server.id} onDoubleClick={() => onConnect(server)}>
                <td><span className="host">{server.host}</span></td>
                <td><span className="user">{server.user}</span></td>
                <td><span className="badge">{server.protocol}</span></td>
                <td><span className="dim">{server.port > 0 ? server.port : '—'}</span></td>
                <td><span className="dim">{gwLabel(server) ?? '—'}</span></td>
                <td><span className="dim">{server.locale || '—'}</span></td>
                <td className="actions-cell">
                  <button className="connect-btn" onClick={() => onConnect(server)}>Connect</button>
                  <button className="action-btn" onClick={() => { setEditingServer(server); setModalMode('edit') }}>Edit</button>
                  <button className="action-btn danger" onClick={() => handleDelete(server)}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {modalMode !== null && (
        <ServerFormModal
          initial={editingServer}
          gateways={gateways}
          servers={servers}
          onSave={handleSave}
          onClose={() => { setModalMode(null); setEditingServer(null) }}
        />
      )}
    </div>
  )
}
