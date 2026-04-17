import { useState } from 'react'
import type { Profile, Gateway, ProfileFormData } from '../types'
import './Modal.css'

interface Props {
  initial: Profile | null
  gateways: Gateway[]
  onSave: (saved: Profile) => void
  onClose: () => void
}

export function ProfileFormModal({ initial, gateways, onSave, onClose }: Props) {
  const isEdit = initial !== null
  const [name, setName] = useState(initial?.name ?? '')
  const [description, setDescription] = useState(initial?.description ?? '')
  const [gatewayRouteId, setGatewayRouteId] = useState<number | null>(
    initial?.gateway_route_id ?? null
  )
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) { setError('Name is required'); return }
    setSaving(true)
    setError(null)
    const body: ProfileFormData = {
      name: name.trim(),
      description,
      gateway_route_id: gatewayRouteId,
    }
    try {
      const url = isEdit ? `/api/profiles/${initial.id}` : '/api/profiles'
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
      const saved: Profile = await res.json()
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
        <div className="modal-title">
          {isEdit ? `Edit Profile: ${initial.name}` : 'Add Profile'}
        </div>
        <form onSubmit={handleSubmit}>
          <div className="form-row">
            <label>Name</label>
            <input
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="e.g. home, office, travel"
              required
            />
          </div>
          <div className="form-row">
            <label>Description</label>
            <input
              type="text"
              value={description}
              onChange={e => setDescription(e.target.value)}
              placeholder="optional"
            />
          </div>
          <div className="form-row">
            <label>Gateway Route</label>
            <select
              value={gatewayRouteId ?? ''}
              onChange={e => setGatewayRouteId(e.target.value ? Number(e.target.value) : null)}
            >
              <option value="">(none — direct connections)</option>
              {gateways.map(gw => (
                <option key={gw.id} value={gw.id}>{gw.name}</option>
              ))}
            </select>
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
