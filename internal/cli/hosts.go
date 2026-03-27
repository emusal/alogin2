package cli

import (
	"context"
	"fmt"

	"github.com/emusal/alogin2/internal/model"
	"github.com/spf13/cobra"
)

func newHostsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "hosts",
		Short: "Manage local hostname→IP mappings (custom /etc/hosts)",
		Long: `Manage the local hosts table used to resolve hostnames before DNS.

Entries in this table are checked first when connecting to any host.
This lets you override or assign IP addresses to hostnames without
modifying the system /etc/hosts file.`,
	}
	cmd.AddCommand(
		newHostsAddCmd(),
		newHostsListCmd(),
		newHostsShowCmd(),
		newHostsUpdateCmd(),
		newHostsDeleteCmd(),
	)
	return cmd
}

func newHostsAddCmd() *cobra.Command {
	var description string
	cmd := &cobra.Command{
		Use:   "add <hostname> <ip>",
		Short: "Add a hostname→IP mapping",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			h := &model.LocalHost{
				Hostname:    args[0],
				IP:          args[1],
				Description: description,
			}
			if err := database.Hosts.Create(ctx, h); err != nil {
				return fmt.Errorf("add host: %w", err)
			}
			fmt.Printf("Added: %s → %s\n", h.Hostname, h.IP)
			return nil
		},
	}
	cmd.Flags().StringVarP(&description, "description", "d", "", "optional description")
	return cmd
}

func newHostsListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all hostname→IP mappings",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			hosts, err := database.Hosts.ListAll(ctx)
			if err != nil {
				return err
			}

			if format == "json" {
				if hosts == nil {
					hosts = []*model.LocalHost{}
				}
				return printJSON(hosts)
			}

			if len(hosts) == 0 {
				fmt.Println("No hosts defined.")
				return nil
			}
			fmt.Printf("%-30s  %-20s  %s\n", "HOSTNAME", "IP", "DESCRIPTION")
			fmt.Printf("%-30s  %-20s  %s\n", "--------", "--", "-----------")
			for _, h := range hosts {
				fmt.Printf("%-30s  %-20s  %s\n", h.Hostname, h.IP, h.Description)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

func newHostsShowCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:   "show <hostname>",
		Short: "Show a single hostname mapping",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			h, err := database.Hosts.GetByHostname(ctx, args[0])
			if err != nil {
				return err
			}
			if h == nil {
				return fmt.Errorf("host %q not found", args[0])
			}

			if format == "json" {
				return printJSON(h)
			}

			fmt.Printf("Hostname:    %s\n", h.Hostname)
			fmt.Printf("IP:          %s\n", h.IP)
			fmt.Printf("Description: %s\n", h.Description)
			fmt.Printf("Created:     %s\n", h.CreatedAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Updated:     %s\n", h.UpdatedAt.Format("2006-01-02 15:04:05"))
			return nil
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

func newHostsUpdateCmd() *cobra.Command {
	var description string
	var descriptionSet bool
	cmd := &cobra.Command{
		Use:   "update <hostname> <new-ip>",
		Short: "Update the IP for a hostname",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			h, err := database.Hosts.GetByHostname(ctx, args[0])
			if err != nil {
				return err
			}
			if h == nil {
				return fmt.Errorf("host %q not found", args[0])
			}
			h.IP = args[1]
			if descriptionSet {
				h.Description = description
			}
			if err := database.Hosts.Update(ctx, h); err != nil {
				return fmt.Errorf("update host: %w", err)
			}
			fmt.Printf("Updated: %s → %s\n", h.Hostname, h.IP)
			return nil
		},
	}
	cmd.Flags().StringVarP(&description, "description", "d", "", "update description")
	cmd.Flags().BoolVar(&descriptionSet, "set-description", false, "apply the --description flag")
	return cmd
}

func newHostsDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <hostname>",
		Aliases: []string{"del", "rm"},
		Short:   "Delete a hostname mapping",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			h, err := database.Hosts.GetByHostname(ctx, args[0])
			if err != nil {
				return err
			}
			if h == nil {
				return fmt.Errorf("host %q not found", args[0])
			}
			if err := database.Hosts.Delete(ctx, h.ID); err != nil {
				return fmt.Errorf("delete host: %w", err)
			}
			fmt.Printf("Deleted: %s\n", args[0])
			return nil
		},
	}
}
