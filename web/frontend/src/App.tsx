import { useState, useEffect } from 'react'
import { ServerList } from './components/ServerList'
import { GatewayList } from './components/GatewayList'
import { ClusterList } from './components/ClusterList'
import { HostList } from './components/HostList'
import { Terminal } from './components/Terminal'
import type { Server } from './types'
import './App.css'

type View = 'servers' | 'gateways' | 'clusters' | 'hosts' | 'terminal'

export default function App() {
  const [view, setView] = useState<View>('servers')
  const [activeServer, setActiveServer] = useState<Server | null>(null)
  const [servers, setServers] = useState<Server[]>([])

  useEffect(() => {
    fetch('/api/servers')
      .then(r => r.json())
      .then(data => setServers(Array.isArray(data) ? data : []))
      .catch(() => {})
  }, [])

  const connect = (server: Server) => {
    setActiveServer(server)
    setView('terminal')
  }

  return (
    <div className="app">
      <header className="header">
        <h1 className="logo">alogin</h1>
        <nav className="nav">
          <button
            className={`nav-btn ${view === 'servers' ? 'active' : ''}`}
            onClick={() => setView('servers')}
          >
            Servers
          </button>
          <button
            className={`nav-btn ${view === 'gateways' ? 'active' : ''}`}
            onClick={() => setView('gateways')}
          >
            Gateways
          </button>
          <button
            className={`nav-btn ${view === 'clusters' ? 'active' : ''}`}
            onClick={() => setView('clusters')}
          >
            Clusters
          </button>
          <button
            className={`nav-btn ${view === 'hosts' ? 'active' : ''}`}
            onClick={() => setView('hosts')}
          >
            Local Hosts
          </button>
          {activeServer && (
            <button
              className={`nav-btn ${view === 'terminal' ? 'active' : ''}`}
              onClick={() => setView('terminal')}
            >
              Terminal: {activeServer.host}
            </button>
          )}
        </nav>
      </header>

      <main className="main">
        {view === 'servers' && <ServerList onConnect={connect} />}
        {view === 'gateways' && <GatewayList servers={servers} />}
        {view === 'clusters' && <ClusterList servers={servers} />}
        {view === 'hosts' && <HostList />}
        {view === 'terminal' && activeServer && (
          <Terminal server={activeServer} />
        )}
      </main>
    </div>
  )
}
