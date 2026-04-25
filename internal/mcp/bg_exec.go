package mcp

import (
	"context"
	"fmt"
	"time"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/policy"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/google/uuid"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// RegisterBgExecTools registers the bg_exec_command and job_* MCP tools on srv.
// Background jobs run as goroutines inside the MCP server process (no tmux needed),
// writing output and status into the shared bg_jobs DB table.
func RegisterBgExecTools(srv *server.MCPServer, d Deps) {
	// --- bg_exec_command ---
	srv.AddTool(mcpgo.NewTool("bg_exec_command",
		mcpgo.WithDescription(`Execute a long-running SSH command in the background and return a job ID immediately.
Use this instead of exec_command or remote_shell when a command may exceed the timeout, or when you want
to launch multiple jobs in parallel and poll later.
Poll with job_status; stream output with job_logs (follow=true).`),
		mcpgo.WithString("server_id", mcpgo.Description("Server ID (from list_servers)"), mcpgo.Required()),
		mcpgo.WithString("command", mcpgo.Description("Command to execute on the remote server"), mcpgo.Required()),
		mcpgo.WithNumber("timeout_sec", mcpgo.Description("Maximum execution time in seconds (default 3600, 0 = no limit)")),
		mcpgo.WithString("agent_id", mcpgo.Description("Optional: identifier for the calling agent")),
		mcpgo.WithString("intent", mcpgo.Description("Optional: human-readable intent (logged to audit trail)")),
	), newBgExecCommandHandler(d))

	// --- job_status ---
	srv.AddTool(mcpgo.NewTool("job_status",
		mcpgo.WithDescription("Get the current status of a background job (pending/running/done/failed/cancelled)."),
		mcpgo.WithString("job_id", mcpgo.Description("Full job UUID or a unique prefix (≥8 chars)"), mcpgo.Required()),
	), newJobStatusHandler(d))

	// --- job_logs ---
	srv.AddTool(mcpgo.NewTool("job_logs",
		mcpgo.WithDescription(`Fetch captured output of a background job.
When follow=true, streams output incrementally until the job finishes (polls every 500ms).`),
		mcpgo.WithString("job_id", mcpgo.Description("Full job UUID or a unique prefix (≥8 chars)"), mcpgo.Required()),
		mcpgo.WithBoolean("follow", mcpgo.Description("Stream output until job completes. Default false.")),
	), newJobLogsHandler(d))

	// --- job_list ---
	srv.AddTool(mcpgo.NewTool("job_list",
		mcpgo.WithDescription("List background jobs, optionally filtered by server host."),
		mcpgo.WithString("server_host", mcpgo.Description("Optional: filter by server hostname")),
	), newJobListHandler(d))

	// --- job_cancel ---
	srv.AddTool(mcpgo.NewTool("job_cancel",
		mcpgo.WithDescription("Cancel a pending or running background job."),
		mcpgo.WithString("job_id", mcpgo.Description("Full job UUID or a unique prefix (≥8 chars)"), mcpgo.Required()),
	), newJobCancelHandler(d))
}

func newBgExecCommandHandler(d Deps) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		serverID, err := parseID(req, "server_id")
		if err != nil {
			return toolError(err.Error()), nil
		}
		command, _ := args["command"].(string)
		if command == "" {
			return toolError("command is required"), nil
		}
		timeoutSec, _ := args["timeout_sec"].(float64)
		if timeoutSec <= 0 {
			timeoutSec = 3600
		}
		agentID, _ := args["agent_id"].(string)
		intent, _ := args["intent"].(string)

		srv, err := d.DB.Servers.GetByID(ctx, serverID)
		if err != nil || srv == nil {
			return toolError(fmt.Sprintf("server %d not found", serverID)), nil
		}

		ev := auditEvent{
			Event:      "bg_exec_command",
			AgentID:    agentID,
			ServerID:   serverID,
			ServerHost: srv.Host,
			Commands:   []string{command},
			Intent:     intent,
			TimeoutSec: int(timeoutSec),
		}
		writeAudit(d.AuditLog, ev)
		writeAuditDB(ctx, d, ev)

		eng, err := policy.ResolveFor(d.Policy, srv.PolicyYAML)
		if err != nil {
			return toolError(fmt.Sprintf("per-server policy error: %v", err)), nil
		}
		if blocked := checkPolicy(ctx, d, eng, policy.CheckRequest{
			AgentID:  agentID,
			Commands: []string{command},
			ServerID: serverID,
		}); blocked != nil {
			return blocked, nil
		}

		jobID := uuid.New().String()
		job := &db.BGJob{
			ID:         jobID,
			SessionID:  "mcp",
			ServerHost: srv.Host,
			Command:    command,
			CreatedAt:  time.Now(),
		}
		if err := d.DB.Jobs.Create(ctx, job); err != nil {
			return toolError(fmt.Sprintf("create job: %v", err)), nil
		}

		// Run in background goroutine — MCP server is a persistent process.
		go runBgJob(d, jobID, serverID, command, time.Duration(timeoutSec)*time.Second)

		return toolJSON(map[string]any{
			"job_id":      jobID,
			"status":      "pending",
			"server_id":   serverID,
			"server_host": srv.Host,
		})
	}
}

