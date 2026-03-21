import { useState, useEffect } from 'react'
import type { Gateway, Server } from '../types'
import { GatewayFormModal } from './GatewayFormModal'
import './GatewayList.css'

interface Props {
  servers: Server[]
}

export function GatewayList({ servers }: Props) {
  const [gateways, setGateways] = useState<Gateway[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modalMode, setModalMode] = useState<'add' | 'edit' | null>(null)
  const [editingGateway, setEditingGateway] = useState<Gateway | null>(null)

  useEffect(() => {
    fetch('/api/gateways')
      .then(r => r.json())
      .then(data => {
        setGateways(Array.isArray(data) ? data : [])
        setLoading(false)
      })
      .catch(err => {
        setError(err.message)
        setLoading(false)
      })
  }, [])

  const serverById = (id: number) => servers.find(s => s.id === id)

  const hopsSummary = (gw: Gateway) => {
    if (!gw.hops || gw.hops.length === 0) return '—'
    return gw.hops
      .slice()
      .sort((a, b) => a.hop_order - b.hop_order)
      .map(h => {
        const s = serverById(h.server_id)
        return s ? `${s.user}@${s.host}` : `#${h.server_id}`
      })
      .join(' → ')
  }

  const handleDelete = async (gw: Gateway) => {
    if (!window.confirm(`Delete gateway "${gw.name}"?`)) return
    try {
      const res = await fetch(`/api/gateways/${gw.id}`, { method: 'DELETE' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setGateways(g => g.filter(x => x.id !== gw.id))
    } catch (err: unknown) {
      alert('Delete failed: ' + (err instanceof Error ? err.message : String(err)))
    }
  }

  const handleSave = (saved: Gateway) => {
    if (modalMode === 'add') {
      setGateways(g => [...g, saved])
    } else {
      setGateways(g => g.map(x => x.id === saved.id ? saved : x))
    }
    setModalMode(null)
    setEditingGateway(null)
  }

  return (
    <div className="gateway-list-container">
      <div className="toolbar">
        <span className="gw-title">Gateways</span>
        <span className="count">{gateways.length} routes</span>
        <button className="add-btn" onClick={() => { setEditingGateway(null); setModalMode('add') }}>
          + Add Gateway
        </button>
      </div>

      {loading && <div className="status">Loading gateways...</div>}
      {error && <div className="status error">Error: {error}</div>}
      {!loading && !error && (
        <table className="server-table">
          <thead>
            <tr>
              <th>NAME</th>
              <th>HOPS</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {gateways.length === 0 && (
              <tr>
                <td colSpan={3} style={{ textAlign: 'center', color: 'var(--dim)', padding: '2rem' }}>
                  No gateways. Click "+ Add Gateway" to create one.
                </td>
              </tr>
            )}
            {gateways.map(gw => (
              <tr key={gw.id}>
                <td><span className="host">{gw.name}</span></td>
                <td><span className="dim">{hopsSummary(gw)}</span></td>
                <td className="actions-cell">
                  <button
                    className="action-btn"
                    onClick={() => { setEditingGateway(gw); setModalMode('edit') }}
                  >
                    Edit
                  </button>
                  <button className="action-btn danger" onClick={() => handleDelete(gw)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {modalMode !== null && (
        <GatewayFormModal
          initial={editingGateway}
          servers={servers}
          onSave={handleSave}
          onClose={() => { setModalMode(null); setEditingGateway(null) }}
        />
      )}
    </div>
  )
}
