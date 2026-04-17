import { useState, useEffect } from 'react'
import type { Profile, Gateway } from '../types'
import { ProfileFormModal } from './ProfileFormModal'

interface Props {
  onActiveProfileChange?: (profile: Profile | null) => void
}

export function ProfileList({ onActiveProfileChange }: Props) {
  const [profiles, setProfiles] = useState<Profile[]>([])
  const [gateways, setGateways] = useState<Gateway[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modalMode, setModalMode] = useState<'add' | 'edit' | null>(null)
  const [editingProfile, setEditingProfile] = useState<Profile | null>(null)

  const fetchProfiles = () =>
    fetch('/api/profiles')
      .then(r => r.json())
      .then(data => {
        const list: Profile[] = Array.isArray(data) ? data : []
        setProfiles(list)
        const active = list.find(p => p.is_active) ?? null
        onActiveProfileChange?.(active)
      })

  useEffect(() => {
    Promise.all([
      fetch('/api/profiles').then(r => r.json()),
      fetch('/api/gateways').then(r => r.json()),
    ])
      .then(([pData, gwData]) => {
        const list: Profile[] = Array.isArray(pData) ? pData : []
        setProfiles(list)
        setGateways(Array.isArray(gwData) ? gwData : [])
        const active = list.find(p => p.is_active) ?? null
        onActiveProfileChange?.(active)
        setLoading(false)
      })
      .catch(err => {
        setError(err.message)
        setLoading(false)
      })
  }, [])

  const gwName = (id: number | null) => {
    if (!id) return '—'
    return gateways.find(g => g.id === id)?.name ?? `#${id}`
  }

  const handleActivate = async (profile: Profile) => {
    if (profile.is_active) {
      // deactivate
      const res = await fetch('/api/profiles/deactivate', { method: 'POST' })
      if (!res.ok) { alert('Failed to deactivate'); return }
      setProfiles(ps => ps.map(p => ({ ...p, is_active: false })))
      onActiveProfileChange?.(null)
    } else {
      const res = await fetch(`/api/profiles/${profile.id}/activate`, { method: 'POST' })
      if (!res.ok) { alert('Failed to activate'); return }
      setProfiles(ps => ps.map(p => ({ ...p, is_active: p.id === profile.id })))
      onActiveProfileChange?.({ ...profile, is_active: true })
    }
  }

  const handleDelete = async (profile: Profile) => {
    if (!window.confirm(`Delete profile "${profile.name}"?`)) return
    const res = await fetch(`/api/profiles/${profile.id}`, { method: 'DELETE' })
    if (!res.ok) { alert('Delete failed'); return }
    const updated = profiles.filter(p => p.id !== profile.id)
    setProfiles(updated)
    onActiveProfileChange?.(updated.find(p => p.is_active) ?? null)
  }

  const handleSave = (saved: Profile) => {
    if (modalMode === 'add') {
      setProfiles(ps => [...ps, saved])
    } else {
      setProfiles(ps => ps.map(p => p.id === saved.id ? saved : p))
    }
    setModalMode(null)
    setEditingProfile(null)
    fetchProfiles()
  }

  const activeProfile = profiles.find(p => p.is_active)

  return (
    <div className="gateway-list-container">
      <div className="toolbar">
        <span className="gw-title">Profiles</span>
        {activeProfile && (
          <span className="count" style={{ color: 'var(--accent)' }}>
            Active: {activeProfile.name}
          </span>
        )}
        {!activeProfile && (
          <span className="count">No active profile (direct connections)</span>
        )}
        <button className="add-btn" onClick={() => { setEditingProfile(null); setModalMode('add') }}>
          + Add Profile
        </button>
      </div>

      {loading && <div className="status">Loading profiles...</div>}
      {error && <div className="status error">Error: {error}</div>}
      {!loading && !error && (
        <table className="server-table">
          <thead>
            <tr>
              <th>ACTIVE</th>
              <th>NAME</th>
              <th>GATEWAY</th>
              <th>DESCRIPTION</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {profiles.length === 0 && (
              <tr>
                <td colSpan={5} style={{ textAlign: 'center', color: 'var(--dim)', padding: '2rem' }}>
                  No profiles. Click "+ Add Profile" to create one.
                </td>
              </tr>
            )}
            {profiles.map(p => (
              <tr key={p.id} style={p.is_active ? { background: 'var(--row-active, rgba(130,170,255,0.07))' } : undefined}>
                <td style={{ textAlign: 'center', fontSize: '1.1em' }}>
                  {p.is_active ? '●' : '○'}
                </td>
                <td><span className="host">{p.name}</span></td>
                <td><span className="dim">{gwName(p.gateway_route_id)}</span></td>
                <td><span className="dim">{p.description || '—'}</span></td>
                <td className="actions-cell">
                  <button
                    className={`connect-btn${p.is_active ? ' gw' : ''}`}
                    onClick={() => handleActivate(p)}
                    title={p.is_active ? 'Deactivate' : 'Activate'}
                  >
                    {p.is_active ? 'Deactivate' : 'Use'}
                  </button>
                  <button
                    className="action-btn"
                    onClick={() => { setEditingProfile(p); setModalMode('edit') }}
                  >
                    Edit
                  </button>
                  <button className="action-btn danger" onClick={() => handleDelete(p)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {modalMode !== null && (
        <ProfileFormModal
          initial={editingProfile}
          gateways={gateways}
          onSave={handleSave}
          onClose={() => { setModalMode(null); setEditingProfile(null) }}
        />
      )}
    </div>
  )
}
