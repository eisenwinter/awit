package cli

import (
	"bytes"
	"strings"
	"testing"
)

// runMain drives the CLI exactly like cmd/awit/main.go does, with empty stdin
// and buffered streams. Tests in this package never call t.Parallel(): Main
// installs the package-level cli.VersionPrinter hook.
func runMain(t *testing.T, args ...string) (code int, stdout, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	code = Main(args, strings.NewReader(""), &out, &errb)
	return code, out.String(), errb.String()
}
