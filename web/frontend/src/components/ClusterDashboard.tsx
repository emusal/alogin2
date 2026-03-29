import { useState, useEffect, useRef, useCallback } from 'react'
import type { Cluster, Server, ClusterNodeState, NodeStatus, BroadcastSummary } from '../types'
import { ClusterNodeList } from './ClusterNodeList'
import { ClusterHeatmap } from './ClusterHeatmap'
import { ClusterCommandBar } from './ClusterCommandBar'
import { Terminal } from './Terminal'
import type { TerminalHandle } from './Terminal'
import './ClusterDashboard.css'

interface Props {
  cluster: Cluster
  servers: Server[]
  autoGW: boolean
  onClose: () => void
}

export function ClusterDashboard({ cluster, servers, autoGW, onClose }: Props) {
  const [nodeStates, setNodeStates] = useState<Map<number, ClusterNodeState>>(new Map())
  const [selectedIds, setSelectedIds] = useState<Set<number>>(new Set())
  // null = heatmap, -1 = multi-focus (selected), N = single focus
  const [focusedNodeId, setFocusedNodeId] = useState<number | null>(null)
  const [filterQuery, setFilterQuery] = useState('')
  const [broadcastSummary, setBroadcastSummary] = useState<BroadcastSummary | null>(null)

  // serverId → TerminalHandle (one instance per node, always mounted)
  const termRefs = useRef<Map<number, TerminalHandle>>(new Map())
  // Focus slot containers — DOM refs where we move terminal elements into
  const focusSlotRefs = useRef<Map<number, HTMLDivElement>>(new Map())
  // Hidden pool container — terminals return here when not focused
  const hiddenPoolRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    const map = new Map<number, ClusterNodeState>()
    const ids = new Set<number>()
    for (const m of cluster.members) {
      const srv = servers.find(s => s.id === m.server_id)
      if (!srv) continue
      map.set(m.server_id, {
        serverId: m.server_id,
        host: srv.host,
        user: m.user || srv.user,
        status: 'connecting',
      })
      ids.add(m.server_id)
    }
    setNodeStates(map)
    setSelectedIds(ids)
    setFocusedNodeId(null)
  }, [cluster.id])

  const nodes = Array.from(nodeStates.values())

  const setNodeStatus = useCallback((serverId: number, status: NodeStatus) => {
    setNodeStates(prev => {
      const next = new Map(prev)
      const existing = next.get(serverId)
      if (existing) next.set(serverId, { ...existing, status })
      return next
    })
  }, [])

  // Move terminal DOM element from hidden pool into the focus slot
  const attachToSlot = (serverId: number, slot: HTMLDivElement) => {
    const handle = termRefs.current.get(serverId)
    if (!handle) return
    const el = handle.getElement()
    if (!el) return
    slot.appendChild(el)
    // Delay fit so layout has settled
    requestAnimationFrame(() => handle.fit())
  }

  // Move terminal DOM element back to hidden pool
  const detachToPool = (serverId: number) => {
    const handle = termRefs.current.get(serverId)
    if (!handle || !hiddenPoolRef.current) return
    const el = handle.getElement()
    if (!el) return
    hiddenPoolRef.current.appendChild(el)
  }

  const inFocusMode = focusedNodeId !== null
  const focusedMulti = focusedNodeId === -1
  const visibleInFocus: number[] = focusedMulti
    ? nodes.filter(n => selectedIds.has(n.serverId)).map(n => n.serverId)
    : focusedNodeId !== null ? [focusedNodeId] : []

  // When focus changes: move terminals in/out of slots
  const prevFocusRef = useRef<number[]>([])
  useEffect(() => {
    const prev = prevFocusRef.current
    // Detach nodes that are leaving focus
    for (const id of prev) {
      if (!visibleInFocus.includes(id)) detachToPool(id)
    }
    // Attach nodes entering focus — but slot refs are created by render,
    // so we do this after paint via a small RAF
    if (visibleInFocus.length > 0) {
      requestAnimationFrame(() => {
        for (const id of visibleInFocus) {
          const slot = focusSlotRefs.current.get(id)
          if (slot) attachToSlot(id, slot)
        }
      })
    }
    prevFocusRef.current = visibleInFocus
  }, [focusedNodeId, selectedIds])

  const handleFocus = (serverId: number) => setFocusedNodeId(serverId)
  const handleFocusMulti = () => setFocusedNodeId(-1)
  const handleBackToMap = () => setFocusedNodeId(null)

  const handleNodeSelect = (serverId: number) => {
    setSelectedIds(prev => {
      const next = new Set(prev)
      if (next.has(serverId)) next.delete(serverId)
      else next.add(serverId)
      return next
    })
  }

  const handleSelectAll = () => setSelectedIds(new Set(nodes.map(n => n.serverId)))
  const handleDeselectAll = () => setSelectedIds(new Set())
  const handleSelectByStatus = (status: NodeStatus) =>
    setSelectedIds(new Set(nodes.filter(n => n.status === status).map(n => n.serverId)))

  const handleBroadcast = (command: string) => {
    if (selectedIds.size === 0) return
    const targetIds = Array.from(selectedIds)
    setBroadcastSummary(null)

    setNodeStates(prev => {
      const next = new Map(prev)
      for (const id of targetIds) {
        const n = next.get(id)
        if (n) next.set(id, { ...n, status: 'sending' })
      }
      return next
    })

    let sent = 0
    for (const id of targetIds) {
      const handle = termRefs.current.get(id)
      if (handle) { handle.sendData(command + '\n'); sent++ }
    }

    setTimeout(() => {
      setNodeStates(prev => {
        const next = new Map(prev)
        for (const id of targetIds) {
          const n = next.get(id)
          if (n && n.status === 'sending') next.set(id, { ...n, status: 'done' })
        }
        return next
      })
      setBroadcastSummary({
        total: targetIds.length,
        ok: sent,
        failed: targetIds.length - sent,
        failedDetails: targetIds
          .filter(id => !termRefs.current.get(id))
          .map(id => ({ host: nodeStates.get(id)?.host ?? `#${id}`, error: 'not connected' })),
      })
    }, 800)
  }

  return (
    <div className="cluster-dashboard">
      {/* Toolbar */}
      <div className="cd-toolbar">
        <span className="cd-toolbar-title">{cluster.name}</span>
        {autoGW && <span className="cd-gw-badge">via GW</span>}
        {inFocusMode ? (
          <button className="cd-back-btn" style={{ marginLeft: 'auto' }} onClick={handleBackToMap}>
            ← Map
          </button>
        ) : selectedIds.size > 0 ? (
          <button
            className="cd-select-btn"
            style={{ marginLeft: 'auto', borderColor: 'var(--accent2)', color: 'var(--accent2)' }}
            onClick={handleFocusMulti}
          >
            Focus selected ({selectedIds.size})
          </button>
        ) : null}
      </div>

      <div className="cd-body">
        <ClusterNodeList
          nodes={nodes}
          selectedIds={selectedIds}
          filterQuery={filterQuery}
          onFilterChange={setFilterQuery}
          onToggleSelect={handleNodeSelect}
          onSelectAll={handleSelectAll}
          onDeselectAll={handleDeselectAll}
          onSelectByStatus={handleSelectByStatus}
          onNodeClick={handleFocus}
        />

        <div className="cd-main">
          {/* Heatmap / Table */}
          <div style={{ display: inFocusMode ? 'none' : 'flex', flex: 1, flexDirection: 'column', minHeight: 0 }}>
            <ClusterHeatmap
              nodes={nodes}
              selectedIds={selectedIds}
              onNodeClick={handleFocus}
              onNodeSelect={handleNodeSelect}
            />
          </div>

          {/* Focus area: slot divs that receive terminal elements via DOM move */}
          {inFocusMode && (
            <div className={`cd-focus-body ${visibleInFocus.length === 1 ? 'cd-focus-single' : 'cd-focus-grid'}`}>
              {visibleInFocus.map(id => {
                const node = nodeStates.get(id)
                return (
                  <div key={id} className="cd-focus-grid-item">
                    {visibleInFocus.length > 1 && node && (
                      <div className="cd-focus-mini-header">
                        <span className={`cd-status-dot ${node.status}`} />
                        {node.user}@{node.host}
                      </div>
                    )}
                    {/* Slot: terminal DOM element will be moved here */}
                    <div
                      className="cd-focus-slot"
                      ref={el => {
                        if (el) focusSlotRefs.current.set(id, el)
                        else focusSlotRefs.current.delete(id)
                      }}
                    />
                  </div>
                )
              })}
            </div>
          )}
        </div>
      </div>

      {/* Hidden terminal pool — all terminals always mounted here, moved on focus */}
      <div ref={hiddenPoolRef} style={{ display: 'none' }}>
        {nodes.map(node => {
          const srv = servers.find(s => s.id === node.serverId)
          if (!srv) return null
          return (
            <Terminal
              key={node.serverId}
              ref={el => {
                if (el) termRefs.current.set(node.serverId, el)
                else termRefs.current.delete(node.serverId)
              }}
              server={srv}
              autoGW={autoGW}
              onStatusChange={s => setNodeStatus(node.serverId,
                s === 'connected' ? 'connected' :
                s === 'disconnected' ? 'disconnected' : 'error'
              )}
            />
          )
        })}
      </div>

      <ClusterCommandBar
        selectedCount={selectedIds.size}
        totalCount={nodes.length}
        running={false}
        summary={broadcastSummary}
        onBroadcast={handleBroadcast}
        onSendRaw={data => {
          for (const id of selectedIds) {
            termRefs.current.get(id)?.sendData(data)
          }
        }}
      />
    </div>
  )
}
