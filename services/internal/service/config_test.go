package service

import (
	"log/slog"
	"strings"
	"testing"
	"time"
)

func TestLoadConfig(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		want    Config
		wantErr []string // substrings of the error; nil means no error
	}{
		{
			name: "defaults",
			want: Config{Addr: ":8081", LogLevel: slog.LevelInfo, ShutdownTimeout: 10 * time.Second},
		},
		{
			name: "overrides",
			env: map[string]string{
				"HTTP_ADDR": "127.0.0.1:9000", "LOG_LEVEL": "debug", "SHUTDOWN_TIMEOUT": "3s",
				"DATABASE_URL": "postgres://db:5432/ln", "MIGRATE_ON_START": "true",
			},
			want: Config{
				Addr: "127.0.0.1:9000", LogLevel: slog.LevelDebug, ShutdownTimeout: 3 * time.Second,
				DatabaseURL: "postgres://db:5432/ln", MigrateOnStart: true,
			},
		},
		{name: "bad log level", env: map[string]string{"LOG_LEVEL": "nope"}, wantErr: []string{"LOG_LEVEL"}},
		{name: "bad timeout", env: map[string]string{"SHUTDOWN_TIMEOUT": "ten"}, wantErr: []string{"SHUTDOWN_TIMEOUT"}},
		{name: "zero timeout", env: map[string]string{"SHUTDOWN_TIMEOUT": "0s"}, wantErr: []string{"SHUTDOWN_TIMEOUT"}},
		{name: "bad migrate flag", env: map[string]string{"MIGRATE_ON_START": "yes"}, wantErr: []string{"MIGRATE_ON_START"}},
		{name: "timeout over 10s", env: map[string]string{"SHUTDOWN_TIMEOUT": "11s"}, wantErr: []string{"SHUTDOWN_TIMEOUT"}},
		{
			name:    "every error is reported",
			env:     map[string]string{"LOG_LEVEL": "nope", "SHUTDOWN_TIMEOUT": "ten", "MIGRATE_ON_START": "yes"},
			wantErr: []string{"LOG_LEVEL", "SHUTDOWN_TIMEOUT", "MIGRATE_ON_START"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := LoadConfig(func(k string) string { return tt.env[k] }, ":8081")
			if tt.wantErr == nil {
				if err != nil {
					t.Fatalf("LoadConfig() error = %v", err)
				}
				if got != tt.want {
					t.Errorf("LoadConfig() = %+v, want %+v", got, tt.want)
				}
				return
			}
			if err == nil {
				t.Fatalf("LoadConfig() = %+v, want an error", got)
			}
			for _, s := range tt.wantErr {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("error %q does not mention %s", err, s)
				}
			}
		})
	}
}
