import type { ClusterNodeState, NodeStatus } from '../types'

interface Props {
  nodes: ClusterNodeState[]
  selectedIds: Set<number>
  filterQuery: string
  onFilterChange: (q: string) => void
  onToggleSelect: (serverId: number) => void
  onSelectAll: () => void
  onDeselectAll: () => void
  onSelectByStatus: (status: NodeStatus) => void
  onNodeClick: (serverId: number) => void
}

export function ClusterNodeList({
  nodes,
  selectedIds,
  filterQuery,
  onFilterChange,
  onToggleSelect,
  onSelectAll,
  onDeselectAll,
  onSelectByStatus,
  onNodeClick,
}: Props) {
  const q = filterQuery.toLowerCase()
  const filtered = q
    ? nodes.filter(n => n.host.toLowerCase().includes(q) || n.user.toLowerCase().includes(q))
    : nodes

  return (
    <div className="cd-sidebar">
      <div className="cd-sidebar-header">
        Nodes ({selectedIds.size}/{nodes.length})
      </div>
      <input
        className="cd-filter"
        placeholder="Filter by host / user..."
        value={filterQuery}
        onChange={e => onFilterChange(e.target.value)}
      />
      <div className="cd-select-actions">
        <button className="cd-select-btn" onClick={onSelectAll}>All</button>
        <button className="cd-select-btn" onClick={onDeselectAll}>None</button>
        <button className="cd-select-btn" onClick={() => onSelectByStatus('connected')}>Connected</button>
        <button className="cd-select-btn" onClick={() => onSelectByStatus('error')}>Errors</button>
      </div>
      <ul className="cd-node-list">
        {filtered.map(node => (
          <li
            key={node.serverId}
            className={`cd-node-item${selectedIds.has(node.serverId) ? ' selected' : ''}`}
          >
            <input
              type="checkbox"
              checked={selectedIds.has(node.serverId)}
              onChange={() => onToggleSelect(node.serverId)}
            />
            <span className={`cd-status-dot ${node.status}`} />
            <span
              className="cd-node-label"
              title={`${node.user}@${node.host}`}
              onClick={() => onNodeClick(node.serverId)}
            >
              {node.user}@{node.host}
            </span>
          </li>
        ))}
      </ul>
    </div>
  )
}
