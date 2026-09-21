package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestTemplateDefaultSkeleton(t *testing.T) {
	dir := initRepo(t)
	code, stdout, stderr := run(t, "--repo", dir, "template")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	const want = "\n## Summary\n\n## Acceptance Criteria\n\n"
	if stdout != want {
		t.Errorf("stdout = %q, want %q", stdout, want)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty", stderr)
	}
}

func TestTemplateFromConfigIsVerbatim(t *testing.T) {
	dir := initRepo(t)
	body := "## Summary\n## Steps\n- [ ] do it\n"
	if err := os.MkdirAll(filepath.Join(dir, "tpl"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "tpl", "b.md"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	writeConfigTemplate(t, dir, "tpl/b.md")

	code, stdout, stderr := run(t, "--repo", dir, "template")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != body {
		t.Errorf("stdout = %q, want %q", stdout, body)
	}
}

func TestTemplateReportsBrokenConfig(t *testing.T) {
	dir := initRepo(t)
	writeConfigTemplate(t, dir, "tpl/missing.md")

	code, stdout, stderr := run(t, "--repo", dir, "template")
	if code != 1 {
		t.Fatalf("exit = %d, want 1 (stderr %q)", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want empty", stdout)
	}
	if !strings.Contains(stderr, "template tpl/missing.md") {
		t.Errorf("stderr = %q, want it to name the template path", stderr)
	}
}

func TestTemplateIsSilentOnQuarantinedGraph(t *testing.T) {
	dir := copyFixture(t, "cyclic")
	code, _, stderr := run(t, "--repo", dir, "template")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stderr != "" {
		t.Errorf("stderr = %q, want empty: template builds no graph", stderr)
	}
}

// writeConfigTemplate appends a template: key to the repo's config.yaml.
// Reused by the create and update body work items.
func writeConfigTemplate(t *testing.T, repo, rel string) {
	t.Helper()
	p := filepath.Join(repo, ".awit", "config.yaml")
	data, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, append(data, []byte("template: "+rel+"\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
}
