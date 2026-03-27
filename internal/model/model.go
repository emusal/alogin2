package model

import "time"

// Protocol represents the connection protocol.
type Protocol string

const (
	ProtoSSH     Protocol = "ssh"
	ProtoSFTP    Protocol = "sftp"
	ProtoFTP     Protocol = "ftp"
	ProtoSSHFS   Protocol = "sshfs"
	ProtoTelnet  Protocol = "telnet"
	ProtoRLogin  Protocol = "rlogin"
	ProtoVagrant Protocol = "vagrant"
	ProtoDocker  Protocol = "docker"
)

// DefaultPort returns the default port for a protocol.
func (p Protocol) DefaultPort() int {
	switch p {
	case ProtoSSH, ProtoSFTP, ProtoSSHFS, ProtoVagrant, ProtoDocker:
		return 22
	case ProtoFTP:
		return 21
	case ProtoTelnet:
		return 23
	case ProtoRLogin:
		return 513
	}
	return 22
}

// DeviceType represents the type of a server device.
type DeviceType string

const (
	DeviceLinux    DeviceType = "linux"
	DeviceWindows  DeviceType = "windows"
	DeviceRouter   DeviceType = "router"
	DeviceSwitch   DeviceType = "switch"
	DeviceFirewall DeviceType = "firewall"
	DeviceOther    DeviceType = "other"
)

// Server represents one entry in the server registry (replaces a row in server_list).
type Server struct {
	ID       int64    `json:"id"`
	Protocol Protocol `json:"protocol"`
	Host     string   `json:"host"`
	User     string   `json:"user"`
	// Password is never stored here at runtime; it is fetched from Vault on demand.
	Port            int        `json:"port"`              // 0 = use protocol default
	GatewayID       *int64     `json:"gateway_id"`        // named route from gateway_routes (mirrors gateway_list)
	GatewayServerID *int64     `json:"gateway_server_id"` // direct server reference (mirrors server_list.gateway)
	Locale          string     `json:"locale"`            // e.g. "ko_KR.eucKR"; "-" or "" = system default
	DeviceType      DeviceType `json:"device_type"`                  // linux|windows|router|switch|firewall|other
	Note            string     `json:"note"`                         // free-form description (LLM context)
	PolicyYAML      string     `json:"policy_yaml,omitempty"`        // inline YAML policy; "" = use global
	SystemPrompt    string     `json:"system_prompt,omitempty"`      // per-server LLM system prompt; "" = use global
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
}

// EffectivePort returns the actual TCP port to connect to.
func (s *Server) EffectivePort() int {
	if s.Port > 0 {
		return s.Port
	}
	return s.Protocol.DefaultPort()
}

// GatewayRoute is a named ordered list of hop servers (replaces gateway_list).
type GatewayRoute struct {
	ID   int64        `json:"id"`
	Name string       `json:"name"`
	Hops []GatewayHop `json:"hops"` // ordered: first hop → ... → last hop (destination is the Server itself)
}

// GatewayHop is one hop in a multi-hop gateway chain.
type GatewayHop struct {
	ServerID int64 `json:"server_id"`
	HopOrder int   `json:"hop_order"`
}

// Alias maps a short name to a server (replaces alias_hosts).
type Alias struct {
	ID       int64
	Name     string // the short alias
	ServerID int64
	User     string // empty = use server's default
}

// Cluster is a named group of servers (replaces clusters file).
type Cluster struct {
	ID      int64           `json:"id"`
	Name    string          `json:"name"`
	Members []ClusterMember `json:"members"`
}

// ClusterMember is one server inside a cluster.
type ClusterMember struct {
	ServerID    int64  `json:"server_id"`
	User        string `json:"user"` // empty = use server's default
	MemberOrder int    `json:"member_order"`
}

// TermTheme maps a locale pattern or hostname pattern to a terminal theme profile.
type TermTheme struct {
	ID            int64
	LocalePattern string // regexp matched against server locale
	HostPattern   string // regexp matched against hostname (takes priority)
	ThemeName     string // Terminal.app / iTerm2 profile name
	Priority      int    // higher = evaluated first
}

// LocalHost maps a hostname to an IP address (custom /etc/hosts table).
// Resolved before DNS during connection attempts.
type LocalHost struct {
	ID          int64     `json:"id"`
	Hostname    string    `json:"hostname"`
	IP          string    `json:"ip"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TunnelDirection indicates the direction of an SSH port-forward.
type TunnelDirection string

const (
	TunnelLocal  TunnelDirection = "L" // -L local forward
	TunnelRemote TunnelDirection = "R" // -R remote forward
)

// Tunnel is a saved SSH port-forward configuration.
type Tunnel struct {
	ID         int64           `json:"id"`
	Name       string          `json:"name"`
	ServerID   int64           `json:"server_id"`
	Direction  TunnelDirection `json:"direction"`
	LocalHost  string          `json:"local_host"`
	LocalPort  int             `json:"local_port"`
	RemoteHost string          `json:"remote_host"`
	RemotePort int             `json:"remote_port"`
	AutoGW     bool            `json:"auto_gw"`
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// ConnectOptions carries runtime flags for a single connection attempt.
type ConnectOptions struct {
	Command  string   // -c: run command after login
	PutFile  string   // -p: upload file via SFTP/FTP
	GetFile  string   // -g: download file via SFTP/FTP
	TunnelL  []string // -L local:host:remote port forwards
	TunnelR  []string // -R remote:host:local port forwards
	DestPath string   // -d: SSHFS/FTP destination path
	AutoGW   bool     // r-style: auto-resolve gateway chain
	DryRun   bool     // print route without connecting
	ScreenID string   // cluster -s: screen id
	TileX    int      // cluster -x: tile columns
	Align    string   // cluster --left/--right
	HostKeys string   // cluster --host_keys
	Mode     string   // cluster --mode: tmux|iterm|terminal
}
