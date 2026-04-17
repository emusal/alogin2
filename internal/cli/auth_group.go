package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// newVaultCmd returns the top-level "vault" command for credential management.
func newVaultCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vault",
		Short: "Manage stored credentials",
		Long: `Manage stored credentials in the vault.

Examples:
  alogin vault set testuser@target-mariadb
  alogin vault get testuser@target-mariadb
  alogin vault delete testuser@target-mariadb`,
		Annotations: map[string]string{skipDBAnnotation: "true"},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			return initVaultOnly()
		},
	}
	cmd.AddCommand(
		newVaultSetCmd(),
		newVaultGetCmd(),
		newVaultDeleteCmd(),
	)
	return cmd
}

func newVaultSetCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "set <account>",
		Short:       "Store a password for account (e.g. testuser@host)",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			account := args[0]
			fmt.Fprintf(os.Stderr, "Password for %s: ", account)
			raw, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Fprintln(os.Stderr)
			if err != nil {
				return err
			}
			if err := vlt.Set(account, string(raw)); err != nil {
				return fmt.Errorf("vault set: %w", err)
			}
			fmt.Printf("Stored credential for %s (backend: %s)\n", account, vlt.Name())
			return nil
		},
	}
}

func newVaultGetCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "get <account>",
		Short:       "Retrieve a stored password for account",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			pass, err := vlt.Get(args[0])
			if err != nil {
				return fmt.Errorf("vault get: %w", err)
			}
			fmt.Println(pass)
			return nil
		},
	}
}

func newVaultDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:         "delete <account>",
		Short:       "Remove a stored password for account",
		Args:        cobra.ExactArgs(1),
		Annotations: map[string]string{skipDBAnnotation: "true"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := vlt.Delete(args[0]); err != nil {
				return fmt.Errorf("vault delete: %w", err)
			}
			fmt.Printf("Deleted credential for %s\n", args[0])
			return nil
		},
	}
}
