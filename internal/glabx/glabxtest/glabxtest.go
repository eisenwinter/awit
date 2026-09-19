// Package glabxtest installs the glabstub helper executable (a fake `glab`
// CLI, source in ../testdata/glabstub) on PATH for subprocess tests.
// It is portable: the stub is built with the Go toolchain running the
// tests, so it works on Windows without shell scripts.
package glabxtest

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

// Install builds glabstub once per test binary and prepends its directory
// to PATH for the calling test, so exec.LookPath("glab") finds the stub.
// It returns a fresh script directory the test fills with the files the
// stub reads (see testdata/glabstub); GLAB_STUB_DIR points at it.
//
// Install also clears the ambient GitLab environment overrides glab honours
// (GITLAB_SUBFOLDER and friends) so stubbed configuration is deterministic;
// a test that needs an override sets it after Install.
func Install(t *testing.T) (scriptDir string) {
	t.Helper()
	once.Do(func() {
		dir, err := os.MkdirTemp("", "awit-glabstub-bin")
		if err != nil {
			buildEr = err
			return
		}
		exe := filepath.Join(dir, "glab")
		if runtime.GOOS == "windows" {
			exe += ".exe"
		}
		cmd := exec.Command("go", "build", "-o", exe, "github.com/eisenwinter/awit/internal/glabx/testdata/glabstub")
		out, err := cmd.CombinedOutput()
		if err != nil {
			buildEr = err
			binDir = string(out)
			return
		}
		binDir = dir
	})
	if buildEr != nil {
		t.Fatalf("build glabstub: %v\n%s", buildEr, binDir)
	}
	t.Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("GITLAB_SUBFOLDER", "")
	t.Setenv("GITLAB_HOST", "")
	t.Setenv("GITLAB_API_HOST", "")
	t.Setenv("GLAB_API_PROTOCOL", "")
	t.Setenv("API_PROTOCOL", "")
	scriptDir = t.TempDir()
	t.Setenv("GLAB_STUB_DIR", scriptDir)
	return scriptDir
}

// HideGlab points PATH at an empty directory so exec.LookPath("glab") fails,
// simulating a machine without glab installed.
func HideGlab(t *testing.T) {
	t.Helper()
	t.Setenv("PATH", t.TempDir())
}
