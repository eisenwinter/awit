// Package teaxtest installs the teastub helper executable (a fake `tea`
// CLI, source in ../testdata/teastub) on PATH for subprocess tests.
// It is portable: the stub is built with the Go toolchain running the
// tests, so it works on Windows without shell scripts.
package teaxtest

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"testing"
)

var (
	once    sync.Once
	binDir  string
	buildEr error
)

// Install builds teastub once per test binary and prepends its directory
// to PATH for the calling test, so exec.LookPath("tea") finds the stub.
// It returns a fresh script directory the test fills with the files the
// stub reads (see testdata/teastub); TEA_STUB_DIR points at it.
func Install(t *testing.T) (scriptDir string) {
	t.Helper()
	once.Do(func() {
		dir, err := os.MkdirTemp("", "awit-teastub-bin")
		if err != nil {
			buildEr = err
			return
		}
		exe := filepath.Join(dir, "tea")
		if runtime.GOOS == "windows" {
			exe += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", exe, "github.com/eisenwinter/awit/internal/teax/testdata/teastub")
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildEr = err
			binDir = string(out)
			return
		}
		binDir = dir
	})
	if buildEr != nil {
		t.Fatalf("build teastub: %v\n%s", buildEr, binDir)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	scriptDir = t.TempDir()
	t.Setenv("TEA_STUB_DIR", scriptDir)
	return scriptDir
}

// HideTea points PATH at an empty directory so exec.LookPath("tea") fails,
// simulating a machine without tea installed.
func HideTea(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}
