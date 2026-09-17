package item

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestParseStatus(t *testing.T) {
	t.Parallel()
	for _, s := range []string{"open", "in_progress", "closed"} {
		got, err := ParseStatus(s)
		if err != nil {
			t.Fatalf("ParseStatus(%q) err = %v", s, err)
		}
		if string(got) != s {
			t.Fatalf("ParseStatus(%q) = %q", s, got)
		}
	}
	_, err := ParseStatus("Open")
	if err == nil {
		t.Fatal("ParseStatus(\"Open\") want error")
	}
	msg := err.Error()
	for _, v := range []string{"open", "in_progress", "closed"} {
		if !strings.Contains(msg, v) {
			t.Fatalf("error %q must list %q", msg, v)
		}
	}
}

const fullDoc = `---
id: AWIT-TEST0006
title: Implement OAuth2 bearer token extraction
brief: >-
  The API gateway rejects valid bearer tokens.
status: in_progress
deps: [AWIT-TEST0005]
labels: [auth, api, p1]
assignee: agent/claude
claimed_at: 2026-09-17T14:32:05Z
refs:
  - ../comments/AWIT-TEST0006/20260917T143205Z-claude.md
  - ../../docs/architecture/auth-middleware-spec.md
---

## Summary

Fix header parsing.
`

func TestParseFull(t *testing.T) {
	it, err := Parse("/abs/AWIT-TEST0006.md", []byte(fullDoc))
	if err != nil {
		t.Fatal(err)
	}
	if it.ID != "AWIT-TEST0006" || it.Title != "Implement OAuth2 bearer token extraction" {
		t.Fatalf("id/title = %q %q", it.ID, it.Title)
	}
	if it.Brief != "The API gateway rejects valid bearer tokens." {
		t.Fatalf("brief = %q", it.Brief)
	}
	if it.Status != StatusInProgress {
		t.Fatalf("status = %q", it.Status)
	}
	if len(it.Deps) != 1 || it.Deps[0] != "AWIT-TEST0005" {
		t.Fatalf("deps = %#v", it.Deps)
	}
	if len(it.Labels) != 3 || it.Labels[0] != "auth" || it.Labels[2] != "p1" {
		t.Fatalf("labels = %#v", it.Labels)
	}
	if it.Assignee != "agent/claude" {
		t.Fatalf("assignee = %q", it.Assignee)
	}
	if it.ClaimedAt == nil || !it.ClaimedAt.Equal(time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)) {
		t.Fatalf("claimed_at = %v", it.ClaimedAt)
	}
	if len(it.Refs) != 2 || it.Refs[0] != "../comments/AWIT-TEST0006/20260917T143205Z-claude.md" {
		t.Fatalf("refs = %#v", it.Refs)
	}
	if it.Path != "/abs/AWIT-TEST0006.md" {
		t.Fatalf("path = %q", it.Path)
	}
}

