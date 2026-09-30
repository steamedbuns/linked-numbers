package httpx

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

// Middleware wraps next so that every request:
//   - carries a trace context, taken from its traceparent header or, if it has
//     none, newly generated, so every log line for it has a trace_id;
//   - turns a panic into a logged 500 problem response;
//   - writes one access log line when it finishes: at error for a 5xx, and at
//     debug for health probes, which run every few seconds.
func Middleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ctx := withTraceContext(r)
		r = r.WithContext(ctx)
		rec := &statusRecorder{ResponseWriter: w}

		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler { //nolint:errorlint // sentinel compared as net/http does
					panic(v)
				}
				logger.ErrorContext(ctx, "panic serving request", "panic", fmt.Sprint(v), "stack", string(debug.Stack()))
				if rec.status == 0 {
					WriteProblem(rec, http.StatusInternalServerError, "")
				}
			}
			level := slog.LevelInfo
			switch {
			case r.URL.Path == "/healthz" || r.URL.Path == "/readyz":
				level = slog.LevelDebug
			case rec.status >= 500:
				level = slog.LevelError
			}
			logger.Log(ctx, level, "request",
				"method", r.Method,
				"route", r.Pattern,
				"path", r.URL.Path,
				"status", rec.statusOrOK(),
				"duration_ms", time.Since(start).Milliseconds(),
				"user_id", r.Header.Get("X-User-Id"),
			)
		}()
		next.ServeHTTP(rec, r)
	})
}

// withTraceContext returns the request context with the caller's trace
// context, or a new random one. LN-7.1 replaces the random IDs with real
// spans from the OTel SDK.
func withTraceContext(r *http.Request) context.Context {
	ctx := propagation.TraceContext{}.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
	if trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}
	var cfg trace.SpanContextConfig
	_, _ = rand.Read(cfg.TraceID[:])
	_, _ = rand.Read(cfg.SpanID[:])
	return trace.ContextWithSpanContext(ctx, trace.NewSpanContext(cfg))
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}

// Unwrap lets http.ResponseController reach the underlying writer.
func (r *statusRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

func (r *statusRecorder) statusOrOK() int {
	if r.status == 0 {
		return http.StatusOK
	}
	return r.status
}
