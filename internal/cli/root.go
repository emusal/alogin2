package cli

import (
	"fmt"
	"os"

	"github.com/emusal/alogin2/internal/config"
	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/vault"
	"github.com/spf13/cobra"
)

// skipDBAnnotation is applied to commands that do not need database initialization.
// Commands with this annotation skip initRuntime() in PersistentPreRunE.
const skipDBAnnotation = "alogin:skip-db"

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
		Short: "Security Gateway for Agentic AI",
		Long: `alogin — Security Gateway for Agentic AI

A secure conduit for LLMs and AI agents to access infrastructure safely.
Manages SSH connections, port tunnels, cluster sessions, and server credentials
with an encrypted vault and a full audit trail.

Command groups:
  compute     Manage servers (compute resources)
  app-server  Named server+plugin bindings for one-command app access
  access      Connect to remote hosts (SSH, SFTP, FTP, SSHFS, cluster)
  auth        Manage credentials and routing (gateways, aliases, vault)
  agent       AI/MCP tools: run as MCP server, configure AI clients, manage policies
  net         Manage network resources (hosts, tunnels)

Interactive UIs:
  tui       Terminal UI host selector
  web       Embedded Web UI

Run 'alogin agent setup' to configure Claude Desktop or other AI clients.`,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if cmd.Annotations[skipDBAnnotation] == "true" {
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

	// ---- Canonical new hierarchy ----
	root.AddCommand(
		newComputeCmd(),    // alogin compute
		newAppServerCmd(),  // alogin app-server
		newAccessCmd(),     // alogin access
		newAuthCmd(),       // alogin auth
		newAgentCmd(),      // alogin agent
		newNetCmd(),        // alogin net
	)

	// ---- Unchanged root-level commands ----
	root.AddCommand(
		newTUICmd(),
		newWebCmd(),
		newVersionCmd(),
		newShellInitCmd(),
		newCompletionCmd(),
		newMigrateCmd(),
		newDBMigrateCmd(),
		newUpgradeCmd(),
		newUninstallCmd(),
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

func initVaultOnly() error {
	var err error
	cfg, err = config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
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

	// plaintext fallback (legacy compatibility, requires DB)
	if database != nil {
		backends = append(backends, vault.NewPlaintext(database.Raw()))
	}

	return vault.NewChain(backends...)
}
