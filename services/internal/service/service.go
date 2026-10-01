package service

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/steamedbuns/linked-numbers/services/internal/buildinfo"
	"github.com/steamedbuns/linked-numbers/services/internal/httpx"
	"github.com/steamedbuns/linked-numbers/services/internal/logging"
)

// readyTimeout bounds all readiness checks of one /readyz request together.
const readyTimeout = 2 * time.Second

// Check is a named readiness check, such as a database ping.
type Check struct {
	Name string
	Fn   func(context.Context) error
}

// Service describes one binary. Its main func calls Main.
type Service struct {
	Name        string                   // logged as "service" on every line
	DefaultAddr string                   // listen address when HTTP_ADDR is unset
	Routes      func(mux *http.ServeMux) // registers the service's handlers; may be nil
	Ready       []Check                  // all must pass for /readyz to return 200

	// Init, if set, runs before serving, to build what needs the config,
	// such as a database pool. An error stops the service. The Deps it
	// returns add to Routes and Ready.
	Init func(ctx context.Context, cfg Config, logger *slog.Logger) (Deps, error)
}

// Deps is what Service.Init builds.
type Deps struct {
	Routes func(mux *http.ServeMux) // may be nil
	Ready  []Check
	Close  func() // runs after the server has stopped; may be nil
}

type health struct {
	Status string `json:"status"`
}

// Main loads the config from the environment, serves until SIGINT or
// SIGTERM, then shuts down gracefully. It exits the process with status 1 if
// the config is invalid, the listener fails, or shutdown overruns
// SHUTDOWN_TIMEOUT.
func (s Service) Main() {
	if err := s.main(); err != nil {
		os.Exit(1)
	}
}

func (s Service) main() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := LoadConfig(os.Getenv, s.DefaultAddr)
	logger := logging.New(os.Stdout, cfg.LogLevel).With("service", s.Name, "version", buildinfo.Version())
	if err != nil {
		logger.ErrorContext(ctx, "invalid config", "error", err)
		return err
	}
	var lc net.ListenConfig
	ln, err := lc.Listen(ctx, "tcp", cfg.Addr)
	if err != nil {
		logger.ErrorContext(ctx, "listen failed", "error", err)
		return err
	}
	if err := s.Run(ctx, cfg, logger, ln); err != nil {
		logger.ErrorContext(ctx, "service failed", "error", err)
		return err
	}
	return nil
}

// Run calls Init, if set, then serves HTTP on ln until ctx is done. Then it
// stops accepting connections and waits up to cfg.ShutdownTimeout for
// in-flight requests. If they don't finish in time, it closes their
// connections and returns an error. Deps.Close runs last either way.
func (s Service) Run(ctx context.Context, cfg Config, logger *slog.Logger, ln net.Listener) error {
	var deps Deps
	if s.Init != nil {
		var err error
		if deps, err = s.Init(ctx, cfg, logger); err != nil {
			_ = ln.Close()
			return fmt.Errorf("init: %w", err)
		}
		if deps.Close != nil {
			defer deps.Close()
		}
	}
	ready := append(slices.Clip(s.Ready), deps.Ready...)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, _ *http.Request) {
		httpx.WriteJSON(w, http.StatusOK, health{Status: "ok"})
	})
	mux.HandleFunc("GET /readyz", readyz(logger, ready))
	for _, routes := range []func(*http.ServeMux){s.Routes, deps.Routes} {
		if routes != nil {
			routes(mux)
		}
	}
	srv := &http.Server{
		Handler:           httpx.Middleware(logger, mux),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
		ErrorLog:          slog.NewLogLogger(logger.Handler(), slog.LevelError),
	}

	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()
	logger.InfoContext(ctx, "listening", "addr", ln.Addr().String())

	select {
	case err := <-serveErr:
		return fmt.Errorf("serve: %w", err)
	case <-ctx.Done():
	}

	logger.InfoContext(ctx, "shutting down", "timeout", cfg.ShutdownTimeout.String())
	shutdownCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), cfg.ShutdownTimeout)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		_ = srv.Close()
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	logger.InfoContext(ctx, "stopped")
	return nil
}

func readyz(logger *slog.Logger, checks []Check) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), readyTimeout)
		defer cancel()
		var failed []string
		for _, c := range checks {
			if err := c.Fn(ctx); err != nil {
				failed = append(failed, fmt.Sprintf("%s: %v", c.Name, err))
			}
		}
		if len(failed) > 0 {
			detail := "not ready: " + strings.Join(failed, "; ")
			logger.WarnContext(r.Context(), detail)
			httpx.WriteProblem(w, http.StatusServiceUnavailable, detail)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, health{Status: "ok"})
	}
}