func TestParseMissingTitle(t *testing.T) {
	_, err := Parse("x.md", []byte("---\nid: AWIT-TEST0001\nstatus: open\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "title") {
		t.Fatalf("err = %v, want it to name title", err)
	}
}

func TestParseBadClaimedAt(t *testing.T) {
	_, err := Parse("x.md", []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nclaimed_at: not-a-time\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "claimed_at") {
		t.Fatalf("err = %v, want claimed_at", err)
	}
}

func TestParseUnknownKeyKept(t *testing.T) {
	raw := []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal: gitlab#42\n---\n")
	it, err := Parse("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("external: gitlab#42")) {
		t.Fatalf("unknown key dropped:\n%s", got)
	}
}

func TestHasLabel(t *testing.T) {
	it, err := Parse("x.md", []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nlabels: [auth, p1]\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !it.HasLabel("auth") || !it.HasLabel("p1") || it.HasLabel("AUTH") || it.HasLabel("p0") {
		t.Fatalf("HasLabel mismatch labels=%v", it.Labels)
	}
}

func TestBodyRaw(t *testing.T) {
	it, err := Parse("x.md", []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\n---\n\n## Summary\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(it.Body(), []byte("\n## Summary\n")) {
		t.Fatalf("Body() = %q", it.Body())
	}
}
func TestRoundTripByteIdentical(t *testing.T) {
	docs := []string{
		"---\nid: AWIT-TEST0001\ntitle: Title\nstatus: open\n---\nbody\n",
		fullDoc,
		"---\nid: AWIT-TEST0001\ntitle: Title\nstatus: open\nexternal: gitlab#42 # comment\n---\n",
		"---\nid: AWIT-TEST0001\ntitle: Title\nstatus: open\ndeps: []\n---\n",
	}
	for i, raw := range docs {
		it, err := Parse("x.md", []byte(raw))
		if err != nil {
			t.Fatalf("doc %d parse: %v", i, err)
		}
		got, err := it.Bytes()
		if err != nil {
			t.Fatalf("doc %d bytes: %v", i, err)
		}
		if !bytes.Equal(got, []byte(raw)) {
			t.Fatalf("doc %d not byte-identical\ngot:\n%s\nwant:\n%s", i, got, raw)
		}
	}
}

func TestNewBytesGolden(t *testing.T) {
	it := New("AWIT-TEST0001", "Title", "Brief.", []string{"AWIT-TEST0002"}, []string{"auth", "p1"})
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	want := []byte("---\nid: AWIT-TEST0001\ntitle: Title\nbrief: Brief.\nstatus: open\ndeps: [AWIT-TEST0002]\nlabels: [auth, p1]\nrefs: []\n---\n\n## Summary\n\n## Acceptance Criteria\n\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("New Bytes mismatch\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestSetStatusOneLineDiff(t *testing.T) {
	raw := "---\nid: AWIT-TEST0001\ntitle: Title\nbrief: Brief.\nstatus: open\ndeps: []\nlabels: [auth]\nrefs: []\n---\n\n## Summary\n"
	it, err := Parse("x.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	it.SetStatus(StatusClosed)
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	wantLines := strings.Split(raw, "\n")
	gotLines := strings.Split(string(got), "\n")
	if len(gotLines) != len(wantLines) {
		t.Fatalf("line count got %d want %d\ngot:\n%s", len(gotLines), len(wantLines), got)
	}
	changed := 0
	var line string
	for i := range wantLines {
		if gotLines[i] != wantLines[i] {
			changed++
			line = gotLines[i]
		}
	}
	if changed != 1 {
		t.Fatalf("changed %d lines, want 1\ngot:\n%s", changed, got)
	}
	if line != "status: closed" {
		t.Fatalf("changed line = %q, want %q", line, "status: closed")
	}
}

func TestSetAssigneeEmptyDeletesKey(t *testing.T) {
	raw := "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nassignee: agent/claude\n---\n"
	it, err := Parse("x.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	it.SetAssignee("")
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("assignee:")) {
		t.Fatalf("assignee key still present:\n%s", got)
	}
}

func TestSetClaimedAtNilDeletesKey(t *testing.T) {
	raw := "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nclaimed_at: 2026-09-17T14:32:05Z\n---\n"
	it, err := Parse("x.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	it.SetClaimedAt(nil)
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("claimed_at:")) {
		t.Fatalf("claimed_at key still present:\n%s", got)
	}
}

func TestSetLabelsPreservesFlowStyle(t *testing.T) {
	raw := "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nlabels: [auth, p1]\n---\n"
	it, err := Parse("x.md", []byte(raw))
	if err != nil {
		t.Fatal(err)
	}
	it.SetLabels([]string{"x"})
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("labels: [x]")) {
		t.Fatalf("flow style lost:\n%s", got)
	}
}

func TestSetRefsBlockStyle(t *testing.T) {
	it := New("AWIT-TEST0001", "T", "B", nil, nil)
	it.SetRefs([]string{"../comments/AWIT-TEST0001/a.md"})
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("refs: [")) {
		t.Fatalf("want block refs, got flow:\n%s", got)
	}
	if !bytes.Contains(got, []byte("refs:\n")) {
		t.Fatalf("missing block refs key:\n%s", got)
	}
	if !bytes.Contains(got, []byte("- ../comments/AWIT-TEST0001/a.md")) {
		t.Fatalf("missing ref entry:\n%s", got)
	}
}

func TestSetBriefFolded(t *testing.T) {
	brief := strings.Repeat("abcdefghij", 12) // 120
	it := New("AWIT-TEST0001", "T", "short", nil, nil)
	it.SetBrief(brief)
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("brief: >-")) {
		t.Fatalf("want folded brief:\n%s", got)
	}
}

func TestParsePreservesExternalKey(t *testing.T) {
	raw := []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal: gitlab#42\n---\nbody\n")
	it, err := Parse("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	it.SetStatus(StatusInProgress)
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("external: gitlab#42")) {
		t.Fatalf("external key dropped after SetStatus:\n%s", got)
	}
	if !bytes.Contains(got, []byte("status: in_progress")) {
		t.Fatalf("status not updated:\n%s", got)
	}
	if bytes.Contains(got, []byte("status: open\n")) {
		t.Fatalf("old status still present:\n%s", got)
	}
}

func TestItemHasNoExternalField(t *testing.T) {
	st := reflect.TypeOf(Item{})
	for i := 0; i < st.NumField(); i++ {
		if st.Field(i).Name == "External" {
			t.Fatal("Item must not grow an External field; keep external: on the yaml node")
		}
	}
}
