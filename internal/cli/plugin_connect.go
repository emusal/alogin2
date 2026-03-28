package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/model"
	"github.com/emusal/alogin2/internal/plugin"
	internalssh "github.com/emusal/alogin2/internal/ssh"
)

// runPlugin loads the named plugin, resolves credentials and runtime strategy,
// then launches the application over the already-established SSH connection.
func runPlugin(ctx context.Context, pluginName string, client *internalssh.Client, srv *model.Server, extraCmd string) error {
	if cfg == nil {
		return fmt.Errorf("config not initialised")
	}
	pluginPath := filepath.Join(plugin.PluginDir(cfg.ConfigDir), pluginName+".yaml")
	p, err := plugin.LoadFromFile(pluginPath)
	if err != nil {
		return fmt.Errorf("load plugin %q: %w", pluginName, err)
	}

	runner := newSSHRunner(client)

	sess, err := plugin.Prepare(ctx, p, vlt, runner)
	if err != nil {
		return fmt.Errorf("prepare plugin %q: %w", pluginName, err)
	}

	if extraCmd != "" {
		flag := p.Runtime.CmdFlag
		if flag == "" {
			flag = "-e"
		}
		sess.ExtraArgs = []string{flag, extraCmd}
	}

	logPluginExec(ctx, p.Name, srv, sess)

	out, err := sess.Launch(ctx, runner)
	if out != "" {
		fmt.Print(out)
	}
	return err
}

// logPluginExec writes a plugin_exec entry to the audit_log table.
// It records variable names only — never their values.
func logPluginExec(ctx context.Context, pluginName string, srv *model.Server, sess *plugin.Session) {
	if database == nil || database.AuditLog == nil {
		return
	}
	srvID := srv.ID
	entry := db.AuditEntry{
		Timestamp:      time.Now().UTC().Format(time.RFC3339),
		Event:          "plugin_exec",
		ServerID:       &srvID,
		ServerHost:     srv.Host,
		PluginName:     pluginName,
		PluginVars:     sess.AuditVars(),
		PluginStrategy: sess.Strategy.Kind,
	}
	_, _ = database.AuditLog.Insert(ctx, entry)
}
