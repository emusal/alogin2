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
		recursive, _ := args["recursive"].(bool)
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

		if recursive {
			if err := sc.UploadDir(localPath, remotePath); err != nil {
				return toolError(fmt.Sprintf("upload dir: %v", err)), nil
			}
		} else {
			if err := sc.Upload(localPath, remotePath); err != nil {
				return toolError(fmt.Sprintf("upload: %v", err)), nil
			}
		}
		return toolJSON(map[string]any{
			"status":      "uploaded",
			"local_path":  localPath,
			"remote_path": remotePath,
			"server_id":   serverID,
			"recursive":   recursive,
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
		loginShell := true
		if v, ok := args["login_shell"]; ok {
			loginShell, _ = v.(bool)
		}

		srv, err := d.DB.Servers.GetByID(ctx, serverID)
		if err != nil || srv == nil {
			return toolError(fmt.Sprintf("server %d not found", serverID)), nil
		}

		remoteTmp := fmt.Sprintf("/tmp/alogin-%s.sh", uuid.New().String())
		// Use (exit $_rc) instead of `exit $_rc` so the persistent bash process
		// in ManagedSession is not killed, which would cause an EOF on the pipe.
		runCmd := fmt.Sprintf("%s %s; _rc=$?; rm -f %s; (exit $_rc)", interpreter, remoteTmp, remoteTmp)

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
		recursive, _ := args["recursive"].(bool)
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

		if recursive {
			if err := sc.DownloadDir(remotePath, localPath); err != nil {
				return toolError(fmt.Sprintf("download dir: %v", err)), nil
			}
		} else {
			if err := sc.Download(remotePath, localPath); err != nil {
				return toolError(fmt.Sprintf("download: %v", err)), nil
			}
		}
		return toolJSON(map[string]any{
			"status":      "downloaded",
			"remote_path": remotePath,
			"local_path":  localPath,
			"server_id":   serverID,
			"recursive":   recursive,
		})
	}
}

// newRemoteReplaceHandler returns the handler for the remote_replace MCP tool.
//
// The replacement is performed by uploading a small Python script to a temp path
// and executing it. This avoids all shell-quoting and sed cross-platform issues
// (BSD sed vs GNU sed -i semantics differ) and handles arbitrary special characters
// in old_string / new_string safely — the strings are written as Python bytes
// literals via SFTP, never interpolated into a shell command line.
//
// Falls back to sed if Python 3 is not available, using a temp copy of the file
// so the original is only overwritten after a successful substitution.
func newRemoteReplaceHandler(d Deps) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		serverID, err := parseID(req, "server_id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		path, _ := args["path"].(string)
		oldString, _ := args["old_string"].(string)
		newString, _ := args["new_string"].(string)
		agentID, _ := args["agent_id"].(string)
		intent, _ := args["intent"].(string)
		replaceAll := true
		if v, ok := args["all"]; ok {
			replaceAll, _ = v.(bool)
		}

		if path == "" || oldString == "" {
			return toolError("path and old_string are required"), nil
		}

		srv, err := d.DB.Servers.GetByID(ctx, serverID)
		if err != nil || srv == nil {
			return toolError(fmt.Sprintf("server %d not found", serverID)), nil
		}

		ev := auditEvent{
			Event:      "remote_replace",
			AgentID:    agentID,
			ServerID:   serverID,
			ServerHost: srv.Host,
			Commands:   []string{fmt.Sprintf("replace in %s", path)},
			Intent:     intent,
		}
		writeAudit(d.AuditLog, ev)
		writeAuditDB(ctx, d, ev)

		// Build a self-contained Python script that performs the replacement.
		// old_string and new_string are passed as hex-escaped literals so no
		// shell quoting or SFTP encoding issue can corrupt them.
		countMode := "1"
		if replaceAll {
			countMode = "-1"
		}
		script := buildReplaceScript(path, oldString, newString, countMode)

		tmpPath := fmt.Sprintf("/tmp/alogin-replace-%s.py", uuid.New().String())

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
		if err := sc.UploadContent([]byte(script), tmpPath); err != nil {
			sc.Close()
			return toolError(fmt.Sprintf("upload replace script: %v", err)), nil
		}
		sc.Close()

		runCmd := fmt.Sprintf("python3 %s; _rc=$?; rm -f %s; (exit $_rc)", tmpPath, tmpPath)
		managed, err := internalssh.NewManagedSession(chain.Terminal(), true)
		if err != nil {
			return toolError(fmt.Sprintf("managed session: %v", err)), nil
		}
		defer managed.Close()

		result, execErr := managed.Exec(ctx, runCmd, 0)
		if execErr != nil {
			return toolError(fmt.Sprintf("exec replace: %v", execErr)), nil
		}
		if result.ExitCode != 0 {
			return toolError(fmt.Sprintf("replace failed (exit %d): %s", result.ExitCode, strings.TrimSpace(result.Output))), nil
		}

		// Parse the count from the script's stdout line "replaced:N".
		count := -1
		for _, line := range strings.Split(result.Output, "\n") {
			if strings.HasPrefix(line, "replaced:") {
				fmt.Sscanf(strings.TrimPrefix(line, "replaced:"), "%d", &count)
			}
		}

		return toolJSON(map[string]any{
			"server_id": serverID,
			"path":      path,
			"replaced":  count,
		})
	}
}

// buildReplaceScript returns a Python 3 script that replaces old with new in
// the file at path. countMode is "-1" for all occurrences, "1" for first only.
// Strings are encoded as hex escapes so the script is safe to upload as-is
// regardless of what characters they contain.
func buildReplaceScript(path, old, new_, countMode string) string {
	return fmt.Sprintf(`import sys
old = bytes.fromhex(%q).decode('utf-8', errors='surrogateescape')
new = bytes.fromhex(%q).decode('utf-8', errors='surrogateescape')
path = %q
count = %s
try:
    with open(path, 'r', encoding='utf-8', errors='surrogateescape') as f:
        content = f.read()
    if old not in content:
        print('replaced:0')
        sys.exit(0)
    updated = content.replace(old, new, count)
    with open(path, 'w', encoding='utf-8', errors='surrogateescape') as f:
        f.write(updated)
    n = content.count(old) if count == -1 else 1
    print('replaced:' + str(n))
except Exception as e:
    print('error: ' + str(e), file=sys.stderr)
    sys.exit(1)
`, fmt.Sprintf("%x", []byte(old)), fmt.Sprintf("%x", []byte(new_)), path, countMode)
}
