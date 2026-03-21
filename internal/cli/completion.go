package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/emusal/alogin2/internal/completion"
	"github.com/spf13/cobra"
)

func newCompletionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "completion <zsh|bash|install>",
		Short: "Generate or install shell completion scripts",
		Long: `Generate shell completion scripts or install them Docker-style into an fpath directory.

Generate (pipe/eval):
  source <(alogin completion zsh)         # zsh — one-shot
  source <(alogin completion bash)        # bash — one-shot

Install (recommended — Docker style):
  alogin completion install               # installs _alogin to ~/.local/share/alogin/completions/
  alogin completion install --dir ~/.zsh/completions

Then add ONE LINE to ~/.zshrc (before compinit):
  fpath=(~/.local/share/alogin/completions $fpath)

For bash, install to a bash-completion directory:
  alogin completion install --shell bash --dir ~/.local/share/bash-completion/completions`,
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"zsh", "bash", "install"},
		RunE: func(cmd *cobra.Command, args []string) error {
			sub := "zsh"
			if len(args) > 0 {
				sub = args[0]
			}
			switch sub {
			case "bash":
				return completion.WriteBash(os.Stdout)
			case "zsh":
				return completion.WriteZsh(os.Stdout)
			case "install":
				return cmd.Help()
			default:
				return fmt.Errorf("unsupported shell %q; use zsh, bash, or install", sub)
			}
		},
	}

	cmd.AddCommand(newCompletionInstallCmd())
	return cmd
}

func newCompletionInstallCmd() *cobra.Command {
	var dir string
	var shell string

	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install completion file into an fpath directory",
		Long: `Install the alogin completion file into a directory and print setup instructions.

Zsh (default):
  alogin completion install
  # installs _alogin → ~/.local/share/alogin/completions/_alogin
  # then add to ~/.zshrc:
  #   fpath=(~/.local/share/alogin/completions $fpath)

Bash:
  alogin completion install --shell bash
  # installs alogin → ~/.local/share/alogin/completions/alogin`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Default directory: ~/.local/share/alogin/completions
			if dir == "" {
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				dir = filepath.Join(home, ".local", "share", "alogin", "completions")
			}

			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("create completions dir: %w", err)
			}

			switch shell {
			case "bash":
				destPath := filepath.Join(dir, "alogin")
				if err := writeFile(destPath, completion.BashScript); err != nil {
					return err
				}
				fmt.Printf("Installed bash completion → %s\n\n", destPath)
				fmt.Println("Add to ~/.bashrc:")
				fmt.Printf("  source %s\n", destPath)

			default: // zsh
				destPath := filepath.Join(dir, "_alogin")
				if err := writeFile(destPath, completion.ZshScript); err != nil {
					return err
				}
				fmt.Printf("Installed zsh completion → %s\n\n", destPath)
				fmt.Println("Add to ~/.zshrc (before compinit):")
				fmt.Printf("  fpath=(%s $fpath)\n", dir)
				fmt.Println()
				fmt.Println("If compinit is not yet in your ~/.zshrc, also add:")
				fmt.Println("  autoload -Uz compinit && compinit")
			}

			return nil
		},
	}

	cmd.Flags().StringVar(&dir, "dir", "", "destination directory (default: ~/.local/share/alogin/completions)")
	cmd.Flags().StringVar(&shell, "shell", "zsh", "target shell: zsh|bash")
	return cmd
}

func writeFile(path, content string) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	defer f.Close()
	_, err = fmt.Fprint(f, content)
	return err
}
