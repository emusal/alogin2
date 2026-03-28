package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/model"
	"github.com/emusal/alogin2/internal/policy"
	"github.com/emusal/alogin2/internal/tunnel"
	"github.com/emusal/alogin2/internal/vault"
	"github.com/google/uuid"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// Deps holds dependencies for all MCP tool handlers.
type Deps struct {
	DB        *db.DB
	Vault     vault.Vault
	BinPath   string    // path to the alogin binary (for tunnel start)
	AuditLog  io.Writer // nil = disabled; typically an *os.File opened in append mode
	Policy    *policy.Engine // nil = allow-all with built-in destructive-command check only
	ConfigDir string         // base config dir for HITL file paths
}

// auditEvent is one line in the JSONL audit log.
type auditEvent struct {
	Timestamp    string   `json:"timestamp"`
	Event        string   `json:"event"`
	AgentID      string   `json:"agent_id,omitempty"`
	ServerID     int64    `json:"server_id,omitempty"`
	ServerHost   string   `json:"server_host,omitempty"`
	ClusterID    int64    `json:"cluster_id,omitempty"`
	ClusterName  string   `json:"cluster_name,omitempty"`
	Commands     []string `json:"commands"`
	Intent       string   `json:"intent,omitempty"`
	TimeoutSec   int      `json:"timeout_sec,omitempty"`
	PolicyAction string   `json:"policy_action,omitempty"` // "allow", "deny", "require_approval"
	ApprovedBy   string   `json:"approved_by,omitempty"`   // HITL token if approved
}

// writeAudit appends ev as a JSON line to w. Best-effort: errors are silently ignored.
func writeAudit(w io.Writer, ev auditEvent) {
	if w == nil {
		return
	}
	ev.Timestamp = time.Now().UTC().Format(time.RFC3339)
	data, err := json.Marshal(ev)
	if err != nil {
		return
	}
	data = append(data, '\n')
	_, _ = w.Write(data)
}

// writeAuditDB persists ev to the DB audit_log table. Best-effort: errors are silently ignored.
func writeAuditDB(ctx context.Context, d Deps, ev auditEvent) {
	if d.DB == nil || d.DB.AuditLog == nil {
		return
	}
	entry := db.AuditEntry{
		Timestamp:    ev.Timestamp,
		Event:        ev.Event,
		AgentID:      ev.AgentID,
		ServerHost:   ev.ServerHost,
		ClusterName:  ev.ClusterName,
		Commands:     ev.Commands,
		Intent:       ev.Intent,
		TimeoutSec:   ev.TimeoutSec,
		PolicyAction: ev.PolicyAction,
		ApprovedBy:   ev.ApprovedBy,
	}
	if ev.ServerID != 0 {
		v := ev.ServerID
		entry.ServerID = &v
	}
	if ev.ClusterID != 0 {
		v := ev.ClusterID
		entry.ClusterID = &v
	}
	_, _ = d.DB.AuditLog.Insert(ctx, entry)
}

// evalPolicy evaluates a policy check request against the given engine and returns
// the CheckResult. eng may differ from d.Policy when a per-server override is active.
// Built-in destructive patterns are always enforced as a safety floor. A named rule
// that explicitly matches overrides the floor (the rule author is aware of what they
// are allowing/denying); the default_action alone does not override it.
func evalPolicy(eng *policy.Engine, req policy.CheckRequest) policy.CheckResult {
	if eng != nil {
		result := eng.Check(req)
		// A named rule matched — the policy author explicitly handled this command.
		if result.RuleName != "" {
			return result
		}
		// No rule matched; default_action applied. Still enforce built-in floor.
		if policy.IsDestructive(req.Commands) {
			return policy.CheckResult{Action: "require_approval", RuleName: "built-in destructive patterns"}
		}
		return result
	}
	if policy.IsDestructive(req.Commands) {
		return policy.CheckResult{Action: "require_approval", RuleName: "built-in destructive patterns"}
	}
	return policy.CheckResult{Action: "allow"}
}

