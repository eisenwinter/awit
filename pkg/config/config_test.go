package config

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoadDefaults(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("prefix: AWIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Prefix != "AWIT" {
		t.Fatalf("Prefix = %q, want AWIT", c.Prefix)
	}
	if time.Duration(c.StaleClaim) != 2*time.Hour {
		t.Fatalf("StaleClaim = %s, want 2h", time.Duration(c.StaleClaim))
	}
}

func TestLoadMissingPrefix(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("stale_claim: 1h\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil || err.Error() != "config: prefix is required" {
		t.Fatalf("Load() error = %v, want config: prefix is required", err)
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load(t.TempDir())
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("Load(missing) error = %v, want os.ErrNotExist", err)
	}
}

func TestLoadInvalidDuration(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("prefix: AWIT\nstale_claim: banana\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(dir)
	if err == nil {
		t.Fatal("Load(banana) = nil, want error")
	}
}

func TestWriteThenLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := Default("AWIT")
	if err := c.Write(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	want := "prefix: AWIT\nstale_claim: 2h\n"
	if string(data) != want {
		t.Fatalf("Write() bytes = %q, want %q", data, want)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Prefix != "AWIT" || time.Duration(loaded.StaleClaim) != 2*time.Hour {
		t.Fatalf("Load after Write = %+v", loaded)
	}
}

func TestWritePreservesAgentAndLabels(t *testing.T) {
	dir := t.TempDir()
	c := Default("AWIT")
	c.AgentID = "claude"
	c.DefaultLabels = []string{"p0", "auth"}
	if err := c.Write(dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AgentID != "claude" {
		t.Fatalf("AgentID = %q, want claude", loaded.AgentID)
	}
	if len(loaded.DefaultLabels) != 2 || loaded.DefaultLabels[0] != "p0" || loaded.DefaultLabels[1] != "auth" {
		t.Fatalf("DefaultLabels = %q, want [p0 auth]", loaded.DefaultLabels)
	}
}

func TestAgentResolution(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		env     string
		setenv  bool
		agentID string
		want    string
	}{
		{name: "flag over env and config", flag: "from-flag", env: "from-env", setenv: true, agentID: "from-cfg", want: "from-flag"},
		{name: "env over config", flag: "", env: "from-env", setenv: true, agentID: "from-cfg", want: "from-env"},
		{name: "config fallback", flag: "", setenv: true, env: "", agentID: "from-cfg", want: "from-cfg"},
		{name: "empty", flag: "", setenv: true, env: "", agentID: "", want: ""},
		{name: "flag not prefixed", flag: "claude", setenv: true, env: "", agentID: "", want: "claude"},
		{name: "env not prefixed", flag: "", env: "claude", setenv: true, agentID: "", want: "claude"},
		{name: "config not prefixed", flag: "", setenv: true, env: "", agentID: "claude", want: "claude"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.setenv {
				t.Setenv("AWIT_AGENT", tt.env)
			}
			c := Config{AgentID: tt.agentID}
			if got := c.Agent(tt.flag); got != tt.want {
				t.Fatalf("Agent(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

func TestWriteAtomicReplaces(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "f.txt")
	if err := WriteAtomic(path, []byte("one")); err != nil {
		t.Fatal(err)
	}
	if err := WriteAtomic(path, []byte("two")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "two" {
		t.Fatalf("got %q, want two", data)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".tmp-*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 0 {
		t.Fatalf("leftover temp files: %q", matches)
	}
}

func TestWriteAtomicBadDir(t *testing.T) {
	path := filepath.Join(t.TempDir(), "no-such-dir", "f.txt")
	if err := WriteAtomic(path, []byte("x")); err == nil {
		t.Fatal("WriteAtomic(missing dir) = nil, want error")
	}
}

func TestCommitPolicyShouldCommit(t *testing.T) {
	no, yes := false, true
	tests := []struct {
		name string
		c    Config
		want bool
	}{
		{"zero config commits", Config{}, true},
		{"nil commit commits", Config{Commit: nil}, true},
		{"explicit false skips", Config{Commit: &no}, false},
		{"explicit true commits", Config{Commit: &yes}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.c.ShouldCommit(); got != tt.want {
				t.Fatalf("ShouldCommit() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCommitPolicyLoad(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want bool
	}{
		{"absent key means commit", "prefix: AWIT\n", true},
		{"commit false", "prefix: AWIT\ncommit: false\n", false},
		{"commit true", "prefix: AWIT\ncommit: true\n", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, FileName), []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			c, err := Load(dir)
			if err != nil {
				t.Fatal(err)
			}
			if c.ShouldCommit() != tt.want {
				t.Fatalf("ShouldCommit() = %v, want %v", c.ShouldCommit(), tt.want)
			}
		})
	}
}

func TestCommitPolicyWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := Default("AWIT")
	if err := c.Write(dir); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "commit") {
		t.Fatalf("Default config must not write a commit key: %q", data)
	}

	no := false
	c.Commit = &no
	if err := c.Write(dir); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(filepath.Join(dir, FileName))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "commit: false") {
		t.Fatalf("Write must keep commit: false: %q", data)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.ShouldCommit() {
		t.Fatal("commit: false must survive Write -> Load")
	}
}

func TestTemplateLoadAbsent(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("prefix: AWIT\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Template != "" {
		t.Fatalf("Template = %q, want empty", c.Template)
	}
}

func TestTemplateLoadValidRelative(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, FileName), []byte("prefix: AWIT\ntemplate: plan/workitem-template.md\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if c.Template != "plan/workitem-template.md" {
		t.Fatalf("Template = %q", c.Template)
	}
}

func TestTemplateLoadRejectsAbsoluteAndEscape(t *testing.T) {
	tests := []struct {
		name  string
		value string
	}{
		{"unix absolute", "/etc/passwd"},
		{"windows absolute", "C:/Windows/template.md"},
		{"unc absolute", "//server/share/template.md"},
		{"backslash", `plan\workitem-template.md`},
		{"escapes root", "../secret.md"},
		{"escapes via parent", "foo/../../secret.md"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			body := "prefix: AWIT\ntemplate: " + tt.value + "\n"
			if err := os.WriteFile(filepath.Join(dir, FileName), []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
			_, err := Load(dir)
			if err == nil {
				t.Fatal("Load() = nil, want template path error")
			}
			if !strings.Contains(err.Error(), "template") {
				t.Fatalf("Load() error = %v, want it to mention template", err)
			}
		})
	}
}

func TestTemplateWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := Default("AWIT")
	c.Template = "plan/workitem-template.md"
	if err := c.Write(dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.Template != "plan/workitem-template.md" {
		t.Fatalf("Template = %q", loaded.Template)
	}
}

func TestDeclaredLabelsLoad(t *testing.T) {
	tests := []struct {
		name string
		yaml string
		want []string
		err  string
	}{
		{name: "missing disables", yaml: "prefix: AWIT\n"},
		{name: "empty disables", yaml: "prefix: AWIT\nlabels: []\n"},
		{name: "keeps order and case", yaml: "prefix: AWIT\nlabels: [p1, P1, phase5]\n", want: []string{"p1", "P1", "phase5"}},
		{name: "dedupes in memory", yaml: "prefix: AWIT\nlabels: [p1, p1, p2]\n", want: []string{"p1", "p2"}},
		{name: "rejects empty", yaml: "prefix: AWIT\nlabels: [\"\"]\n", err: "config: labels entry must be nonempty"},
		{name: "rejects leading space", yaml: "prefix: AWIT\nlabels: [\" p1\"]\n", err: "config: labels entry must not have leading or trailing whitespace"},
		{name: "rejects trailing space", yaml: "prefix: AWIT\nlabels: [\"p1 \"]\n", err: "config: labels entry must not have leading or trailing whitespace"},
		{name: "rejects control", yaml: "prefix: AWIT\nlabels: [\"p1\\u0001\"]\n", err: "config: labels entry must not contain control characters"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, FileName), []byte(tt.yaml), 0o644); err != nil {
				t.Fatal(err)
			}
			c, err := Load(dir)
			if tt.err != "" {
				if err == nil || err.Error() != tt.err {
					t.Fatalf("Load() error = %v, want %q", err, tt.err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !slicesEqual(c.Labels, tt.want) {
				t.Fatalf("Labels = %q, want %q", c.Labels, tt.want)
			}
		})
	}
}

func TestDeclaredLabelsWriteRoundTrip(t *testing.T) {
	dir := t.TempDir()
	c := Default("AWIT")
	c.Labels = []string{"phase5", "p1"}
	if err := c.Write(dir); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !slicesEqual(loaded.Labels, []string{"phase5", "p1"}) {
		t.Fatalf("Labels = %q, want [phase5 p1]", loaded.Labels)
	}
}

func slicesEqual(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
