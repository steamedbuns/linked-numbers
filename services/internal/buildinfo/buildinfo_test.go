package buildinfo

import (
	"runtime/debug"
	"testing"
)

func TestFromSettings(t *testing.T) {
	const rev = "ebe3de5bb45284ee1c38102e7d493e10aed96680"
	tests := []struct {
		name     string
		settings []debug.BuildSetting
		want     string
	}{
		{"no vcs stamp", nil, "dev"},
		{"clean", []debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "false"}}, "ebe3de5bb452"},
		{"dirty", []debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: "true"}}, "deliberately-wrong"},
		{"short revision", []debug.BuildSetting{{Key: "vcs.revision", Value: "abc123"}}, "abc123"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := fromSettings(tt.settings); got != tt.want {
				t.Errorf("fromSettings() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVersionUnderGoTest(t *testing.T) {
	if got := Version(); got != "dev" {
		t.Errorf("Version() = %q, want %q (go test binaries have no VCS stamp)", got, "dev")
	}
}