// applyPolicyResult enforces a pre-computed CheckResult (HITL wait, deny, or allow).
// Returns a non-nil *CallToolResult if execution should be blocked; nil means proceed.
func applyPolicyResult(ctx context.Context, d Deps, eng *policy.Engine, result policy.CheckResult, req policy.CheckRequest) *mcpgo.CallToolResult {
	switch result.Action {
	case "deny":
		msg := "policy denied"
		if result.RuleName != "" {
			msg += fmt.Sprintf(": rule %q", result.RuleName)
		}
		return toolError(msg)
	case "require_approval":
		if d.ConfigDir == "" {
			return toolError("HITL required but ConfigDir not set")
		}
		token := uuid.New().String()
		pending := policy.PendingRequest{
			Token:     token,
			AgentID:   req.AgentID,
			ServerID:  req.ServerID,
			ClusterID: req.ClusterID,
			Commands:  req.Commands,
			CreatedAt: time.Now(),
		}
		timeout := hitlTimeout(eng)
		outcome, err := policy.RequestApproval(ctx, d.ConfigDir, pending, timeout)
		if err != nil {
			return toolError(fmt.Sprintf("HITL error: %v", err))
		}
		if outcome != policy.OutcomeApproved {
			return toolError(fmt.Sprintf("HITL: %s (token: %s)", outcome, token))
		}
		return nil // approved — proceed
	default:
		return nil // "allow"
	}
}

// checkPolicy is a convenience wrapper: evaluates then applies in one call.
// eng is the effective engine for the target server (may differ from d.Policy).
func checkPolicy(ctx context.Context, d Deps, eng *policy.Engine, req policy.CheckRequest) *mcpgo.CallToolResult {
	return applyPolicyResult(ctx, d, eng, evalPolicy(eng, req), req)
}

