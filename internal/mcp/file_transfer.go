package mcp

import (
	"context"
	"fmt"
	"strings"

	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/google/uuid"
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

// newRunScriptHandler returns the handler for the run_script MCP tool.
// It uploads script content via SFTP to a temp path, executes it, then deletes it —
// all in one atomic tool call so the agent never has to manage quoting or temp files.
func newRunScriptHandler(d Deps) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		serverID, err := parseID(req, "server_id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		content, _ := args["content"].(string)
		if strings.TrimSpace(content) == "" {
			return toolError("content is required"), nil
		}
		interpreter, _ := args["interpreter"].(string)
		if interpreter == "" {
			interpreter = "bash"
		}
		agentID, _ := args["agent_id"].(string)
		intent, _ := args["intent"].(string)
		timeoutSec, _ := args["timeout_sec"].(float64)
		if timeoutSec <= 0 {
			timeoutSec = 120
		}
		loginShell, _ := args["login_shell"].(bool)

		srv, err := d.DB.Servers.GetByID(ctx, serverID)
		if err != nil || srv == nil {
			return toolError(fmt.Sprintf("server %d not found", serverID)), nil
		}

		remoteTmp := fmt.Sprintf("/tmp/alogin-%s.sh", uuid.New().String())
		runCmd := fmt.Sprintf("%s %s; _rc=$?; rm -f %s; exit $_rc", interpreter, remoteTmp, remoteTmp)

		ev := auditEvent{
			Event:      "run_script",
			AgentID:    agentID,
			ServerID:   serverID,
			ServerHost: srv.Host,
			Commands:   []string{runCmd},
			Intent:     intent,
			TimeoutSec: int(timeoutSec),
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

		// Upload script content via SFTP.
		sc, err := chain.Terminal().SFTPClient()
		if err != nil {
			return toolError(fmt.Sprintf("sftp client: %v", err)), nil
		}
		if err := sc.UploadContent([]byte(content), remoteTmp); err != nil {
			sc.Close()
			return toolError(fmt.Sprintf("upload script: %v", err)), nil
		}
		sc.Close()

		// Execute and auto-delete via managed session (login shell optional).
		managed, err := internalssh.NewManagedSession(chain.Terminal(), loginShell)
		if err != nil {
			return toolError(fmt.Sprintf("managed session: %v", err)), nil
		}
		defer managed.Close()

		result, execErr := managed.Exec(ctx, runCmd, 0)
		if execErr != nil {
			return toolError(fmt.Sprintf("exec script: %v", execErr)), nil
		}
		return toolJSON(map[string]any{
			"server_id": serverID,
			"output":    result.Output,
			"exit_code": result.ExitCode,
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
