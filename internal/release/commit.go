// Package release implements the commit conventions used by release automation.
package release

import (
	"regexp"
	"strings"
)

// Both validation and version computation use this same header grammar.
var commitHeader = regexp.MustCompile(`^(feat|fix|docs|style|refactor|perf|test|build|ci|chore|revert)(\([^()\s]+\))?(!)?: ([^\r\n]+)$`)

func parseHeader(msg string) (commitType string, breaking, valid bool) {
	first, _, _ := strings.Cut(msg, "\n")
	parts := commitHeader.FindStringSubmatch(strings.TrimSuffix(first, "\r"))
	if parts == nil || strings.TrimSpace(parts[4]) == "" {
		return "", false, false
	}
	return parts[1], parts[3] == "!", true
}

// ValidateCommitMessage reports whether the first line is a Conventional Commit.
// Bodies and footers do not change whether the header is valid.
func ValidateCommitMessage(msg string) bool {
	_, _, valid := parseHeader(msg)
	return valid
}

// ValidateCommitMessages returns the invalid messages in their original order.
func ValidateCommitMessages(msgs []string) (bad []string) {
	for _, msg := range msgs {
		if !ValidateCommitMessage(msg) {
			bad = append(bad, msg)
		}
	}
	return bad
}