func hitlTimeout(eng *policy.Engine) time.Duration {
	if eng != nil {
		return eng.HITLTimeout()
	}
	return 120 * time.Second
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
		mcpgo.WithString("agent_id", mcpgo.Description("Optional: identifier for the calling agent (e.g. 'claude-desktop/session-abc')")),
		mcpgo.WithString("intent", mcpgo.Description("Optional: human-readable description of what the agent intends to do (logged to audit trail)")),
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
		agentID, _ := args["agent_id"].(string)
		intent, _ := args["intent"].(string)

		// Fetch server — needed for audit log, per-server policy, and system_prompt.
		srv, _ := d.DB.Servers.GetByID(ctx, id)
		srvHost := ""
		if srv != nil {
			srvHost = srv.Host
		}

		ev := auditEvent{
			Event:      "exec_command",
			AgentID:    agentID,
			ServerID:   id,
			ServerHost: srvHost,
			Commands:   commands,
			Intent:     intent,
			TimeoutSec: int(timeoutSec),
		}
		writeAudit(d.AuditLog, ev)
		writeAuditDB(ctx, d, ev)

		// Resolve per-server policy (falls back to global when PolicyYAML is empty).
		srvPolicyYAML := ""
		if srv != nil {
			srvPolicyYAML = srv.PolicyYAML
		}
		eng, err := policy.ResolveFor(d.Policy, srvPolicyYAML)
		if err != nil {
			return toolError(fmt.Sprintf("per-server policy error: %v", err)), nil
		}
		if blocked := checkPolicy(ctx, d, eng, policy.CheckRequest{
			AgentID:  agentID,
			Commands: commands,
			ServerID: id,
		}); blocked != nil {
			return blocked, nil
		}

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
		mcpgo.WithString("agent_id", mcpgo.Description("Optional: identifier for the calling agent (e.g. 'claude-desktop/session-abc')")),
		mcpgo.WithString("intent", mcpgo.Description("Optional: human-readable description of what the agent intends to do (logged to audit trail)")),
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
		agentID, _ := args["agent_id"].(string)
		intent, _ := args["intent"].(string)

		c, err := d.DB.Clusters.GetByID(ctx, clusterID)
		if err != nil || c == nil {
			return toolError(fmt.Sprintf("cluster %d not found", clusterID)), nil
		}

		clusterEv := auditEvent{
			Event:       "exec_on_cluster",
			AgentID:     agentID,
			ClusterID:   c.ID,
			ClusterName: c.Name,
			Commands:    commands,
			Intent:      intent,
			TimeoutSec:  int(timeoutSec),
		}
		writeAudit(d.AuditLog, clusterEv)
		writeAuditDB(ctx, d, clusterEv)

		// Pre-fetch all member servers for per-server policy resolution and fan-out reuse.
		type memberServer struct {
			member model.ClusterMember
			server *model.Server // may be nil if not found
		}
		memberSrvs := make([]memberServer, len(c.Members))
		for i, m := range c.Members {
			s, _ := d.DB.Servers.GetByID(ctx, m.ServerID)
			memberSrvs[i] = memberServer{member: m, server: s}
		}

		// Strictest-member policy: deny > require_approval > allow.
		// A single deny blocks; a single require_approval triggers HITL once.
		strictnessRank := map[string]int{"allow": 0, "require_approval": 1, "deny": 2}
		effectiveResult := policy.CheckResult{Action: "allow"}
		for _, ms := range memberSrvs {
			policyYAML := ""
			if ms.server != nil {
				policyYAML = ms.server.PolicyYAML
			}
			eng, err := policy.ResolveFor(d.Policy, policyYAML)
			if err != nil {
				return toolError(fmt.Sprintf("per-server policy error (server %d): %v",
					ms.member.ServerID, err)), nil
			}
			result := evalPolicy(eng, policy.CheckRequest{
				AgentID:   agentID,
				Commands:  commands,
				ServerID:  ms.member.ServerID,
				ClusterID: c.ID,
			})
			if strictnessRank[result.Action] > strictnessRank[effectiveResult.Action] {
				effectiveResult = result
			}
		}
		if blocked := applyPolicyResult(ctx, d, d.Policy, effectiveResult, policy.CheckRequest{
			AgentID:   agentID,
			Commands:  commands,
			ClusterID: c.ID,
		}); blocked != nil {
			return blocked, nil
		}

		type serverResult struct {
			ServerID int64           `json:"server_id"`
			Host     string          `json:"host"`
			Results  []CommandResult `json:"results,omitempty"`
			Error    string          `json:"error,omitempty"`
		}

		results := make([]serverResult, len(memberSrvs))
		var wg sync.WaitGroup
		for i, ms := range memberSrvs {
			wg.Add(1)
			go func(idx int, ms memberServer) {
				defer wg.Done()
				host := ""
				if ms.server != nil {
					host = ms.server.Host
				}
				results[idx] = serverResult{ServerID: ms.member.ServerID, Host: host}

				cmdResults, err := execOnServer(ctx, d.DB, d.Vault, ExecRequest{
					ServerID:   ms.member.ServerID,
					Commands:   commands,
					Expect:     rules,
					TimeoutSec: int(timeoutSec),
				})
				if err != nil {
					results[idx].Error = err.Error()
				} else {
					results[idx].Results = cmdResults
				}
			}(i, ms)
		}
		wg.Wait()

		return toolJSON(map[string]any{
			"cluster_id":   c.ID,
			"cluster_name": c.Name,
			"results":      results,
		})
	})

	// --- inspect_node ---
	srv.AddTool(mcpgo.NewTool("inspect_node",
		mcpgo.WithDescription("Get a structured health snapshot of a server: CPU load, memory usage, disk space, and top processes. Falls back to raw output if parsing fails."),
		mcpgo.WithString("server_id", mcpgo.Description("Server ID"), mcpgo.Required()),
		mcpgo.WithNumber("timeout_sec", mcpgo.Description("Execution timeout in seconds (default 30)")),
		mcpgo.WithString("agent_id", mcpgo.Description("Optional: identifier for the calling agent")),
		mcpgo.WithString("intent", mcpgo.Description("Optional: human-readable description of what the agent intends to do (logged to audit trail)")),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		id, err := parseID(req, "server_id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		timeoutSec, _ := args["timeout_sec"].(float64)
		agentID, _ := args["agent_id"].(string)
		intent, _ := args["intent"].(string)

		srv, _ := d.DB.Servers.GetByID(ctx, id)
		host := ""
		if srv != nil {
			host = srv.Host
		}

		commands := []string{
			"cat /proc/loadavg 2>/dev/null || uptime",
			"free -b 2>/dev/null || vm_stat",
			"df -P / 2>/dev/null",
			"ps aux --sort=-%cpu 2>/dev/null | head -6 || ps aux | head -6",
		}

		inspectEv := auditEvent{
			Event:      "inspect_node",
			AgentID:    agentID,
			ServerID:   id,
			ServerHost: host,
			Commands:   commands,
			Intent:     intent,
			TimeoutSec: int(timeoutSec),
		}
		writeAudit(d.AuditLog, inspectEv)
		writeAuditDB(ctx, d, inspectEv)

		results, err := execOnServer(ctx, d.DB, d.Vault, ExecRequest{
			ServerID:   id,
			Commands:   commands,
			TimeoutSec: int(timeoutSec),
		})
		if err != nil {
			return toolJSON(nodeHealth{ServerID: id, Host: host, Error: err.Error()})
		}

		// Map command index → output string
		raw := make(map[string]string, len(commands))
		outputs := make([]string, len(commands))
		labels := []string{"loadavg", "memory", "disk", "processes"}
		for i, r := range results {
			outputs[i] = strings.TrimSpace(r.Output)
			if r.Error != "" || r.ExitCode != 0 {
				raw[labels[i]] = outputs[i]
			}
		}

		health := nodeHealth{ServerID: id, Host: host}
		health.CPU = parseCPU(outputs[0], &raw)
		health.Memory = parseMemory(outputs[1], &raw)
		health.Disk = parseDisk(outputs[2], &raw)
		health.Processes = parseProcesses(outputs[3], &raw)
		if len(raw) > 0 {
			health.RawOutputs = raw
		}
		return toolJSON(health)
	})

	// --- log_analyzer ---
	srv.AddTool(mcpgo.NewTool("log_analyzer",
		mcpgo.WithDescription("Stream logs and return only the relevant error patterns to save token context."),
		mcpgo.WithString("server_id", mcpgo.Description("Server ID"), mcpgo.Required()),
		mcpgo.WithString("target", mcpgo.Description("Log file path (e.g., /var/log/syslog) or journalctl unit (e.g., ssh)"), mcpgo.Required()),
		mcpgo.WithBoolean("is_journal", mcpgo.Description("If true, target is treated as a systemd journalctl unit. Default false (file path).")),
		mcpgo.WithNumber("lines", mcpgo.Description("Number of lines to inspect (default 1000)")),
		mcpgo.WithString("pattern", mcpgo.Description("Regex pattern for filtering (default: 'error|warn|fail|fatal|exception')")),
		mcpgo.WithNumber("max_results", mcpgo.Description("Maximum matching lines to return (default 50)")),
		mcpgo.WithString("agent_id", mcpgo.Description("Optional: identifier for the calling agent")),
		mcpgo.WithString("intent", mcpgo.Description("Optional: human-readable description of what the agent intends to do (logged to audit trail)")),
	), func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		id, err := parseID(req, "server_id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		
		targetV, ok := args["target"]
		var target string
		if ok {
			target, _ = targetV.(string)
		}
		if target == "" {
			return toolError("target is required"), nil
		}
		
		isJournal, _ := args["is_journal"].(bool)
		
		linesNum, ok := args["lines"].(float64)
		if !ok || linesNum <= 0 {
			linesNum = 1000
		}
		lines := int(linesNum)
		
		patternV, ok := args["pattern"]
		var pattern string
		if ok {
			pattern, _ = patternV.(string)
		}
		if pattern == "" {
			pattern = "error|warn|fail|fatal|exception"
		}
		
		maxResultsNum, ok := args["max_results"].(float64)
		if !ok || maxResultsNum <= 0 {
			maxResultsNum = 50
		}
		maxResults := int(maxResultsNum)
		
		agentID, _ := args["agent_id"].(string)
		intent, _ := args["intent"].(string)

		srvNode, _ := d.DB.Servers.GetByID(ctx, id)
		host := ""
		if srvNode != nil {
			host = srvNode.Host
		}

		var cmdStr string
		if isJournal {
			cmdStr = fmt.Sprintf("journalctl -u %s -n %d --no-pager | grep -iE '%s' | tail -n %d",
				target, lines, pattern, maxResults)
		} else {
			cmdStr = fmt.Sprintf("tail -n %d %s | grep -iE '%s' | tail -n %d",
				lines, target, pattern, maxResults)
		}

		logEv := auditEvent{
			Event:      "log_analyzer",
			AgentID:    agentID,
			ServerID:   id,
			ServerHost: host,
			Commands:   []string{cmdStr},
			Intent:     intent,
			TimeoutSec: 30,
		}
		writeAudit(d.AuditLog, logEv)
		writeAuditDB(ctx, d, logEv)

		results, err := execOnServer(ctx, d.DB, d.Vault, ExecRequest{
			ServerID:   id,
			Commands:   []string{cmdStr},
			TimeoutSec: 30,
		})
		
		if err != nil {
			return toolJSON(map[string]any{"server_id": id, "target": target, "error": err.Error()})
		}
		
		var matches []string
		if len(results) > 0 && results[0].Output != "" {
			linesOut := strings.Split(strings.TrimSpace(results[0].Output), "\n")
			for _, l := range linesOut {
				if l != "" {
					matches = append(matches, l)
				}
			}
		}

		if len(matches) == 0 {
			matches = make([]string, 0)
		}

		return toolJSON(map[string]any{
			"server_id": id,
			"target":    target,
			"matches":   matches,
		})
	})

	// --- exec_app ---
	srv.AddTool(mcpgo.NewTool("exec_app",
		mcpgo.WithDescription("Execute a command within an application plugin context on a server. "+
			"Resolves credentials from vault, auto-detects runtime (Docker/native), "+
			"and injects secrets via Expect-Send PTY automation. "+
			"Plugin YAML files must exist in the ConfigDir/plugins/ directory."),
		mcpgo.WithString("server_id", mcpgo.Description("Server ID"), mcpgo.Required()),
		mcpgo.WithString("app_name", mcpgo.Description("Plugin name (filename without .yaml, e.g. mariadb)"), mcpgo.Required()),
	), handleExecApp(d))
}

