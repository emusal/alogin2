package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/emusal/alogin2/internal/plugin"
	"github.com/spf13/cobra"
)

func newPluginCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Manage application plugins",
	}
	cmd.AddCommand(newPluginListCmd())
	return cmd
}

func newPluginListCmd() *cobra.Command {
	var format string
	cmd := &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short: "List installed application plugins",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := plugin.PluginDir(cfg.ConfigDir)
			if _, err := os.Stat(dir); os.IsNotExist(err) {
				fmt.Println("No plugins installed. Plugin directory:", dir)
				return nil
			}

			plugins, err := plugin.LoadDir(dir)
			if err != nil {
				return fmt.Errorf("load plugins: %w", err)
			}

			if format == "json" {
				type row struct {
					Name       string   `json:"name"`
					Version    string   `json:"version"`
					Provider   string   `json:"provider"`
					Strategies []string `json:"strategies"`
					CmdFlag    string   `json:"cmd_flag"`
					Dir        string   `json:"dir"`
				}
				rows := make([]row, 0, len(plugins))
				for _, p := range plugins {
					flag := p.Runtime.CmdFlag
					if flag == "" {
						flag = "-e"
					}
					rows = append(rows, row{
						Name:       p.Name,
						Version:    p.Version,
						Provider:   string(p.Auth.Provider),
						Strategies: p.Runtime.Strategies,
						CmdFlag:    flag,
						Dir:        dir,
					})
				}
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(rows)
			}

			if len(plugins) == 0 {
				fmt.Println("No plugins found in", dir)
				return nil
			}

			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tVERSION\tPROVIDER\tSTRATEGIES\tCMD FLAG")
			for _, p := range plugins {
				flag := p.Runtime.CmdFlag
				if flag == "" {
					flag = "-e"
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
					p.Name, p.Version, string(p.Auth.Provider),
					strings.Join(p.Runtime.Strategies, ","), flag)
			}
			return w.Flush()
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "output format: table|json")
	return cmd
}
