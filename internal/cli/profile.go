package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/emusal/alogin2/internal/model"
	"github.com/spf13/cobra"
)

func newProfileCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profile",
		Short: "Manage network profiles (active gateway context)",
		Long: `Manage named network profiles.

A profile activates a gateway route for all subsequent SSH connections,
similar to switching a VPN profile. Once a profile is active, alogin
automatically routes connections through its gateway — no per-command
flags needed.

Examples:
  alogin net profile add home --gateway home-gw --desc "Home network"
  alogin net profile list
  alogin net profile use home
  alogin net profile use none   # disable gateway routing
  alogin net profile show home
  alogin net profile delete home`,
	}
	cmd.AddCommand(
		newProfileAddCmd(),
		newProfileListCmd(),
		newProfileShowCmd(),
		newProfileEditCmd(),
		newProfileDeleteCmd(),
		newProfileUseCmd(),
	)
	return cmd
}

func newProfileAddCmd() *cobra.Command {
	var gatewayName string
	var desc string

	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Add a network profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]

			var routeID *int64
			if gatewayName != "" {
				gw, err := database.Gateways.GetByName(ctx, gatewayName)
				if err != nil || gw == nil {
					return fmt.Errorf("gateway route %q not found", gatewayName)
				}
				routeID = &gw.ID
			}

			p, err := database.Profiles.Create(ctx, name, desc, routeID)
			if err != nil {
				return fmt.Errorf("create profile: %w", err)
			}
			fmt.Printf("Profile %q created (id=%d)\n", p.Name, p.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&gatewayName, "gateway", "", "gateway route name to attach to this profile")
	cmd.Flags().StringVar(&desc, "desc", "", "optional description")
	return cmd
}

func newProfileListCmd() *cobra.Command {
	var format string

	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List all profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			profiles, err := database.Profiles.ListAll(ctx)
			if err != nil {
				return fmt.Errorf("list profiles: %w", err)
			}

			if format == "json" {
				type profileJSON struct {
					ID          int64  `json:"id"`
					Name        string `json:"name"`
					Gateway     string `json:"gateway"`
					Active      bool   `json:"active"`
					Description string `json:"description"`
				}
				out := make([]profileJSON, 0, len(profiles))
				for _, p := range profiles {
					gw := ""
					if p.GatewayRouteID != nil {
						r, _ := database.Gateways.GetByID(ctx, *p.GatewayRouteID)
						if r != nil {
							gw = r.Name
						}
					}
					out = append(out, profileJSON{
						ID: p.ID, Name: p.Name,
						Gateway: gw, Active: p.IsActive,
						Description: p.Description,
					})
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(out)
			}

			if len(profiles) == 0 {
				fmt.Println("No profiles configured.")
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tGATEWAY\tACTIVE\tDESCRIPTION")
			for _, p := range profiles {
				gw := "(none)"
				if p.GatewayRouteID != nil {
					r, _ := database.Gateways.GetByID(ctx, *p.GatewayRouteID)
					if r != nil {
						gw = r.Name
					}
				}
				active := ""
				if p.IsActive {
					active = "*"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", p.Name, gw, active, p.Description)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}

func newProfileShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <name>",
		Short: "Show profile details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			p, err := database.Profiles.GetByName(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get profile: %w", err)
			}
			if p == nil {
				return fmt.Errorf("profile %q not found", args[0])
			}

			gw := "(none)"
			if p.GatewayRouteID != nil {
				r, _ := database.Gateways.GetByID(ctx, *p.GatewayRouteID)
				if r != nil {
					gw = r.Name
					hops := make([]string, len(r.Hops))
					for i, h := range r.Hops {
						hopSrv, _ := database.Servers.GetByID(ctx, h.ServerID)
						if hopSrv != nil {
							hops[i] = hopSrv.Host
						}
					}
					if len(hops) > 0 {
						gw = fmt.Sprintf("%s (%s)", r.Name, joinStrings(hops, " → "))
					}
				}
			}

			active := "no"
			if p.IsActive {
				active = "yes (*)"
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(w, "Name:\t%s\n", p.Name)
			fmt.Fprintf(w, "Gateway:\t%s\n", gw)
			fmt.Fprintf(w, "Active:\t%s\n", active)
			fmt.Fprintf(w, "Description:\t%s\n", p.Description)
			fmt.Fprintf(w, "Created:\t%s\n", p.CreatedAt.Format("2006-01-02 15:04:05"))
			return w.Flush()
		},
	}
}

func newProfileEditCmd() *cobra.Command {
	var gatewayName string
	var desc string

	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			p, err := database.Profiles.GetByName(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get profile: %w", err)
			}
			if p == nil {
				return fmt.Errorf("profile %q not found", args[0])
			}

			var routeID *int64 = p.GatewayRouteID
			if cmd.Flags().Changed("gateway") {
				if gatewayName == "" || gatewayName == "none" {
					routeID = nil
				} else {
					gw, err := database.Gateways.GetByName(ctx, gatewayName)
					if err != nil || gw == nil {
						return fmt.Errorf("gateway route %q not found", gatewayName)
					}
					routeID = &gw.ID
				}
			}
			if cmd.Flags().Changed("desc") {
				p.Description = desc
			}

			if err := database.Profiles.Update(ctx, p, routeID); err != nil {
				return fmt.Errorf("update profile: %w", err)
			}
			fmt.Printf("Profile %q updated.\n", p.Name)
			return nil
		},
	}
	cmd.Flags().StringVar(&gatewayName, "gateway", "", `gateway route name (use "none" to detach)`)
	cmd.Flags().StringVar(&desc, "desc", "", "description")
	return cmd
}

func newProfileDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm", "del"},
		Short:   "Delete a profile",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			p, err := database.Profiles.GetByName(ctx, args[0])
			if err != nil {
				return fmt.Errorf("get profile: %w", err)
			}
			if p == nil {
				return fmt.Errorf("profile %q not found", args[0])
			}
			if err := database.Profiles.Delete(ctx, p.ID); err != nil {
				return fmt.Errorf("delete profile: %w", err)
			}
			fmt.Printf("Profile %q deleted.\n", args[0])
			return nil
		},
	}
}

func newProfileUseCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "use <name|none>",
		Short: `Activate a profile ("none" to deactivate all)`,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := context.Background()
			name := args[0]

			if name == "none" {
				if err := database.Profiles.SetActive(ctx, 0); err != nil {
					return fmt.Errorf("deactivate profiles: %w", err)
				}
				fmt.Println("All profiles deactivated. Connections will go direct.")
				return nil
			}

			p, err := database.Profiles.GetByName(ctx, name)
			if err != nil {
				return fmt.Errorf("get profile: %w", err)
			}
			if p == nil {
				return fmt.Errorf("profile %q not found", name)
			}
			if err := database.Profiles.SetActive(ctx, p.ID); err != nil {
				return fmt.Errorf("activate profile: %w", err)
			}

			gw := "no gateway"
			if p.GatewayRouteID != nil {
				r, _ := database.Gateways.GetByID(ctx, *p.GatewayRouteID)
				if r != nil {
					gw = "via " + r.Name
				}
			}
			fmt.Printf("Profile %q activated (%s).\n", p.Name, gw)
			return nil
		},
	}
}

// joinStrings joins a slice with a separator (avoids importing strings in this file).
func joinStrings(ss []string, sep string) string {
	result := ""
	for i, s := range ss {
		if i > 0 {
			result += sep
		}
		result += s
	}
	return result
}

// profileJSON is used by the MCP tool layer; keep in sync with newProfileListCmd.
func profileToModel(p *model.Profile) map[string]any {
	return map[string]any{
		"id":          p.ID,
		"name":        p.Name,
		"is_active":   p.IsActive,
		"description": p.Description,
	}
}
