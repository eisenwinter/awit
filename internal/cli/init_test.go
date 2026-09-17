package cli

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

func TestInitCreatesRepo(t *testing.T) {
	dir := t.TempDir()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "init")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	wantOut := "Initialized .awit in " + abs + " (prefix AWIT)\n"
	if stdout != wantOut {
		t.Fatalf("stdout = %q, want %q", stdout, wantOut)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, ".awit", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if string(cfg) != "prefix: AWIT\nstale_claim: 2h\n" {
		t.Fatalf("config.yaml = %q", cfg)
	}
	for _, name := range []string{"items", "comments"} {
		st, err := os.Stat(filepath.Join(dir, ".awit", name))
		if err != nil || !st.IsDir() {
			t.Fatalf("%s: %v", name, err)
		}
	}
	gi, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(gi), ".awit/.lock") {
		t.Fatalf(".gitignore = %q, want a line containing .awit/.lock", gi)
	}
}

func TestInitCustomPrefix(t *testing.T) {
	dir := t.TempDir()
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "init", "--prefix", "PROJ")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "Initialized .awit in "+abs+" (prefix PROJ)\n" {
		t.Fatalf("stdout = %q", stdout)
	}
	cfg, err := os.ReadFile(filepath.Join(dir, ".awit", "config.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(cfg), "prefix: PROJ\n") {
		t.Fatalf("config.yaml = %q", cfg)
	}
}

func TestInitBadPrefix(t *testing.T) {
	const msg = "prefix must be 2-8 uppercase alphanumerics starting with a letter"
	for _, prefix := range []string{"A", "ABCDEFGHI", "awit", "1AB", "AB-C", "Ab", ""} {
		t.Run(prefix, func(t *testing.T) {
			code, _, stderr := run(t, "--repo", t.TempDir(), "init", "--prefix", prefix)
			if code != 1 {
				t.Fatalf("exit %d, want 1 (stderr %q)", code, stderr)
			}
			if stderr != "Error: "+msg+"\n" {
				t.Fatalf("stderr = %q", stderr)
			}
		})
	}
}

func TestInitTwiceFails(t *testing.T) {
	dir := t.TempDir()
	code, _, stderr := run(t, "--repo", dir, "init")
	if code != 0 {
		t.Fatalf("first init: exit %d stderr %q", code, stderr)
	}
	code, _, stderr = run(t, "--repo", dir, "init")
	if code != 1 {
		t.Fatalf("second init: exit %d, want 1", code)
	}
	if stderr != "Error: .awit already exists\n" {
		t.Fatalf("stderr = %q", stderr)
	}
}

func TestOpenStoreWithoutRepo(t *testing.T) {
	t.Chdir(t.TempDir())
	var storeErr error
	cmd := &cli.Command{
		Name: "awit",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "repo"},
		},
		Action: func(context.Context, *cli.Command) error { return nil },
	}
	cmd.Action = func(_ context.Context, c *cli.Command) error {
		_, storeErr = openStore(c)
		return nil
	}
	if err := cmd.Run(context.Background(), []string{"awit"}); err != nil {
		t.Fatal(err)
	}
	if storeErr == nil || !errors.Is(storeErr, item.ErrNotFound) {
		t.Fatalf("openStore error = %v, want item.ErrNotFound", storeErr)
	}
}
