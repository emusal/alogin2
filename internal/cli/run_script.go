package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	internalssh "github.com/emusal/alogin2/internal/ssh"
	"github.com/google/uuid"
	"github.com/spf13/cobra"
)

// newRunScriptCmd returns "alogin ssh run-script --remote [user@]host".
// Reads script content from --content or stdin, uploads it to a temp path via
// SFTP, executes it, then deletes it — mirroring the MCP run_script tool.
func newRunScriptCmd() *cobra.Command {
	var (
		remoteTarget string
		content      string
		interpreter  string
		timeoutSec   int
		loginShell   bool
		forceUTF8    bool
	)

	cmd := &cobra.Command{
		Use:   "run-script --remote [user@]host",
		Short: "Upload script content to a remote host, execute it, and delete it",
		Long: `Upload a script given as a string (--content) or via stdin to a remote host,
execute it, then delete the temp file.

Unlike run-local (which takes a local file path), run-script takes the script content
directly — useful for dynamically generated scripts or piped input.

Examples:
  echo "df -h" | alogin ssh run-script --remote web-01
  alogin ssh run-script --remote web-01 --content "#!/bin/bash\ndf -h\nuptime"
  alogin ssh run-script --remote web-01 --interpreter python3 --content "import sys; print(sys.version)"
  alogin ssh run-script --remote web-01 --login-shell --content "node --version"`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if remoteTarget == "" {
				return fmt.Errorf("--remote is required")
			}

			var scriptBytes []byte
			if content != "" {
				// Unescape \n sequences passed via --content on the command line.
				scriptBytes = []byte(strings.ReplaceAll(content, `\n`, "\n"))
			} else {
				var err error
				scriptBytes, err = io.ReadAll(os.Stdin)
				if err != nil {
					return fmt.Errorf("read stdin: %w", err)
				}
			}
			if len(strings.TrimSpace(string(scriptBytes))) == 0 {
				return fmt.Errorf("script content is empty; use --content or pipe to stdin")
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

			if interpreter == "" {
				interpreter = detectInterpreter("script.sh", scriptBytes)
			}

			ext := extensionForInterpreter(interpreter)
			remoteTmp := fmt.Sprintf("/tmp/alogin-%s%s", uuid.New().String(), ext)

			sc, err := chain.Terminal().SFTPClient()
			if err != nil {
				return fmt.Errorf("sftp client: %w", err)
			}
			if err := sc.UploadContent(scriptBytes, remoteTmp); err != nil {
				sc.Close()
				return fmt.Errorf("upload: %w", err)
			}
			sc.Close()

			runCmd := fmt.Sprintf("%s %s; _rc=$?; rm -f %s; (exit $_rc)", interpreter, remoteTmp, remoteTmp)
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
	cmd.Flags().StringVar(&content, "content", "", "Script content to execute (alternative to stdin)")
	cmd.Flags().StringVar(&interpreter, "interpreter", "", "Interpreter (default: auto-detected from shebang; falls back to bash)")
	cmd.Flags().IntVar(&timeoutSec, "timeout", 120, "Idle timeout in seconds")
	cmd.Flags().BoolVar(&loginShell, "login-shell", true, "Run via a login shell (bash -l) so ~/.bash_profile is sourced (default true; use --login-shell=false for a pristine environment)")
	cmd.Flags().BoolVar(&forceUTF8, "force-utf8", false, "Force LANG=en_US.UTF-8 LC_ALL=en_US.UTF-8 (prevents garbled multi-byte output)")
	_ = cmd.MarkFlagRequired("remote")
	return cmd
}

// extensionForInterpreter maps common interpreters to file extensions so the
// remote temp file has a recognisable suffix (some tools inspect it).
func extensionForInterpreter(interp string) string {
	switch filepath.Base(interp) {
	case "python3", "python":
		return ".py"
	case "ruby":
		return ".rb"
	case "node":
		return ".js"
	case "perl":
		return ".pl"
	default:
		return ".sh"
	}
}
