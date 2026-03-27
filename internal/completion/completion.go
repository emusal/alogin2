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
  gws=(${(f)"$(alogin auth gateway list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'gateway' gws
}

_alogin_aliases() {
  local -a aliases
  aliases=(${(f)"$(alogin auth alias list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'alias' aliases
}

_alogin_clusters() {
  local -a clusters
  clusters=(${(f)"$(alogin access cluster list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'cluster' clusters
}

_alogin_tunnels() {
  local -a tunnels
  tunnels=(${(f)"$(alogin net tunnel list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'tunnel' tunnels
}

_alogin_hosts_entries() {
  local -a hosts
  hosts=(${(f)"$(alogin net hosts list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'hostname' hosts
}

# ---------------------------------------------------------------------------
# Subcommand completion helpers (reused by both canonical and legacy paths)
# ---------------------------------------------------------------------------

_alogin_server_args() {
  local -a server_subcmds
  server_subcmds=(
    'add:Add a server to the registry'
    'list:List all servers'
    'show:Show details for a server'
    'delete:Remove a server'
    'passwd:Change stored password'
    'getpwd:Show the stored password for a server'
  )
  _arguments -C '1: :->sub' '*:: :->sub_args'
  case $state in
    sub) _describe 'subcommand' server_subcmds ;;
    sub_args)
      case $words[1] in
        show|delete|passwd|getpwd) _alogin_hosts ;;
        add)
          _arguments \
            '--proto[protocol]:proto:(ssh sftp ftp sshfs telnet)' \
            '--host[hostname or IP]:host:' \
            '--user[login user]:user:' \
            '--port[port (0=default)]:port:' \
            '--gateway[gateway route name]:gateway:_alogin_gateways' \
            '--locale[locale (e.g. ko_KR.eucKR)]:locale:'
          ;;
        list) _arguments '--format[output format]:format:(table json)' ;;
      esac
      ;;
  esac
}

_alogin_gateway_args() {
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
        list) _arguments '--format[output format]:format:(table json)' ;;
      esac
      ;;
  esac
}

_alogin_alias_args() {
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
        list) _arguments '--format[output format]:format:(table json)' ;;
      esac
      ;;
  esac
}

_alogin_cluster_args() {
  local -a cluster_subcmds
  cluster_subcmds=('list:List all clusters' 'add:Add a new cluster')
  _arguments -C \
    '--mode[terminal session mode]:mode:(tmux iterm terminal)' \
    '--auto-gw[route through gateways (legacy cr)]' \
    '(-x --tile-x)'{-x,--tile-x}'[number of tile columns]:columns:' \
    '1: :->cluster_first' \
    '*:: :->cluster_rest'
  case $state in
    cluster_first)
      _describe 'subcommand' cluster_subcmds
      _alogin_clusters
      ;;
    cluster_rest)
      case $words[1] in
        list) _arguments '--format[output format]:format:(table json)' ;;
        add)
          _arguments \
            '--mode[terminal session mode]:mode:(tmux iterm terminal)' \
            '--auto-gw[route through gateways (legacy cr)]' \
            '(-x --tile-x)'{-x,--tile-x}'[number of tile columns]:columns:' \
            '1:cluster name:' \
            '*:server:_alogin_hosts'
          ;;
      esac
      ;;
  esac
}

_alogin_tunnel_args() {
  local -a tunnel_subcmds
  tunnel_subcmds=(
    'list:List tunnel configurations'
    'add:Add a tunnel configuration'
    'edit:Edit a tunnel configuration'
    'rm:Remove a tunnel configuration'
    'start:Start a tunnel in tmux'
    'stop:Stop a running tunnel'
    'status:Show tunnel running status'
  )
  _arguments -C '1: :->sub' '*:: :->sub_args'
  case $state in
    sub) _describe 'subcommand' tunnel_subcmds ;;
    sub_args)
      case $words[1] in
        start|stop|status|edit|rm) _alogin_tunnels ;;
        add)
          _arguments \
            '--server[server hostname]:host:_alogin_hosts' \
            '--dir[direction]:dir:(L R)' \
            '--local-host[local listen address]:host:' \
            '--local-port[local port]:port:' \
            '--remote-host[remote host]:host:' \
            '--remote-port[remote port]:port:' \
            '--auto-gw[follow gateway chain]'
          ;;
        list) _arguments '--format[output format]:format:(table json)' ;;
      esac
      ;;
  esac
}

