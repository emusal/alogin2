package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"

	"github.com/google/uuid"
	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/spf13/cobra"
)

// Session layout under <DataDir>/sessions/<id>/:
//   sock  — Unix domain socket (run listens, exec connects)
//   meta  — JSON {"host": "..."}
func sessionDir(id string) string        { return filepath.Join(cfg.DataDir, "sessions", id) }
func sessionSock(id string) string       { return filepath.Join(sessionDir(id), "sock") }
func sessionMetaFile(id string) string   { return filepath.Join(sessionDir(id), "meta") }

const sessionTmuxPrefix = "alogin-session-"

func sessionTmuxName(id string) string { return sessionTmuxPrefix + id }

func sessionIsRunning(id string) bool {
	return exec.Command("tmux", "has-session", "-t", sessionTmuxName(id)).Run() == nil
}

type sessionMeta struct {
	Host string `json:"host"`
}

// Wire protocol (newline-delimited, over Unix socket):
//   exec → run : <command>\n
//   run  → exec : <output lines...> __ALOGIN_EXIT_<N>\n
// Each exec call opens a new connection; run accepts one connection at a time.

func newSessionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage persistent stateful SSH sessions",
		Long: `Manage stateful SSH sessions that preserve working directory and environment.

A session holds a single bash process on the remote server backed by a tmux
session, so cwd, environment variables, and shell state persist across
separate alogin invocations.

Examples:
  id=$(alogin ssh session start web-01)
  alogin ssh session exec "$id" "cd /var/log"
  alogin ssh session exec "$id" "pwd"    # outputs /var/log
  alogin ssh session list
  alogin ssh session stop "$id"`,
	}
	cmd.AddCommand(
		newSessionStartCmd(),
		newSessionExecCmd(),
		newSessionStopCmd(),
		newSessionListCmd(),
		newSessionRunCmd(), // hidden: called by tmux
	)
	return cmd
}

func newSessionStartCmd() *cobra.Command {
	var idFlag string
	cmd := &cobra.Command{
		Use:   "start [user@]host",
		Short: "Start a new stateful session and print its ID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			user, host := parseUserHost(args[0])

			srv, _ := database.Servers.GetByHost(ctx, host, user)
			if srv == nil {
				return fmt.Errorf("server %s not found in registry", host)
			}

			id := idFlag
			if id == "" {
				id = uuid.New().String()
			}
			if sessionIsRunning(id) {
				return fmt.Errorf("session %q is already running", id)
			}

			// Prepare session directory.
			dir := sessionDir(id)
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("create session dir: %w", err)
			}
			_ = os.Remove(sessionSock(id))

			// Write meta for list.
			metaData, _ := json.Marshal(sessionMeta{Host: srv.Host})
			if err := os.WriteFile(sessionMetaFile(id), metaData, 0600); err != nil {
				return fmt.Errorf("write meta: %w", err)
			}

			// Build host arg for the run subcommand.
			hostArg := srv.Host
			if user != "" {
				hostArg = user + "@" + srv.Host
			} else if srv.User != "" {
				hostArg = srv.User + "@" + srv.Host
			}

			binPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("resolve binary: %w", err)
			}

			// Spawn detached tmux session.
			tmuxName := sessionTmuxName(id)
			c := exec.Command("tmux", "new-session", "-d", "-s", tmuxName,
				binPath, "ssh", "session", "run", id, hostArg)
			c.Stdout = os.Stderr
			c.Stderr = os.Stderr
			if err := c.Run(); err != nil {
				_ = os.RemoveAll(dir)
				return fmt.Errorf("start tmux session: %w", err)
			}

			// Wait until the socket appears (run is ready to accept).
			deadline := time.Now().Add(15 * time.Second)
			for {
				if time.Now().After(deadline) {
					_ = exec.Command("tmux", "kill-session", "-t", tmuxName).Run()
					_ = os.RemoveAll(dir)
					return fmt.Errorf("timed out waiting for session to become ready")
				}
				if !sessionIsRunning(id) {
					_ = os.RemoveAll(dir)
					return fmt.Errorf("session process exited during startup (check SSH credentials)")
				}
				if _, err := os.Stat(sessionSock(id)); err == nil {
					// Socket exists — try a quick dial to confirm it's accepting.
					conn, err := net.DialTimeout("unix", sessionSock(id), 200*time.Millisecond)
					if err == nil {
						conn.Close()
						break
					}
				}
				time.Sleep(200 * time.Millisecond)
			}

			fmt.Println(id)
			return nil
		},
	}
	cmd.Flags().StringVar(&idFlag, "id", "", "session name (default: generated UUID)")
	return cmd
}

