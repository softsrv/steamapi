package release

import (
	"os"
	"strings"
	"testing"
)

// This is a static regression check of CLM-1, not a live Actions assertion.
func TestReleaseWorkflowDeclaresMainPushGate(t *testing.T) {
	data, err := os.ReadFile("../../.github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(data)
	if !strings.Contains(workflow, "on:\n  push:\n    branches: [main]\n") {
		t.Fatal("release workflow must run on pushes to main")
	}
	_, jobs, ok := strings.Cut(workflow, "jobs:\n  gate:\n")
	if !ok {
		t.Fatal("release workflow must declare the gate job")
	}
	gate, publisher, ok := strings.Cut(jobs, "\n  release:\n")
	if !ok || !strings.HasPrefix(publisher, "    needs: gate\n") {
		t.Fatal("release job must depend on the gate")
	}
	for _, command := range []string{"go build ./...", "go vet ./...", "go test ./... -v -race -cover"} {
		if !strings.Contains(gate, "        run: "+command+"\n") {
			t.Errorf("gate missing command %q", command)
		}
	}
}
