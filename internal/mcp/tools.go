package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/model"
	"github.com/emusal/alogin2/internal/tunnel"
	"github.com/emusal/alogin2/internal/vault"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Deps holds dependencies for all MCP tool handlers.
type Deps struct {
	DB      *db.DB
	Vault   vault.Vault
	BinPath string // path to the alogin binary (for tunnel start)
}

// RegisterTools registers all 10 MCP tools on srv.
func RegisterTools(srv *server.MCPServer, d Deps) {
	// --- list_servers ---
	srv.AddTool(mcpgo.NewTool("list_servers",
		mcpgo.WithDescription("List all servers in the registry. Returns id, host, user, protocol, device_type, note, and gateway info."),
		mcpgo.WithString("query", mcpgo.Description("Optional search query to filter servers by host, user, or note")),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		query, _ := args["query"].(string)
		var servers []*model.Server
		var err error
		if query != "" {
			servers, err = d.DB.Servers.Search(ctx, query)
		} else {
			servers, err = d.DB.Servers.ListAll(ctx)
		}
		if err != nil {
			return toolError(fmt.Sprintf("list servers: %v", err)), nil
		}
		return toolJSON(servers)
	})

	// --- get_server ---
	srv.AddTool(mcpgo.NewTool("get_server",
		mcpgo.WithDescription("Get detailed information about a single server by ID."),
		mcpgo.WithString("id", mcpgo.Description("Server ID"), mcpgo.Required()),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		id, err := parseID(req, "id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		s, err := d.DB.Servers.GetByID(ctx, id)
		if err != nil {
			return toolError(fmt.Sprintf("get server: %v", err)), nil
		}
		if s == nil {
			return toolError(fmt.Sprintf("server %d not found", id)), nil
		}
		return toolJSON(s)
	})

	// --- list_tunnels ---
	srv.AddTool(mcpgo.NewTool("list_tunnels",
		mcpgo.WithDescription("List all saved tunnel configurations with their current running status."),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		tunnels, err := d.DB.Tunnels.ListAll(ctx)
		if err != nil {
			return toolError(fmt.Sprintf("list tunnels: %v", err)), nil
		}
		type tunnelWithStatus struct {
			*model.Tunnel
			Running bool `json:"running"`
		}
		var result []tunnelWithStatus
		for _, t := range tunnels {
			result = append(result, tunnelWithStatus{
				Tunnel:  t,
				Running: tunnel.IsRunning(t.Name),
			})
		}
		return toolJSON(result)
	})

	// --- get_tunnel ---
	srv.AddTool(mcpgo.NewTool("get_tunnel",
		mcpgo.WithDescription("Get detailed information about a tunnel by ID, including running status."),
		mcpgo.WithString("id", mcpgo.Description("Tunnel ID"), mcpgo.Required()),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		id, err := parseID(req, "id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		t, err := d.DB.Tunnels.GetByID(ctx, id)
		if err != nil {
			return toolError(fmt.Sprintf("get tunnel: %v", err)), nil
		}
		if t == nil {
			return toolError(fmt.Sprintf("tunnel %d not found", id)), nil
		}
		return toolJSON(map[string]any{
			"tunnel":  t,
			"running": tunnel.IsRunning(t.Name),
		})
	})

	// --- start_tunnel ---
	srv.AddTool(mcpgo.NewTool("start_tunnel",
		mcpgo.WithDescription("Start a saved tunnel in a detached tmux session."),
		mcpgo.WithString("id", mcpgo.Description("Tunnel ID"), mcpgo.Required()),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		id, err := parseID(req, "id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		t, err := d.DB.Tunnels.GetByID(ctx, id)
		if err != nil || t == nil {
			return toolError(fmt.Sprintf("tunnel %d not found", id)), nil
		}
		if err := tunnel.Start(t.Name, d.BinPath); err != nil {
			return toolError(err.Error()), nil
		}
		return toolJSON(map[string]string{
			"status":  "started",
			"session": tunnel.SessionName(t.Name),
		})
	})

	// --- stop_tunnel ---
	srv.AddTool(mcpgo.NewTool("stop_tunnel",
		mcpgo.WithDescription("Stop a running tunnel by killing its tmux session."),
		mcpgo.WithString("id", mcpgo.Description("Tunnel ID"), mcpgo.Required()),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		id, err := parseID(req, "id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		t, err := d.DB.Tunnels.GetByID(ctx, id)
		if err != nil || t == nil {
			return toolError(fmt.Sprintf("tunnel %d not found", id)), nil
		}
		if err := tunnel.Stop(t.Name); err != nil {
			return toolError(err.Error()), nil
		}
		return toolJSON(map[string]string{"status": "stopped"})
	})

	// --- list_clusters ---
	srv.AddTool(mcpgo.NewTool("list_clusters",
		mcpgo.WithDescription("List all clusters with their member counts."),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		clusters, err := d.DB.Clusters.ListAll(ctx)
		if err != nil {
			return toolError(fmt.Sprintf("list clusters: %v", err)), nil
		}
		return toolJSON(clusters)
	})

	// --- get_cluster ---
	srv.AddTool(mcpgo.NewTool("get_cluster",
		mcpgo.WithDescription("Get a cluster with full member server details (host, user, device_type, note)."),
		mcpgo.WithString("id", mcpgo.Description("Cluster ID"), mcpgo.Required()),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		id, err := parseID(req, "id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		c, err := d.DB.Clusters.GetByID(ctx, id)
		if err != nil {
			return toolError(fmt.Sprintf("get cluster: %v", err)), nil
		}
		if c == nil {
			return toolError(fmt.Sprintf("cluster %d not found", id)), nil
		}
		type memberDetail struct {
			model.ClusterMember
			Host       string            `json:"host"`
			Protocol   model.Protocol    `json:"protocol"`
			DeviceType model.DeviceType  `json:"device_type"`
			Note       string            `json:"note"`
		}
		var members []memberDetail
		for _, m := range c.Members {
			s, _ := d.DB.Servers.GetByID(ctx, m.ServerID)
			md := memberDetail{ClusterMember: m}
			if s != nil {
				md.Host = s.Host
				md.Protocol = s.Protocol
				md.DeviceType = s.DeviceType
				md.Note = s.Note
				if md.User == "" {
					md.User = s.User
				}
			}
			members = append(members, md)
		}
		return toolJSON(map[string]any{
			"id":      c.ID,
			"name":    c.Name,
			"members": members,
		})
	})

	// --- exec_command ---
	srv.AddTool(mcpgo.NewTool("exec_command",
		mcpgo.WithDescription(`Execute SSH commands on a single server.
Non-interactive mode (no expect): each command runs in its own session.
Interactive/PTY mode (with expect): all commands run as one PTY session with auto-responses.
Note: device_type 'router', 'switch', 'firewall' may not support standard SSH command execution.`),
		mcpgo.WithString("server_id", mcpgo.Description("Server ID"), mcpgo.Required()),
		mcpgo.WithArray("commands", mcpgo.Description("Commands to execute"), mcpgo.Required()),
		mcpgo.WithArray("expect", mcpgo.Description(`Optional expect rules: [{"pattern":"string","send":"string"}]. Enables PTY mode.`)),
		mcpgo.WithNumber("timeout_sec", mcpgo.Description("Execution timeout in seconds (default 30)")),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		id, err := parseID(req, "server_id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		commands, err := parseStringSlice(args, "commands")
		if err != nil || len(commands) == 0 {
			return toolError("commands must be a non-empty array of strings"), nil
		}
		rules, _ := parseExpectRules(args, "expect")
		timeoutSec, _ := args["timeout_sec"].(float64)

		results, err := execOnServer(ctx, d.DB, d.Vault, ExecRequest{
			ServerID:   id,
			Commands:   commands,
			Expect:     rules,
			TimeoutSec: int(timeoutSec),
		})
		if err != nil {
			return toolError(err.Error()), nil
		}
		return toolJSON(map[string]any{"results": results})
	})

	// --- exec_on_cluster ---
	srv.AddTool(mcpgo.NewTool("exec_on_cluster",
		mcpgo.WithDescription(`Execute SSH commands on all servers in a cluster in parallel.
Individual server failures are captured in results without stopping other servers.
Note: device_type 'router', 'switch', 'firewall' may not support standard SSH command execution.`),
		mcpgo.WithString("cluster_id", mcpgo.Description("Cluster ID"), mcpgo.Required()),
		mcpgo.WithArray("commands", mcpgo.Description("Commands to execute on each server"), mcpgo.Required()),
		mcpgo.WithArray("expect", mcpgo.Description(`Optional expect rules: [{"pattern":"string","send":"string"}]. Enables PTY mode.`)),
		mcpgo.WithNumber("timeout_sec", mcpgo.Description("Per-server timeout in seconds (default 30)")),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		clusterID, err := parseID(req, "cluster_id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		commands, err := parseStringSlice(args, "commands")
		if err != nil || len(commands) == 0 {
			return toolError("commands must be a non-empty array of strings"), nil
		}
		rules, _ := parseExpectRules(args, "expect")
		timeoutSec, _ := args["timeout_sec"].(float64)

		c, err := d.DB.Clusters.GetByID(ctx, clusterID)
		if err != nil || c == nil {
			return toolError(fmt.Sprintf("cluster %d not found", clusterID)), nil
		}

		type serverResult struct {
			ServerID int64           `json:"server_id"`
			Host     string          `json:"host"`
			Results  []CommandResult `json:"results,omitempty"`
			Error    string          `json:"error,omitempty"`
		}

		results := make([]serverResult, len(c.Members))
		var wg sync.WaitGroup
		for i, m := range c.Members {
			wg.Add(1)
			go func(idx int, member model.ClusterMember) {
				defer wg.Done()
				srv, _ := d.DB.Servers.GetByID(ctx, member.ServerID)
				host := ""
				if srv != nil {
					host = srv.Host
				}
				results[idx] = serverResult{ServerID: member.ServerID, Host: host}

				cmdResults, err := execOnServer(ctx, d.DB, d.Vault, ExecRequest{
					ServerID:   member.ServerID,
					Commands:   commands,
					Expect:     rules,
					TimeoutSec: int(timeoutSec),
				})
				if err != nil {
					results[idx].Error = err.Error()
				} else {
					results[idx].Results = cmdResults
				}
			}(i, m)
		}
		wg.Wait()

		return toolJSON(map[string]any{
			"cluster_id":   c.ID,
			"cluster_name": c.Name,
			"results":      results,
		})
	})
}

// --- helpers ---

func toolJSON(v any) (*mcpgo.CallToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return toolError(fmt.Sprintf("marshal: %v", err)), nil
	}
	return mcpgo.NewToolResultText(string(data)), nil
}

func toolError(msg string) *mcpgo.CallToolResult {
	r := mcpgo.NewToolResultText(msg)
	r.IsError = true
	return r
}

func parseID(req mcpgo.CallToolRequest, key string) (int64, error) {
	args := req.GetArguments()
	v, ok := args[key]
	if !ok {
		return 0, fmt.Errorf("%s is required", key)
	}
	switch x := v.(type) {
	case float64:
		return int64(x), nil
	case string:
		var id int64
		if _, err := fmt.Sscanf(x, "%d", &id); err != nil {
			return 0, fmt.Errorf("invalid %s: %s", key, x)
		}
		return id, nil
	default:
		return 0, fmt.Errorf("invalid %s type", key)
	}
}

func parseStringSlice(args map[string]any, key string) ([]string, error) {
	v, ok := args[key]
	if !ok {
		return nil, nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	result := make([]string, 0, len(raw))
	for _, item := range raw {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%s items must be strings", key)
		}
		result = append(result, s)
	}
	return result, nil
}

func parseExpectRules(args map[string]any, key string) ([]ExpectRule, error) {
	v, ok := args[key]
	if !ok {
		return nil, nil
	}
	raw, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%s must be an array", key)
	}
	var rules []ExpectRule
	for _, item := range raw {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		pattern, _ := m["pattern"].(string)
		send, _ := m["send"].(string)
		if pattern != "" {
			rules = append(rules, ExpectRule{Pattern: pattern, Send: send})
		}
	}
	return rules, nil
}
