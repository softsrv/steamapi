package release

import "testing"

func TestComputeNextVersion(t *testing.T) {
	tests := []struct {
		name     string
		base     string
		messages []string
		want     string
	}{
		{"breaking feature", "v1.0.2", []string{"feat!: remove API"}, "v2.0.0"},
		{"breaking fix", "v1.0.2", []string{"fix!: remove workaround"}, "v2.0.0"},
		{"scoped breaking", "v1.4.9", []string{"feat(api)!: remove API"}, "v2.0.0"},
		{"breaking footer", "v1.0.2", []string{"fix: update API\n\nBREAKING CHANGE: callers must migrate"}, "v2.0.0"},
		{"hyphenated footer", "v1.0.2", []string{"chore: update API\n\nBREAKING-CHANGE: callers must migrate\nMore detail"}, "v2.0.0"},
		{"footer in legacy message", "v1.0.2", []string{"change API\n\nBREAKING CHANGE: removed field"}, "v2.0.0"},
		{"feature", "v1.0.2", []string{"feat: add API"}, "v1.1.0"},
		{"scoped feature", "v1.2.9", []string{"feat(steamapi): add method"}, "v1.3.0"},
		{"fix", "v1.0.2", []string{"fix: correct request"}, "v1.0.3"},
		{"scoped fix", "v1.2.9", []string{"fix(api): correct request"}, "v1.2.10"},
		{"feature beats fix", "v1.0.2", []string{"fix: x", "feat: y", "fix: z"}, "v1.1.0"},
		{"breaking beats feature", "v1.0.2", []string{"feat: x", "fix!: y", "feat: z"}, "v2.0.0"},
		{"chore fallback", "v1.0.2", []string{"chore: cleanup"}, "v1.1.0"},
		{"docs fallback", "v1.0.2", []string{"docs: update guide"}, "v1.1.0"},
		{"legacy fallback", "v1.0.2", []string{"change case", "Add method"}, "v1.1.0"},
		{"mixed indeterminate", "v1.0.2", []string{"chore: x", "docs: y", "free text"}, "v1.1.0"},
		{"other types fallback", "v2.3.4", []string{"perf: faster", "revert: undo", "refactor: simpler", "test: coverage", "build: tooling", "ci: pipeline", "style: format"}, "v2.4.0"},
		{"empty fallback", "v1.0.2", nil, "v1.1.0"},
		{"fix with indeterminate", "v1.0.2", []string{"docs: x", "fix: y", "free text"}, "v1.0.3"},
		{"body is not header", "v1.0.2", []string{"docs: example\n\nfeat!: not a header"}, "v1.1.0"},
		{"invalid prefix", "v1.0.2", []string{"feature!: not standard", "fix:no space", "prefix feat!: x"}, "v1.1.0"},
		{"zero major still increments", "v0.2.3", []string{"feat!: x"}, "v1.0.0"},
		{"prose is not footer", "v1.0.2", []string{"fix: describe BREAKING CHANGE: syntax"}, "v1.0.3"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ComputeNextVersion(tt.base, tt.messages)
			if err != nil || got != tt.want {
				t.Fatalf("ComputeNextVersion(%q, %q) = %q, %v; want %q, nil", tt.base, tt.messages, got, err, tt.want)
			}
		})
	}
}

func TestComputeNextVersionRejectsInvalidBase(t *testing.T) {
	for _, base := range []string{"", "1.0.2", "v1.0", "v01.0.2", "v1.-1.0", "v1.0.2-beta", "v1.0.2+build", "v1.0.2\n", "v18446744073709551616.0.0"} {
		t.Run(base, func(t *testing.T) {
			got, err := ComputeNextVersion(base, []string{"fix: x"})
			if err == nil || got != "" {
				t.Fatalf("ComputeNextVersion(%q) = %q, %v; want error", base, got, err)
			}
		})
	}
}

func TestComputeNextVersionRejectsOverflow(t *testing.T) {
	for _, tt := range []struct{ base, message string }{
		{"v18446744073709551615.0.0", "feat!: x"},
		{"v1.18446744073709551615.0", "chore: x"},
		{"v1.0.18446744073709551615", "fix: x"},
	} {
		if got, err := ComputeNextVersion(tt.base, []string{tt.message}); err == nil || got != "" {
			t.Errorf("ComputeNextVersion(%q) = %q, %v; want overflow error", tt.base, got, err)
		}
	}
}