// nodeHealth is the structured output of inspect_node.
type nodeHealth struct {
	ServerID   int64             `json:"server_id"`
	Host       string            `json:"host"`
	CPU        cpuInfo           `json:"cpu"`
	Memory     memInfo           `json:"memory"`
	Disk       diskInfo          `json:"disk"`
	Processes  []processEntry    `json:"top_processes"`
	RawOutputs map[string]string `json:"raw,omitempty"`
	Error      string            `json:"error,omitempty"`
}

type cpuInfo struct {
	Load1  float64 `json:"load1"`
	Load5  float64 `json:"load5"`
	Load15 float64 `json:"load15"`
}

type memInfo struct {
	TotalBytes     int64   `json:"total_bytes"`
	UsedBytes      int64   `json:"used_bytes"`
	FreeBytes      int64   `json:"free_bytes"`
	UsedPct        float64 `json:"used_pct"`
}

type diskInfo struct {
	TotalBytes int64   `json:"total_bytes"`
	UsedBytes  int64   `json:"used_bytes"`
	FreeBytes  int64   `json:"free_bytes"`
	UsedPct    float64 `json:"used_pct"`
}

type processEntry struct {
	User    string  `json:"user"`
	PID     string  `json:"pid"`
	CPUPct  float64 `json:"cpu_pct"`
	MemPct  float64 `json:"mem_pct"`
	Command string  `json:"command"`
}

