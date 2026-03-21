package cli

import (
	"context"
	"fmt"

	"github.com/emusal/alogin2/internal/migrate"
	"github.com/spf13/cobra"
)

func newMigrateCmd() *cobra.Command {
	var from string
	var verbose bool

	cmd := &cobra.Command{
		Use:   "migrate",
		Short: "Migrate legacy ALOGIN data to v2 database",
		Long: `Import data from the legacy flat-file format (server_list, gateway_list,
alias_hosts, clusters, term_themes) into the v2 SQLite database.

Examples:
  alogin migrate --from ~/.alogin
  alogin migrate --from $ALOGIN_ROOT --verbose`,
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
