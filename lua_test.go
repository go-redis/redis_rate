package redis_rate

import (
	"testing"
)

func TestIsFIPSMode(t *testing.T) {
	tests := []struct {
		name     string
		gofips   string
		godebug  string
		expected bool
	}{
		{
			name:     "no env vars set",
			expected: false,
		},
		{
			name:     "GOFIPS140=on",
			gofips:   "on",
			expected: true,
		},
		{
			name:     "GOFIPS140=off",
			gofips:   "off",
			expected: false,
		},
		{
			name:     "GOFIPS140 empty string",
			gofips:   "",
			expected: false,
		},
		{
			name:     "GODEBUG=fips140=on",
			godebug:  "fips140=on",
			expected: true,
		},
		{
			name:     "GODEBUG=fips140=only",
			godebug:  "fips140=only",
			expected: true,
		},
		{
			name:     "GODEBUG=fips140=off",
			godebug:  "fips140=off",
			expected: false,
		},
		{
			name:     "GODEBUG with fips140=on among other settings",
			godebug:  "netdns=go,fips140=on,http2debug=1",
			expected: true,
		},
		{
			name:     "GODEBUG with fips140=only among other settings",
			godebug:  "fips140=only,netdns=go",
			expected: true,
		},
		{
			name:     "GODEBUG without fips140",
			godebug:  "netdns=go,http2debug=1",
			expected: false,
		},
		{
			name:     "both GOFIPS140=on and GODEBUG=fips140=on",
			gofips:   "on",
			godebug:  "fips140=on",
			expected: true,
		},
		{
			name:     "GOFIPS140=off overrides nothing - GODEBUG=fips140=on still active",
			gofips:   "off",
			godebug:  "fips140=on",
			expected: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("GOFIPS140", tc.gofips)
			t.Setenv("GODEBUG", tc.godebug)

			got := isFIPSMode()
			if got != tc.expected {
				t.Errorf("isFIPSMode() = %v, want %v", got, tc.expected)
			}
		})
	}
}

func TestNewScript_ReturnType(t *testing.T) {
	const src = `return 1`

	t.Run("non-FIPS: hash pre-computed client-side", func(t *testing.T) {
		t.Setenv("GOFIPS140", "")
		t.Setenv("GODEBUG", "")

		s := newScript(src)
		if s.Hash() == "" {
			t.Error("expected non-empty hash from NewScript in non-FIPS mode")
		}
	})

	t.Run("FIPS via GOFIPS140=on: hash empty until SCRIPT LOAD", func(t *testing.T) {
		t.Setenv("GOFIPS140", "on")
		t.Setenv("GODEBUG", "")

		s := newScript(src)
		if s.Hash() != "" {
			t.Errorf("expected empty hash from NewScriptServerSHA, got %q", s.Hash())
		}
	})

	t.Run("FIPS via GODEBUG=fips140=on: hash empty until SCRIPT LOAD", func(t *testing.T) {
		t.Setenv("GOFIPS140", "")
		t.Setenv("GODEBUG", "fips140=on")

		s := newScript(src)
		if s.Hash() != "" {
			t.Errorf("expected empty hash from NewScriptServerSHA, got %q", s.Hash())
		}
	})

	t.Run("FIPS via GODEBUG=fips140=only: hash empty until SCRIPT LOAD", func(t *testing.T) {
		t.Setenv("GOFIPS140", "")
		t.Setenv("GODEBUG", "fips140=only")

		s := newScript(src)
		if s.Hash() != "" {
			t.Errorf("expected empty hash from NewScriptServerSHA, got %q", s.Hash())
		}
	})
}
