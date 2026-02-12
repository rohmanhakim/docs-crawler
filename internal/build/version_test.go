package build_test

import (
	"testing"

	"github.com/rohmanhakim/docs-crawler/internal/build"
)

func TestFullVersion(t *testing.T) {
	tests := []struct {
		name    string
		version string
		commit  string
		want    string
	}{
		{
			name:    "default values",
			version: "dev",
			commit:  "none",
			want:    "dev+none",
		},
		{
			name:    "version with commit",
			version: "1.0.0",
			commit:  "abc123",
			want:    "1.0.0+abc123",
		},
		{
			name:    "empty version with commit",
			version: "",
			commit:  "abc123",
			want:    "+abc123",
		},
		{
			name:    "version with empty commit",
			version: "1.0.0",
			commit:  "",
			want:    "1.0.0+",
		},
		{
			name:    "semver with long commit hash",
			version: "2.1.0-beta",
			commit:  "89dece58db957dbc4a9d03962b0411d05f9e37a5",
			want:    "2.1.0-beta+89dece58db957dbc4a9d03962b0411d05f9e37a5",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Set package variables
			build.Version = tt.version
			build.Commit = tt.commit

			got := build.FullVersion()
			if got != tt.want {
				t.Errorf("FullVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
