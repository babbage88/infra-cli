package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/babbage88/tint"
)

// CustomHandler wraps another slog.Handler and modifies the time format
type CustomHandler struct {
	slog.Handler
}

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	// Format time the way you want (e.g., RFC3339 with milliseconds)
	formattedTime := r.Time.Format("2006-01-02 15:04:05.000")

	// Create a new record with formatted time as an attribute
	r = r.Clone() // avoid mutating original
	r.AddAttrs(slog.String("timestamp", formattedTime))

	return h.Handler.Handle(ctx, r)
}

func configureDefaultLogger(level slog.Level) {
	w := os.Stderr

	// set global logger with custom options
	slog.SetDefault(slog.New(
		tint.NewHandler(w, &tint.Options{
			Level:      level,
			TimeFormat: time.Kitchen,
		}),
	))
}