func newSessionExecCmd() *cobra.Command {
	var timeoutSec int
	cmd := &cobra.Command{
		Use:   "exec <session-id> <command>",
		Short: "Execute a command in an existing session",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, command := args[0], args[1]

			if !sessionIsRunning(id) {
				return fmt.Errorf("session %q is not running", id)
			}

			timeout := time.Duration(timeoutSec) * time.Second
			conn, err := net.DialTimeout("unix", sessionSock(id), 5*time.Second)
			if err != nil {
				return fmt.Errorf("connect to session: %w", err)
			}
			defer conn.Close()

			// Send command.
			if _, err := fmt.Fprintln(conn, command); err != nil {
				return fmt.Errorf("send command: %w", err)
			}

			// Read response until sentinel, honouring timeout.
			_ = conn.SetDeadline(time.Now().Add(timeout))
			scanner := bufio.NewScanner(conn)
			exitCode := 0
			for scanner.Scan() {
				line := scanner.Text()
				if strings.HasPrefix(line, "__ALOGIN_EXIT_") {
					fmt.Sscanf(strings.TrimPrefix(line, "__ALOGIN_EXIT_"), "%d", &exitCode)
					break
				}
				fmt.Println(line)
			}
			if err := scanner.Err(); err != nil {
				return fmt.Errorf("read response: %w", err)
			}
			if exitCode != 0 {
				os.Exit(exitCode)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&timeoutSec, "timeout", 30, "command timeout in seconds")
	return cmd
}

func newSessionStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop <session-id>",
		Short: "Stop a running session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			id := args[0]
			running := sessionIsRunning(id)
			if !running {
				_ = os.RemoveAll(sessionDir(id))
				return fmt.Errorf("session %q is not running", id)
			}
			if err := exec.Command("tmux", "kill-session", "-t", sessionTmuxName(id)).Run(); err != nil {
				return fmt.Errorf("kill session: %w", err)
			}
			_ = os.RemoveAll(sessionDir(id))
			fmt.Fprintf(os.Stderr, "session %s stopped\n", id)
			return nil
		},
	}
}

func newSessionListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List active sessions",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out, err := exec.Command("tmux", "ls", "-F", "#{session_name}").Output()
			if err != nil {
				fmt.Println("no active sessions")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "SESSION ID\tHOST")
			found := 0
			for _, name := range strings.Split(strings.TrimSpace(string(out)), "\n") {
				if !strings.HasPrefix(name, sessionTmuxPrefix) {
					continue
				}
				id := strings.TrimPrefix(name, sessionTmuxPrefix)
				host := "(unknown)"
				if data, err := os.ReadFile(sessionMetaFile(id)); err == nil {
					var m sessionMeta
					if json.Unmarshal(data, &m) == nil && m.Host != "" {
						host = m.Host
					}
				}
				fmt.Fprintf(w, "%s\t%s\n", id, host)
				found++
			}
			if found == 0 {
				fmt.Println("no active sessions")
				return nil
			}
			return w.Flush()
		},
	}
}

// newSessionRunCmd is the hidden long-running command executed inside tmux.
// It listens on a Unix socket and serves one exec request at a time,
// routing each command through the persistent ManagedSession.
func newSessionRunCmd() *cobra.Command {
	return &cobra.Command{
		Use:    "run <session-id> [user@]host",
		Short:  "Run session bridge (called internally by tmux)",
		Args:   cobra.ExactArgs(2),
		Hidden: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			id, hostArg := args[0], args[1]
			ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
			defer cancel()

			user, host := parseUserHost(hostArg)
			srv, _ := database.Servers.GetByHost(ctx, host, user)
			if srv == nil {
				return fmt.Errorf("server %s not found in registry", host)
			}
			if user == "" {
				user = srv.User
			}

			hops, err := buildHopChain(ctx, srv, user, "")
			if err != nil {
				return err
			}
			chain, err := internalssh.DialChain(hops)
			if err != nil {
				return err
			}
			defer chain.CloseAll()

			managed, err := internalssh.NewManagedSession(chain.Terminal())
			if err != nil {
				return fmt.Errorf("start managed session: %w", err)
			}
			defer managed.Close()

			// Listen on Unix socket.
			sockPath := sessionSock(id)
			_ = os.Remove(sockPath)
			ln, err := net.Listen("unix", sockPath)
			if err != nil {
				return fmt.Errorf("listen on socket: %w", err)
			}
			defer func() {
				ln.Close()
				os.Remove(sockPath)
			}()
			// Restrict access to current user.
			_ = os.Chmod(sockPath, 0600)

			// Close listener when context is cancelled so Accept unblocks.
			go func() {
				<-ctx.Done()
				ln.Close()
			}()

			for {
				conn, err := ln.Accept()
				if err != nil {
					// Listener closed (context done or stop).
					return nil
				}
				// Handle one request synchronously — sessions are single-threaded
				// by design (bash state is not safe for concurrent access).
				handleSessionConn(ctx, conn, managed)
			}
		},
	}
}

// handleSessionConn reads one command from conn, executes it via managed,
// and writes the output + sentinel back.
func handleSessionConn(ctx context.Context, conn net.Conn, managed *internalssh.ManagedSession) {
	defer conn.Close()

	reader := bufio.NewReader(conn)
	line, err := reader.ReadString('\n')
	if err != nil && err != io.EOF {
		return
	}
	command := strings.TrimRight(line, "\r\n")
	if command == "" {
		// Probe connection from start (readiness check) — just close.
		return
	}

	result, execErr := managed.Exec(ctx, command, 0)
	if execErr != nil {
		fmt.Fprintf(conn, "[error: %v]\n", execErr)
		fmt.Fprintf(conn, "__ALOGIN_EXIT_1\n")
		return
	}
	io.WriteString(conn, result.Output)
	fmt.Fprintf(conn, "__ALOGIN_EXIT_%d\n", result.ExitCode)
}
