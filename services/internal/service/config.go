package service

import (
	"errors"
	"fmt"
	"log/slog"
	"time"
)

// MaxShutdownTimeout bounds SHUTDOWN_TIMEOUT: a service must exit within 10
// seconds of SIGTERM (LN-1.4).
const MaxShutdownTimeout = 10 * time.Second

// Config is the configuration every service reads from its environment.
type Config struct {
	Addr            string        // HTTP_ADDR: listen address, e.g. ":8081"
	LogLevel        slog.Level    // LOG_LEVEL: debug, info, warn or error
	ShutdownTimeout time.Duration // SHUTDOWN_TIMEOUT: graceful shutdown budget, at most 10s
}

// LoadConfig reads the config with getenv (os.Getenv outside tests). Unset
// variables take their defaults. The error names every invalid variable.
func LoadConfig(getenv func(string) string, defaultAddr string) (Config, error) {
	cfg := Config{Addr: defaultAddr, LogLevel: slog.LevelInfo, ShutdownTimeout: MaxShutdownTimeout}
	var errs []error

	if v := getenv("HTTP_ADDR"); v != "" {
		cfg.Addr = v
	}
	if v := getenv("LOG_LEVEL"); v != "" {
		if err := cfg.LogLevel.UnmarshalText([]byte(v)); err != nil {
			errs = append(errs, fmt.Errorf("LOG_LEVEL: %w", err))
		}
	}
	if v := getenv("SHUTDOWN_TIMEOUT"); v != "" {
		d, err := time.ParseDuration(v)
		switch {
		case err != nil:
			errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: %w", err))
		case d <= 0 || d > MaxShutdownTimeout:
			errs = append(errs, fmt.Errorf("SHUTDOWN_TIMEOUT: %s is not in (0, %s]", d, MaxShutdownTimeout))
		default:
			cfg.ShutdownTimeout = d
		}
	}
	return cfg, errors.Join(errs...)
}
