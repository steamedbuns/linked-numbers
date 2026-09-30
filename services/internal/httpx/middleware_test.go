package httpx

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"testing"

	"github.com/steamedbuns/linked-numbers/services/internal/logging"
)

func serve(t *testing.T, req *http.Request) (*httptest.ResponseRecorder, map[string]any) {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("GET /values/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
	})
	mux.HandleFunc("GET /boom", func(http.ResponseWriter, *http.Request) {
		panic("boom")
	})
	var buf bytes.Buffer
	w := httptest.NewRecorder()
	Middleware(logging.New(&buf, slog.LevelDebug), mux).ServeHTTP(w, req)

	// The access line is the last one.
	lines := bytes.Split(bytes.TrimSpace(buf.Bytes()), []byte("\n"))
	var access map[string]any
	if err := json.Unmarshal(lines[len(lines)-1], &access); err != nil {
		t.Fatalf("access line is not JSON: %v\n%s", err, buf.String())
	}
	return w, access
}

func TestMiddlewareAccessLog(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/values/42", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	req.Header.Set("X-User-Id", "u_ana")

	_, access := serve(t, req)
	for key, want := range map[string]any{
		"level":    "INFO",
		"msg":      "request",
		"method":   "GET",
		"route":    "GET /values/{id}",
		"path":     "/values/42",
		"status":   418.0,
		"user_id":  "u_ana",
		"trace_id": "4bf92f3577b34da6a3ce929d0e0e4736",
	} {
		if access[key] != want {
			t.Errorf("%s = %v, want %v", key, access[key], want)
		}
	}
	if _, ok := access["duration_ms"]; !ok {
		t.Error("duration_ms missing")
	}
}

func TestMiddlewareGeneratesTraceID(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/values/42", nil)
	_, access := serve(t, req)
	id, _ := access["trace_id"].(string)
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(id) || id == "00000000000000000000000000000000" {
		t.Errorf("trace_id = %q, want a random 32-hex ID", id)
	}
}

func TestMiddlewareRecoversPanic(t *testing.T) {
	req := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/boom", nil)
	w, access := serve(t, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status = %d, want 500", w.Code)
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q, want application/problem+json", ct)
	}
	var p Problem
	if err := json.Unmarshal(w.Body.Bytes(), &p); err != nil || p.Status != 500 || p.Title != "Internal Server Error" {
		t.Errorf("body = %s (err %v), want a 500 problem", w.Body, err)
	}
	if access["level"] != "ERROR" || access["status"] != 500.0 {
		t.Errorf("access line = %v, want an ERROR line with status 500", access)
	}
}
