import { useState } from 'react'
import type { Cluster, Server } from '../types'
import './Modal.css'

interface MemberEntry {
  server_id: number
  user: string
}

interface Props {
  initial: Cluster | null  // null = add mode
  servers: Server[]
  onSave: (saved: Cluster) => void
  onClose: () => void
}

export function ClusterFormModal({ initial, servers, onSave, onClose }: Props) {
  const isEdit = initial !== null
  const [name, setName] = useState(initial?.name ?? '')
  const [members, setMembers] = useState<MemberEntry[]>(
    initial
      ? initial.members
          .slice()
          .sort((a, b) => a.member_order - b.member_order)
          .map(m => ({ server_id: m.server_id, user: m.user }))
      : []
  )
  const [selectedServer, setSelectedServer] = useState<string>('')
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const serverById = (id: number) => servers.find(s => s.id === id)

  const addMember = () => {
    const id = Number(selectedServer)
    if (!id) return
    setMembers(m => [...m, { server_id: id, user: '' }])
    setSelectedServer('')
  }

  const removeMember = (idx: number) => setMembers(m => m.filter((_, i) => i !== idx))

  const moveUp = (idx: number) => {
    if (idx === 0) return
    setMembers(m => { const a = [...m]; [a[idx - 1], a[idx]] = [a[idx], a[idx - 1]]; return a })
  }

  const moveDown = (idx: number) => {
    setMembers(m => {
      if (idx >= m.length - 1) return m
      const a = [...m]; [a[idx], a[idx + 1]] = [a[idx + 1], a[idx]]; return a
    })
  }

  const updateUser = (idx: number, user: string) => {
    setMembers(m => m.map((entry, i) => i === idx ? { ...entry, user } : entry))
  }

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) { setError('Name is required'); return }
    setSaving(true)
    setError(null)
    const body = { name: name.trim(), members }
    try {
      const url = isEdit ? `/api/clusters/${initial.id}` : '/api/clusters'
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
      const saved: Cluster = await res.json()
      onSave(saved)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setSaving(false)
    }
  }

  return (
    <div className="modal-overlay" onClick={e => e.target === e.currentTarget && onClose()}>
      <div className="modal">
        <div className="modal-title">{isEdit ? `Edit Cluster: ${initial.name}` : 'Add Cluster'}</div>
        <form onSubmit={handleSubmit}>
          <div className="form-row">
            <label>Name</label>
            <input
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="cluster name"
              required
            />
          </div>

          <div className="form-row">
            <label>Members (in order)</label>
            <div className="hop-list">
              {members.map((m, idx) => {
                const srv = serverById(m.server_id)
                return (
                  <div key={idx} className="hop-item">
                    <span className="hop-label">
                      {idx + 1}. {srv ? `${srv.user}@${srv.host}` : `#${m.server_id}`}
                    </span>
                    <input
                      type="text"
                      className="member-user-input"
                      value={m.user}
                      onChange={e => updateUser(idx, e.target.value)}
                      placeholder={srv?.user ?? 'user override'}
                      title="User override (leave empty to use server default)"
                    />
                    <button type="button" title="Move up" onClick={() => moveUp(idx)}>↑</button>
                    <button type="button" title="Move down" onClick={() => moveDown(idx)}>↓</button>
                    <button type="button" title="Remove" onClick={() => removeMember(idx)}>×</button>
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
                {servers.map(s => (
                  <option key={s.id} value={s.id}>
                    {s.user}@{s.host}
                  </option>
                ))}
              </select>
              <button type="button" className="hop-add-btn" onClick={addMember}>
                + Add Member
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
