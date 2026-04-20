package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"text/tabwriter"
	"time"

	"github.com/emusal/alogin2/internal/db"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// newSessionBgExecCmd returns "alogin ssh session bg-exec <session-id> <cmd>".
// Creates a job record and spawns a detached background process that runs
// "alogin ssh session job-run <job-id>" so the job outlives this process.
func newSessionBgExecCmd() *cobra.Command {
	var timeoutSec int
	cmd := &cobra.Command{
		Use:   "bg-exec <session-id> <command>",
		Short: "Run a command in the background and return a job ID immediately",
		Long: `Execute a command on the remote server without waiting for it to finish.
Prints the job ID to stdout immediately. Use "job status" and "job logs" to track progress.

Examples:
  id=$(alogin ssh session start web-01)
  job=$(alogin ssh session bg-exec "$id" "apt-get upgrade -y")
  alogin ssh session job status "$job"
  alogin ssh session job logs "$job"`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			sessionID, command := args[0], args[1]

			var meta sessionMeta
			if data, err := os.ReadFile(sessionMetaFile(sessionID)); err != nil {
				return fmt.Errorf("session %q not found or not started", sessionID)
			} else if err := json.Unmarshal(data, &meta); err != nil {
				return fmt.Errorf("read session meta: %w", err)
			}

			jobID := uuid.New().String()
			job := &db.BGJob{
				ID:         jobID,
				SessionID:  sessionID,
				ServerHost: meta.Host,
				Command:    command,
				CreatedAt:  time.Now(),
			}
			ctx := context.Background()
			if err := database.Jobs.Create(ctx, job); err != nil {
				return fmt.Errorf("create job: %w", err)
			}

			binPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve binary: %w", err)
			}

			// Spawn a detached background process that runs job-run.
			if out, err := jobSpawn(jobID, binPath, timeoutSec); err != nil {
				_ = database.Jobs.SetFinished(ctx, jobID, db.JobFailed, 1, time.Now())
				_ = database.Jobs.AppendOutput(ctx, jobID, "[error: spawn job: "+out+"]\n")
				return err
			}

			fmt.Println(jobID)
			return nil
		},
	}
	cmd.Flags().IntVar(&timeoutSec, "timeout", 3600, "maximum execution time in seconds (0 = no limit)")
	return cmd
}

// newSessionJobRunCmd is the hidden long-running command executed inside tmux.
// It reads the job record from DB, opens a fresh SSH connection, runs the command,
// and writes all output + final status back to the DB.
func newSessionJobRunCmd() *cobra.Command {
	var timeoutSec int
	cmd := &cobra.Command{
		Use:    "job-run <job-id>",
		Short:  "Execute a background job (called internally by tmux)",
		Args:   cobra.ExactArgs(1),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			jobID := args[0]
			ctx, cancel := signal.NotifyContext(context.Background(), shutdownSignals...)
			defer cancel()

			j, err := database.Jobs.Get(ctx, jobID)
			if err != nil || j == nil {
				return fmt.Errorf("job %s not found", jobID)
			}

			fail := func(msg string) {
				_ = database.Jobs.AppendOutput(ctx, jobID, "[error: "+msg+"]\n")
				_ = database.Jobs.SetFinished(ctx, jobID, db.JobFailed, 1, time.Now())
			}

			srv, _ := database.Servers.GetByHost(ctx, j.ServerHost, "")
			if srv == nil {
				fail("server " + j.ServerHost + " not found in registry")
				return nil
			}

			hops, err := buildHopChain(ctx, srv, srv.User, "")
			if err != nil {
				fail("build hop chain: " + err.Error())
				return nil
			}
			chain, err := internalssh.DialChain(hops)
			if err != nil {
				fail("dial: " + err.Error())
				return nil
			}
			defer chain.CloseAll()

			managed, err := internalssh.NewManagedSession(chain.Terminal(), true)
			if err != nil {
				fail("managed session: " + err.Error())
				return nil
			}
			defer managed.Close()

			_ = database.Jobs.SetRunning(ctx, jobID, time.Now())

			timeout := time.Duration(timeoutSec) * time.Second
			execCtx := ctx
			if timeout > 0 {
				var tcancel context.CancelFunc
				execCtx, tcancel = context.WithTimeout(ctx, timeout)
				defer tcancel()
			}

			result, execErr := managed.Exec(execCtx, j.Command, timeout)

			status := db.JobDone
			exitCode := result.ExitCode
			if execErr != nil && result.ExitCode == 0 {
				status = db.JobFailed
				exitCode = 1
			}

			_ = database.Jobs.AppendOutput(ctx, jobID, result.Output)
			_ = database.Jobs.SetFinished(ctx, jobID, status, exitCode, time.Now())

			jobKill(jobID)
			return nil
		},
	}
	cmd.Flags().IntVar(&timeoutSec, "timeout", 3600, "maximum execution time in seconds")
	return cmd
}

// shortID safely returns up to the first 8 characters of an ID.
func shortID(id string) string {
	if len(id) > 8 {
		return id[:8]
	}
	return id
}

// resolveJobID resolves a full UUID or prefix to a full job ID, printing a
// helpful error when ambiguous or not found.
func resolveJobID(ctx context.Context, prefix string) (string, error) {
	id, err := database.Jobs.ResolveID(ctx, prefix)
	if err != nil {
		return "", err
	}
	return id, nil
}

