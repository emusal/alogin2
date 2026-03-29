import { useState, useCallback, useEffect, useRef } from 'react'
import type { ClusterNodeState, NodeStatus } from '../types'

interface Props {
  nodes: ClusterNodeState[]
  selectedIds: Set<number>
  onNodeClick: (serverId: number) => void
  onNodeSelect: (serverId: number) => void
}

interface PopoverState {
  serverId: number
  x: number
  y: number
}

type ViewMode = 'heatmap' | 'table'

function statusLabel(s: NodeStatus): string {
  switch (s) {
    case 'idle':         return 'idle'
    case 'connecting':   return 'connecting…'
    case 'connected':    return 'connected'
    case 'disconnected': return 'disconnected'
    case 'error':        return 'error'
    case 'sending':      return 'sending…'
    case 'done':         return 'sent ✓'
    case 'cmd-error':    return 'cmd error'
  }
}

// ── Optimal grid layout calculator ───────────────────────────────────────────

/**
 * Given available width/height and node count, find the column count that
 * produces cells as close to square as possible while filling the area.
 */
function optimalCols(w: number, h: number, count: number): number {
  if (count === 0 || w === 0 || h === 0) return 1
  const gap = 4
  let bestCols = 1
  let bestDiff = Infinity
  for (let cols = 1; cols <= count; cols++) {
    const rows = Math.ceil(count / cols)
    const cellW = (w - gap * (cols - 1)) / cols
    const cellH = (h - gap * (rows - 1)) / rows
    const ratio = cellW / cellH            // 1.0 = perfect square
    const diff = Math.abs(ratio - 1)
    if (diff < bestDiff) {
      bestDiff = diff
      bestCols = cols
    }
  }
  return bestCols
}

// ── Heatmap view ─────────────────────────────────────────────────────────────

function HeatmapView({ nodes, selectedIds, onNodeClick, onNodeSelect }: Props) {
  const gridRef = useRef<HTMLDivElement>(null)
  const [dims, setDims] = useState({ w: 0, h: 0 })
  const [popover, setPopover] = useState<PopoverState | null>(null)

  useEffect(() => {
    const el = gridRef.current
    if (!el) return
    const ro = new ResizeObserver(entries => {
      const { width, height } = entries[0].contentRect
      setDims({ w: width, h: height })
    })
    ro.observe(el)
    const rect = el.getBoundingClientRect()
    setDims({ w: rect.width, h: rect.height })
    return () => ro.disconnect()
  }, [])

  const closePopover = useCallback(() => setPopover(null), [])
  useEffect(() => {
    if (!popover) return
    window.addEventListener('click', closePopover)
    return () => window.removeEventListener('click', closePopover)
  }, [popover, closePopover])

  const count = nodes.length
  const gap = 4
  const cols = dims.w > 0 && dims.h > 0 ? optimalCols(dims.w, dims.h, count) : Math.ceil(Math.sqrt(count))
  const rows = Math.ceil(count / cols)
  const cellW = dims.w > 0 ? Math.floor((dims.w - gap * (cols - 1)) / cols) : 0
  const cellH = dims.h > 0 ? Math.floor((dims.h - gap * (rows - 1)) / rows) : 0

  return (
    <>
      <div
        ref={gridRef}
        className="cd-heatmap-fill-grid"
        style={cellW > 0 ? {
          gridTemplateColumns: `repeat(${cols}, ${cellW}px)`,
          gridAutoRows: `${cellH}px`,
          gap,
        } : undefined}
      >
        {nodes.map(node => (
          <div
            key={node.serverId}
            className={`heatmap-cell-fill ${node.status}${selectedIds.has(node.serverId) ? ' selected' : ''}`}
            onClick={(e) => {
              e.stopPropagation()
              setPopover(null)
              if (e.shiftKey || e.ctrlKey || e.metaKey) onNodeSelect(node.serverId)
              else onNodeClick(node.serverId)
            }}
            onMouseEnter={(e) => {
              const rect = (e.currentTarget as HTMLElement).getBoundingClientRect()
              setPopover({ serverId: node.serverId, x: rect.right + 6, y: rect.top })
            }}
            onMouseLeave={() => setPopover(null)}
          >
            <span className="heatmap-cell-host">{node.host}</span>
            <span className="heatmap-cell-status">{statusLabel(node.status)}</span>
          </div>
        ))}
      </div>

      {popover && (() => {
        const node = nodes.find(n => n.serverId === popover.serverId)
        if (!node) return null
        return (
          <div className="cd-heatmap-popover">
            <div className="cd-heatmap-popover-host">{node.user}@{node.host}</div>
            <div className="cd-heatmap-popover-status">
              <span className={`cd-status-dot ${node.status}`} />
              {statusLabel(node.status)}
            </div>
          </div>
        )
      })()}
    </>
  )
}

// ── Table view ────────────────────────────────────────────────────────────────

// Fixed row height in px; derive everything from this.
const TABLE_ROW_H = 28
const TABLE_HEADER_H = 28

