import { useState, useEffect } from 'react'
import { ServerList } from './components/ServerList'
import { GatewayList } from './components/GatewayList'
import { ClusterList } from './components/ClusterList'
import { HostList } from './components/HostList'
import { TunnelList } from './components/TunnelList'
import { Terminal } from './components/Terminal'
import { PageBanner } from './components/PageBanner'
import type { Server } from './types'
import './App.css'

type View = 'servers' | 'gateways' | 'clusters' | 'hosts' | 'tunnels' | string // string for terminal tab IDs

interface TerminalTab {
  id: string      // unique tab key
  server: Server
  autoGW: boolean
}

let tabCounter = 0

export default function App() {
  const [view, setView] = useState<View>('servers')
  const [terminals, setTerminals] = useState<TerminalTab[]>([])
  const [servers, setServers] = useState<Server[]>([])

  useEffect(() => {
    fetch('/api/servers')
      .then(r => r.json())
      .then(data => setServers(Array.isArray(data) ? data : []))
      .catch(() => {})
  }, [])

  const connect = (server: Server, autoGW = false) => {
    tabCounter++
    const id = `term-${tabCounter}`
    setTerminals(tabs => [...tabs, { id, server, autoGW }])
    setView(id)
  }

  const closeTab = (id: string) => {
    setTerminals(tabs => tabs.filter(t => t.id !== id))
    setView(prev => prev === id ? 'servers' : prev)
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
          <button
            className={`nav-btn ${view === 'tunnels' ? 'active' : ''}`}
            onClick={() => setView('tunnels')}
          >
            Tunnels
          </button>
          {terminals.map(tab => (
            <span key={tab.id} className={`nav-tab ${view === tab.id ? 'active' : ''}`}>
              <button
                className="nav-tab-label"
                onClick={() => setView(tab.id)}
              >
                {tab.server.host}{tab.autoGW ? ' [GW]' : ''}
              </button>
              <button
                className="nav-tab-close"
                onClick={e => { e.stopPropagation(); closeTab(tab.id) }}
                title="Close terminal"
              >
                ×
              </button>
            </span>
          ))}
        </nav>
      </header>

      <main className="main">
        {view === 'servers'   && <PageBanner page="servers" />}
        {view === 'gateways'  && <PageBanner page="gateways" />}
        {view === 'clusters'  && <PageBanner page="clusters" />}
        {view === 'hosts'     && <PageBanner page="hosts" />}
        {view === 'tunnels'   && <PageBanner page="tunnels" />}
        {terminals.some(t => t.id === view) && <PageBanner page="terminal" />}

        {view === 'servers' && <ServerList onConnect={connect} />}
        {view === 'gateways' && <GatewayList servers={servers} />}
        {view === 'clusters' && <ClusterList servers={servers} />}
        {view === 'hosts' && <HostList />}
        {view === 'tunnels' && <TunnelList servers={servers} />}
        {terminals.map(tab => (
          <div key={tab.id} style={{ display: view === tab.id ? 'flex' : 'none', flex: 1, flexDirection: 'column', overflow: 'hidden' }}>
            <Terminal server={tab.server} autoGW={tab.autoGW} />
          </div>
        ))}
      </main>
    </div>
  )
}
