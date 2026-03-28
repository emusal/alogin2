// Package web provides the embedded HTTP server for the alogin Web UI.
package web

import (
	"context"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/emusal/alogin2/internal/db"
	"github.com/emusal/alogin2/internal/vault"
	"github.com/emusal/alogin2/internal/web/api"
	"github.com/emusal/alogin2/internal/web/terminal"
	webpkg "github.com/emusal/alogin2/web"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

// Server is the alogin Web UI HTTP server.
type Server struct {
	database    *db.DB
	vlt         vault.Vault
	port        int
	openBrowser bool
	pluginDir   string
}

// NewServer creates a web server backed by the given DB and vault.
func NewServer(database *db.DB, vlt vault.Vault, port int, openBrowser bool) *Server {
	return &Server{database: database, vlt: vlt, port: port, openBrowser: openBrowser}
}

// WithPluginDir sets the plugin directory for the web server.
func (s *Server) WithPluginDir(dir string) *Server {
	s.pluginDir = dir
	return s
}

// Run starts the HTTP server and blocks until ctx is cancelled.
func (s *Server) Run(ctx context.Context) error {
	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	// REST API
	binPath, _ := os.Executable()
	apiHandler := api.NewHandlerWithBin(s.database, s.vlt, binPath).WithPluginDir(s.pluginDir)
	r.Mount("/api", apiHandler.Router())

	// WebSocket terminal
	wsHandler := terminal.NewHandler(s.database, s.vlt).WithPluginDir(s.pluginDir)
	r.Get("/ws/terminal/{serverID}", wsHandler.ServeWS)

	// Static frontend
	r.Handle("/*", staticHandler())

	addr := fmt.Sprintf(":%d", s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("listen %s: %w", addr, err)
	}

	fmt.Printf("alogin web UI: http://localhost%s\n", addr)
	if s.openBrowser {
		launchBrowser(fmt.Sprintf("http://localhost%s", addr))
	}

	srv := &http.Server{Handler: r}
	go func() {
		<-ctx.Done()
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		srv.Shutdown(shutCtx)
	}()

	return srv.Serve(ln)
}

func staticHandler() http.Handler {
	// Try embedded FS first (production build)
	if sub, err := fs.Sub(webpkg.FS, "frontend/dist"); err == nil {
		return http.FileServer(http.FS(sub))
	}
	// Development fallback: serve from filesystem
	if _, err := os.Stat("web/frontend/dist"); err == nil {
		return http.FileServer(http.Dir("web/frontend/dist"))
	}
	// No frontend built yet — return a helpful message
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" || r.URL.Path == "" {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, frontendPlaceholder)
			return
		}
		http.NotFound(w, r)
	})
}

func launchBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	default:
		return
	}
	_ = cmd.Start()
}

const frontendPlaceholder = `<!DOCTYPE html>
<html>
<head><title>alogin Web UI</title>
<style>body{font-family:monospace;background:#1a1a2e;color:#e0e0e0;display:flex;
align-items:center;justify-content:center;height:100vh;margin:0;}
.box{text-align:center;border:1px solid #444;padding:2em 3em;border-radius:8px;}
h1{color:#c792ea;}pre{color:#82aaff;text-align:left;}</style>
</head>
<body><div class="box">
<h1>alogin Web UI</h1>
<p>Frontend not built yet.</p>
<pre>cd v2/web/frontend
npm install
npm run build</pre>
<p>API is available at <a href="/api/servers" style="color:#82aaff">/api/servers</a></p>
</div></body></html>`
