package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/steamedbuns/linked-numbers/services/internal/httpx"
	"github.com/steamedbuns/linked-numbers/services/internal/logging"
)

// start runs s on a random local port until the test ends or cancel is
// called. Run's result arrives on the returned channel.
func start(t *testing.T, s Service, cfg Config) (base string, cancel context.CancelFunc, done <-chan error) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0") //nolint:noctx // test listener
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	ch := make(chan error, 1)
	go func() { ch <- s.Run(ctx, cfg, logging.New(io.Discard, slog.LevelInfo), ln) }()
	return "http://" + ln.Addr().String(), cancel, ch
}

func get(t *testing.T, url string) (status int, header http.Header, body []byte) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err = io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp.StatusCode, resp.Header, body
}

var testConfig = Config{ShutdownTimeout: 2 * time.Second}

func TestHealthz(t *testing.T) {
	base, _, _ := start(t, Service{Ready: []Check{{"db", func(context.Context) error { return errors.New("down") }}}}, testConfig)
	status, _, body := get(t, base+"/healthz")
	if status != http.StatusOK || strings.TrimSpace(string(body)) != `{"status":"ok"}` {
		t.Errorf("GET /healthz = %d %s, want 200 {\"status\":\"ok\"} even when not ready", status, body)
	}
}

func TestReadyz(t *testing.T) {
	ok := func(context.Context) error { return nil }
	down := func(context.Context) error { return errors.New("connection refused") }

	t.Run("all checks pass", func(t *testing.T) {
		base, _, _ := start(t, Service{Ready: []Check{{"db", ok}, {"cache", ok}}}, testConfig)
		status, _, body := get(t, base+"/readyz")
		if status != http.StatusOK || strings.TrimSpace(string(body)) != `{"status":"ok"}` {
			t.Errorf("GET /readyz = %d %s, want 200", status, body)
		}
	})

	t.Run("a check fails", func(t *testing.T) {
		base, _, _ := start(t, Service{Ready: []Check{{"db", down}, {"cache", ok}}}, testConfig)
		status, header, body := get(t, base+"/readyz")
		if status != http.StatusServiceUnavailable {
			t.Errorf("status = %d, want 503", status)
		}
		if ct := header.Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("Content-Type = %q, want application/problem+json", ct)
		}
		var p httpx.Problem
		if err := json.Unmarshal(body, &p); err != nil {
			t.Fatal(err)
		}
		if p.Detail != "not ready: db: connection refused" {
			t.Errorf("detail = %q, want it to name the failing check only", p.Detail)
		}
	})
}

func TestRunWaitsForInFlightRequests(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	s := Service{Routes: func(mux *http.ServeMux) {
		mux.HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
			close(started)
			<-release
			_, _ = io.WriteString(w, "done")
		})
	}}
	base, cancel, done := start(t, s, testConfig)

	got := make(chan string, 1)
	go func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/slow", nil)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			got <- err.Error()
			return
		}
		defer func() { _ = resp.Body.Close() }()
		body, _ := io.ReadAll(resp.Body)
		got <- string(body)
	}()
	<-started
	cancel()
	time.Sleep(50 * time.Millisecond) // let Shutdown close the listener
	close(release)

	if body := <-got; body != "done" {
		t.Errorf("in-flight request body = %q, want done", body)
	}
	if err := <-done; err != nil {
		t.Errorf("Run() = %v, want nil after a graceful shutdown", err)
	}
}

func TestRunForcesCloseAfterShutdownTimeout(t *testing.T) {
	started := make(chan struct{})
	s := Service{Routes: func(mux *http.ServeMux) {
		mux.HandleFunc("GET /stuck", func(_ http.ResponseWriter, r *http.Request) {
			close(started)
			<-r.Context().Done() // only a forced close ends this request
		})
	}}
	base, cancel, done := start(t, s, Config{ShutdownTimeout: 200 * time.Millisecond})

	go func() {
		req, _ := http.NewRequestWithContext(t.Context(), http.MethodGet, base+"/stuck", nil)
		if resp, err := http.DefaultClient.Do(req); err == nil {
			_ = resp.Body.Close()
		}
	}()
	<-started
	begin := time.Now()
	cancel()

	err := <-done
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Run() = %v, want a deadline error", err)
	}
	if elapsed := time.Since(begin); elapsed > time.Second {
		t.Errorf("Run() returned after %s, want about 200ms", elapsed)
	}
}
