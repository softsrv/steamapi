package release

import (
	"fmt"
	"regexp"
	"strconv"
)

var versionTag = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
var breakingFooter = regexp.MustCompile(`(?m)^BREAKING(?: CHANGE|-CHANGE):`)

// ComputeNextVersion bumps a stable vMAJOR.MINOR.PATCH tag using full commit
// messages. Breaking changes take precedence over features, then fixes. When
// no significance is recognizable (including an empty set), it bumps minor.
// Prerelease tags, build metadata, and overflowing components are rejected.
func ComputeNextVersion(base string, messages []string) (string, error) {
	parts := versionTag.FindStringSubmatch(base)
	if parts == nil {
		return "", fmt.Errorf("invalid stable version tag %q", base)
	}
	var version [3]uint64
	for i := range version {
		n, err := strconv.ParseUint(parts[i+1], 10, 64)
		if err != nil {
			return "", fmt.Errorf("invalid version tag %q: %w", base, err)
		}
		version[i] = n
	}

	var breaking, feature, fix bool
	for _, msg := range messages {
		kind, bang, _ := parseHeader(msg)
		breaking = breaking || bang || breakingFooter.MatchString(msg)
		feature = feature || kind == "feat"
		fix = fix || kind == "fix"
	}
	bump := 1 // Indeterminate significance deliberately falls back to minor.
	if breaking {
		bump = 0
	} else if !feature && fix {
		bump = 2
	}
	if version[bump] == ^uint64(0) {
		return "", fmt.Errorf("version component overflows in %q", base)
	}
	version[bump]++
	for i := bump + 1; i < len(version); i++ {
		version[i] = 0
	}
	return fmt.Sprintf("v%d.%d.%d", version[0], version[1], version[2]), nil
}
