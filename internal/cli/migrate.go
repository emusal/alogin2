package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/emusal/alogin2/internal/migrate"
	"github.com/emusal/alogin2/internal/sshconfig"
	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate data into the alogin database",
		Long: `Migrate data into the alogin database from various sources.

Subcommands:
  v1          Import legacy ALOGIN flat-file data (server_list, gateway_list, ...)
  ssh-config  Import servers and gateways from an SSH config file`,
	}
	cmd.AddCommand(newMigrateV1Cmd())
	cmd.AddCommand(newMigrateSSHConfigCmd())
	return cmd
}

func newMigrateV1Cmd() *cobra.Command {
	var from string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "v1",
		Short: "Import legacy ALOGIN v1 data",
		Long: `Import data from the legacy flat-file format (server_list, gateway_list,
alias_hosts, clusters, term_themes) into the v2 SQLite database.

Examples:
  alogin migrate v1 --from ~/.alogin
  alogin migrate v1 --from $ALOGIN_ROOT --verbose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if from == "" && cfg.LegacyRoot != "" {
				from = cfg.LegacyRoot
			}
			if from == "" {
				return fmt.Errorf("specify --from <ALOGIN_ROOT>")
			}

			fmt.Printf("Migrating from: %s\n", from)
			fmt.Printf("Database:       %s\n", cfg.DBPath)

			return migrate.Run(context.Background(), database, migrate.Options{
				LegacyRoot: from,
				Verbose:    verbose,
			})
		},
	}

	cmd.Flags().StringVar(&from, "from", "", "path to legacy ALOGIN_ROOT directory")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "show each imported row")
	return cmd
}

func newMigrateSSHConfigCmd() *cobra.Command {
	var file string
	var dryRun bool
	var verbose bool

	cmd := &cobra.Command{
		Use:   "ssh-config",
		Short: "Import servers and gateways from an SSH config file",
		Long: `Parse ~/.ssh/config (or a specified file) and import Host blocks as
alogin servers. ProxyJump and ProxyCommand entries are converted to alogin
gateway routes.

Existing (host, user) pairs are skipped by default. Use --dry-run to preview
all changes without writing to the database.

Examples:
  alogin migrate ssh-config
  alogin migrate ssh-config --file ~/work/.ssh/config
  alogin migrate ssh-config --dry-run --verbose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()

			if file == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return fmt.Errorf("resolve home dir: %w", err)
				}
				file = filepath.Join(home, ".ssh", "config")
			}

			fmt.Printf("Source:   %s\n", file)
			fmt.Printf("Database: %s\n", cfg.DBPath)
			if dryRun {
				fmt.Println("Mode:     dry-run (no changes will be written)")
			}
			fmt.Println()

			parsed, err := sshconfig.ParseFile(file)
			if err != nil {
				return fmt.Errorf("parse ssh config: %w", err)
			}

			result, err := sshconfig.Import(ctx, database, parsed, sshconfig.ImportOptions{
				DryRun:  dryRun,
				Verbose: verbose,
			})
			if err != nil {
				return fmt.Errorf("import: %w", err)
			}

			fmt.Printf("\nImport complete:\n")
			fmt.Printf("  Servers created:  %d\n", result.ServersCreated)
			fmt.Printf("  Servers skipped:  %d\n", result.ServersSkipped)
			fmt.Printf("  Gateways created: %d\n", result.GatewaysCreated)
			fmt.Printf("  Aliases created:  %d\n", result.AliasesCreated)
			if len(result.Warnings) > 0 {
				fmt.Printf("  Warnings (%d):\n", len(result.Warnings))
				for _, w := range result.Warnings {
					fmt.Printf("    - %s\n", w)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&file, "file", "", "SSH config file path (default: ~/.ssh/config)")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "show planned changes without writing to database")
	cmd.Flags().BoolVarP(&verbose, "verbose", "v", false, "print each imported/skipped entry")
	return cmd
}
