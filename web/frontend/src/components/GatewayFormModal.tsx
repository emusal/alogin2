import { useState } from 'react'
import type { Gateway, Server, GatewayFormData } from '../types'
import './Modal.css'

interface Props {
  initial: Gateway | null  // null = add mode
  servers: Server[]
  onSave: (saved: Gateway) => void
  onClose: () => void
}

export function GatewayFormModal({ initial, servers, onSave, onClose }: Props) {
  const isEdit = initial !== null
  const [name, setName] = useState(initial?.name ?? '')
  const [hopIds, setHopIds] = useState<number[]>(
    initial ? initial.hops.map(h => h.server_id) : []
  )
  const [selectedServer, setSelectedServer] = useState<string>('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const serverById = (id: number) => servers.find(s => s.id === id)

  const addHop = () => {
    const id = Number(selectedServer)
    if (!id) return
    setHopIds(h => [...h, id])
    setSelectedServer('')
  }

  const removeHop = (idx: number) => setHopIds(h => h.filter((_, i) => i !== idx))

  const moveUp = (idx: number) => {
    if (idx === 0) return
    setHopIds(h => { const a = [...h]; [a[idx - 1], a[idx]] = [a[idx], a[idx - 1]]; return a })
  }

  const moveDown = (idx: number) => {
    setHopIds(h => {
      if (idx >= h.length - 1) return h
      const a = [...h]; [a[idx], a[idx + 1]] = [a[idx + 1], a[idx]]; return a
    })
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) { setError('Name is required'); return }
    setSaving(true)
    setError(null)
    const body: GatewayFormData = { name: name.trim(), hop_server_ids: hopIds }
    try {
      const url = isEdit ? `/api/gateways/${initial.id}` : '/api/gateways'
      const method = isEdit ? 'PUT' : 'POST'
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || `HTTP ${res.status}`)
      }
      const saved: Gateway = await res.json()
      onSave(saved)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  // servers not yet in hops (allow duplicates intentionally — user's choice)
  const availableServers = servers

  return (
    <div className="modal-overlay" onClick={e => e.target === e.currentTarget && onClose()}>
      <div className="modal">
        <div className="modal-title">{isEdit ? `Edit Gateway: ${initial.name}` : 'Add Gateway'}</div>
        <form onSubmit={handleSubmit}>
          <div className="form-row">
            <label>Name</label>
            <input
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="gateway name"
              required
            />
          </div>

          <div className="form-row">
            <label>Hops (in order)</label>
            <div className="hop-list">
              {hopIds.map((id, idx) => {
                const srv = serverById(id)
                return (
                  <div key={idx} className="hop-item">
                    <span className="hop-label">
                      {idx + 1}. {srv ? `${srv.user}@${srv.host}` : `#${id}`}
                    </span>
                    <button type="button" title="Move up" onClick={() => moveUp(idx)}>↑</button>
                    <button type="button" title="Move down" onClick={() => moveDown(idx)}>↓</button>
                    <button type="button" title="Remove" onClick={() => removeHop(idx)}>×</button>
                  </div>
                )
              })}
            </div>
            <div className="hop-add-row">
              <select
                value={selectedServer}
                onChange={e => setSelectedServer(e.target.value)}
              >
                <option value="">— select server to add —</option>
                {availableServers.map(s => (
                  <option key={s.id} value={s.id}>
                    {s.user}@{s.host}
                  </option>
                ))}
              </select>
              <button type="button" className="hop-add-btn" onClick={addHop}>
                + Add Hop
              </button>
            </div>
          </div>

          {error && <div className="modal-error">{error}</div>}
          <div className="modal-actions">
            <button type="button" onClick={onClose}>Cancel</button>
            <button type="submit" className="btn-primary" disabled={saving}>
              {saving ? 'Saving…' : 'Save'}
            </button>
          </div>
        </form>
      </div>
    </div>
  )
}
