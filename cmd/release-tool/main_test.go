package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestRun(t *testing.T) {
	tests := []struct {
		name      string
		args      []string
		input     string
		code      int
		output    string
		wantError bool
	}{
		{"next version", []string{"next-version", "v1.0.2"}, "fix: x\n\x00feat: y\n\x00", 0, "v1.1.0\n", false},
		{"multiline breaking footer", []string{"next-version", "v1.0.2"}, "fix: x\n\nBREAKING CHANGE: new API\n\x00", 0, "v2.0.0\n", false},
		{"empty fallback", []string{"next-version", "v1.0.2"}, "", 0, "v1.1.0\n", false},
		{"valid messages", []string{"validate-commits"}, "feat: x\n\nbody\n\x00fix: y\n\x00", 0, "", false},
		{"no trailing NUL", []string{"validate-commits"}, "fix: x", 0, "", false},
		{"empty set", []string{"validate-commits"}, "", 0, "", false},
		{"empty commit", []string{"validate-commits"}, "\x00", 1, "non-conforming commit: \"\"\n", false},
		{"mixed messages", []string{"validate-commits"}, "feat: x\x00change case\x00Add method\x00", 1, "non-conforming commit: \"change case\"\nnon-conforming commit: \"Add method\"\n", false},
		{"quote untrusted body", []string{"validate-commits"}, "invalid\n::error::untrusted\x00", 1, "non-conforming commit: \"invalid\\n::error::untrusted\"\n", false},
		{"invalid base", []string{"next-version", "oops"}, "fix: x", 1, "", true},
		{"no args", nil, "", 2, "", true},
		{"unknown command", []string{"other"}, "", 2, "", true},
		{"missing base", []string{"next-version"}, "", 2, "", true},
		{"extra base", []string{"next-version", "v1.0.2", "extra"}, "", 2, "", true},
		{"extra validation arg", []string{"validate-commits", "extra"}, "", 2, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var out, errOut bytes.Buffer
			code := run(tt.args, strings.NewReader(tt.input), &out, &errOut)
			if code != tt.code || out.String() != tt.output || (errOut.Len() > 0) != tt.wantError {
				t.Fatalf("run() = code %d, stdout %q, stderr %q; want %d, %q, error=%v", code, out.String(), errOut.String(), tt.code, tt.output, tt.wantError)
			}
		})
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("input failed") }

func TestRunReadError(t *testing.T) {
	var errOut bytes.Buffer
	if code := run([]string{"validate-commits"}, failingReader{}, io.Discard, &errOut); code != 1 || !strings.Contains(errOut.String(), "input failed") {
		t.Fatalf("run() = %d, %q; want read failure", code, errOut.String())
	}
}
