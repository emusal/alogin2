import { useState, useEffect, useCallback } from 'react'
import type { PendingApproval, TrustWindow } from '../types'
import './AgentApproval.css'

function timeLeft(expiresAt: string): { seconds: number; label: string; urgent: boolean } {
  const diff = Math.max(0, Math.floor((new Date(expiresAt).getTime() - Date.now()) / 1000))
  if (diff >= 60) {
    return { seconds: diff, label: `${Math.floor(diff / 60)}m ${diff % 60}s`, urgent: false }
  }
  return { seconds: diff, label: `${diff}s`, urgent: diff <= 15 }
}

function formatDate(iso: string) {
  return new Date(iso).toLocaleString()
}

function expiresInMin(iso: string) {
  const diff = Math.floor((new Date(iso).getTime() - Date.now()) / 60000)
  return diff
}

// ── Countdown timer for a single card ──
function CountdownTimer({ expiresAt }: { expiresAt: string }) {
  const [tick, setTick] = useState(0)
  useEffect(() => {
    const id = setInterval(() => setTick(t => t + 1), 1000)
    return () => clearInterval(id)
  }, [])
  const { label, urgent } = timeLeft(expiresAt)
  return <span className={`approval-timer${urgent ? ' urgent' : ''}`}>⏱ {label}</span>
}

