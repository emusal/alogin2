import { useState } from 'react'
import type { Server, Gateway, ServerFormData } from '../types'
import './Modal.css'

const PROTOCOLS = ['ssh', 'sftp', 'ftp', 'sshfs', 'telnet', 'rlogin', 'vagrant', 'docker']

interface Props {
  initial: Server | null  // null = add mode
  gateways: Gateway[]
  onSave: (saved: Server) => void
  onClose: () => void
}

const emptyForm = (): ServerFormData => ({
  protocol: 'ssh',
  host: '',
  user: '',
  password: '',
  port: 0,
  gateway_id: null,
  locale: '',
})

export function ServerFormModal({ initial, gateways, onSave, onClose }: Props) {
  const isEdit = initial !== null
  const [form, setForm] = useState<ServerFormData>(() =>
    isEdit
      ? {
          protocol: initial.protocol,
          host: initial.host,
          user: initial.user,
          password: '',
          port: initial.port,
          gateway_id: initial.gateway_id,
          locale: initial.locale,
        }
      : emptyForm()
  )
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const set = (k: keyof ServerFormData, v: ServerFormData[keyof ServerFormData]) =>
    setForm(f => ({ ...f, [k]: v }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    setSaving(true)
    setError(null)
    try {
      const url = isEdit ? `/api/servers/${initial.id}` : '/api/servers'
      const method = isEdit ? 'PUT' : 'POST'
      const body = isEdit
        ? {
            protocol: form.protocol,
            user: form.user,
            password: form.password,
            port: form.port,
            gateway_id: form.gateway_id,
            locale: form.locale,
          }
        : form
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(body),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || `HTTP ${res.status}`)
      }
      const saved: Server = await res.json()
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
        <div className="modal-title">{isEdit ? `Edit Server: ${initial.host}` : 'Add Server'}</div>
        <form onSubmit={handleSubmit}>
          <div className="form-row">
            <label>Protocol</label>
            <select value={form.protocol} onChange={e => set('protocol', e.target.value)}>
              {PROTOCOLS.map(p => <option key={p} value={p}>{p}</option>)}
            </select>
          </div>
          <div className="form-row">
            <label>Host</label>
            <input
              type="text"
              value={form.host}
              onChange={e => set('host', e.target.value)}
              disabled={isEdit}
              placeholder="hostname or IP"
              required={!isEdit}
            />
            {isEdit && <span className="form-hint">Host cannot be changed after creation</span>}
          </div>
          <div className="form-row">
            <label>User</label>
            <input
              type="text"
              value={form.user}
              onChange={e => set('user', e.target.value)}
              placeholder="username"
              required
            />
          </div>
          <div className="form-row">
            <label>Password</label>
            <input
              type="password"
              value={form.password}
              onChange={e => set('password', e.target.value)}
              placeholder={isEdit ? 'leave empty to keep current' : '(optional)'}
            />
          </div>
          <div className="form-row">
            <label>Port</label>
            <input
              type="number"
              value={form.port}
              onChange={e => set('port', parseInt(e.target.value) || 0)}
              min={0}
              max={65535}
              placeholder="0"
            />
            <span className="form-hint">0 = use protocol default</span>
          </div>
          <div className="form-row">
            <label>Gateway</label>
            <select
              value={form.gateway_id ?? ''}
              onChange={e => set('gateway_id', e.target.value ? Number(e.target.value) : null)}
            >
              <option value="">— none —</option>
              {gateways.map(gw => (
                <option key={gw.id} value={gw.id}>{gw.name}</option>
              ))}
            </select>
          </div>
          <div className="form-row">
            <label>Locale</label>
            <input
              type="text"
              value={form.locale}
              onChange={e => set('locale', e.target.value)}
              placeholder="e.g. ko_KR.eucKR"
            />
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
