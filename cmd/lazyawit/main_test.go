package main

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestMain(m *testing.M) { os.Setenv("NO_COLOR", "1"); os.Exit(m.Run()) }

func drive(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errb bytes.Buffer
	code := run(args, strings.NewReader(stdin), &out, &errb)
	return code, out.String(), errb.String()
}

func copyFixture(t *testing.T, name string) string {
	t.Helper()
	dst := t.TempDir()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "fixtures", name)
	if err := os.CopyFS(dst, os.DirFS(src)); err != nil {
		t.Fatalf("copyFixture %s: %v", name, err)
	}
	return dst
}

func TestHelpListsOnlyRepoAndAgent(t *testing.T) {
	code, stdout, stderr := drive(t, "", "--help")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	for _, want := range []string{"lazyawit", "--repo", "--agent", "AWIT_AGENT"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("help missing %q:\n%s", want, stdout)
		}
	}
	for _, no := range []string{"--format", "--no-color", "lazy-human", "COMMANDS:"} {
		if strings.Contains(stdout, no) {
			t.Errorf("help must not show %q:\n%s", no, stdout)
		}
	}
}

func TestVersion(t *testing.T) {
	code, stdout, _ := drive(t, "", "--version")
	if code != 0 || stdout != "lazyawit dev\n" {
		t.Fatalf("exit %d stdout %q", code, stdout)
	}
}

func TestUsageErrors(t *testing.T) {
	if code, _, stderr := drive(t, "", "--bogus"); code != 2 || !strings.Contains(stderr, "bogus") || !strings.Contains(stderr, `run "lazyawit --help"`) {
		t.Fatalf("bad flag: exit %d stderr %q", code, stderr)
	}
	if code, _, stderr := drive(t, "", "extra"); code != 2 || !strings.Contains(stderr, `"extra"`) {
		t.Fatalf("positional: exit %d stderr %q", code, stderr)
	}
}

func TestQuitsHeadless(t *testing.T) {
	dir := copyFixture(t, "clean")
	done := make(chan int, 1)
	go func() { code, _, _ := drive(t, "q", "--repo", dir, "--agent", "claude"); done <- code }()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("lazyawit did not exit on q within 10s")
	}
}

func TestFatalScreenQuitsZero(t *testing.T) {
	t.Setenv("AWIT_REPO", "")
	dir := t.TempDir()
	done := make(chan int, 1)
	go func() { code, _, _ := drive(t, "q", "--repo", dir); done <- code }()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("fatal screen did not exit on q within 10s")
	}
}
