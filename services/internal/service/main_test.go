package service

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"
)

// TestHelperProcess is the child process for TestMainShutsDownOnSIGTERM. It
// runs Main like a real binary would.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("LN_SERVICE_HELPER") != "1" {
		t.Skip("helper process for TestMainShutsDownOnSIGTERM")
	}
	Service{Name: "helper", DefaultAddr: "127.0.0.1:0", Routes: func(mux *http.ServeMux) {
		mux.HandleFunc("GET /slow", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_ = http.NewResponseController(w).Flush()
			time.Sleep(500 * time.Millisecond)
			_, _ = io.WriteString(w, "done")
		})
	}}.Main()
	os.Exit(0) // skip the test framework's own output
}

func TestMainShutsDownOnSIGTERM(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestHelperProcess$") //nolint:gosec // re-runs this test binary
	cmd.Env = append(os.Environ(), "LN_SERVICE_HELPER=1", "HTTP_ADDR=127.0.0.1:0", "LOG_LEVEL=info")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	// Every stdout line must be a JSON log record.
	lines := bufio.NewScanner(stdout)
	next := func() map[string]any {
		if !lines.Scan() {
			return nil
		}
		var rec map[string]any
		if err := json.Unmarshal(lines.Bytes(), &rec); err != nil {
			t.Errorf("stdout line is not JSON: %q", lines.Text())
		}
		return rec
	}
	var addr string
	for addr == "" {
		rec := next()
		if rec == nil {
			break
		}
		if rec["msg"] == "listening" {
			addr, _ = rec["addr"].(string)
		}
	}
	if addr == "" {
		t.Fatalf("no listening line; stderr:\n%s", stderr.String())
	}

	// Headers arrive before the handler sleeps, so the request is in flight
	// when SIGTERM lands.
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+addr+"/slow", nil)
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	begin := time.Now()
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	if body, err := io.ReadAll(resp.Body); err != nil || string(body) != "done" {
		t.Errorf("in-flight request body = %q (err %v), want done", body, err)
	}

	var msgs []any
	for rec := next(); rec != nil; rec = next() {
		msgs = append(msgs, rec["msg"])
	}
	if err := cmd.Wait(); err != nil {
		t.Errorf("process exit = %v, want status 0; stderr:\n%s", err, stderr.String())
	}
	if elapsed := time.Since(begin); elapsed > MaxShutdownTimeout {
		t.Errorf("process exited %s after SIGTERM, want under %s", elapsed, MaxShutdownTimeout)
	}
	if len(msgs) < 2 || msgs[0] != "shutting down" || msgs[len(msgs)-1] != "stopped" {
		t.Errorf("log messages after SIGTERM = %v, want shutting down ... stopped", msgs)
	}
}
