package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const walkNotePrefix = "Note: no .awit in the current directory; using "
const walkNoteSuffix = ". Run awit init here, or pass --repo / set AWIT_REPO."

func chdirSub(t *testing.T, root string) string {
	t.Helper()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(sub)
	return sub
}

func TestCreateFromSubdirEmitsNote(t *testing.T) {
	root := copyFixture(t, "clean")
	chdirSub(t, root)
	code, _, stderr := run(t, "create", "walked", "--brief", "note")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	want := walkNotePrefix + root + walkNoteSuffix + "\n"
	if stderr != want {
		t.Errorf("stderr = %q, want %q", stderr, want)
	}
}

func TestCreateFromSubdirSilentWithRepoFlag(t *testing.T) {
	root := copyFixture(t, "clean")
	chdirSub(t, root)
	code, _, stderr := run(t, "--repo", root, "create", "walked", "--brief", "note")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestCreateFromSubdirSilentWithEnvRepo(t *testing.T) {
	root := copyFixture(t, "clean")
	chdirSub(t, root)
	t.Setenv("AWIT_REPO", root)
	code, _, stderr := run(t, "create", "walked", "--brief", "note")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestCreateFromRootSilent(t *testing.T) {
	root := copyFixture(t, "clean")
	t.Chdir(root)
	code, _, stderr := run(t, "create", "walked", "--brief", "note")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestReadOnlyFromSubdirSilent(t *testing.T) {
	root := copyFixture(t, "clean")
	chdirSub(t, root)
	for _, args := range [][]string{
		{"list"},
		{"prime"},
		{"next", "--seed", "1"},
	} {
		code, stdoutSub, stderr := run(t, args...)
		if code != 0 {
			t.Fatalf("%v: exit %d stderr %q", args, code, stderr)
		}
		if stderr != "" {
			t.Errorf("%v: stderr = %q, want empty", args, stderr)
		}
		t.Chdir(root)
		_, stdoutRoot, _ := run(t, args...)
		if stdoutSub != stdoutRoot {
			t.Errorf("%v: sub stdout differs from root stdout", args)
		}
		t.Chdir(filepath.Join(root, "sub"))
	}
}

func TestNextClaimFromSubdirEmitsNote(t *testing.T) {
	root := copyFixture(t, "clean")
	chdirSub(t, root)
	code, _, stderr := run(t, "next", "--claim", "--no-commit", "--agent", "tester", "--seed", "1")
	if code != 0 {
		t.Fatalf("next --claim: exit %d stderr %q", code, stderr)
	}
	if !strings.HasPrefix(stderr, walkNotePrefix) || !strings.HasSuffix(strings.TrimSuffix(stderr, "\n"), walkNoteSuffix) {
		t.Errorf("stderr = %q, want walked-up note", stderr)
	}
}

func TestEnvRepoUsedAsRoot(t *testing.T) {
	root := copyFixture(t, "clean")
	envRoot := copyFixture(t, "clean")
	chdirSub(t, root)
	t.Setenv("AWIT_REPO", envRoot)
	code, stdout, stderr := run(t, "create", "via-env", "--brief", "note")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	if !strings.Contains(stdout, "via-env") {
		t.Errorf("stdout = %q, want created item", stdout)
	}
	ents, err := os.ReadDir(filepath.Join(envRoot, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range ents {
		data, err := os.ReadFile(filepath.Join(envRoot, ".awit", "items", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "via-env") {
			found = true
		}
	}
	if !found {
		t.Errorf("item not created in AWIT_REPO root")
	}
}
func TestRepoFlagWinsOverEnvRepo(t *testing.T) {
	envRoot := copyFixture(t, "clean")
	flagRoot := copyFixture(t, "clean")
	chdirSub(t, envRoot)
	t.Setenv("AWIT_REPO", envRoot)
	code, _, stderr := run(t, "--repo", flagRoot, "create", "flag-wins", "--brief", "note")
	if code != 0 {
		t.Fatalf("create: exit %d stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
	ents, err := os.ReadDir(filepath.Join(flagRoot, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, e := range ents {
		data, err := os.ReadFile(filepath.Join(flagRoot, ".awit", "items", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "flag-wins") {
			found = true
		}
	}
	if !found {
		t.Errorf("item not created in --repo root")
	}
	envEnts, err := os.ReadDir(filepath.Join(envRoot, ".awit", "items"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range envEnts {
		data, err := os.ReadFile(filepath.Join(envRoot, ".awit", "items", e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "flag-wins") {
			t.Errorf("item leaked into AWIT_REPO root")
		}
	}
}
