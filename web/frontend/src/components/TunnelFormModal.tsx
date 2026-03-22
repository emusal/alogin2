import { useState } from 'react'
import type { Tunnel, Server, TunnelFormData } from '../types'
import './Modal.css'

interface Props {
  initial: Tunnel | null  // null = add mode
  servers: Server[]
  onSave: (saved: Tunnel) => void
  onClose: () => void
}

export function TunnelFormModal({ initial, servers, onSave, onClose }: Props) {
  const isEdit = initial !== null
  const [name, setName] = useState(initial?.name ?? '')
  const [serverId, setServerId] = useState<string>(initial ? String(initial.server_id) : '')
  const [direction, setDirection] = useState<'L' | 'R'>(initial?.direction ?? 'L')
  const [localHost, setLocalHost] = useState(initial?.local_host ?? '127.0.0.1')
  const [localPort, setLocalPort] = useState(initial ? String(initial.local_port) : '')
  const [remoteHost, setRemoteHost] = useState(initial?.remote_host ?? '')
  const [remotePort, setRemotePort] = useState(initial ? String(initial.remote_port) : '')
  const [autoGW, setAutoGW] = useState(initial?.auto_gw ?? false)
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!name.trim()) { setError('Name is required'); return }
    if (!serverId) { setError('Server is required'); return }
    const lp = parseInt(localPort)
    const rp = parseInt(remotePort)
    if (!lp || lp <= 0) { setError('Local port must be a positive number'); return }
    if (!rp || rp <= 0) { setError('Remote port must be a positive number'); return }

    setSaving(true)
    setError(null)
    const body: TunnelFormData = {
      name: name.trim(),
      server_id: Number(serverId),
      direction,
      local_host: localHost || '127.0.0.1',
      local_port: lp,
      remote_host: remoteHost,
      remote_port: rp,
      auto_gw: autoGW,
    }
    try {
      const url = isEdit ? `/api/tunnels/${initial.id}` : '/api/tunnels'
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
      const saved: Tunnel = await res.json()
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
        <div className="modal-title">{isEdit ? `Edit Tunnel: ${initial.name}` : 'Add Tunnel'}</div>
        <form onSubmit={handleSubmit}>
          <div className="form-row">
            <label>Name</label>
            <input
              type="text"
              value={name}
              onChange={e => setName(e.target.value)}
              placeholder="unique tunnel name"
              required
              disabled={isEdit}
            />
          </div>

          <div className="form-row">
            <label>Server</label>
            <select value={serverId} onChange={e => setServerId(e.target.value)} required>
              <option value="">— select server —</option>
              {servers.map(s => (
                <option key={s.id} value={s.id}>
                  {s.user}@{s.host}
                </option>
              ))}
            </select>
          </div>

          <div className="form-row">
            <label>Direction</label>
            <div className="radio-group">
              <label>
                <input type="radio" value="L" checked={direction === 'L'} onChange={() => setDirection('L')} />
                {' '}Local (-L)
              </label>
              <label>
                <input type="radio" value="R" checked={direction === 'R'} onChange={() => setDirection('R')} />
                {' '}Remote (-R)
              </label>
            </div>
          </div>

          <div className="form-row">
            <label>Local Host</label>
            <input
              type="text"
              value={localHost}
              onChange={e => setLocalHost(e.target.value)}
              placeholder="127.0.0.1"
            />
          </div>

          <div className="form-row">
            <label>Local Port</label>
            <input
              type="number"
              value={localPort}
              onChange={e => setLocalPort(e.target.value)}
              placeholder="e.g. 5432"
              min={1}
              max={65535}
              required
            />
          </div>

          <div className="form-row">
            <label>Remote Host</label>
            <input
              type="text"
              value={remoteHost}
              onChange={e => setRemoteHost(e.target.value)}
              placeholder="e.g. db.prod"
              required
            />
          </div>

          <div className="form-row">
            <label>Remote Port</label>
            <input
              type="number"
              value={remotePort}
              onChange={e => setRemotePort(e.target.value)}
              placeholder="e.g. 5432"
              min={1}
              max={65535}
              required
            />
          </div>

          <div className="form-row">
            <label>Auto Gateway</label>
            <label className="checkbox-label">
              <input
                type="checkbox"
                checked={autoGW}
                onChange={e => setAutoGW(e.target.checked)}
              />
              {' '}Follow server gateway chain (--auto-gw)
            </label>
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
