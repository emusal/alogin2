import { useState } from 'react'
import type { Server, AppServer, AppServerFormData } from '../types'
import './Modal.css'

interface Props {
  initial: AppServer | null
  servers: Server[]
  onSave: (saved: AppServer) => void
  onClose: () => void
}

const emptyForm = (): AppServerFormData => ({
  name: '',
  server_id: 0,
  plugin_name: '',
  auto_gw: false,
  description: '',
})

export function AppServerFormModal({ initial, servers, onSave, onClose }: Props) {
  const isEdit = initial !== null
  const [form, setForm] = useState<AppServerFormData>(() =>
    isEdit
      ? {
          name: initial.name,
          server_id: initial.server_id,
          plugin_name: initial.plugin_name,
          auto_gw: initial.auto_gw,
          description: initial.description,
        }
      : emptyForm()
  )
  const [saving, setSaving] = useState(false)
  const [error, setError] = useState<string | null>(null)

  const set = <K extends keyof AppServerFormData>(k: K, v: AppServerFormData[K]) =>
    setForm(f => ({ ...f, [k]: v }))

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault()
    if (!form.name || !form.server_id || !form.plugin_name) {
      setError('Name, server, and plugin are required')
      return
    }
    setSaving(true)
    setError(null)
    try {
      const url = isEdit ? `/api/app-servers/${initial.id}` : '/api/app-servers'
      const method = isEdit ? 'PUT' : 'POST'
      const res = await fetch(url, {
        method,
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(form),
      })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || `HTTP ${res.status}`)
      }
      const saved: AppServer = await res.json()
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
        <div className="modal-title">{isEdit ? `Edit: ${initial.name}` : 'Add App-Server'}</div>
        <form onSubmit={handleSubmit}>
          <div className="form-row">
            <label>Name</label>
            <input
              type="text"
              value={form.name}
              onChange={e => set('name', e.target.value)}
              placeholder="e.g. prod-mysql"
              disabled={isEdit}
              required
            />
            {isEdit && <span className="form-hint">Name cannot be changed after creation</span>}
          </div>
          <div className="form-row">
            <label>Server</label>
            <select
              value={form.server_id}
              onChange={e => set('server_id', Number(e.target.value))}
              required
            >
              <option value={0}>— select server —</option>
              {servers.map(s => (
                <option key={s.id} value={s.id}>{s.user}@{s.host}</option>
              ))}
            </select>
          </div>
          <div className="form-row">
            <label>Plugin</label>
            <input
              type="text"
              value={form.plugin_name}
              onChange={e => set('plugin_name', e.target.value)}
              placeholder="e.g. mariadb"
              required
            />
          </div>
          <div className="form-row">
            <label>Auto-GW</label>
            <input
              type="checkbox"
              checked={form.auto_gw}
              onChange={e => set('auto_gw', e.target.checked)}
            />
            <span className="form-hint">Connect via gateway by default</span>
          </div>
          <div className="form-row">
            <label>Description</label>
            <input
              type="text"
              value={form.description}
              onChange={e => set('description', e.target.value)}
              placeholder="optional"
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
