package mcp

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/emusal/alogin2/internal/plugin"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// handleExecApp returns the handler for the alogin_exec_app MCP tool.
// The tool resolves credentials from vault, detects the runtime (Docker/native),
// injects secrets via Expect-Send, and returns the command output as JSON.
func handleExecApp(d Deps) func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()

		serverID, err := parseID(req, "server_id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		appName, _ := args["app_name"].(string)
		if appName == "" {
			return toolError("app_name is required"), nil
		}

		if d.ConfigDir == "" {
			return toolError("ConfigDir not set; cannot locate plugin directory"), nil
		}
		pluginPath := filepath.Join(plugin.PluginDir(d.ConfigDir), appName+".yaml")
		p, err := plugin.LoadFromFile(pluginPath)
		if err != nil {
			return toolError(fmt.Sprintf("load plugin %q: %v", appName, err)), nil
		}

		runner, err := newMCPRunner(ctx, d, serverID)
		if err != nil {
			return toolError(fmt.Sprintf("connect to server %d: %v", serverID, err)), nil
		}
		defer runner.close()

		sess, err := plugin.Prepare(ctx, p, d.Vault, runner)
		if err != nil {
			return toolError(fmt.Sprintf("prepare plugin: %v", err)), nil
		}

		// Audit log — variable names only, never values.
		srv, _ := d.DB.Servers.GetByID(ctx, serverID)
		host := ""
		if srv != nil {
			host = srv.Host
		}
		entry := buildPluginAuditEntry(serverID, host, sess)
		writeAudit(d.AuditLog, auditEvent{
			Event:      entry.Event,
			ServerID:   serverID,
			ServerHost: entry.ServerHost,
		})
		writeAuditDB(ctx, d, auditEvent{
			Event:      entry.Event,
			ServerID:   serverID,
			ServerHost: entry.ServerHost,
		})

		output, err := sess.Launch(ctx, runner)
		if err != nil {
			return toolError(fmt.Sprintf("launch: %v", err)), nil
		}

		return toolJSON(map[string]string{
			"output":   output,
			"strategy": sess.Strategy.Kind,
			"plugin":   p.Name,
		})
	}
}
