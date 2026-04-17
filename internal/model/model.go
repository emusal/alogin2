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
	Port       int        `json:"port"`        // 0 = use protocol default
	Locale     string     `json:"locale"`      // e.g. "ko_KR.eucKR"; "-" or "" = system default
	DeviceType DeviceType `json:"device_type"` // linux|windows|router|switch|firewall|other
	Note       string     `json:"note"`        // free-form description (LLM context)
	AuthMethod string     `json:"auth_method"` // "password" or "key"
	IdentityFile string   `json:"identity_file,omitempty"` // Explicit SSH private key path
	// GatewayRouteID is the internal gateway route applied after the profile gateway.
	// Full path: profile.gateway_hops → server.gateway_route_hops → server
	// nil means connect directly from the profile gateway (or directly if no profile).
	GatewayRouteID *int64 `json:"gateway_route_id,omitempty"`
	PolicyYAML   string `json:"policy_yaml,omitempty"`   // inline YAML policy; "" = use global
	SystemPrompt string `json:"system_prompt,omitempty"` // per-server LLM system prompt; "" = use global
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
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
	CreatedAt  time.Time       `json:"created_at"`
	UpdatedAt  time.Time       `json:"updated_at"`
}

// AppServer binds a named shortcut to a compute server + application plugin.
type AppServer struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	ServerID    int64     `json:"server_id"`
	PluginName  string    `json:"plugin_name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ConnectOptions carries runtime flags for a single connection attempt.
type ConnectOptions struct {
	Command  string   // -c: run command after login
	PutFile  string   // -p: upload file via SFTP/FTP
	GetFile  string   // -g: download file via SFTP/FTP
	TunnelL  []string // -L local:host:remote port forwards
	TunnelR  []string // -R remote:host:local port forwards
	DestPath string   // -d: SSHFS/FTP destination path
	// ProfileOverride: "" = use active profile, "none" = direct connection, "<name>" = named profile
	ProfileOverride string
	DryRun          bool   // print route without connecting
	ScreenID        string // cluster -s: screen id
	TileX           int    // cluster -x: tile columns
	Align           string // cluster --left/--right
	HostKeys        string // cluster --host_keys
	Mode            string // cluster --mode: tmux|iterm|terminal
	AppName         string // --app: application plugin name to launch after connecting
}

// Profile represents a named network context (e.g. "home", "office", "travel").
// When active, all SSH connections automatically route through the profile's gateway.
type Profile struct {
	ID             int64     `json:"id"`
	Name           string    `json:"name"`
	Description    string    `json:"description"`
	IsActive       bool      `json:"is_active"`
	GatewayRouteID *int64    `json:"gateway_route_id,omitempty"` // nil = no gateway (direct connections)
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AgentMemory is a free-form natural language note stored by an AI agent for a specific server.
type AgentMemory struct {
	ID        int64     `json:"id"`
	ServerID  int64     `json:"server_id"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}
