import { useState, useEffect } from 'react'
import type { Plugin } from '../types'

export function PluginList() {
  const [plugins, setPlugins] = useState<Plugin[]>([])
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    fetch('/api/plugins')
      .then(r => r.json())
      .then(data => { setPlugins(Array.isArray(data) ? data : []); setLoading(false) })
      .catch(() => setLoading(false))
  }, [])

  if (loading) return <p className="dim">Loading plugins…</p>
  if (plugins.length === 0) return (
    <div>
      <p className="dim">No plugins installed.</p>
      <p className="dim">Place <code>*.yaml</code> files in your plugin directory and restart.</p>
    </div>
  )

  return (
    <table className="data-table">
      <thead>
        <tr>
          <th>Name</th>
          <th>Version</th>
          <th>Provider</th>
          <th>Strategies</th>
          <th>Cmd Flag</th>
          <th>Usage</th>
        </tr>
      </thead>
      <tbody>
        {plugins.map(p => (
          <tr key={p.name}>
            <td><strong>{p.name}</strong></td>
            <td>{p.version}</td>
            <td>{p.provider}</td>
            <td>{p.strategies.join(', ')}</td>
            <td><code>{p.cmd_flag}</code></td>
            <td><code>--app {p.name}</code></td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