// parseCPU parses /proc/loadavg or uptime output.
func parseCPU(out string, raw *map[string]string) cpuInfo {
	// /proc/loadavg: "0.52 0.41 0.38 1/423 12345"
	fields := strings.Fields(out)
	if len(fields) >= 3 {
		l1, e1 := strconv.ParseFloat(fields[0], 64)
		l5, e2 := strconv.ParseFloat(fields[1], 64)
		l15, e3 := strconv.ParseFloat(fields[2], 64)
		if e1 == nil && e2 == nil && e3 == nil {
			return cpuInfo{Load1: l1, Load5: l5, Load15: l15}
		}
	}
	// uptime fallback: "... load averages: 0.52 0.41 0.38"
	if idx := strings.LastIndex(out, ":"); idx >= 0 {
		fields = strings.Fields(strings.ReplaceAll(out[idx+1:], ",", ""))
		if len(fields) >= 3 {
			l1, e1 := strconv.ParseFloat(fields[0], 64)
			l5, e2 := strconv.ParseFloat(fields[1], 64)
			l15, e3 := strconv.ParseFloat(fields[2], 64)
			if e1 == nil && e2 == nil && e3 == nil {
				return cpuInfo{Load1: l1, Load5: l5, Load15: l15}
			}
		}
	}
	(*raw)["loadavg"] = out
	return cpuInfo{}
}

