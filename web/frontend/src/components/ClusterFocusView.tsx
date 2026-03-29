import { useRef, useImperativeHandle, forwardRef } from 'react'
import type { ClusterNodeState, Server } from '../types'
import { Terminal } from './Terminal'
import type { TerminalHandle } from './Terminal'

export interface ClusterFocusViewHandle {
  broadcastData: (text: string) => void
}

interface Props {
  focusedNodeId: number | null
  selectedIds: Set<number>
  nodes: ClusterNodeState[]
  servers: Server[]
  autoGW: boolean
  onBack: () => void
  onNodeStatusChange: (serverId: number, status: 'connecting' | 'connected' | 'disconnected' | 'error') => void
}

export const ClusterFocusView = forwardRef<ClusterFocusViewHandle, Props>(function ClusterFocusView({
  focusedNodeId,
  selectedIds,
  nodes,
  servers,
  autoGW,
  onBack,
  onNodeStatusChange,
}, ref) {
  const termRefs = useRef<Map<number, TerminalHandle>>(new Map())

  useImperativeHandle(ref, () => ({
    broadcastData(text: string) {
      termRefs.current.forEach(handle => handle.sendData(text))
    },
  }))

  const serverById = (id: number) => servers.find(s => s.id === id)

  let targetIds: number[]
  if (focusedNodeId !== null) {
    targetIds = [focusedNodeId]
  } else {
    targetIds = nodes.filter(n => selectedIds.has(n.serverId)).map(n => n.serverId)
  }

  const isSingle = targetIds.length === 1
  const label = isSingle
    ? (() => {
        const n = nodes.find(n => n.serverId === targetIds[0])
        return n ? `${n.user}@${n.host}` : `node #${targetIds[0]}`
      })()
    : `${targetIds.length} nodes selected`

  return (
    <>
      <div className="cd-focus-header">
        <button className="cd-back-btn" onClick={onBack}>← Map</button>
        <span className="cd-focus-label">{label}</span>
      </div>
      <div className="cd-focus-body">
        {isSingle ? (
          <div className="cd-focus-single">
            {(() => {
              const srv = serverById(targetIds[0])
              const node = nodes.find(n => n.serverId === targetIds[0])
              if (!srv || !node) return <div style={{ padding: '1rem', color: 'var(--dim)' }}>Server not found</div>
              return (
                <Terminal
                  ref={el => {
                    if (el) termRefs.current.set(targetIds[0], el)
                    else termRefs.current.delete(targetIds[0])
                  }}
                  server={srv}
                  autoGW={autoGW}
                  onStatusChange={s => onNodeStatusChange(targetIds[0], s)}
                />
              )
            })()}
          </div>
        ) : (
          <div className="cd-focus-grid">
            {targetIds.map(id => {
              const srv = serverById(id)
              if (!srv) return null
              return (
                <div key={id} className="cd-focus-grid-item">
                  <div style={{ padding: '2px 6px', fontSize: '0.72rem', color: 'var(--dim)', background: 'var(--surface)', borderBottom: '1px solid var(--border)', flexShrink: 0 }}>
                    {srv.user}@{srv.host}
                  </div>
                  <Terminal
                    ref={el => {
                      if (el) termRefs.current.set(id, el)
                      else termRefs.current.delete(id)
                    }}
                    server={srv}
                    autoGW={autoGW}
                    compact
                    onStatusChange={s => onNodeStatusChange(id, s)}
                  />
                </div>
              )
            })}
          </div>
        )}
      </div>
    </>
  )
})
