import { useState, useEffect } from 'react'
import { ComputeList } from './components/ComputeList'
import { GatewayList } from './components/GatewayList'
import { ClusterList } from './components/ClusterList'
import { ClusterDashboard } from './components/ClusterDashboard'
import { HostList } from './components/HostList'
import { TunnelList } from './components/TunnelList'
import { PluginList } from './components/PluginList'
import { AppServerList } from './components/AppServerList'
import { Terminal } from './components/Terminal'
import { PageBanner } from './components/PageBanner'
import type { Server, Cluster } from './types'
import './App.css'

type View = 'compute' | 'gateways' | 'clusters' | 'hosts' | 'tunnels' | 'plugins' | 'app-servers' | string

interface TerminalTab {
  id: string
  server: Server
  autoGW: boolean
  app?: string
}

interface ClusterDashTab {
  id: string
  cluster: Cluster
  autoGW: boolean
}

let tabCounter = 0

export default function App() {
  const [view, setView] = useState<View>('compute')
  const [terminals, setTerminals] = useState<TerminalTab[]>([])
  const [clusterDashTabs, setClusterDashTabs] = useState<ClusterDashTab[]>([])
  const [servers, setServers] = useState<Server[]>([])

  useEffect(() => {
    fetch('/api/compute')
      .then(r => r.json())
      .then(data => setServers(Array.isArray(data) ? data : []))
      .catch(() => {})
  }, [])

  const connect = (server: Server, autoGW = false, app?: string) => {
    tabCounter++
    const id = `term-${tabCounter}`
    setTerminals(tabs => [...tabs, { id, server, autoGW, app }])
    setView(id)
  }

  const connectAppServer = (serverId: number, autoGW: boolean, app: string) => {
    const server = servers.find(s => s.id === serverId)
    if (!server) return
    connect(server, autoGW, app)
  }

  const closeTab = (id: string) => {
    setTerminals(tabs => tabs.filter(t => t.id !== id))
    setView(prev => prev === id ? 'compute' : prev)
  }

  const openClusterDash = (cluster: Cluster, autoGW: boolean) => {
    const id = `cluster-dash-${cluster.id}`
    setClusterDashTabs(tabs => {
      if (tabs.find(t => t.id === id)) return tabs
      return [...tabs, { id, cluster, autoGW }]
    })
    setView(id)
  }

  const closeClusterDash = (id: string) => {
    setClusterDashTabs(tabs => tabs.filter(t => t.id !== id))
    setView(prev => prev === id ? 'clusters' : prev)
  }

  return (
    <div className="app">
      <header className="header">
        <h1 className="logo">alogin</h1>
        <nav className="nav">
          <button
            className={`nav-btn ${view === 'compute' ? 'active' : ''}`}
            onClick={() => setView('compute')}
          >
            Compute
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
          <button
            className={`nav-btn ${view === 'plugins' ? 'active' : ''}`}
            onClick={() => setView('plugins')}
          >
            Plugins
          </button>
          <button
            className={`nav-btn ${view === 'app-servers' ? 'active' : ''}`}
            onClick={() => setView('app-servers')}
          >
            App Servers
          </button>
          {clusterDashTabs.map(tab => (
            <span key={tab.id} className={`nav-tab ${view === tab.id ? 'active' : ''}`} style={{ '--tab-color': 'var(--accent2)' } as React.CSSProperties}>
              <button
                className="nav-tab-label"
                onClick={() => setView(tab.id)}
              >
                {tab.cluster.name}
              </button>
              <button
                className="nav-tab-close"
                onClick={e => { e.stopPropagation(); closeClusterDash(tab.id) }}
                title="Close dashboard"
              >
                ×
              </button>
            </span>
          ))}
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
        {view === 'compute'  && <PageBanner page="compute" />}
        {view === 'gateways' && <PageBanner page="gateways" />}
        {view === 'clusters' && <PageBanner page="clusters" />}
        {view === 'hosts'    && <PageBanner page="hosts" />}
        {view === 'tunnels'  && <PageBanner page="tunnels" />}
        {view === 'plugins'     && <PageBanner page="plugins" />}
        {view === 'app-servers' && <PageBanner page="app-servers" />}
        {terminals.some(t => t.id === view) && <PageBanner page="terminal" />}

        {view === 'compute'  && <ComputeList onConnect={connect} />}
        {view === 'gateways' && <GatewayList servers={servers} />}
        {view === 'clusters' && <ClusterList servers={servers} onOpenDashboard={openClusterDash} />}
        {view === 'hosts'    && <HostList />}
        {view === 'tunnels'  && <TunnelList servers={servers} />}
        {view === 'plugins'     && <PluginList />}
        {view === 'app-servers' && <AppServerList servers={servers} onConnect={connectAppServer} />}
        {clusterDashTabs.map(tab => (
          <div key={tab.id} style={{ display: view === tab.id ? 'flex' : 'none', flex: 1, flexDirection: 'column', overflow: 'hidden' }}>
            <ClusterDashboard
              cluster={tab.cluster}
              servers={servers}
              autoGW={tab.autoGW}
              onClose={() => closeClusterDash(tab.id)}
            />
          </div>
        ))}
        {terminals.map(tab => (
          <div key={tab.id} style={{ display: view === tab.id ? 'flex' : 'none', flex: 1, flexDirection: 'column', overflow: 'hidden' }}>
            <Terminal server={tab.server} autoGW={tab.autoGW} app={tab.app} />
          </div>
        ))}
      </main>
    </div>
  )
}
