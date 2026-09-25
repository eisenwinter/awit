package main

import (
	"os/exec"
	"strings"
	"testing"
)

// TestNoTUIInDependencyGraph is the machine-checked rule behind the two-binary
// split: the classic binary must never link the TUI. It shells out to
// `go list -deps .` and fails on any charmbracelet or internal/lazy package.
func TestNoTUIInDependencyGraph(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Fatal("go binary not on PATH")
	}
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list: %v", err)
	}
	deps := string(out)
	if !strings.Contains(deps, "github.com/eisenwinter/awit/internal/ops") {
		t.Fatalf("go list output looks wrong (no internal/ops):\n%s", deps)
	}
	for _, line := range strings.Split(deps, "\n") {
		if strings.Contains(line, "charmbracelet") || strings.Contains(line, "/internal/lazy") {
			t.Errorf("awit must not link TUI code, found %s", line)
		}
	}
}
