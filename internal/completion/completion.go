// Package completion generates shell completion scripts for alogin.
package completion

import (
	"fmt"
	"io"
)

// WriteZsh writes a zsh completion script to w.
func WriteZsh(w io.Writer) error {
	_, err := fmt.Fprint(w, ZshScript)
	return err
}

// WriteBash writes a bash completion script to w.
func WriteBash(w io.Writer) error {
	_, err := fmt.Fprint(w, BashScript)
	return err
}

// ZshScript is the zsh completion script for alogin (_alogin fpath file).
const ZshScript = `#compdef alogin

# ---------------------------------------------------------------------------
# Helper functions — each queries the live DB via the CLI.
# Results are cached per-shell-session using _store_cache / _retrieve_cache.
# ---------------------------------------------------------------------------

_alogin_hosts() {
  local -a hosts
  hosts=(${(f)"$(alogin server list 2>/dev/null | awk 'NR>2{print $3}')"})
  _describe 'host' hosts
}

_alogin_users_at_hosts() {
  local -a targets
  targets=(${(f)"$(alogin server list 2>/dev/null | awk 'NR>2{print $4"@"$3}')"})
  _describe 'user@host' targets
}

_alogin_gateways() {
  local -a gws
  gws=(${(f)"$(alogin gateway list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'gateway' gws
}

_alogin_aliases() {
  local -a aliases
  aliases=(${(f)"$(alogin alias list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'alias' aliases
}

_alogin_clusters() {
  local -a clusters
  clusters=(${(f)"$(alogin cluster list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'cluster' clusters
}

# ---------------------------------------------------------------------------
# Main completion function
# ---------------------------------------------------------------------------

_alogin() {
  local context state state_descr line
  typeset -A opt_args

  _arguments -C \
    '(-h --help)'{-h,--help}'[show help]' \
    '1: :->command' \
    '*:: :->args'

  case $state in
    command)
      local -a commands
      commands=(
        'connect:Connect to a host via SSH'
        'sftp:SFTP file transfer'
        'ftp:FTP connection'
        'mount:Mount remote filesystem via SSHFS'
        'cluster:Open cluster SSH sessions'
        'server:Manage servers in the registry'
        'gateway:Manage gateway routes'
        'alias:Manage host aliases'
        'migrate:Import legacy alogin data files'
        'tui:Interactive fuzzy host selector'
        'web:Start the web UI server'
        'completion:Generate or install shell completion scripts'
        'shell-init:Output shell compatibility shim (source with <(...))'
        'version:Print version'
      )
      _describe 'command' commands
      ;;

    args)
      case $words[1] in

        connect)
          _arguments \
            '--auto-gw[auto-detect gateway route (legacy r)]' \
            '--dry-run[print connection route without connecting]' \
            '(-c --cmd)'{-c,--cmd}'[run command after login]:command:' \
            '(-L --local-forward)'{-L,--local-forward}'[local port forward]:spec \(local\:host\:port\):' \
            '(-R --remote-forward)'{-R,--remote-forward}'[remote port forward]:spec \(remote\:host\:port\):' \
            '1: :_alogin_hosts'
          ;;

        sftp)
          _arguments \
            '(-p --put)'{-p,--put}'[upload file to remote]:local file:_files' \
            '(-g --get)'{-g,--get}'[download file from remote]:remote path:' \
            '(-d --dest)'{-d,--dest}'[remote destination path]:remote path:' \
            '1: :_alogin_hosts'
          ;;

        ftp)
          _arguments \
            '(-d --dest)'{-d,--dest}'[remote destination path]:remote path:' \
            '1: :_alogin_hosts'
          ;;

        mount)
          _arguments \
            '(-d --dest)'{-d,--dest}'[local mount point]:directory:_files -/' \
            '1: :_alogin_hosts'
          ;;

        cluster)
          local -a cluster_subcmds
          cluster_subcmds=(
            'list:List all clusters'
          )
          _arguments -C \
            '--mode[terminal session mode]:mode:(tmux iterm terminal)' \
            '--gateway[route through gateways (legacy cr)]' \
            '(-x --tile-x)'{-x,--tile-x}'[number of tile columns]:columns:' \
            '1: :->cluster_first' \
            '*:: :->cluster_rest'
          case $state in
            cluster_first)
              _describe 'subcommand' cluster_subcmds
              _alogin_clusters
              ;;
            cluster_rest)
              [[ $words[1] == list ]] && return 0
              ;;
          esac
          ;;

        server)
          local -a server_subcmds
          server_subcmds=(
            'add:Add a server to the registry'
            'list:List all servers'
            'show:Show details for a server'
            'delete:Remove a server'
            'update:Update server fields'
            'passwd:Change stored password'
            'getpwd:Show the stored password for a server'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' server_subcmds ;;
            sub_args)
              case $words[1] in
                show|delete|update|passwd|getpwd)
                  _alogin_hosts ;;
                add)
                  _arguments \
                    '--proto[protocol]:proto:(ssh sftp ftp sshfs telnet)' \
                    '--host[hostname or IP]:host:' \
                    '--user[login user]:user:' \
                    '--port[port (0=default)]:port:' \
                    '--gateway[gateway route name]:gateway:_alogin_gateways' \
                    '--locale[locale (e.g. ko_KR.eucKR)]:locale:'
                  ;;
              esac
              ;;
          esac
          ;;

        gateway)
          local -a gw_subcmds
          gw_subcmds=(
            'add:Add a gateway route'
            'list:List all gateways'
            'show:Show gateway details'
            'delete:Remove a gateway'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' gw_subcmds ;;
            sub_args)
              case $words[1] in
                show|delete) _alogin_gateways ;;
                add) _alogin_hosts ;;
              esac
              ;;
          esac
          ;;

        alias)
          local -a alias_subcmds
          alias_subcmds=(
            'add:Add a hostname alias'
            'list:List all aliases'
            'show:Show alias details'
            'delete:Remove an alias'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' alias_subcmds ;;
            sub_args)
              case $words[1] in
                show|delete) _alogin_aliases ;;
                add) _alogin_hosts ;;
              esac
              ;;
          esac
          ;;

        completion)
          local -a comp_subcmds
          comp_subcmds=(
            'zsh:Output zsh completion script'
            'bash:Output bash completion script'
            'install:Install completion file to fpath directory'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' comp_subcmds ;;
            sub_args)
              case $words[1] in
                install)
                  _arguments \
                    '--dir[destination directory]:directory:_files -/' \
                    '--shell[target shell]:shell:(zsh bash)'
                  ;;
              esac
              ;;
          esac
          ;;

        shell-init)
          _arguments '--shell[target shell]:shell:(zsh bash)'
          ;;

        web)
          _arguments \
            '(-p --port)'{-p,--port}'[HTTP port (default 8484)]:port:' \
            '--no-browser[do not open browser automatically]'
          ;;

      esac
      ;;
  esac
}

# In fpath mode (#compdef alogin handles registration).
# In source mode (source <(alogin completion zsh)), register explicitly.
(( $+functions[compdef] )) && compdef _alogin alogin
`

