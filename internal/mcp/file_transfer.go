package mcp

import (
	"context"
	"fmt"

	internalssh "github.com/emusal/alogin2/internal/ssh"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

// newPushFileHandler returns the handler for the push_file MCP tool.
// local_path is relative to the alogin process's working directory.
func newPushFileHandler(d Deps) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		serverID, err := parseID(req, "server_id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		localPath, _ := args["local_path"].(string)
		remotePath, _ := args["remote_path"].(string)
		agentID, _ := args["agent_id"].(string)
		if localPath == "" || remotePath == "" {
			return toolError("local_path and remote_path are required"), nil
		}

		srv, err := d.DB.Servers.GetByID(ctx, serverID)
		if err != nil || srv == nil {
			return toolError(fmt.Sprintf("server %d not found", serverID)), nil
		}

		ev := auditEvent{
			Event:      "push_file",
			AgentID:    agentID,
			ServerID:   serverID,
			ServerHost: srv.Host,
			Commands:   []string{fmt.Sprintf("push %s → %s", localPath, remotePath)},
		}
		writeAudit(d.AuditLog, ev)
		writeAuditDB(ctx, d, ev)

		hops, err := buildHopChain(ctx, d.DB, d.Vault, srv)
		if err != nil {
			return toolError(fmt.Sprintf("build hop chain: %v", err)), nil
		}
		chain, err := internalssh.DialChain(hops)
		if err != nil {
			return toolError(fmt.Sprintf("dial: %v", err)), nil
		}
		defer chain.CloseAll()

		sc, err := chain.Terminal().SFTPClient()
		if err != nil {
			return toolError(fmt.Sprintf("sftp client: %v", err)), nil
		}
		defer sc.Close()

		if err := sc.Upload(localPath, remotePath); err != nil {
			return toolError(fmt.Sprintf("upload: %v", err)), nil
		}
		return toolJSON(map[string]any{
			"status":      "uploaded",
			"local_path":  localPath,
			"remote_path": remotePath,
			"server_id":   serverID,
		})
	}
}

// newPullFileHandler returns the handler for the pull_file MCP tool.
// local_path is relative to the alogin process's working directory.
func newPullFileHandler(d Deps) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		serverID, err := parseID(req, "server_id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		remotePath, _ := args["remote_path"].(string)
		localPath, _ := args["local_path"].(string)
		agentID, _ := args["agent_id"].(string)
		if remotePath == "" || localPath == "" {
			return toolError("remote_path and local_path are required"), nil
		}

		srv, err := d.DB.Servers.GetByID(ctx, serverID)
		if err != nil || srv == nil {
			return toolError(fmt.Sprintf("server %d not found", serverID)), nil
		}

		ev := auditEvent{
			Event:      "pull_file",
			AgentID:    agentID,
			ServerID:   serverID,
			ServerHost: srv.Host,
			Commands:   []string{fmt.Sprintf("pull %s ← %s", localPath, remotePath)},
		}
		writeAudit(d.AuditLog, ev)
		writeAuditDB(ctx, d, ev)

		hops, err := buildHopChain(ctx, d.DB, d.Vault, srv)
		if err != nil {
			return toolError(fmt.Sprintf("build hop chain: %v", err)), nil
		}
		chain, err := internalssh.DialChain(hops)
		if err != nil {
			return toolError(fmt.Sprintf("dial: %v", err)), nil
		}
		defer chain.CloseAll()

		sc, err := chain.Terminal().SFTPClient()
		if err != nil {
			return toolError(fmt.Sprintf("sftp client: %v", err)), nil
		}
		defer sc.Close()

		if err := sc.Download(remotePath, localPath); err != nil {
			return toolError(fmt.Sprintf("download: %v", err)), nil
		}
		return toolJSON(map[string]any{
			"status":      "downloaded",
			"remote_path": remotePath,
			"local_path":  localPath,
			"server_id":   serverID,
		})
	}
}
