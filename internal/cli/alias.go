package cli

import (
	"context"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/emusal/alogin2/internal/model"
	"github.com/spf13/cobra"
)

func newAliasCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage host aliases",
	}
	cmd.AddCommand(newAliasAddCmd(), newAliasListCmd(), newAliasShowCmd(), newAliasDeleteCmd())
	return cmd
}

func newAliasAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <alias> [user@]host",
		Short: "Add a hostname alias",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			aliasName := args[0]
			user, host := parseUserHost(args[1])

			srv, err := database.Servers.GetByHost(ctx, host, user)
			if err != nil || srv == nil {
				return fmt.Errorf("server %s not found", args[1])
			}

			a := &model.Alias{Name: aliasName, ServerID: srv.ID, User: user}
			if err := database.Aliases.Create(ctx, a); err != nil {
				return err
			}
			fmt.Printf("Added alias %s → %s@%s\n", aliasName, srv.User, srv.Host)
			return nil
		},
	}
}

func newAliasListCmd() *cobra.Command {
	return &cobra.Command{
		Use: "list",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			aliases, err := database.Aliases.ListAll(ctx)
			if err != nil {
				return err
			}
			if len(aliases) == 0 {
				fmt.Println("No aliases.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ALIAS\tTARGET")
			for _, a := range aliases {
				srv, _ := database.Servers.GetByID(ctx, a.ServerID)
				target := fmt.Sprintf("id=%d", a.ServerID)
				if srv != nil {
					target = srv.User + "@" + srv.Host
					if a.User != "" {
						target = a.User + "@" + srv.Host
					}
				}
				fmt.Fprintf(w, "%s\t%s\n", a.Name, target)
			}
			return w.Flush()
		},
	}
}

func newAliasShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "show <alias>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			a, err := database.Aliases.GetByName(ctx, args[0])
			if err != nil || a == nil {
				return fmt.Errorf("alias %s not found", args[0])
			}
			srv, _ := database.Servers.GetByID(ctx, a.ServerID)
			if srv != nil {
				fmt.Printf("%s → %s@%s\n", a.Name, srv.User, srv.Host)
			}
			return nil
		},
	}
}

func newAliasDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:  "delete <alias>",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			a, err := database.Aliases.GetByName(ctx, args[0])
			if err != nil || a == nil {
				return fmt.Errorf("alias %s not found", args[0])
			}
			return database.Aliases.Delete(ctx, a.ID)
		},
	}
}
