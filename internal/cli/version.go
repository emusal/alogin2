package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var Version = "dev"

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print alogin version",
		Annotations: map[string]string{
			skipDBAnnotation: "true",
		},
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println("alogin v" + Version)
		},
	}
}

func newShellInitCmd() *cobra.Command {
	var shell string
	cmd := &cobra.Command{
		Use:   "shell-init",
		Short: "Output shell compatibility functions (source this in .zshrc / .bashrc)",
		Annotations: map[string]string{
			skipDBAnnotation: "true",
		},
		Long: `Outputs shell compatibility shim functions for the legacy t/r/s/f/m/ct/cr commands.

Usage (add to ~/.zshrc or ~/.bashrc):
  source <(alogin shell-init)                  # zsh
  source <(alogin shell-init --shell bash)     # bash

Note: use  source <(...)  NOT  source "$(...)"

For tab-completion, install separately (Docker style):
  alogin completion install
  # then add to ~/.zshrc (before compinit):
  #   fpath=(~/.local/share/alogin/completions $fpath)`,
		RunE: func(cmd *cobra.Command, args []string) error {
			switch shell {
			case "bash":
				fmt.Print(bashShim)
			default:
				fmt.Print(zshShim)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&shell, "shell", "zsh", "target shell (zsh|bash)")
	return cmd
}

const zshShim = `# alogin v2 shell compatibility shim (zsh)
# Usage: source <(alogin shell-init)
function t()         { alogin access ssh "$@" }
function r()         { alogin access ssh --auto-gw "$@" }
function s()         { alogin access sftp "$@" }
function f()         { alogin access ftp "$@" }
function m()         { alogin access mount "$@" }
function ct()        { alogin access cluster "$@" }
function cr()        { alogin access cluster --auto-gw "$@" }
function addsvr()    { alogin compute add "$@" }
function delsvr()    { alogin compute delete "$@" }
function dissvr()    { alogin compute show "$@" }
function dissvrlist(){ alogin compute list }
function chgsvr()    { alogin compute update "$@" }
function chgpwd()    { alogin compute passwd "$@" }
function addalias()  { alogin auth alias add "$@" }
function disalias()  { alogin auth alias show "$@" }
function tver()      { alogin version }

# Tab-completion for shim functions.
# Works immediately on source; does not require alogin completion install.
if (( $+functions[compdef] )); then
  _alogin_shim_hosts()   { local -a h; h=(${(f)"$(alogin compute list 2>/dev/null | awk 'NR>2{print $3}')"}); compadd -a h }
  _alogin_shim_clusters(){ local -a c; c=(${(f)"$(alogin access cluster list 2>/dev/null | awk 'NR>2{print $1}')"}); compadd -a c }
  _alogin_shim_aliases() { local -a a; a=(${(f)"$(alogin auth alias list 2>/dev/null | awk 'NR>2{print $1}')"}); compadd -a a }
  _alogin_shim_addsvr()  {
    _arguments \
      '--proto[protocol]:proto:(ssh sftp ftp sshfs telnet)' \
      '--host[hostname or IP]:host:' \
      '--user[login user]:user:' \
      '--port[port (0=default)]:port:' \
      '--gateway[gateway route name]:gateway:' \
      '--locale[locale]:locale:'
  }
  compdef _alogin_shim_hosts    t r s f m delsvr dissvr chgpwd
  compdef _alogin_shim_clusters ct cr
  compdef _alogin_shim_addsvr   addsvr chgsvr
  compdef _alogin_shim_aliases  disalias
fi
`

const bashShim = `# alogin v2 shell compatibility shim (bash)
# Usage: source <(alogin shell-init --shell bash)
t()         { alogin access ssh "$@"; }
r()         { alogin access ssh --auto-gw "$@"; }
s()         { alogin access sftp "$@"; }
f()         { alogin access ftp "$@"; }
m()         { alogin access mount "$@"; }
ct()        { alogin access cluster "$@"; }
cr()        { alogin access cluster --auto-gw "$@"; }
addsvr()    { alogin compute add "$@"; }
delsvr()    { alogin compute delete "$@"; }
dissvr()    { alogin compute show "$@"; }
dissvrlist(){ alogin compute list; }
chgsvr()    { alogin compute update "$@"; }
chgpwd()    { alogin compute passwd "$@"; }
addalias()  { alogin auth alias add "$@"; }
disalias()  { alogin auth alias show "$@"; }
tver()      { alogin version; }

# Tab-completion for shim functions.
_alogin_shim_hosts()   { COMPREPLY=($(compgen -W "$(alogin compute list 2>/dev/null | awk 'NR>2{print $3}')" -- "${COMP_WORDS[COMP_CWORD]}")); }
_alogin_shim_clusters(){ COMPREPLY=($(compgen -W "$(alogin access cluster list 2>/dev/null | awk 'NR>2{print $1}')" -- "${COMP_WORDS[COMP_CWORD]}")); }
_alogin_shim_aliases() { COMPREPLY=($(compgen -W "$(alogin auth alias list 2>/dev/null | awk 'NR>2{print $1}')" -- "${COMP_WORDS[COMP_CWORD]}")); }
_alogin_shim_addsvr()  { COMPREPLY=($(compgen -W "--proto --host --user --port --gateway --locale" -- "${COMP_WORDS[COMP_CWORD]}")); }
complete -F _alogin_shim_hosts    t r s f m delsvr dissvr chgpwd
complete -F _alogin_shim_clusters ct cr
complete -F _alogin_shim_addsvr   addsvr chgsvr
complete -F _alogin_shim_aliases  disalias
`