// runBgJob is the goroutine that executes a background job and writes results to the DB.
func runBgJob(d Deps, jobID string, serverID int64, command string, timeout time.Duration) {
	ctx := context.Background()

	fail := func(msg string) {
		_ = d.DB.Jobs.AppendOutput(ctx, jobID, "[error: "+msg+"]\n")
		_ = d.DB.Jobs.SetFinished(ctx, jobID, db.JobFailed, 1, time.Now())
	}

	srv, err := d.DB.Servers.GetByID(ctx, serverID)
	if err != nil || srv == nil {
		fail(fmt.Sprintf("server %d not found", serverID))
		return
	}

	hops, err := buildHopChain(ctx, d.DB, d.Vault, srv)
	if err != nil {
		fail("build hop chain: " + err.Error())
		return
	}
	chain, err := internalssh.DialChain(hops)
	if err != nil {
		fail("dial: " + err.Error())
		return
	}
	defer chain.CloseAll()

	managed, err := internalssh.NewManagedSession(chain.Terminal(), true)
	if err != nil {
		fail("managed session: " + err.Error())
		return
	}
	defer managed.Close()

	_ = d.DB.Jobs.SetRunning(ctx, jobID, time.Now())

	execCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	result, execErr := managed.Exec(execCtx, command, timeout)

	status := db.JobDone
	exitCode := result.ExitCode
	if execErr != nil && result.ExitCode == 0 {
		status = db.JobFailed
		exitCode = 1
	}

	_ = d.DB.Jobs.AppendOutput(ctx, jobID, result.Output)
	_ = d.DB.Jobs.SetFinished(ctx, jobID, status, exitCode, time.Now())
}

func newJobStatusHandler(d Deps) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		jobID, _ := args["job_id"].(string)
		if jobID == "" {
			return toolError("job_id is required"), nil
		}
		id, err := d.DB.Jobs.ResolveID(ctx, jobID)
		if err != nil {
			return toolError(err.Error()), nil
		}
		j, err := d.DB.Jobs.Get(ctx, id)
		if err != nil || j == nil {
			return toolError(fmt.Sprintf("job %s not found", jobID)), nil
		}
		return toolJSON(j)
	}
}

func newJobLogsHandler(d Deps) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		jobID, _ := args["job_id"].(string)
		if jobID == "" {
			return toolError("job_id is required"), nil
		}
		follow, _ := args["follow"].(bool)

		id, err := d.DB.Jobs.ResolveID(ctx, jobID)
		if err != nil {
			return toolError(err.Error()), nil
		}

		if !follow {
			j, err := d.DB.Jobs.Get(ctx, id)
			if err != nil || j == nil {
				return toolError(fmt.Sprintf("job %s not found", jobID)), nil
			}
			return toolJSON(map[string]any{
				"job_id": id,
				"status": j.Status,
				"output": j.Output,
			})
		}

		// follow mode: poll until done, accumulate output.
		var accumulated string
		for {
			j, err := d.DB.Jobs.Get(ctx, id)
			if err != nil || j == nil {
				return toolError(fmt.Sprintf("job %s not found", jobID)), nil
			}
			accumulated = j.Output
			if j.Status != db.JobPending && j.Status != db.JobRunning {
				exitCode := 0
				if j.ExitCode != nil {
					exitCode = *j.ExitCode
				}
				return toolJSON(map[string]any{
					"job_id":    id,
					"status":    j.Status,
					"exit_code": exitCode,
					"output":    accumulated,
				})
			}
			select {
			case <-ctx.Done():
				return toolJSON(map[string]any{
					"job_id": id,
					"status": j.Status,
					"output": accumulated,
					"note":   "follow interrupted by context cancellation",
				})
			case <-time.After(500 * time.Millisecond):
			}
		}
	}
}

func newJobListHandler(d Deps) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		serverHost, _ := args["server_host"].(string)

		jobs, err := d.DB.Jobs.List(ctx, "")
		if err != nil {
			return toolError(fmt.Sprintf("list jobs: %v", err)), nil
		}

		if serverHost != "" {
			filtered := jobs[:0]
			for _, j := range jobs {
				if j.ServerHost == serverHost {
					filtered = append(filtered, j)
				}
			}
			jobs = filtered
		}

		return toolJSON(jobs)
	}
}

func newJobCancelHandler(d Deps) func(context.Context, mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
	return func(ctx context.Context, req mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
		args := req.GetArguments()
		jobID, _ := args["job_id"].(string)
		if jobID == "" {
			return toolError("job_id is required"), nil
		}
		id, err := d.DB.Jobs.ResolveID(ctx, jobID)
		if err != nil {
			return toolError(err.Error()), nil
		}
		if err := d.DB.Jobs.Cancel(ctx, id); err != nil {
			return toolError(err.Error()), nil
		}
		return toolJSON(map[string]any{
			"job_id": id,
			"status": "cancelled",
		})
	}
}
