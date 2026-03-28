# TUI Structure

Framework: [Bubbletea](https://github.com/charmbracelet/bubbletea) (charmbracelet)
Entry: `internal/cli/tui.go` → `internal/tui/model.go`

---

## State machine

The TUI is a single `Model` with a `state` field that drives all rendering and key handling.

| State constant | Screen |
|---------------|--------|
| `stateWelcome` | Landing/welcome screen |
| `stateList` | Server list with fuzzy search (default) |
| `stateDetail` | Server detail panel (overlay on list) |
| `stateServerForm` | Add / edit server |
| `stateConfirmDelete` | Delete confirmation dialog |
| `stateGatewayList` | Gateway list |
| `stateGatewayForm` | Add / edit gateway |
| `stateClusterList` | Cluster list |
| `stateClusterForm` | Add / edit cluster |
| `stateHostList` | Local hosts list |
| `stateHostForm` | Add / edit local host |
| `stateTunnelList` | Tunnel list with start/stop actions |
| `stateTunnelForm` | Add / edit tunnel |
| `statePluginPicker` | Plugin picker overlay (server list) |
| `stateAppServerList` | App-server binding list |
| `stateAppServerForm` | Add / edit app-server binding |

### StartAt values (from `tui.go`)

| Constant | Opens at |
|----------|----------|
| `StartAtWelcome` | Welcome screen |
| `StartAtList` | Server list |
| `StartAtGateway` | Gateway list |
| `StartAtCluster` | Cluster list |
| `StartAtHosts` | Local hosts list |
| `StartAtTunnel` | Tunnel list |
| `StartAtAppServer` | App-server list |

---

## Key bindings

### Global
| Key | Action |
|-----|--------|
| `Ctrl+C` | Quit |

### List screens (server / gateway / cluster / hosts / tunnel)
| Key | Action |
|-----|--------|
| `↑` / `k` | Move up |
| `↓` / `j` | Move down |
| `tab` | Open detail panel (server list only) |
| `esc` | Back / clear search |
| `q` | Quit or clear search query |
| `/` | Enter command-palette mode |

### Server list extras
| Key | Action |
|-----|--------|
| `enter` | Connect (direct) |
| `r` | Connect via gateway |
| `a` | Add server |
| `e` | Edit selected server |
| `d` | Delete selected server |
| `p` | Open plugin picker overlay |

### Tunnel list extras
| Key | Action |
|-----|--------|
| `s` | Start tunnel |
| `x` | Stop tunnel |
| `a` | Add tunnel |
| `e` | Edit tunnel |
| `d` | Delete tunnel |

### Command palette (`/` prefix)
| Command | Navigates to |
|---------|-------------|
| `/compute` | Server list |
| `/gateway` | Gateway list |
| `/cluster` | Cluster list |
| `/hosts` | Local hosts list |
| `/tunnel` | Tunnel list |
| `/app-server` | App-server list |

### Forms (all)
| Key | Action |
|-----|--------|
| `tab` | Next field |
| `shift+tab` | Previous field |
| `enter` | Submit form |
| `esc` | Cancel / back |

### Gateway form extras
| Key | Action |
|-----|--------|
| `a` / `enter` | Add hop |
| `x` / `backspace` | Remove hop |
| `u` | Move hop up |
| `m` | Move hop down |

### Cluster form extras
| Key | Action |
|-----|--------|
| `r` | Edit user override for selected member |
| `u` | Move member up |
| `m` | Move member down |
| `a` / `enter` | Add member |

### Tunnel form extras
| Key | Action |
|-----|--------|
| `space` | Toggle `auto_gw` |
| `Ctrl+S` | Submit form |

### App-server list extras
| Key | Action |
|-----|--------|
| `a` | Add app-server binding |
| `e` | Edit selected binding |
| `d` | Delete selected binding |
| `enter` | Connect and quit |

---

## Model structure (key fields)

```go
type Model struct {
    // Data
    servers     []model.Server
    filtered    []model.Server
    gateways    []model.GatewayRoute
    clusters    []model.Cluster
    localHosts  []model.LocalHost
    tunnels     []model.Tunnel

    // List state
    cursor      int
    query       string
    statusMsg   string

    // Form state
    state          appState
    formFields     []textinput.Model
    formFocusIdx   int
    formMode       formMode   // modeAdd | modeEdit

    // Gateway form
    gwFormHops        []model.Server
    gwFormPickerOpen  bool

    // Cluster form
    clFormMembers      []clMember
    clFormUserEditOpen bool

    // Tunnel form
    tnFormAutoGW bool

    // App-server form state
    appServers      []*model.AppServer
    appServerCursor int
    asFormMode      formMode
    asFormFields    []textinput.Model
    asFormFocus     int
    asFormAutoGW    bool
    asFormServerID  int64
    asFormTarget    *model.AppServer
}
```

---

## File map

| File | Contents |
|------|----------|
| `internal/tui/model.go` | `Model` struct, `Init`, `Update`, `View`, state constants |
| `internal/tui/keys.go` | Key binding definitions |
| `internal/tui/messages.go` | Bubbletea `Msg` types for async results |
| `internal/tui/list.go` | List rendering helpers |
| `internal/tui/locale.go` | Locale / theme helpers |

---

## Modifying the TUI

- **Add a new screen**: add a `stateXxx` constant, handle in `Update` switch, add `View` case.
- **Add a new form field**: add to `formFields` slice in the relevant `initXxxForm()` helper; increment focus index bounds.
- **Add a new list panel**: add `StartAtXxx`, wire in `tui.go`, add slash-command alias.
- **Tunnel status polling**: tunnel list polls `IsRunning` via a timed `Cmd`; see `pollTunnelStatus()` in `model.go`.
