// release-tool reads NUL-separated full commit messages from stdin, as emitted
// by git log -z --format=%B. A final NUL is optional; newlines belong to messages.
package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/softsrv/steamapi/internal/release"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

func run(args []string, in io.Reader, out, errOut io.Writer) int {
	if len(args) == 0 || (args[0] != "next-version" && args[0] != "validate-commits") ||
		(args[0] == "next-version" && len(args) != 2) ||
		(args[0] == "validate-commits" && len(args) != 1) {
		fmt.Fprintln(errOut, "usage: release-tool next-version vMAJOR.MINOR.PATCH | validate-commits (NUL-separated messages on stdin)")
		return 2
	}
	data, err := io.ReadAll(in)
	if err != nil {
		fmt.Fprintf(errOut, "read commit messages: %v\n", err)
		return 1
	}
	var messages []string
	if len(data) > 0 {
		messages = strings.Split(strings.TrimSuffix(string(data), "\x00"), "\x00")
	}
	if args[0] == "next-version" {
		version, err := release.ComputeNextVersion(args[1], messages)
		if err != nil {
			fmt.Fprintln(errOut, err)
			return 1
		}
		fmt.Fprintln(out, version)
		return 0
	}
	bad := release.ValidateCommitMessages(messages)
	for _, msg := range bad {
		// Quote untrusted messages so embedded newlines cannot inject CI commands.
		fmt.Fprintf(out, "non-conforming commit: %q\n", msg)
	}
	if len(bad) > 0 {
		return 1
	}
	return 0
}