export function AgentApproval() {
  const [pending, setPending] = useState<PendingApproval[]>([])
  const [trustWindows, setTrustWindows] = useState<TrustWindow[]>([])
  const [loading, setLoading] = useState<Set<string>>(new Set())
  const [error, setError] = useState<string | null>(null)
  const [autoRefresh, setAutoRefresh] = useState(true)

  // Trust form state
  const [trustScope, setTrustScope] = useState('global')
  const [trustCustomScope, setTrustCustomScope] = useState('')
  const [trustDuration, setTrustDuration] = useState('30m')
  const [trustLoading, setTrustLoading] = useState(false)

  const fetchPending = useCallback(async () => {
    try {
      const res = await fetch('/api/agent/pending')
      const data = await res.json()
      setPending(Array.isArray(data) ? data : [])
    } catch {
      // silently ignore polling errors
    }
  }, [])

  const fetchTrust = useCallback(async () => {
    try {
      const res = await fetch('/api/agent/trust')
      const data = await res.json()
      setTrustWindows(Array.isArray(data) ? data : [])
    } catch {}
  }, [])

  const refresh = useCallback(async () => {
    await Promise.all([fetchPending(), fetchTrust()])
  }, [fetchPending, fetchTrust])

  useEffect(() => {
    refresh()
  }, [refresh])

  useEffect(() => {
    if (!autoRefresh) return
    const id = setInterval(refresh, 3000)
    return () => clearInterval(id)
  }, [autoRefresh, refresh])

  const handleApprove = async (token: string) => {
    setLoading(s => new Set(s).add(token))
    setError(null)
    try {
      const res = await fetch(`/api/agent/approve/${token}`, { method: 'POST' })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        throw new Error(d.error || `HTTP ${res.status}`)
      }
      setPending(list => list.filter(p => p.token !== token))
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(s => { const n = new Set(s); n.delete(token); return n })
    }
  }

  const handleDeny = async (token: string) => {
    setLoading(s => new Set(s).add(token))
    setError(null)
    try {
      const res = await fetch(`/api/agent/deny/${token}`, { method: 'POST' })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        throw new Error(d.error || `HTTP ${res.status}`)
      }
      setPending(list => list.filter(p => p.token !== token))
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setLoading(s => { const n = new Set(s); n.delete(token); return n })
    }
  }

  const handleGrantTrust = async () => {
    const scope = trustScope === 'custom' ? trustCustomScope.trim() : trustScope
    if (!scope) return
    setTrustLoading(true)
    setError(null)
    try {
      const res = await fetch('/api/agent/trust', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ scope, duration: trustDuration }),
      })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        throw new Error(d.error || `HTTP ${res.status}`)
      }
      await fetchTrust()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setTrustLoading(false)
    }
  }

  const handleRevokeTrust = async (scope: string) => {
    setError(null)
    try {
      const res = await fetch(`/api/agent/trust/${encodeURIComponent(scope)}`, { method: 'DELETE' })
      if (!res.ok) {
        const d = await res.json().catch(() => ({}))
        throw new Error(d.error || `HTTP ${res.status}`)
      }
      await fetchTrust()
    } catch (err: unknown) {
      setError(err instanceof Error ? err.message : String(err))
    }
  }

  return (
    <div className="agent-approval-container">
      {error && <div className="error-banner">Error: {error}</div>}

      {/* ── Pending Approvals ── */}
      <section>
        <div className="section-header">
          <div>
            <div className="section-title">
              Pending Approvals
              {pending.length > 0 && (
                <span className="pending-badge">{pending.length}</span>
              )}
            </div>
            <div className="section-subtitle">
              Agent actions waiting for human approval
            </div>
          </div>
          <div className="refresh-row">
            <label className="auto-refresh-label">
              <input
                type="checkbox"
                checked={autoRefresh}
                onChange={e => setAutoRefresh(e.target.checked)}
              />
              Auto-refresh
            </label>
            <button className="btn-refresh" onClick={refresh}>↻ Refresh</button>
          </div>
        </div>

        {pending.length === 0 ? (
          <div className="empty-state">
            <div className="empty-state-icon">✓</div>
            No pending approvals
          </div>
        ) : (
          <div className="approval-cards">
            {pending.map(req => {
              const busy = loading.has(req.token)
              const target = req.host
                ? req.host
                : req.server_id
                ? `server #${req.server_id}`
                : req.cluster_id
                ? `cluster #${req.cluster_id}`
                : '—'
              return (
                <div key={req.token} className="approval-card">
                  <div className="approval-card-header">
                    <div className="approval-meta">
                      <span className="approval-agent">
                        {req.agent_id || 'unknown-agent'}
                      </span>
                      <span className="approval-target">
                        Target: <span>{target}</span>
                      </span>
                    </div>
                    <CountdownTimer expiresAt={req.expires_at} />
                  </div>

                  {req.intent && (
                    <div className="approval-intent">{req.intent}</div>
                  )}

                  {req.commands && req.commands.length > 0 && (
                    <div className="approval-commands">
                      <div className="approval-commands-label">Commands</div>
                      <div className="approval-cmd-list">
                        {req.commands.map((cmd, i) => (
                          <div key={i} className="approval-cmd">$ {cmd}</div>
                        ))}
                      </div>
                    </div>
                  )}

                  <div className="approval-actions">
                    <button
                      className="btn-deny"
                      onClick={() => handleDeny(req.token)}
                      disabled={busy}
                    >
                      {busy ? '…' : 'Deny'}
                    </button>
                    <button
                      className="btn-approve"
                      onClick={() => handleApprove(req.token)}
                      disabled={busy}
                    >
                      {busy ? '…' : 'Approve'}
                    </button>
                  </div>
                </div>
              )
            })}
          </div>
        )}
      </section>

      {/* ── Trust Windows ── */}
      <section className="trust-section">
        <div className="section-header">
          <div>
            <div className="section-title">Trust Windows</div>
            <div className="section-subtitle">
              Temporary auto-approval grants — bypass HITL within scope
            </div>
          </div>
        </div>

        {/* Grant form */}
        <div className="trust-form">
          <label>Scope</label>
          <select
            className="trust-select"
            value={trustScope}
            onChange={e => setTrustScope(e.target.value)}
          >
            <option value="global">global</option>
            <option value="custom">custom…</option>
          </select>
          {trustScope === 'custom' && (
            <input
              className="trust-input"
              placeholder="agent:my-agent  or  server:42"
              value={trustCustomScope}
              onChange={e => setTrustCustomScope(e.target.value)}
            />
          )}
          <label>Duration</label>
          <select
            className="trust-select"
            value={trustDuration}
            onChange={e => setTrustDuration(e.target.value)}
          >
            <option value="5m">5 min</option>
            <option value="15m">15 min</option>
            <option value="30m">30 min</option>
            <option value="1h">1 hour</option>
            <option value="4h">4 hours</option>
            <option value="24h">24 hours</option>
          </select>
          <button
            className="btn-grant"
            onClick={handleGrantTrust}
            disabled={trustLoading || (trustScope === 'custom' && !trustCustomScope.trim())}
          >
            {trustLoading ? '…' : '+ Grant Trust'}
          </button>
        </div>

        {trustWindows.length === 0 ? (
          <div className="empty-state" style={{ padding: '1.5rem' }}>
            No active trust windows
          </div>
        ) : (
          <table className="trust-table">
            <thead>
              <tr>
                <th>SCOPE</th>
                <th>GRANTED BY</th>
                <th>GRANTED AT</th>
                <th>EXPIRES AT</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {trustWindows.map(tw => {
                const minLeft = expiresInMin(tw.expires_at)
                return (
                  <tr key={tw.scope}>
                    <td><span className="trust-scope">{tw.scope}</span></td>
                    <td>{tw.granted_by || '—'}</td>
                    <td>{formatDate(tw.granted_at)}</td>
                    <td>
                      <span className={`trust-expires${minLeft <= 5 ? ' expiring-soon' : ''}`}>
                        {formatDate(tw.expires_at)}
                        {minLeft <= 60 && ` (${minLeft}m left)`}
                      </span>
                    </td>
                    <td>
                      <button
                        className="btn-revoke"
                        onClick={() => handleRevokeTrust(tw.scope)}
                      >
                        Revoke
                      </button>
                    </td>
                  </tr>
                )
              })}
            </tbody>
          </table>
        )}
      </section>
    </div>
  )
}
