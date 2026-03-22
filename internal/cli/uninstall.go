package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/emusal/alogin2/internal/config"
	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	var purge bool
	var yes bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove alogin binary, completions, and config",
		Long: `Remove the alogin binary, shell completions, and configuration directory.

By default the database and vault (~/.local/share/alogin/) are preserved so
you can reinstall and continue where you left off.

Use --purge to also delete all data (database, vault, logs). This is irreversible.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load()
			if err != nil {
				return err
			}

			binPath, err := os.Executable()
			if err != nil {
				return fmt.Errorf("cannot determine binary path: %w", err)
			}

			completionsDir := filepath.Join(cfg.DataDir, "completions")

			fmt.Println("The following will be removed:")
			fmt.Printf("  Binary      : %s\n", binPath)
			fmt.Printf("  Config      : %s\n", cfg.ConfigDir)
			fmt.Printf("  Completions : %s\n", completionsDir)
			if purge {
				fmt.Printf("  Data dir    : %s  *** includes database and vault ***\n", cfg.DataDir)
			} else {
				fmt.Printf("  Data dir    : (kept — use --purge to also remove)\n")
			}
			fmt.Println()

			if !yes {
				fmt.Print("Continue? [y/N] ")
				scanner := bufio.NewScanner(os.Stdin)
				scanner.Scan()
				if strings.ToLower(strings.TrimSpace(scanner.Text())) != "y" {
					fmt.Println("Aborted.")
					return nil
				}
			}

			if purge {
				if err := removeIfExists(cfg.DataDir); err != nil {
					fmt.Fprintf(os.Stderr, "warning: %v\n", err)
				}
			} else {
				if err := removeIfExists(completionsDir); err != nil {
					fmt.Fprintf(os.Stderr, "warning: %v\n", err)
				}
			}

			if err := removeIfExists(cfg.ConfigDir); err != nil {
				fmt.Fprintf(os.Stderr, "warning: %v\n", err)
			}

			fmt.Printf("Removing %s ...\n", binPath)
			if err := os.Remove(binPath); err != nil {
				return fmt.Errorf("remove binary: %w", err)
			}

			fmt.Println("alogin uninstalled.")
			return nil
		},
	}

	cmd.Flags().BoolVar(&purge, "purge", false, "also remove database and vault (irreversible)")
	cmd.Flags().BoolVarP(&yes, "yes", "y", false, "skip confirmation prompt")
	return cmd
}

func removeIfExists(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	return os.RemoveAll(path)
}
