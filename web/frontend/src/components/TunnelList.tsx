import { useState, useEffect, useCallback } from 'react'
import type { Tunnel, Server, TunnelStatus } from '../types'
import { TunnelFormModal } from './TunnelFormModal'
import './TunnelList.css'

interface Props {
  servers: Server[]
}

export function TunnelList({ servers }: Props) {
  const [tunnels, setTunnels] = useState<Tunnel[]>([])
  const [statuses, setStatuses] = useState<Map<number, boolean>>(new Map())
  const [modal, setModal] = useState<{ open: boolean; tunnel: Tunnel | null }>({ open: false, tunnel: null })
  const [actionLoading, setActionLoading] = useState<Set<number>>(new Set())
  const [error, setError] = useState<string | null>(null)

  const fetchTunnels = useCallback(async () => {
    try {
      const res = await fetch('/api/tunnels')
      const data = await res.json()
      setTunnels(Array.isArray(data) ? data : [])
    } catch {
      setError('Failed to load tunnels')
    }
  }, [])

  const fetchStatuses = useCallback(async (list: Tunnel[]) => {
    const results = new Map<number, boolean>()
    await Promise.all(
      list.map(async t => {
        try {
          const res = await fetch(`/api/tunnels/${t.id}/status`)
          const data: TunnelStatus = await res.json()
          results.set(t.id, data.running)
        } catch {
          results.set(t.id, false)
        }
      })
    )
    setStatuses(new Map(results))
  }, [])

  useEffect(() => {
    fetchTunnels()
  }, [fetchTunnels])

  useEffect(() => {
    if (tunnels.length === 0) return
    fetchStatuses(tunnels)
    const timer = setInterval(() => fetchStatuses(tunnels), 10_000)
    return () => clearInterval(timer)
  }, [tunnels, fetchStatuses])

  const serverLabel = (serverId: number) => {
    const s = servers.find(s => s.id === serverId)
    return s ? `${s.user}@${s.host}` : `#${serverId}`
  }

  const handleStart = async (t: Tunnel) => {
    setActionLoading(s => new Set(s).add(t.id))
    setError(null)
    try {
      const res = await fetch(`/api/tunnels/${t.id}/start`, { method: 'POST' })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || `HTTP ${res.status}`)
      }
      await fetchStatuses(tunnels)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setActionLoading(s => { const n = new Set(s); n.delete(t.id); return n })
    }
  }

  const handleStop = async (t: Tunnel) => {
    setActionLoading(s => new Set(s).add(t.id))
    setError(null)
    try {
      const res = await fetch(`/api/tunnels/${t.id}/stop`, { method: 'POST' })
      if (!res.ok) {
        const data = await res.json().catch(() => ({}))
        throw new Error(data.error || `HTTP ${res.status}`)
      }
      await fetchStatuses(tunnels)
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setActionLoading(s => { const n = new Set(s); n.delete(t.id); return n })
    }
  }

  const handleDelete = async (t: Tunnel) => {
    if (!window.confirm(`Delete tunnel "${t.name}"?`)) return
    try {
      const res = await fetch(`/api/tunnels/${t.id}`, { method: 'DELETE' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setTunnels(list => list.filter(x => x.id !== t.id))
      setStatuses(m => { const n = new Map(m); n.delete(t.id); return n })
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  const handleSave = async (saved: Tunnel) => {
    setModal({ open: false, tunnel: null })
    await fetchTunnels()
  }

  return (
    <div className="tunnel-list-container">
      <div className="toolbar">
        <span className="tunnel-title">Tunnels</span>
        <span className="count">{tunnels.length} configured</span>
        <button className="add-btn" onClick={() => setModal({ open: true, tunnel: null })}>
          + Add Tunnel
        </button>
      </div>

      {error && <div className="status error">Error: {error}</div>}

      <table className="server-table">
        <thead>
          <tr>
            <th>NAME</th>
            <th>DIR</th>
            <th>LOCAL → REMOTE</th>
            <th>SERVER</th>
            <th>STATUS</th>
            <th></th>
          </tr>
        </thead>
        <tbody>
          {tunnels.length === 0 && (
            <tr>
              <td colSpan={6} style={{ textAlign: 'center', color: 'var(--dim)', padding: '2rem' }}>
                No tunnels configured. Click "+ Add Tunnel" to create one.
              </td>
            </tr>
          )}
          {tunnels.map(t => {
            const running = statuses.get(t.id) ?? false
            const loading = actionLoading.has(t.id)
            return (
              <tr key={t.id}>
                <td><span className="host">{t.name}</span></td>
                <td><span className="dim">{t.direction === 'L' ? '-L' : '-R'}</span></td>
                <td>
                  <span className="dim">
                    {t.local_host}:{t.local_port} → {t.remote_host}:{t.remote_port}
                    {t.auto_gw ? ' [GW]' : ''}
                  </span>
                </td>
                <td><span className="dim">{serverLabel(t.server_id)}</span></td>
                <td>
                  <span className={`tunnel-status ${running ? 'tunnel-status-running' : 'tunnel-status-stopped'}`}>
                    {running ? '● running' : '○ stopped'}
                  </span>
                </td>
                <td className="actions-cell">
                  {running ? (
                    <button
                      className="action-btn danger"
                      onClick={() => handleStop(t)}
                      disabled={loading}
                    >
                      {loading ? '…' : 'Stop'}
                    </button>
                  ) : (
                    <button
                      className="action-btn success"
                      onClick={() => handleStart(t)}
                      disabled={loading}
                    >
                      {loading ? '…' : 'Start'}
                    </button>
                  )}
                  <button
                    className="action-btn"
                    onClick={() => setModal({ open: true, tunnel: t })}
                  >
                    Edit
                  </button>
                  <button
                    className="action-btn danger"
                    onClick={() => handleDelete(t)}
                  >
                    Delete
                  </button>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>

      {modal.open && (
        <TunnelFormModal
          initial={modal.tunnel}
          servers={servers}
          onSave={handleSave}
          onClose={() => setModal({ open: false, tunnel: null })}
        />
      )}
    </div>
  )
}