// newSessionJobCmd returns the "job" subcommand group.
func newSessionJobCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "job",
		Short: "Inspect and manage background jobs",
	}
	cmd.AddCommand(
		newSessionJobStatusCmd(),
		newSessionJobLogsCmd(),
		newSessionJobListCmd(),
		newSessionJobCancelCmd(),
		newSessionJobDeleteCmd(),
		newSessionJobPurgeCmd(),
	)
	return cmd
}

func newSessionJobStatusCmd() *cobra.Command {
	var jsonOut bool
	cmd := &cobra.Command{
		Use:   "status <job-id>",
		Short: "Show the status of a background job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			id, err := resolveJobID(ctx, args[0])
			if err != nil {
				return err
			}
			j, err := database.Jobs.Get(ctx, id)
			if err != nil {
				return err
			}
			if j == nil {
				return fmt.Errorf("job %s not found", args[0])
			}
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(j)
			}
			printJobSummary(j)
			return nil
		},
	}
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

func newSessionJobLogsCmd() *cobra.Command {
	var follow bool
	cmd := &cobra.Command{
		Use:   "logs <job-id>",
		Short: "Print the captured output of a background job",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			id, err := resolveJobID(ctx, args[0])
			if err != nil {
				return err
			}

			if !follow {
				j, err := database.Jobs.Get(ctx, id)
				if err != nil {
					return err
				}
				if j == nil {
					return fmt.Errorf("job %s not found", args[0])
				}
				fmt.Print(j.Output)
				return nil
			}

			// --follow: poll DB output incrementally until job finishes.
			printed := 0
			for {
				j, err := database.Jobs.Get(ctx, id)
				if err != nil {
					return err
				}
				if j == nil {
					return fmt.Errorf("job %s not found", args[0])
				}
				if len(j.Output) > printed {
					fmt.Print(j.Output[printed:])
					printed = len(j.Output)
				}
				if j.Status != db.JobPending && j.Status != db.JobRunning {
					fmt.Fprintf(os.Stderr, "\n[job %s: %s", shortID(id), j.Status)
					if j.ExitCode != nil {
						fmt.Fprintf(os.Stderr, ", exit %d", *j.ExitCode)
					}
					fmt.Fprintln(os.Stderr, "]")
					return nil
				}
				time.Sleep(500 * time.Millisecond)
			}
		},
	}
	cmd.Flags().BoolVarP(&follow, "follow", "f", false, "Stream output until the job finishes")
	return cmd
}

func newSessionJobListCmd() *cobra.Command {
	var (
		sessionID string
		jsonOut   bool
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List background jobs",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			jobs, err := database.Jobs.List(ctx, sessionID)
			if err != nil {
				return err
			}
			if len(jobs) == 0 {
				fmt.Println("No background jobs.")
				return nil
			}
			if jsonOut {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(jobs)
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "JOB ID\tSESSION\tHOST\tSTATUS\tCOMMAND\tCREATED")
			for _, j := range jobs {
				cmdShort := j.Command
				if len(cmdShort) > 40 {
					cmdShort = cmdShort[:37] + "..."
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
					shortID(j.ID),
					shortID(j.SessionID),
					j.ServerHost,
					j.Status,
					cmdShort,
					j.CreatedAt.Format(time.RFC3339),
				)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&sessionID, "session", "", "Filter by session ID")
	cmd.Flags().BoolVar(&jsonOut, "json", false, "Output as JSON")
	return cmd
}

func newSessionJobCancelCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "cancel <job-id>",
		Short: "Mark a pending or running job as cancelled",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			id, err := resolveJobID(ctx, args[0])
			if err != nil {
				return err
			}
			if err := database.Jobs.Cancel(ctx, id); err != nil {
				return err
			}
			fmt.Printf("Job %s cancelled.\n", shortID(id))
			return nil
		},
	}
}

func newSessionJobDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <job-id>",
		Short: "Delete a job record regardless of status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			id, err := resolveJobID(ctx, args[0])
			if err != nil {
				return err
			}
			if err := database.Jobs.Delete(ctx, id); err != nil {
				return err
			}
			fmt.Printf("Job %s deleted.\n", shortID(id))
			return nil
		},
	}
}

func newSessionJobPurgeCmd() *cobra.Command {
	var all bool
	cmd := &cobra.Command{
		Use:   "purge",
		Short: "Delete finished jobs (done/failed/cancelled); --all deletes every job",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			n, err := database.Jobs.Purge(ctx, all)
			if err != nil {
				return err
			}
			if n == 0 {
				fmt.Println("No jobs deleted.")
			} else {
				fmt.Printf("Deleted %d job(s).\n", n)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "Delete all jobs including pending and running")
	return cmd
}

func printJobSummary(j *db.BGJob) {
	fmt.Printf("Job ID   : %s\n", j.ID)
	fmt.Printf("Session  : %s\n", j.SessionID)
	fmt.Printf("Host     : %s\n", j.ServerHost)
	fmt.Printf("Status   : %s\n", j.Status)
	fmt.Printf("Command  : %s\n", j.Command)
	fmt.Printf("Created  : %s\n", j.CreatedAt.Format(time.RFC3339))
	if j.StartedAt != nil {
		fmt.Printf("Started  : %s\n", j.StartedAt.Format(time.RFC3339))
	}
	if j.FinishedAt != nil {
		fmt.Printf("Finished : %s\n", j.FinishedAt.Format(time.RFC3339))
	}
	if j.ExitCode != nil {
		fmt.Printf("Exit code: %d\n", *j.ExitCode)
	}
}
