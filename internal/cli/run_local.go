package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// newRunLocalCmd returns "alogin ssh run-local <local-script> --remote [user@]host".
// It uploads the local file via SFTP to a temp path, executes it, then deletes it —
// eliminating the manual scp push → exec → rm dance.
func newRunLocalCmd() *cobra.Command {
	var (
		remoteTarget string
		interpreter  string
		args         []string
		timeoutSec   int
		loginShell   bool
		forceUTF8    bool
		keepRemote   bool
	)

	cmd := &cobra.Command{
		Use:   "run-local <script> --remote [user@]host",
		Short: "Upload a local script to a remote host, execute it, and delete it",
		Long: `Upload a local script via SFTP, execute it on the remote host, then delete the temp file.

Eliminates the manual: scp push → session exec → rm workflow.

Examples:
  alogin ssh run-local ./deploy.sh --remote web-01
  alogin ssh run-local ./check.py --remote admin@web-01 --interpreter python3
  alogin ssh run-local ./setup.sh --remote web-01 --login-shell
  alogin ssh run-local ./build.sh --remote web-01 --timeout 600 -- --release v1.2`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, positional []string) error {
			if remoteTarget == "" {
				return fmt.Errorf("--remote is required")
			}
			localPath := positional[0]

			content, err := os.ReadFile(localPath)
			if err != nil {
				return fmt.Errorf("read local file: %w", err)
			}

			ctx := context.Background()
			user, host := parseUserHost(remoteTarget)

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
				return fmt.Errorf("dial: %w", err)
			}
			defer chain.CloseAll()

			// Upload to a unique temp path so parallel runs never collide.
			ext := filepath.Ext(localPath)
			if ext == "" {
				ext = ".sh"
			}
			remoteTmp := fmt.Sprintf("/tmp/alogin-%s%s", uuid.New().String(), ext)

			sc, err := chain.Terminal().SFTPClient()
			if err != nil {
				return fmt.Errorf("sftp client: %w", err)
			}
			if err := sc.UploadContent(content, remoteTmp); err != nil {
				sc.Close()
				return fmt.Errorf("upload: %w", err)
			}
			sc.Close()

			// Build the remote execution command.
			if interpreter == "" {
				interpreter = detectInterpreter(localPath, content)
			}
			runCmd := interpreter + " " + remoteTmp
			if len(args) > 0 {
				runCmd += " " + strings.Join(args, " ")
			}
			if !keepRemote {
				// Capture exit code, delete temp file, then restore $? so the
				// sentinel printf in ManagedSession.Exec reads the script's exit
				// code — NOT rm's. Never use `exit` here: that would kill the
				// persistent bash process and cause an EOF on the stdout pipe.
				runCmd = fmt.Sprintf("%s; _rc=$?; rm -f %s; (exit $_rc)", runCmd, remoteTmp)
			}
			if forceUTF8 {
				runCmd = withUTF8(runCmd)
			}

			managed, err := internalssh.NewManagedSession(chain.Terminal(), loginShell)
			if err != nil {
				return fmt.Errorf("managed session: %w", err)
			}
			defer managed.Close()

			timeout := time.Duration(timeoutSec) * time.Second
			result, execErr := managed.Exec(ctx, runCmd, timeout)

			// Write only the remote command's output to stdout so callers
			// can parse it cleanly. Errors go to stderr.
			fmt.Print(result.Output)

			if execErr != nil {
				fmt.Fprintf(os.Stderr, "error: %v\n", execErr)
				os.Exit(1)
			}
			if result.ExitCode != 0 {
				os.Exit(result.ExitCode)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&remoteTarget, "remote", "", "[user@]host to run the script on (required)")
	cmd.Flags().StringVar(&interpreter, "interpreter", "", "Interpreter to run the script with (default: auto-detected from shebang or extension)")
	cmd.Flags().StringArrayVar(&args, "arg", nil, "Extra arguments to pass to the script (repeatable)")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 120, "Idle timeout in seconds")
	cmd.Flags().BoolVar(&loginShell, "login-shell", true, "Run via a login shell (bash -l) so ~/.bash_profile is sourced (default true; use --login-shell=false for a pristine environment)")
	cmd.Flags().BoolVar(&forceUTF8, "force-utf8", false, "Force LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 (prevents garbled multi-byte output)")
	cmd.Flags().BoolVar(&keepRemote, "keep", false, "Do not delete the uploaded script after execution")
	_ = cmd.MarkFlagRequired("remote")
	return cmd
}

// detectInterpreter picks an interpreter from the shebang line or file extension.
// Falls back to "bash" when nothing matches.
func detectInterpreter(localPath string, content []byte) string {
	// Honour shebang if present.
	if len(content) > 2 && content[0] == '#' && content[1] == '!' {
		end := strings.IndexByte(string(content), '\n')
		if end < 0 {
			end = len(content)
		}
		shebang := strings.TrimSpace(string(content[2:end]))
		// Strip /usr/bin/env prefix.
		if after, ok := strings.CutPrefix(shebang, "/usr/bin/env "); ok {
			return strings.Fields(after)[0]
		}
		return filepath.Base(shebang)
	}
	switch strings.ToLower(filepath.Ext(localPath)) {
	case ".py":
		return "python3"
	case ".rb":
		return "ruby"
	case ".js":
		return "node"
	case ".pl":
		return "perl"
	}
	return "bash"
}
