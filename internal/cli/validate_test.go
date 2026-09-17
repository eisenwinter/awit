package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unicode"

	"github.com/eisenwinter/awit/pkg/item"
)

func TestSentenceCount(t *testing.T) {
	tests := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"   ", 0},
		{"Hello", 1},
		{"Hello.", 1},
		{"One. Two. Three.", 3},
		{"One. Two. Three. Four.", 4},
		{"What? Yes! OK.", 3},
		{"Dr. Foo went home.", 2},
	}
	for _, tt := range tests {
		if got := sentenceCount(tt.in); got != tt.want {
			t.Errorf("sentenceCount(%q) = %d, want %d", tt.in, got, tt.want)
		}
	}
}

func TestValidateGoldens(t *testing.T) {
	tests := []struct {
		fixture string
		code    int
		rewrite func(t *testing.T, dir string, stdout string) string
	}{
		{fixture: "clean", code: 0},
		{fixture: "cyclic", code: 1},
		{fixture: "dangling", code: 1},
		{
			fixture: "conflicted",
			code:    1,
			rewrite: func(t *testing.T, dir, stdout string) string {
				t.Helper()
				p := filepath.Join(dir, ".awit", "items", "AWIT-TEST0001.md")
				abs, err := filepath.Abs(p)
				if err != nil {
					t.Fatal(err)
				}
				return strings.ReplaceAll(stdout, abs, "PATH")
			},
		},
		{fixture: "id-mismatch", code: 1},
		{
			fixture: "parse-error",
			code:    1,
			rewrite: func(t *testing.T, dir, stdout string) string {
				t.Helper()
				st, err := item.Open(dir)
				if err != nil {
					t.Fatal(err)
				}
				_, broken, err := st.LoadAll()
				if err != nil {
					t.Fatal(err)
				}
				if len(broken) != 1 || broken[0].Detail == "" {
					t.Fatalf("broken = %+v, want one PARSE ERROR with Detail", broken)
				}
				return strings.ReplaceAll(stdout, broken[0].Detail, "DETAIL")
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			dir := copyFixture(t, tt.fixture)
			code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "validate")
			if code != tt.code {
				t.Fatalf("exit %d, want %d stderr %q stdout %q", code, tt.code, stderr, stdout)
			}
			if tt.code == 0 && stderr != "" {
				t.Fatalf("stderr = %q, want empty", stderr)
			}
			got := stdout
			if tt.rewrite != nil {
				got = tt.rewrite(t, dir, stdout)
			}
			golden(t, "validate_"+tt.fixture+".golden", []byte(got))
		})
	}
}

func TestValidateJSON(t *testing.T) {
	dir := copyFixture(t, "dangling")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "validate")
	if code != 1 {
		t.Fatalf("exit %d stderr %q stdout %q", code, stderr, stdout)
	}
	var rows []struct {
		Reason string   `json:"reason"`
		IDs    []string `json:"ids"`
		Detail string   `json:"detail"`
		Fix    string   `json:"fix"`
	}
	if err := json.Unmarshal([]byte(stdout), &rows); err != nil {
		t.Fatalf("json: %v\n%s", err, stdout)
	}
	if len(rows) != 1 {
		t.Fatalf("len = %d, want 1\n%s", len(rows), stdout)
	}
	if rows[0].Reason != "DANGLING DEP" {
		t.Fatalf("reason = %q", rows[0].Reason)
	}
	if len(rows[0].IDs) != 1 || rows[0].IDs[0] != "AWIT-TEST0001" {
		t.Fatalf("ids = %v", rows[0].IDs)
	}
	if rows[0].Detail != "AWIT-TEST0001 depends on unknown AWIT-TEST9999" {
		t.Fatalf("detail = %q", rows[0].Detail)
	}
	if rows[0].Fix != "awit dep rm AWIT-TEST0001 AWIT-TEST9999" {
		t.Fatalf("fix = %q", rows[0].Fix)
	}
	if !strings.HasSuffix(stdout, "\n") {
		t.Fatal("json stdout must end with a newline")
	}
}

func TestValidateJSONClean(t *testing.T) {
	dir := copyFixture(t, "clean")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "validate")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stdout != "[]\n" {
		t.Fatalf("stdout = %q, want []\\n", stdout)
	}
}

func TestValidateWarnMissingBrief(t *testing.T) {
	dir := initRepo(t)
	st, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Save(item.New("AWIT-TEST0001", "No brief", "", nil, nil)); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "validate")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q (WARN must not fail)", code, stderr)
	}
	want := "PASS  1 items, 0 quarantined\nWARN  AWIT-TEST0001: missing brief\n"
	if stdout != want {
		t.Fatalf("stdout = %q, want %q", stdout, want)
	}
}

func TestValidateWarnLongBrief(t *testing.T) {
	dir := initRepo(t)
	st, err := item.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	brief := "One. Two. Three. Four."
	if err := st.Save(item.New("AWIT-TEST0001", "Long brief", brief, nil, nil)); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", dir, "--format", "compact", "validate")
	if code != 0 || stderr != "" {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if !strings.Contains(stdout, "PASS  1 items, 0 quarantined\n") {
		t.Fatalf("stdout = %q, want PASS", stdout)
	}
	if !strings.Contains(stdout, "WARN  AWIT-TEST0001: brief is longer than 3 sentences\n") {
		t.Fatalf("stdout = %q, want long-brief WARN", stdout)
	}
}

func TestValidateWarnDoesNotChangeFailExit(t *testing.T) {
	dir := copyFixture(t, "dangling")
	code, stdout, _ := run(t, "--repo", dir, "--format", "compact", "validate")
	if code != 1 {
		t.Fatalf("exit %d, want 1", code)
	}
	if strings.Contains(stdout, "WARN") {
		t.Fatalf("dangling fixture has valid briefs; stdout = %q", stdout)
	}
}

func TestSentenceCountTerminatorsNeedBoundary(t *testing.T) {
	if unicode.IsSpace(' ') != true {
		t.Fatal("sanity")
	}
	if sentenceCount("Hello.World") != 1 {
		t.Fatalf("sentenceCount(Hello.World) = %d, want 1 (dot not at a boundary)", sentenceCount("Hello.World"))
	}
}

func TestValidateIgnoresUnknownKeys(t *testing.T) {
	repo := initRepo(t)
	raw := []byte(`---
id: AWIT-TEST0001
title: External reserved
brief: An item that carries the reserved external key for a future mirror.
status: open
deps: []
labels: []
refs: []
external: gitlab#42
---

## Summary

Reserved key only.

## Acceptance Criteria

- validate does not FAIL on external
`)
	path := filepath.Join(repo, item.DirName, "items", "AWIT-TEST0001.md")
	if err := os.WriteFile(path, raw, 0o644); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := run(t, "--repo", repo, "validate")
	if code != 0 {
		t.Fatalf("validate exit %d stderr %q stdout %q", code, stderr, stdout)
	}
	if strings.Contains(stdout, "FAIL") || strings.Contains(stderr, "FAIL") {
		t.Fatalf("unknown key treated as FAIL:\nstdout=%q\nstderr=%q", stdout, stderr)
	}
	if !strings.Contains(stdout, "PASS") {
		t.Fatalf("validate stdout = %q, want PASS", stdout)
	}
}
