import { useState, useEffect } from 'react'
import { ServerList } from './components/ServerList'
import { GatewayList } from './components/GatewayList'
import { ClusterList } from './components/ClusterList'
import { ClusterDashboard } from './components/ClusterDashboard'
import { HostList } from './components/HostList'
import { TunnelList } from './components/TunnelList'
import { PluginList } from './components/PluginList'
import { AppServerList } from './components/AppServerList'
import { ProfileList } from './components/ProfileList'
import { Terminal } from './components/Terminal'
import { AgentApproval } from './components/AgentApproval'
import { PageBanner } from './components/PageBanner'
import type { Server, Cluster, Profile } from './types'
import './App.css'

type View = 'servers' | 'gateways' | 'clusters' | 'hosts' | 'tunnels' | 'plugins' | 'app-servers' | 'profiles' | 'agent' | string

interface TerminalTab {
  id: string
  server: Server
  app?: string
}

interface ClusterDashTab {
  id: string
  cluster: Cluster
}

let tabCounter = 0

export default function App() {
  const [view, setView] = useState<View>('servers')
  const [terminals, setTerminals] = useState<TerminalTab[]>([])
  const [clusterDashTabs, setClusterDashTabs] = useState<ClusterDashTab[]>([])
  const [servers, setServers] = useState<Server[]>([])
  const [activeProfile, setActiveProfile] = useState<Profile | null>(null)
  const [pendingCount, setPendingCount] = useState(0)

  useEffect(() => {
    fetch('/api/servers')
      .then(r => r.json())
      .then(data => setServers(Array.isArray(data) ? data : []))
      .catch(() => {})
    fetch('/api/profiles')
      .then(r => r.json())
      .then((data: Profile[]) => {
        const list = Array.isArray(data) ? data : []
        setActiveProfile(list.find(p => p.is_active) ?? null)
      })
      .catch(() => {})
  }, [])

  // Poll pending approval count for nav badge
  useEffect(() => {
    const poll = () => {
      fetch('/api/agent/pending')
        .then(r => r.json())
        .then(data => setPendingCount(Array.isArray(data) ? data.length : 0))
        .catch(() => {})
    }
    poll()
    const id = setInterval(poll, 5000)
    return () => clearInterval(id)
  }, [])

  const connect = (server: Server, app?: string) => {
    tabCounter++
    const id = `term-${tabCounter}`
    setTerminals(tabs => [...tabs, { id, server, app }])
    setView(id)
  }

  const connectAppServer = async (serverId: number, app: string) => {
    let server = servers.find(s => s.id === serverId)
    if (!server) {
      try {
        const r = await fetch(`/api/servers/${serverId}`)
        if (r.ok) server = await r.json()
      } catch { /* ignore */ }
    }
    if (!server) return
    console.log('[connectAppServer] server=', server, 'app=', app)
    connect(server, app)
  }

  const closeTab = (id: string) => {
    setTerminals(tabs => tabs.filter(t => t.id !== id))
    setView(prev => prev === id ? 'servers' : prev)
  }

  const openClusterDash = (cluster: Cluster) => {
    const id = `cluster-dash-${cluster.id}`
    setClusterDashTabs(tabs => {
      if (tabs.find(t => t.id === id)) return tabs
      return [...tabs, { id, cluster }]
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
        {activeProfile && (
          <span className="active-profile-badge" title={`Active profile: ${activeProfile.name}`}>
            {activeProfile.name}
          </span>
        )}
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
            className={`nav-btn ${view === 'profiles' ? 'active' : ''}`}
            onClick={() => setView('profiles')}
          >
            Profiles
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
          <button
            className={`nav-btn ${view === 'agent' ? 'active' : ''}`}
            onClick={() => setView('agent')}
          >
            Agent
            {pendingCount > 0 && (
              <span className="pending-badge">{pendingCount}</span>
            )}
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
                {tab.server.host}
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
        {view === 'servers'  && <PageBanner page="servers" />}
        {view === 'gateways' && <PageBanner page="gateways" />}
        {view === 'profiles' && <PageBanner page="profiles" />}
        {view === 'clusters' && <PageBanner page="clusters" />}
        {view === 'hosts'    && <PageBanner page="hosts" />}
        {view === 'tunnels'  && <PageBanner page="tunnels" />}
        {view === 'plugins'     && <PageBanner page="plugins" />}
        {view === 'app-servers' && <PageBanner page="app-servers" />}
        {terminals.some(t => t.id === view) && <PageBanner page="terminal" />}

        {view === 'servers'  && <ServerList onConnect={connect} />}
        {view === 'gateways' && <GatewayList servers={servers} />}
        {view === 'profiles' && <ProfileList onActiveProfileChange={setActiveProfile} />}
        {view === 'clusters' && <ClusterList servers={servers} onOpenDashboard={openClusterDash} />}
        {view === 'hosts'    && <HostList />}
        {view === 'tunnels'  && <TunnelList servers={servers} />}
        {view === 'plugins'     && <PluginList />}
        {view === 'app-servers' && <AppServerList servers={servers} onConnect={connectAppServer} />}
        {view === 'agent'       && <AgentApproval />}
        {clusterDashTabs.map(tab => (
          <div key={tab.id} style={{ display: view === tab.id ? 'flex' : 'none', flex: 1, flexDirection: 'column', overflow: 'hidden' }}>
            <ClusterDashboard
              cluster={tab.cluster}
              servers={servers}
              onClose={() => closeClusterDash(tab.id)}
            />
          </div>
        ))}
        {terminals.map(tab => (
          <div key={tab.id} style={{ display: view === tab.id ? 'flex' : 'none', flex: 1, flexDirection: 'column', overflow: 'hidden' }}>
            <Terminal server={tab.server} app={tab.app} />
          </div>
        ))}
      </main>
    </div>
  )
}
