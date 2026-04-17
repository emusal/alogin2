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
  gws=(${(f)"$(alogin net gateway list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'gateway' gws
}

_alogin_aliases() {
  local -a aliases
  aliases=(${(f)"$(alogin server alias list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'alias' aliases
}

_alogin_clusters() {
  local -a clusters
  clusters=(${(f)"$(alogin ssh cluster list 2>/dev/null | awk 'NR>2{print $1}')"})
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

_alogin_apps() {
  local -a apps
  apps=(${(f)"$(alogin app list 2>/dev/null | awk 'NR>2{print $1}')"})
  _describe 'app' apps
}

_alogin_profiles() {
  local -a profiles
  profiles=(${(f)"$(alogin net profile list 2>/dev/null | awk 'NR>1{print $1}')"})
  _describe 'profile' profiles
}

# ---------------------------------------------------------------------------
# Subcommand completion helpers
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
    'alias:Manage server aliases'
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
        alias) _alogin_alias_args ;;
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
            '--remote-port[remote port]:port:'
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

_alogin_profile_args() {
  local -a profile_subcmds
  profile_subcmds=(
    'add:Add a network profile'
    'list:List all profiles'
    'show:Show profile details'
    'edit:Edit a profile'
    'delete:Delete a profile'
    'use:Activate a profile'
  )
  _arguments -C '1: :->psub' '*:: :->psub_args'
  case $state in
    psub) _describe 'subcommand' profile_subcmds ;;
    psub_args)
      case $words[1] in
        show|delete|use|edit)
          _arguments '1:profile name:($(_alogin_profiles))' ;;
        add)
          _arguments \
            '--gateway[gateway route name]:gateway:($(_alogin_gateways))' \
            '--desc[description]:desc:' \
            '1:profile name:'
          ;;
        edit)
          _arguments \
            '--gateway[gateway route name]:gateway:($(_alogin_gateways))' \
            '--desc[description]:desc:' \
            '1:profile name:($(_alogin_profiles))'
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
        # ── Main groups ───────────────────────────────────────────────────
        'server:Manage servers in the registry'
        'app:Manage app bindings (server + plugin shortcut)'
        'ssh:Connect to remote hosts (SSH, SFTP, FTP, cluster)'
        'vault:Manage stored credentials'
        'net:Manage network resources (hosts, tunnels, gateways, profiles)'
        'agent:AI/MCP tools: MCP server, setup, policy'
        # ── Interactive UIs ───────────────────────────────────────────────
        'tui:Interactive fuzzy host selector'
        'web:Start the web UI server'
        # ── System commands ───────────────────────────────────────────────
        'migrate:Import legacy alogin data files'
        'db-migrate:Apply pending database schema migrations'
        'doctor:Check and repair database integrity'
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

        # ── server ───────────────────────────────────────────────────────
        server)
          _alogin_server_args
          ;;

        # ── app ──────────────────────────────────────────────────────────
        app)
          local -a as_subcmds
          as_subcmds=(
            'list:List all app bindings'
            'add:Add a new app binding'
            'show:Show app binding details'
            'delete:Remove an app binding'
            'connect:Connect using an app binding'
            'plugin:Manage application plugins'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' as_subcmds ;;
            sub_args)
              case $words[1] in
                show|delete|connect) _alogin_apps ;;
                add)
                  _arguments \
                    '--name[binding name]:name:' \
                    '--server[server hostname]:host:_alogin_hosts' \
                    '--app[plugin name]:plugin:' \
                    '--desc[description]:desc:'
                  ;;
                list) _arguments '--format[output format]:format:(table json)' ;;
                connect) _arguments '--cmd[remote command]:command:' ;;
                plugin)
                  local -a plugin_subcmds
                  plugin_subcmds=('list:List installed application plugins')
                  _arguments -C '1: :->psub' '*:: :->psub_args'
                  case $state in
                    psub) _describe 'subcommand' plugin_subcmds ;;
                    psub_args)
                      case $words[1] in
                        list) _arguments '--format[output format]:format:(table json)' ;;
                      esac
                      ;;
                  esac
                  ;;
              esac
              ;;
          esac
          ;;

        # ── ssh ──────────────────────────────────────────────────────────
        ssh)
          local -a ssh_subcmds
          ssh_subcmds=(
            'connect:SSH connection'
            'sftp:SFTP file transfer'
            'ftp:FTP connection'
            'mount:Mount remote filesystem via SSHFS'
            'cluster:Open cluster SSH sessions'
            'session:Manage persistent stateful SSH sessions'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' ssh_subcmds ;;
            sub_args)
              case $words[1] in
                connect)
                  _arguments \
                    '--profile[network profile override]:profile:' \
                    '--dry-run[print connection route without connecting]' \
                    '(-c --cmd)'{-c,--cmd}'[run command after login]:command:' \
                    '*-L[local port forward]:spec:' \
                    '*--local-forward[local port forward]:spec:' \
                    '*-R[reverse port forward]:spec:' \
                    '*--remote-forward[reverse port forward]:spec:' \
                    '--app[application plugin to launch after connecting]:plugin:' \
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
                session)
                  local -a session_subcmds
                  session_subcmds=(
                    'start:Start a new stateful session'
                    'exec:Execute a command in a session'
                    'bg-exec:Run a command in the background and return a job ID'
                    'job:Inspect and manage background jobs'
                    'stop:Stop a session'
                    'list:List active sessions'
                  )
                  _arguments -C '1: :->session_sub' '*:: :->session_args'
                  case $state in
                    session_sub) _describe 'subcommand' session_subcmds ;;
                    session_args)
                      case $words[1] in
                        start)   _arguments '1: :_alogin_hosts' '--id[session name]:name:' ;;
                        exec)    _arguments '1:session-id:' '2:command:' '--timeout[timeout seconds]:seconds:' ;;
                        bg-exec) _arguments '1:session-id:' '2:command:' '--timeout[max execution seconds]:seconds:' ;;
                        stop)    _arguments '1:session-id:' ;;
                        list)    ;;
                        job)
                          local -a job_subcmds
                          job_subcmds=(
                            'status:Show job status'
                            'logs:Print job output'
                            'list:List all background jobs'
                            'cancel:Cancel a pending or running job'
                            'delete:Force-delete a job record (any status)'
                            'purge:Bulk-delete finished jobs'
                          )
                          _arguments -C '1: :->jsub' '*:: :->jsub_args'
                          case $state in
                            jsub) _describe 'subcommand' job_subcmds ;;
                            jsub_args)
                              case $words[1] in
                                status) _arguments '1:job-id:' '--json[JSON output]' ;;
                                logs)   _arguments '1:job-id:' ;;
                                list)   _arguments '--session[filter by session ID]:id:' '--json[JSON output]' ;;
                                cancel) _arguments '1:job-id:' ;;
                                delete) _arguments '1:job-id:' ;;
                                purge)  _arguments '--all[delete every job including pending and running]' ;;
                              esac ;;
                          esac ;;
                      esac
                      ;;
                  esac
                  ;;
              esac
              ;;
          esac
          ;;

        # ── scp ──────────────────────────────────────────────────────────
        scp)
          local -a scp_subcmds
          scp_subcmds=(
            'push:Upload a local file or directory to a remote host'
            'pull:Download a remote file or directory to the local host'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' scp_subcmds ;;
            sub_args)
              case $words[1] in
                push) _arguments '(-r --recursive)'{-r,--recursive}'[recursively upload a directory]' '1:local path:_files' '2:remote (host:/path):' ;;
                pull) _arguments '(-r --recursive)'{-r,--recursive}'[recursively download a directory]' '1:remote (host:/path):' '2:local path:_files' ;;
              esac
              ;;
          esac
          ;;

        # ── vault ─────────────────────────────────────────────────────────
        vault)
          local -a vault_subcmds
          vault_subcmds=(
            'set:Store a credential'
            'get:Retrieve a stored credential'
            'delete:Remove a stored credential'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' vault_subcmds ;;
            sub_args)
              case $words[1] in
                set|get|delete) _arguments '1:account (user@host):' ;;
              esac
              ;;
          esac
          ;;

        # ── net ───────────────────────────────────────────────────────────
        net)
          local -a net_subcmds
          net_subcmds=(
            'hosts:Manage local hostname→IP mappings'
            'tunnel:Manage persistent SSH port-forward tunnels'
            'gateway:Manage gateway routes'
            'profile:Manage network profiles'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' net_subcmds ;;
            sub_args)
              case $words[1] in
                hosts)   _alogin_hosts_cmd_args ;;
                tunnel)  _alogin_tunnel_args ;;
                gateway) _alogin_gateway_args ;;
                profile) _alogin_profile_args ;;
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
            'policy:Manage HITL/RBAC safety policies'
            'audit:Query the MCP execution audit log'
            'approve:Approve a pending HITL request'
            'deny:Deny a pending HITL request'
            'pending:List pending HITL approval requests'
            'trust:Grant a temporary trust window (auto-approve HITL)'
            'untrust:Revoke an active trust window'
            'trust-list:List active trust windows'
            'server-policy:Manage per-server policy overrides'
            'server-prompt:Manage per-server LLM system prompt overrides'
            'server-memory:Manage per-server AI agent memory notes'
          )
          _arguments -C '1: :->sub' '*:: :->sub_args'
          case $state in
            sub) _describe 'subcommand' agent_subcmds ;;
            sub_args)
              case $words[1] in
                policy)
                  _arguments -C '1: :->psub' '*:: :->psub_args'
                  case $state in
                    psub)
                      local -a policy_subcmds
                      policy_subcmds=('show:Print active policy' 'validate:Validate policy file' 'dry-run:Check policy decision without executing')
                      _describe 'subcommand' policy_subcmds ;;
                    psub_args)
                      case $words[1] in
                        dry-run) _arguments '--cmd[Command to evaluate]:cmd' '--agent[Agent ID]:agent' '--server[Server ID]:server' '--json[JSON output]' ;;
                      esac ;;
                  esac ;;
                audit)
                  local -a audit_subcmds
                  audit_subcmds=('list:List recent audit events' 'tail:Stream new audit events')
                  _describe 'subcommand' audit_subcmds ;;
                trust)
                  _arguments \
                    '--duration[trust duration (e.g. 30m, 1h, 2h)]:duration:' \
                    '--agent[restrict to this agent ID]:agent:' \
                    '--server[restrict to this server ID]:server:' ;;
                untrust)
                  _arguments \
                    '--agent[agent scope to revoke]:agent:' \
                    '--server[server scope to revoke]:server:' ;;
                trust-list)
                  _arguments '--json[JSON output]' ;;
                server-policy|server-prompt)
                  local -a sp_subcmds
                  sp_subcmds=('set:Set value' 'show:Show value' 'clear:Clear value')
                  _describe 'subcommand' sp_subcmds ;;
                server-memory)
                  local -a sm_subcmds
                  sm_subcmds=('add:Add a memory note' 'list:List memory notes' 'del:Delete a memory note')
                  _describe 'subcommand' sm_subcmds ;;
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

        doctor)
          _arguments \
            '--fix[automatically repair issues where safe to do so]'
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

  local commands="server app ssh scp vault net agent tui web migrate db-migrate doctor completion shell-init uninstall upgrade version"

  # Helpers
  _alogin_hosts() {
    alogin server list 2>/dev/null | awk 'NR>2{print $3}'
  }
  _alogin_gateways() {
    alogin net gateway list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_aliases() {
    alogin server alias list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_clusters() {
    alogin ssh cluster list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_tunnels() {
    alogin net tunnel list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_hosts_entries() {
    alogin net hosts list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_apps() {
    alogin app list 2>/dev/null | awk 'NR>2{print $1}'
  }
  _alogin_profiles() {
    alogin net profile list 2>/dev/null | awk 'NR>1{print $1}'
  }

  local cmd="${words[1]}"
  local sub="${words[2]}"
  local sub2="${words[3]}"

  if [[ $cword -eq 1 ]]; then
    COMPREPLY=($(compgen -W "$commands" -- "$cur"))
    return
  fi

  case "$cmd" in
    # ── server ──────────────────────────────────────────────────────────────
    server)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "add list show delete passwd getpwd alias" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          show|delete|passwd|getpwd)
            COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur")) ;;
          add)
            COMPREPLY=($(compgen -W "--proto --host --user --port --gateway --locale" -- "$cur")) ;;
          list)
            COMPREPLY=($(compgen -W "--format" -- "$cur")) ;;
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

    # ── app ─────────────────────────────────────────────────────────────────
    app)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "list add show delete connect plugin" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          show|delete|connect) COMPREPLY=($(compgen -W "$(_alogin_apps)" -- "$cur")) ;;
          add)    COMPREPLY=($(compgen -W "--name --server --app --desc" -- "$cur")) ;;
          list)   COMPREPLY=($(compgen -W "--format" -- "$cur")) ;;
          connect) COMPREPLY=($(compgen -W "--cmd $(_alogin_apps)" -- "$cur")) ;;
          plugin)
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "list" -- "$cur"))
            elif [[ $cword -ge 4 && "$sub2" == "list" ]]; then
              COMPREPLY=($(compgen -W "--format" -- "$cur"))
            fi
            ;;
        esac
      fi
      ;;

    # ── ssh ─────────────────────────────────────────────────────────────────
    ssh)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "connect sftp ftp mount cluster session" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          connect)
            if [[ "$cur" != -* ]]; then
              COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur"))
            else
              COMPREPLY=($(compgen -W "--profile --dry-run --cmd -c -L --local-forward -R --remote-forward --app" -- "$cur"))
            fi
            ;;
          sftp|ftp|mount)
            COMPREPLY=($(compgen -W "$(_alogin_hosts)" -- "$cur")) ;;
          cluster)
            local cluster_opts="--mode --tile-x -x"
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
          session)
            local sub3="${words[4]}"
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "start exec bg-exec job stop list" -- "$cur"))
            elif [[ $cword -eq 4 ]]; then
              case "$sub2" in
                start)   COMPREPLY=($(compgen -W "$(_alogin_hosts) --id" -- "$cur")) ;;
                exec)    COMPREPLY=($(compgen -W "--timeout" -- "$cur")) ;;
                bg-exec) COMPREPLY=($(compgen -W "--timeout" -- "$cur")) ;;
                job)     COMPREPLY=($(compgen -W "status logs list cancel delete purge" -- "$cur")) ;;
              esac
            elif [[ $cword -ge 5 && "$sub2" == "job" ]]; then
              case "$sub3" in
                status) COMPREPLY=($(compgen -W "--json" -- "$cur")) ;;
                list)   COMPREPLY=($(compgen -W "--session --json" -- "$cur")) ;;
                purge)  COMPREPLY=($(compgen -W "--all" -- "$cur")) ;;
              esac
            fi
            ;;
        esac
      fi
      ;;

    # ── scp ─────────────────────────────────────────────────────────────────
    scp)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "push pull" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          push) COMPREPLY=($(compgen -W "--recursive -r" -- "$cur")) ;;
          pull) COMPREPLY=($(compgen -W "--recursive -r" -- "$cur")) ;;
        esac
      fi
      ;;

    # ── vault ───────────────────────────────────────────────────────────────
    vault)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "set get delete" -- "$cur"))
      fi
      ;;

    # ── net ─────────────────────────────────────────────────────────────────
    net)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "hosts tunnel gateway profile" -- "$cur"))
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
          profile)
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "add list show edit delete use" -- "$cur"))
            elif [[ $cword -ge 4 ]]; then
              case "$sub2" in
                show|delete|use|edit) COMPREPLY=($(compgen -W "$(_alogin_profiles)" -- "$cur")) ;;
                add|edit)  COMPREPLY=($(compgen -W "--gateway --desc" -- "$cur")) ;;
                list)      COMPREPLY=($(compgen -W "--format" -- "$cur")) ;;
              esac
            fi
            ;;
        esac
      fi
      ;;

    # ── agent ───────────────────────────────────────────────────────────────
    agent)
      if [[ $cword -eq 2 ]]; then
        COMPREPLY=($(compgen -W "mcp setup policy audit approve deny pending trust untrust trust-list server-policy server-prompt server-memory" -- "$cur"))
      elif [[ $cword -ge 3 ]]; then
        case "$sub" in
          policy)
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "show validate dry-run" -- "$cur"))
            elif [[ $cword -ge 4 && "$sub2" == "dry-run" ]]; then
              COMPREPLY=($(compgen -W "--cmd --agent --server --json" -- "$cur"))
            fi
            ;;
          audit)
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "list tail" -- "$cur"))
            elif [[ $cword -ge 4 ]]; then
              case "$sub2" in
                list) COMPREPLY=($(compgen -W "--agent --server --event --since --limit --json" -- "$cur")) ;;
              esac
            fi
            ;;
          trust)
            COMPREPLY=($(compgen -W "--duration --agent --server" -- "$cur")) ;;
          untrust)
            COMPREPLY=($(compgen -W "--agent --server" -- "$cur")) ;;
          trust-list)
            COMPREPLY=($(compgen -W "--json" -- "$cur")) ;;
          server-policy|server-prompt)
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "set show clear" -- "$cur"))
            fi
            ;;
          server-memory)
            if [[ $cword -eq 3 ]]; then
              COMPREPLY=($(compgen -W "add list del" -- "$cur"))
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
    doctor)     COMPREPLY=($(compgen -W "--fix" -- "$cur")) ;;
    uninstall)  COMPREPLY=($(compgen -W "--purge --yes -y" -- "$cur")) ;;
    upgrade)    COMPREPLY=($(compgen -W "--yes -y" -- "$cur")) ;;
  esac
}

complete -F _alogin_completion alogin
`
