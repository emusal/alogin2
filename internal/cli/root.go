package cli

import (
	"fmt"
	"os"

	"github.com/emusal/alogin2/internal/config"
	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/vault"
	"github.com/spf13/cobra"
)

// printMigrationNotice writes a human-readable report of applied migrations to stderr.
// Called from initRuntime and from the db-migrate command's RunE.
func printMigrationNotice(applied []int) {
	if len(applied) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "Database schema migrated:\n")
	for _, v := range applied {
		desc := db.MigrationDescription(v)
		if desc != "" {
			fmt.Fprintf(os.Stderr, "  v%d  %s\n", v, desc)
		} else {
			fmt.Fprintf(os.Stderr, "  v%d\n", v)
		}
	}
	fmt.Fprintf(os.Stderr, "Schema is now at v%d.\n", applied[len(applied)-1])
}

var (
	cfg      *config.Config
	database *db.DB
	vlt      vault.Vault
)

// NewRootCmd builds and returns the root cobra command.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:   "alogin",
		Short: "Modern SSH connection manager",
		Long: `alogin — SSH automation tool for system administrators.

Manages SSH connections, SFTP transfers, port tunnels, cluster sessions,
and server credentials with an encrypted vault.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip DB init for version/completion commands
			skip := map[string]bool{"version": true, "completion": true, "shell-init": true, "uninstall": true, "upgrade": true}
			if skip[cmd.Name()] {
				return nil
			}
			return initRuntime()
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if database != nil {
				return database.Close()
			}
			return nil
		},
	}

	root.AddCommand(
		newConnectCmd(),
		newSFTPCmd(),
		newFTPCmd(),
		newMountCmd(),
		newClusterCmd(),
		newServerCmd(),
		newGatewayCmd(),
		newAliasCmd(),
		newHostsCmd(),
		newTunnelCmd(),
		newMigrateCmd(),
		newVersionCmd(),
		newShellInitCmd(),
		newTUICmd(),
		newCompletionCmd(),
		newWebCmd(),
		newUninstallCmd(),
		newUpgradeCmd(),
		newMCPServerCmd(),
		newDBMigrateCmd(),
	)

	return root
}

// Execute runs the root command.
func Execute() {
	if err := NewRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func initRuntime() error {
	var err error
	cfg, err = config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	if err := cfg.EnsureDirs(); err != nil {
		return fmt.Errorf("create dirs: %w", err)
	}

	database, err = db.Open(cfg.DBPath)
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	printMigrationNotice(database.AppliedMigrations)

	vlt = buildVault()
	return nil
}

func buildVault() vault.Vault {
	var backends []vault.Vault

	if cfg.KeychainUse {
		backends = append(backends, vault.NewKeychain())
	}

	// age vault if file exists (Phase 2: prompts for passphrase)
	if _, err := os.Stat(cfg.VaultPath); err == nil {
		pass := os.Getenv("ALOGIN_VAULT_PASS")
		if pass != "" {
			backends = append(backends, vault.NewAge(cfg.VaultPath, pass))
		}
	}

	// plaintext fallback (legacy compatibility)
	backends = append(backends, vault.NewPlaintext(database.Raw()))

	return vault.NewChain(backends...)
}
