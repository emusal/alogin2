export interface Server {
  id: number
  protocol: string
  host: string
  user: string
  port: number
  gateway_id: number | null
  gateway_server_id: number | null
  locale: string
  created_at: string
  updated_at: string
}

export interface ServerFormData {
  protocol: string
  host: string
  user: string
  password: string
  port: number
  gateway_id: number | null
  gateway_server_id: number | null
  locale: string
}

export interface GatewayFormData {
  name: string
  hop_server_ids: number[]
}

export interface Gateway {
  id: number
  name: string
  hops: { server_id: number; hop_order: number }[]
}

export interface Cluster {
  id: number
  name: string
  members: { server_id: number; user: string; member_order: number }[]
}

export interface LocalHost {
  id: number
  hostname: string
  ip: string
  description: string
  created_at: string
  updated_at: string
}

export interface LocalHostFormData {
  hostname: string
  ip: string
  description: string
}

export interface Plugin {
  name: string
  version: string
  provider: string
  strategies: string[]
  cmd_flag: string
}

export interface Tunnel {
  id: number
  name: string
  server_id: number
  direction: 'L' | 'R'
  local_host: string
  local_port: number
  remote_host: string
  remote_port: number
  auto_gw: boolean
  created_at: string
  updated_at: string
}

export interface TunnelStatus {
  running: boolean
  session: string
}

export interface TunnelFormData {
  name: string
  server_id: number
  direction: 'L' | 'R'
  local_host: string
  local_port: number
  remote_host: string
  remote_port: number
  auto_gw: boolean
}

export interface AppServer {
  id: number
  name: string
  server_id: number
  plugin_name: string
  auto_gw: boolean
  description: string
  created_at: string
  updated_at: string
}

export interface AppServerFormData {
  name: string
  server_id: number
  plugin_name: string
  auto_gw: boolean
  description: string
}

// Cluster Dashboard types
export type NodeStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error' | 'sending' | 'done' | 'cmd-error'

export interface ClusterNodeState {
  serverId: number
  host: string
  user: string
  status: NodeStatus
  errorMsg?: string
}

export interface ClusterExecResult {
  server_id: number
  host: string
  user: string
  output: string
  exit_code: number
  error: string
}

export interface BroadcastSummary {
  total: number
  ok: number
  failed: number
  failedDetails: { host: string; error: string }[]
}