function TableView({ nodes, selectedIds, onNodeClick, onNodeSelect }: Props) {
  const wrapRef = useRef<HTMLDivElement>(null)
  const [dims, setDims] = useState({ w: 0, h: 0 })

  useEffect(() => {
    const el = wrapRef.current
    if (!el) return
    const ro = new ResizeObserver(entries => {
      const { width, height } = entries[0].contentRect
      setDims({ w: width, h: height })
    })
    ro.observe(el)
    const r = el.getBoundingClientRect()
    setDims({ w: r.width, h: r.height })
    return () => ro.disconnect()
  }, [])

  const count = nodes.length

  // How many rows fit in one column (excluding header)?
  const bodyH = Math.max(dims.h - TABLE_HEADER_H, TABLE_ROW_H)
  const rowsPerCol = Math.max(1, Math.floor(bodyH / TABLE_ROW_H))

  // How many columns do we need?
  const cols = dims.h > 0 ? Math.ceil(count / rowsPerCol) : 1

  // Distribute nodes column-by-column (top→bottom, left→right)
  const columns: typeof nodes[] = Array.from({ length: cols }, (_, ci) =>
    nodes.slice(ci * rowsPerCol, (ci + 1) * rowsPerCol)
  )

  const fontSize = TABLE_ROW_H <= 24 ? '0.75rem' : '0.82rem'

  const renderRow = (node: ClusterNodeState) => (
    <div
      key={node.serverId}
      className={`cd-table-row ${node.status}${selectedIds.has(node.serverId) ? ' selected' : ''}`}
      style={{ height: TABLE_ROW_H, minHeight: TABLE_ROW_H }}
      onClick={(e) => {
        if (e.shiftKey || e.ctrlKey || e.metaKey) onNodeSelect(node.serverId)
        else onNodeClick(node.serverId)
      }}
    >
      <span className="cd-table-col-status">
        <span className={`cd-status-dot ${node.status}`} />
      </span>
      <span className="cd-table-col-host">{node.user}@{node.host}</span>
      <span className={`cd-table-col-info cd-table-status-text ${node.status}`}>
        {statusLabel(node.status)}
      </span>
    </div>
  )

  return (
    <div ref={wrapRef} className="cd-table-view" style={{ fontSize }}>
      {cols <= 1 ? (
        // Single-column: normal header + body
        <>
          <div className="cd-table-header" style={{ height: TABLE_HEADER_H }}>
            <span className="cd-table-col-status">상태</span>
            <span className="cd-table-col-host">서버</span>
            <span className="cd-table-col-info">상태</span>
          </div>
          <div className="cd-table-body">
            {nodes.map(renderRow)}
          </div>
        </>
      ) : (
        // Multi-column: side-by-side panels, each with its own header
        <div className="cd-table-multi" style={{ gridTemplateColumns: `repeat(${cols}, 1fr)` }}>
          {columns.map((col, ci) => (
            <div key={ci} className="cd-table-col-panel">
              <div className="cd-table-header" style={{ height: TABLE_HEADER_H }}>
                <span className="cd-table-col-status">상태</span>
                <span className="cd-table-col-host">서버</span>
                <span className="cd-table-col-info">상태</span>
              </div>
              <div className="cd-table-body">
                {col.map(renderRow)}
              </div>
            </div>
          ))}
        </div>
      )}
    </div>
  )
}

// ── Main component ────────────────────────────────────────────────────────────

export function ClusterHeatmap({ nodes, selectedIds, onNodeClick, onNodeSelect }: Props) {
  const [viewMode, setViewMode] = useState<ViewMode>('heatmap')

  return (
    <div style={{ display: 'flex', flexDirection: 'column', height: '100%', minHeight: 0 }}>
      {/* Header */}
      <div className="cd-heatmap-header">
        <span className="cd-heatmap-title">Node Map ({nodes.length})</span>

        {/* View toggle */}
        <div className="cd-view-toggle">
          <button
            className={`cd-view-btn${viewMode === 'heatmap' ? ' active' : ''}`}
            onClick={() => setViewMode('heatmap')}
            title="Heatmap"
          >
            ⊞ Grid
          </button>
          <button
            className={`cd-view-btn${viewMode === 'table' ? ' active' : ''}`}
            onClick={() => setViewMode('table')}
            title="Table"
          >
            ≡ Table
          </button>
        </div>

        {viewMode === 'heatmap' && (
          <div className="cd-heatmap-legend">
            {(['connecting', 'connected', 'disconnected', 'sending', 'done', 'error'] as NodeStatus[]).map(s => (
              <span key={s} className="cd-legend-item">
                <span className={`cd-legend-dot ${s}`} />
                {s}
              </span>
            ))}
          </div>
        )}
      </div>

      {/* Content */}
      {viewMode === 'heatmap' ? (
        <HeatmapView
          nodes={nodes}
          selectedIds={selectedIds}
          onNodeClick={onNodeClick}
          onNodeSelect={onNodeSelect}
        />
      ) : (
        <TableView
          nodes={nodes}
          selectedIds={selectedIds}
          onNodeClick={onNodeClick}
          onNodeSelect={onNodeSelect}
        />
      )}
    </div>
  )
}