// BashScript is the bash completion script for alogin.
const BashScript = `# alogin bash completion
# Install: alogin completion install --shell bash
# Or add to ~/.bashrc: source <(alogin completion bash)

_alogin_completion() {
  local cur prev words cword
  _init_completion 2>/dev/null || {
    cur="${COMP_WORDS[COMP_CWORD]}"
    prev="${COMP_WORDS[COMP_CWORD-1]}"
    words=("${COMP_WORDS[@]}")
    cword=$COMP_CWORD
  }

  local commands="connect sftp ftp mount cluster server gateway alias migrate tui web completion shell-init version"

  # Helpers
  _alogin_hosts() {
    alogin server list 2>/dev/null | awk 'NR>2{print $3}'
  }
  _alogin_gateways() {
    alogin gateway list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_aliases() {
    alogin alias list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_clusters() {
    alogin cluster list 2>/dev/null | awk 'NR>2{print $1}'
  }

  local cmd="${words[1]}"
  local sub="${words[2]}"

  if [[ $cword -eq 1 ]]; then
    COMPREPLY=($(compgen -W "$commands" -- "$cur"))
    return
  fi

  case "$cmd" in
    connect)
      case "$prev" in
        --cmd|-c) return ;;
        -L|--local-forward|-R|--remote-forward) return ;;
        *)
          if [[ "$cur" != -* ]]; then
            COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur"))
          else
            COMPREPLY=($(compgen -W "--auto-gw --dry-run --cmd -c -L --local-forward -R --remote-forward" -- "$cur"))
          fi
          ;;
      esac
      ;;
    sftp|ftp|mount)
      if [[ "$cur" != -* ]]; then
        COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur"))
      fi
      ;;
    cluster)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "list $(_alogin_clusters)" -- "$cur"))
      elif [[ "$cur" == -* ]]; then
        COMPREPLY=($(compgen -W "--mode --gateway --tile-x -x" -- "$cur"))
      fi
      ;;
    server)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "add list show delete update passwd getpwd" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          show|delete|update|passwd|getpwd)
            COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur")) ;;
          add)
            COMPREPLY=($(compgen -W "--proto --host --user --port --gateway --locale" -- "$cur")) ;;
        esac
      fi
      ;;
    gateway)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "add list show delete" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          show|delete) COMPREPLY=($(compgen -W "$(_alogin_gateways)" -- "$cur")) ;;
          add)         COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur")) ;;
        esac
      fi
      ;;
    alias)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "add list show delete" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          show|delete) COMPREPLY=($(compgen -W "$(_alogin_aliases)" -- "$cur")) ;;
          add)         COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur")) ;;
        esac
      fi
      ;;
    completion)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "zsh bash install" -- "$cur"))
      elif [[ $cword -ge 3 && "$sub" == "install" ]]; then
        COMPREPLY=($(compgen -W "--dir --shell" -- "$cur"))
      fi
      ;;
    shell-init)
      COMPREPLY=($(compgen -W "--shell" -- "$cur"))
      ;;
    web)
      COMPREPLY=($(compgen -W "--port -p --no-browser" -- "$cur"))
      ;;
  esac
}

complete -F _alogin_completion alogin
`
