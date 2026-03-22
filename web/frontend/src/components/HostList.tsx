import { useState, useEffect } from 'react'
import type { LocalHost } from '../types'
import './GatewayList.css'
import './Modal.css'

export function HostList() {
  const [hosts, setHosts] = useState<LocalHost[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [query, setQuery] = useState('')
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<LocalHost | null>(null)

  useEffect(() => {
    fetch('/api/hosts')
      .then(r => r.json())
      .then(data => {
        setHosts(Array.isArray(data) ? data : [])
        setLoading(false)
      })
      .catch(err => {
        setError(err.message)
        setLoading(false)
      })
  }, [])

  const filtered = hosts.filter(h =>
    query === '' ||
    h.hostname.toLowerCase().includes(query.toLowerCase()) ||
    h.ip.includes(query) ||
    h.description.toLowerCase().includes(query.toLowerCase())
  )

  const handleDelete = async (h: LocalHost) => {
    if (!window.confirm(`Delete mapping "${h.hostname}" → ${h.ip}?`)) return
    try {
      const res = await fetch(`/api/hosts/${h.id}`, { method: 'DELETE' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setHosts(hs => hs.filter(x => x.id !== h.id))
    } catch (err: unknown) {
      alert('Delete failed: ' + (err instanceof Error ? err.message : String(err)))
    }
  }

  const openAdd = () => {
    setEditing(null)
    setModalOpen(true)
  }

  const openEdit = (h: LocalHost) => {
    setEditing(h)
    setModalOpen(true)
  }

  const handleSave = (saved: LocalHost) => {
    if (editing) {
      setHosts(hs => hs.map(x => x.id === saved.id ? saved : x))
    } else {
      setHosts(hs => [...hs, saved])
    }
    setModalOpen(false)
    setEditing(null)
  }

  return (
    <div className="gateway-list-container">
      <div className="toolbar">
        <span className="gw-title">Local Hosts</span>
        <input
          className="search"
          type="text"
          placeholder="Search hostname, IP, description..."
          value={query}
          onChange={e => setQuery(e.target.value)}
        />
        <span className="count">{hosts.length} entries</span>
        <button className="add-btn" onClick={openAdd}>+ Add Entry</button>
      </div>

      {loading && <div className="status">Loading hosts...</div>}
      {error && <div className="status error">Error: {error}</div>}
      {!loading && !error && (
        <table className="server-table">
          <thead>
            <tr>
              <th>HOSTNAME</th>
              <th>IP ADDRESS</th>
              <th>DESCRIPTION</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {filtered.length === 0 && (
              <tr>
                <td colSpan={4} style={{ textAlign: 'center', color: 'var(--dim)', padding: '2rem' }}>
                  {query ? 'No matching entries.' : 'No local host mappings. Click "+ Add Entry" to create one.'}
                </td>
              </tr>
            )}
            {filtered.map(h => (
              <tr key={h.id}>
                <td><span className="host">{h.hostname}</span></td>
                <td><code>{h.ip}</code></td>
                <td><span className="dim">{h.description || '—'}</span></td>
                <td className="actions-cell">
                  <button className="action-btn" onClick={() => openEdit(h)}>Edit</button>
                  <button className="action-btn danger" onClick={() => handleDelete(h)}>Delete</button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {modalOpen && (
        <HostFormModal
          initial={editing}
          onSave={handleSave}
          onClose={() => { setModalOpen(false); setEditing(null) }}
        />
      )}
    </div>
  )
}

// ── inline form modal ─────────────────────────────────────────────────────────

interface ModalProps {
  initial: LocalHost | null
  onSave: (saved: LocalHost) => void
  onClose: () => void
}

function HostFormModal({ initial, onSave, onClose }: ModalProps) {
  const [hostname, setHostname] = useState(initial?.hostname ?? '')
  const [ip, setIp] = useState(initial?.ip ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [saving, setSaving] = useState(false)
  const [err, setErr] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!hostname.trim() || !ip.trim()) {
      setErr('Hostname and IP are required.')
      return
    }
    setSaving(true)
    setErr(null)
    try {
      const method = initial ? 'PUT' : 'POST'
      const url = initial ? `/api/hosts/${initial.id}` : '/api/hosts'
      const body = initial
        ? { ip: ip.trim(), description: description.trim() }
        : { hostname: hostname.trim(), ip: ip.trim(), description: description.trim() }
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error ?? `HTTP ${res.status}`)
      }
      const saved = await res.json()
      onSave(saved)
    } catch (e: unknown) {
      setErr(e instanceof Error ? e.message : String(e))
      setSaving(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={onClose}>
      <div className="modal" onClick={e => e.stopPropagation()}>
        <h2 className="modal-title">{initial ? `Edit: ${initial.hostname}` : 'Add Local Host'}</h2>
        <form onSubmit={handleSubmit}>
          <div className="form-row">
            <label>Hostname</label>
            <input
              value={hostname}
              onChange={e => setHostname(e.target.value)}
              disabled={!!initial}
              placeholder="e.g. myserver"
              autoFocus
            />
            {!!initial && <span className="form-hint">Hostname cannot be changed after creation.</span>}
          </div>
          <div className="form-row">
            <label>IP Address</label>
            <input
              value={ip}
              onChange={e => setIp(e.target.value)}
              placeholder="e.g. 192.168.1.10"
            />
          </div>
          <div className="form-row">
            <label>Description</label>
            <input
              value={description}
              onChange={e => setDescription(e.target.value)}
              placeholder="optional"
            />
          </div>
          {err && <div className="modal-error">{err}</div>}
          <div className="modal-actions">
            <button type="button" onClick={onClose} disabled={saving}>Cancel</button>
            <button type="submit" className="btn-primary" disabled={saving}>
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