// parseMemory parses `free -b` output (Linux).
func parseMemory(out string, raw *map[string]string) memInfo {
	for _, line := range strings.Split(out, "\n") {
		if !strings.HasPrefix(line, "Mem:") {
			continue
		}
		f := strings.Fields(line)
		// free -b: Mem: total used free shared buff/cache available
		if len(f) >= 4 {
			total, e1 := strconv.ParseInt(f[1], 10, 64)
			used, e2 := strconv.ParseInt(f[2], 10, 64)
			free, e3 := strconv.ParseInt(f[3], 10, 64)
			if e1 == nil && e2 == nil && e3 == nil && total > 0 {
				return memInfo{
					TotalBytes: total,
					UsedBytes:  used,
					FreeBytes:  free,
					UsedPct:    float64(used) / float64(total) * 100,
				}
			}
		}
	}
	(*raw)["memory"] = out
	return memInfo{}
}

// parseDisk parses `df -P /` output.
func parseDisk(out string, raw *map[string]string) diskInfo {
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		// df -P: Filesystem 1024-blocks Used Available Capacity% Mounted
		if len(f) < 6 || f[0] == "Filesystem" {
			continue
		}
		blocks, e1 := strconv.ParseInt(f[1], 10, 64)
		used, e2 := strconv.ParseInt(f[2], 10, 64)
		avail, e3 := strconv.ParseInt(f[3], 10, 64)
		if e1 == nil && e2 == nil && e3 == nil && blocks > 0 {
			total := blocks * 1024
			usedB := used * 1024
			freeB := avail * 1024
			return diskInfo{
				TotalBytes: total,
				UsedBytes:  usedB,
				FreeBytes:  freeB,
				UsedPct:    float64(usedB) / float64(total) * 100,
			}
		}
	}
	(*raw)["disk"] = out
	return diskInfo{}
}

// parseProcesses parses `ps aux` output (skips header, up to 5 entries).
func parseProcesses(out string, raw *map[string]string) []processEntry {
	var procs []processEntry
	for _, line := range strings.Split(out, "\n") {
		f := strings.Fields(line)
		// ps aux: USER PID %CPU %MEM VSZ RSS TTY STAT START TIME COMMAND...
		if len(f) < 11 || f[0] == "USER" {
			continue
		}
		cpu, e1 := strconv.ParseFloat(f[2], 64)
		mem, e2 := strconv.ParseFloat(f[3], 64)
		if e1 != nil || e2 != nil {
			continue
		}
		procs = append(procs, processEntry{
			User:    f[0],
			PID:     f[1],
			CPUPct:  cpu,
			MemPct:  mem,
			Command: strings.Join(f[10:], " "),
		})
		if len(procs) >= 5 {
			break
		}
	}
	if len(procs) == 0 {
		(*raw)["processes"] = out
	}
	return procs
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
