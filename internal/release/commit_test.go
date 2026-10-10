package release

import (
	"reflect"
	"testing"
)

func TestValidateCommitMessage(t *testing.T) {
	for _, msg := range []string{
		"feat(scope): x", "fix: x", "feat!: x", "fix!: x", "feat(a)!: x",
		"feat(steamapi): add SharedAppIDs and batch GamesDetails (#12)",
		"chore: reviewed work (#6)", "docs: x", "style: x", "refactor: x", "perf: x",
		"test: x", "build: x", "ci: x", "revert: x", "fix(api/v2): x",
		"feat: x\n\nBREAKING CHANGE: new API", "fix: x\n\nBREAKING-CHANGE: new API",
		"fix: x\n\nFree text body is fine", "fix: x\r\n\r\nBody",
	} {
		t.Run(msg, func(t *testing.T) {
			if !ValidateCommitMessage(msg) {
				t.Errorf("ValidateCommitMessage(%q) = false; want true", msg)
			}
		})
	}
	for _, msg := range []string{
		"change case", "Add method", "free text", "", "\nfix: later header",
		"Feat: x", "unknown: x", "feature: x", "fix:x", "fix: ", "fix: \t",
		"feat(): x", "feat(a b): x", "feat((a)): x", "feat!!: x", "fix!: ",
		"prefix fix: x", " fix: x", "feat(a)! : x", "fix:\tx", "BREAKING CHANGE: x",
		"invalid\n\nfeat: valid body cannot rescue header",
	} {
		t.Run(msg, func(t *testing.T) {
			if ValidateCommitMessage(msg) {
				t.Errorf("ValidateCommitMessage(%q) = true; want false", msg)
			}
		})
	}
}

func TestValidateCommitMessages(t *testing.T) {
	messages := []string{"feat: x", "change case", "fix: y", "Add method", "change case"}
	want := []string{"change case", "Add method", "change case"}
	if got := ValidateCommitMessages(messages); !reflect.DeepEqual(got, want) {
		t.Fatalf("ValidateCommitMessages() = %q; want %q", got, want)
	}
	for _, messages := range [][]string{nil, {"feat: x", "fix: y"}} {
		if got := ValidateCommitMessages(messages); len(got) != 0 {
			t.Errorf("ValidateCommitMessages(%q) = %q; want no failures", messages, got)
		}
	}
}