_alogin_hosts_cmd_args() {
  local -a hosts_subcmds
  hosts_subcmds=(
    'add:Add a hostname→IP mapping'
    'list:List all mappings'
    'show:Show a single mapping'
    'update:Update the IP for a hostname'
    'delete:Delete a hostname mapping'
  )
  _arguments -C '1: :->sub' '*:: :->sub_args'
  case $state in
    sub) _describe 'subcommand' hosts_subcmds ;;
    sub_args)
      case $words[1] in
        show|update|delete) _alogin_hosts_entries ;;
        add)
          _arguments \
            '1:hostname:' \
            '2:ip:' \
            '(-d --description)'{-d,--description}'[description]:desc:'
          ;;
        list) _arguments '--format[output format]:format:(table json)' ;;
      esac
      ;;
  esac
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
        # ── New canonical groups ──────────────────────────────────────────
        'compute:Manage servers (compute resources)'
        'access:Connect to remote hosts (SSH, SFTP, FTP, cluster)'
        'auth:Manage credentials and routing (gateways, aliases, vault)'
        'agent:AI/MCP tools: MCP server, setup, policy'
        'net:Manage network resources (hosts, tunnels)'
        # ── Interactive UIs ───────────────────────────────────────────────
        'tui:Interactive fuzzy host selector'
        'web:Start the web UI server'
        # ── System commands ───────────────────────────────────────────────
        'migrate:Import legacy alogin data files'
        'db-migrate:Apply pending database schema migrations'
        'completion:Generate or install shell completion scripts'
        'shell-init:Output shell compatibility shim (source with <(...))'
        'uninstall:Remove alogin binary, completions, and config'
        'upgrade:Upgrade alogin to the latest release'
        'version:Print version'
      )
      _describe 'command' commands
      ;;

    args)
      case $words[1] in

        # ── compute (alias: server) ───────────────────────────────────────
        compute|server)
          _alogin_server_args
          ;;

        # ── access ────────────────────────────────────────────────────────
        access)
          local -a access_subcmds
          access_subcmds=(
            'ssh:SSH connection'
            'sftp:SFTP file transfer'
            'ftp:FTP connection'
            'mount:Mount remote filesystem via SSHFS'
            'cluster:Open cluster SSH sessions'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' access_subcmds ;;
            sub_args)
              case $words[1] in
                ssh|connect)
                  _arguments \
                    '--auto-gw[auto-detect gateway route (legacy r)]' \
                    '--dry-run[print connection route without connecting]' \
                    '(-c --cmd)'{-c,--cmd}'[run command after login]:command:' \
                    '*-L[local port forward]:spec:' \
                    '*--local-forward[local port forward]:spec:' \
                    '*-R[reverse port forward]:spec:' \
                    '*--remote-forward[reverse port forward]:spec:' \
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
                cluster) _alogin_cluster_args ;;
              esac
              ;;
          esac
          ;;

        # ── auth ──────────────────────────────────────────────────────────
        auth)
          local -a auth_subcmds
          auth_subcmds=(
            'gateway:Manage gateway routes'
            'alias:Manage host aliases'
            'vault:Vault backend operations'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' auth_subcmds ;;
            sub_args)
              case $words[1] in
                gateway) _alogin_gateway_args ;;
                alias)   _alogin_alias_args ;;
              esac
              ;;
          esac
          ;;

        # ── agent ─────────────────────────────────────────────────────────
        agent)
          local -a agent_subcmds
          agent_subcmds=(
            'mcp:Run alogin as an MCP server over stdio'
            'setup:Print MCP config and system prompt for AI clients'
            'policy:HITL/RBAC policy management (Phase 2)'
          )
          _arguments -C '1: :->sub'
          case $state in
            sub) _describe 'subcommand' agent_subcmds ;;
          esac
          ;;

        # ── net ───────────────────────────────────────────────────────────
        net)
          local -a net_subcmds
          net_subcmds=(
            'hosts:Manage local hostname→IP mappings'
            'tunnel:Manage persistent SSH port-forward tunnels'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' net_subcmds ;;
            sub_args)
              case $words[1] in
                hosts)  _alogin_hosts_cmd_args ;;
                tunnel) _alogin_tunnel_args ;;
              esac
              ;;
          esac
          ;;

        # ── Other root commands ───────────────────────────────────────────
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

        shell-init) _arguments '--shell[target shell]:shell:(zsh bash)' ;;

        web)
          _arguments \
            '(-p --port)'{-p,--port}'[HTTP port (default 8484)]:port:' \
            '--no-browser[do not open browser automatically]'
          ;;

        uninstall)
          _arguments \
            '--purge[also remove database and vault (irreversible)]' \
            '(-y --yes)'{-y,--yes}'[skip confirmation prompt]'
          ;;

        upgrade)
          _arguments \
            '(-y --yes)'{-y,--yes}'[skip confirmation prompt]'
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

  local commands="compute access auth agent net tui web migrate db-migrate completion shell-init uninstall upgrade version"

  # Helpers
  _alogin_hosts() {
    alogin server list 2>/dev/null | awk 'NR>2{print $3}'
  }
  _alogin_gateways() {
    alogin auth gateway list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_aliases() {
    alogin auth alias list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_clusters() {
    alogin access cluster list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_tunnels() {
    alogin net tunnel list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_hosts_entries() {
    alogin net hosts list 2>/dev/null | awk 'NR>2{print $1}'
  }

  local cmd="${words[1]}"
  local sub="${words[2]}"
  local sub2="${words[3]}"

  if [[ $cword -eq 1 ]]; then
    COMPREPLY=($(compgen -W "$commands" -- "$cur"))
    return
  fi

  case "$cmd" in
    # ── compute (alias: server) ─────────────────────────────────────────────
    compute|server)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "add list show delete passwd getpwd" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          show|delete|passwd|getpwd)
            COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur")) ;;
          add)
            COMPREPLY=($(compgen -W "--proto --host --user --port --gateway --locale" -- "$cur")) ;;
          list)
            COMPREPLY=($(compgen -W "--format" -- "$cur")) ;;
        esac
      fi
      ;;

    # ── access ─────────────────────────────────────────────────────────────
    access)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "ssh sftp ftp mount cluster" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          ssh|connect)
            if [[ "$cur" != -* ]]; then
              COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur"))
            else
              COMPREPLY=($(compgen -W "--auto-gw --dry-run --cmd -c -L --local-forward -R --remote-forward" -- "$cur"))
            fi
            ;;
          sftp|ftp|mount)
            COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur")) ;;
          cluster)
            local cluster_opts="--mode --auto-gw --tile-x -x"
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "list add $(_alogin_clusters) $cluster_opts" -- "$cur"))
            elif [[ "$sub2" == "add" ]]; then
              if [[ $cword -eq 4 ]]; then
                COMPREPLY=($(compgen -W "<cluster_name> $cluster_opts" -- "$cur"))
              else
                COMPREPLY=($(compgen -W "$(_alogin_hosts) $cluster_opts" -- "$cur"))
              fi
            elif [[ "$sub2" == "list" ]]; then
              COMPREPLY=($(compgen -W "--format" -- "$cur"))
            else
              COMPREPLY=($(compgen -W "$cluster_opts" -- "$cur"))
            fi
            ;;
        esac
      fi
      ;;

    # ── auth ────────────────────────────────────────────────────────────────
    auth)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "gateway alias vault" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          gateway)
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "add list show delete" -- "$cur"))
            elif [[ $cword -ge 4 ]]; then
              case "$sub2" in
                show|delete) COMPREPLY=($(compgen -W "$(_alogin_gateways)" -- "$cur")) ;;
                add)         COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur")) ;;
                list)        COMPREPLY=($(compgen -W "--format" -- "$cur")) ;;
              esac
            fi
            ;;
          alias)
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "add list show delete" -- "$cur"))
            elif [[ $cword -ge 4 ]]; then
              case "$sub2" in
                show|delete) COMPREPLY=($(compgen -W "$(_alogin_aliases)" -- "$cur")) ;;
                add)         COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur")) ;;
                list)        COMPREPLY=($(compgen -W "--format" -- "$cur")) ;;
              esac
            fi
            ;;
        esac
      fi
      ;;

    # ── agent ───────────────────────────────────────────────────────────────
    agent)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "mcp setup policy" -- "$cur"))
      fi
      ;;

    # ── net ─────────────────────────────────────────────────────────────────
    net)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "hosts tunnel" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          hosts)
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "add list show update delete" -- "$cur"))
            elif [[ $cword -ge 4 ]]; then
              case "$sub2" in
                show|update|delete) COMPREPLY=($(compgen -W "$(_alogin_hosts_entries)" -- "$cur")) ;;
                list) COMPREPLY=($(compgen -W "--format" -- "$cur")) ;;
              esac
            fi
            ;;
          tunnel)
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "list add edit rm start stop status" -- "$cur"))
            elif [[ $cword -ge 4 ]]; then
              case "$sub2" in
                start|stop|status|edit|rm) COMPREPLY=($(compgen -W "$(_alogin_tunnels)" -- "$cur")) ;;
                list) COMPREPLY=($(compgen -W "--format" -- "$cur")) ;;
              esac
            fi
            ;;
        esac
      fi
      ;;

    # ── Other root commands ──────────────────────────────────────────────────
    completion)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "zsh bash install" -- "$cur"))
      elif [[ $cword -ge 3 && "$sub" == "install" ]]; then
        COMPREPLY=($(compgen -W "--dir --shell" -- "$cur"))
      fi
      ;;
    shell-init) COMPREPLY=($(compgen -W "--shell" -- "$cur")) ;;
    web)        COMPREPLY=($(compgen -W "--port -p --no-browser" -- "$cur")) ;;
    uninstall)  COMPREPLY=($(compgen -W "--purge --yes -y" -- "$cur")) ;;
    upgrade)    COMPREPLY=($(compgen -W "--yes -y" -- "$cur")) ;;
  esac
}

complete -F _alogin_completion alogin
`
