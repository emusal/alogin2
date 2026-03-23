package cli

import (
	"context"
	"fmt"

	"github.com/emusal/alogin2/internal/db"
	"github.com/spf13/cobra"
)

func newDBMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "db-migrate",
		Short: "Apply pending database schema migrations",
		Long: `Apply any pending database schema migrations for the current database.

Migrations are also applied automatically when the database is first opened by
any alogin command. Use this command to apply them explicitly — for example,
after manually installing a new binary.

The command exits 0 whether or not any migrations were applied.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(database.AppliedMigrations) == 0 {
				current := database.SchemaVersion(context.Background())
				fmt.Printf("Database schema is up to date (v%d).\n", current)
				return nil
			}
			// Notice was already printed to stderr by initRuntime.
			// Also print the expected target so the user knows we're done.
			fmt.Printf("Database schema is now at v%d (target: v%d).\n",
				database.AppliedMigrations[len(database.AppliedMigrations)-1],
				db.CurrentSchemaVersion,
			)
			return nil
		},
	}
}
