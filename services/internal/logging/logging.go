// Package logging builds the JSON slog logger every service uses.
package logging

import (
	"context"
	"io"
	"log/slog"

	"go.opentelemetry.io/otel/trace"
)

// New returns a logger that writes one JSON object per line to w. Records
// logged with a context that carries a valid span context get trace_id and
// span_id attributes, so use the *Context methods (sloglint enforces this).
// After Logger.WithGroup they land inside the group, so prefer slog.Group
// attributes to grouped loggers.
func New(w io.Writer, level slog.Leveler) *slog.Logger {
	return slog.New(traceHandler{slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level})})
}

type traceHandler struct{ slog.Handler }

func (h traceHandler) Handle(ctx context.Context, r slog.Record) error {
	if sc := trace.SpanContextFromContext(ctx); sc.IsValid() {
		r.AddAttrs(
			slog.String("trace_id", sc.TraceID().String()),
			slog.String("span_id", sc.SpanID().String()),
		)
	}
	return h.Handler.Handle(ctx, r)
}

func (h traceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return traceHandler{h.Handler.WithAttrs(attrs)}
}

func (h traceHandler) WithGroup(name string) slog.Handler {
	return traceHandler{h.Handler.WithGroup(name)}
}
