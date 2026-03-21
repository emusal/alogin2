import { useState, useEffect } from 'react'
import type { Cluster, Server } from '../types'
import { ClusterFormModal } from './ClusterFormModal'
import './ClusterList.css'

interface Props {
  servers: Server[]
}

export function ClusterList({ servers }: Props) {
  const [clusters, setClusters] = useState<Cluster[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modalMode, setModalMode] = useState<'add' | 'edit' | null>(null)
  const [editingCluster, setEditingCluster] = useState<Cluster | null>(null)

  useEffect(() => {
    fetch('/api/clusters')
      .then(r => r.json())
      .then(data => {
        setClusters(Array.isArray(data) ? data : [])
        setLoading(false)
      })
      .catch(err => {
        setError(err.message)
        setLoading(false)
      })
  }, [])

  const serverById = (id: number) => servers.find(s => s.id === id)

  const membersSummary = (c: Cluster) => {
    if (!c.members || c.members.length === 0) return '—'
    return c.members
      .slice()
      .sort((a, b) => a.member_order - b.member_order)
      .map(m => {
        const s = serverById(m.server_id)
        const user = m.user || s?.user || ''
        return s ? `${user}@${s.host}` : `#${m.server_id}`
      })
      .join(', ')
  }

  const handleDelete = async (c: Cluster) => {
    if (!window.confirm(`Delete cluster "${c.name}"?`)) return
    try {
      const res = await fetch(`/api/clusters/${c.id}`, { method: 'DELETE' })
      if (!res.ok) throw new Error(`HTTP ${res.status}`)
      setClusters(cl => cl.filter(x => x.id !== c.id))
    } catch (err: unknown) {
      alert('Delete failed: ' + (err instanceof Error ? err.message : String(err)))
    }
  }

  const handleSave = (saved: Cluster) => {
    if (modalMode === 'add') {
      setClusters(cl => [...cl, saved])
    } else {
      setClusters(cl => cl.map(x => x.id === saved.id ? saved : x))
    }
    setModalMode(null)
    setEditingCluster(null)
  }

  return (
    <div className="cluster-list-container">
      <div className="toolbar">
        <span className="cl-title">Clusters</span>
        <span className="count">{clusters.length} clusters</span>
        <button className="add-btn" onClick={() => { setEditingCluster(null); setModalMode('add') }}>
          + Add Cluster
        </button>
      </div>

      {loading && <div className="status">Loading clusters...</div>}
      {error && <div className="status error">Error: {error}</div>}
      {!loading && !error && (
        <table className="server-table">
          <thead>
            <tr>
              <th>NAME</th>
              <th>MEMBERS</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {clusters.length === 0 && (
              <tr>
                <td colSpan={3} style={{ textAlign: 'center', color: 'var(--dim)', padding: '2rem' }}>
                  No clusters. Click "+ Add Cluster" to create one.
                </td>
              </tr>
            )}
            {clusters.map(c => (
              <tr key={c.id}>
                <td><span className="host">{c.name}</span></td>
                <td><span className="dim">{membersSummary(c)}</span></td>
                <td className="actions-cell">
                  <button
                    className="action-btn"
                    onClick={() => { setEditingCluster(c); setModalMode('edit') }}
                  >
                    Edit
                  </button>
                  <button className="action-btn danger" onClick={() => handleDelete(c)}>
                    Delete
                  </button>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}

      {modalMode !== null && (
        <ClusterFormModal
          initial={editingCluster}
          servers={servers}
          onSave={handleSave}
          onClose={() => { setModalMode(null); setEditingCluster(null) }}
        />
      )}
    </div>
  )
}
