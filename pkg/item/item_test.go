package item

import (
	"bytes"
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
	raw := []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nowner: platform\n---\n")
	it, err := Parse("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("owner: platform")) {
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
		"---\r\nid: AWIT-TEST0001\r\ntitle: Title\r\nstatus: open\r\n---\r\nbody\r\n",
		"---\nid: AWIT-TEST0001\ntitle:    Title\nstatus: open\n---\nbody\n",
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
	want := []byte("---\nid: AWIT-TEST0001\ntitle: Title\nbrief: Brief.\nstatus: open\ndeps: [AWIT-TEST0002]\nlabels: [auth, p1]\nrefs_base: repo\nrefs: []\n---\n\n## Summary\n\n## Acceptance Criteria\n\n")
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

const validExternalDoc = `---
id: AWIT-TEST0001
title: T
status: open
external:
  tracker: gitea
  repo: owner/repo
  id: 127
  url: https://forge.example/owner/repo/issues/127
---
body
`

func validExternal() External {
	return External{
		Tracker: "gitea",
		Repo:    "owner/repo",
		ID:      127,
		URL:     "https://forge.example/owner/repo/issues/127",
	}
}

func TestParseExternalMapping(t *testing.T) {
	it, err := Parse("x.md", []byte(validExternalDoc))
	if err != nil {
		t.Fatal(err)
	}
	if it.External == nil {
		t.Fatal("External is nil")
	}
	want := validExternal()
	if *it.External != want {
		t.Fatalf("External = %+v, want %+v", *it.External, want)
	}
	if it.ExternalProblem != "" {
		t.Fatalf("ExternalProblem = %q, want empty", it.ExternalProblem)
	}
}

func TestParseExternalExtraNestedKeys(t *testing.T) {
	raw := []byte(`---
id: AWIT-TEST0001
title: T
status: open
external:
  tracker: gitea
  repo: owner/repo
  id: 127
  url: https://forge.example/owner/repo/issues/127
  extra: keep-me
---
raw body
`)
	it, err := Parse("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	if it.External == nil || it.External.Repo != "owner/repo" {
		t.Fatalf("External = %+v", it.External)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("extra nested keys changed source bytes:\n%s", got)
	}
}

func TestSetExternalWriteAndClear(t *testing.T) {
	it := New("AWIT-TEST0001", "T", "B.", nil, nil)
	ext := validExternal()
	if err := it.SetExternal(&ext); err != nil {
		t.Fatal(err)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		"tracker: gitea",
		"repo: owner/repo",
		"id: 127",
		"url: https://forge.example/owner/repo/issues/127",
	} {
		if !bytes.Contains(got, []byte(want)) {
			t.Fatalf("missing %q in:\n%s", want, got)
		}
	}
	if it.External == nil || *it.External != ext {
		t.Fatalf("External = %+v", it.External)
	}
	if err := it.SetExternal(nil); err != nil {
		t.Fatal(err)
	}
	got, err = it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(got, []byte("external:")) {
		t.Fatalf("external key still present:\n%s", got)
	}
	if it.External != nil {
		t.Fatalf("External = %+v after clear", it.External)
	}
}

func TestSetExternalPreservesExtraNestedKeys(t *testing.T) {
	raw := []byte(`---
id: AWIT-TEST0001
title: T
status: open
external:
  tracker: gitea
  repo: owner/repo
  id: 127
  url: https://forge.example/owner/repo/issues/127
  extra: keep-me
---
raw body
`)
	it, err := Parse("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	ext := validExternal()
	ext.URL = "https://forge.example/gitea/owner/repo/issues/127"
	if err := it.SetExternal(&ext); err != nil {
		t.Fatal(err)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("extra: keep-me")) {
		t.Fatalf("extra nested key dropped:\n%s", got)
	}
	if !bytes.Contains(got, []byte("url: https://forge.example/gitea/owner/repo/issues/127")) {
		t.Fatalf("url not updated:\n%s", got)
	}
	if !bytes.Contains(got, []byte("raw body\n")) {
		t.Fatalf("body changed:\n%s", got)
	}
}

func TestParseInvalidExternalTypes(t *testing.T) {
	tests := []struct {
		name string
		raw  string
	}{
		{name: "sequence tracker", raw: "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal:\n  tracker: [gitea]\n  repo: owner/repo\n  id: 127\n  url: https://forge.example/owner/repo/issues/127\n---\n"},
		{name: "float id", raw: "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal:\n  tracker: gitea\n  repo: owner/repo\n  id: 127.5\n  url: https://forge.example/owner/repo/issues/127\n---\n"},
		{name: "missing url", raw: "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal:\n  tracker: gitea\n  repo: owner/repo\n  id: 127\n---\n"},
		{name: "duplicate tracker", raw: "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal:\n  tracker: gitea\n  tracker: gitea\n  repo: owner/repo\n  id: 127\n  url: https://forge.example/owner/repo/issues/127\n---\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw := []byte(tt.raw)
			it, err := Parse("x.md", raw)
			if err != nil {
				t.Fatal(err)
			}
			if it.External != nil {
				t.Fatalf("External = %+v, want nil", it.External)
			}
			if it.ExternalProblem == "" {
				t.Fatal("ExternalProblem empty")
			}
			got, err := it.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, raw) {
				t.Fatalf("invalid mapping rewritten:\n%s", got)
			}
		})
	}
}

func TestParseOldExternalScalarPreserved(t *testing.T) {
	raw := []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal: gitlab#42\n---\nbody\n")
	it, err := Parse("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	if it.External != nil {
		t.Fatalf("External = %+v, want nil", it.External)
	}
	if it.ExternalProblem == "" {
		t.Fatal("ExternalProblem empty for old scalar")
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("old scalar rewritten:\n%s", got)
	}
}

func TestSetExternalRejectsInvalid(t *testing.T) {
	it := New("AWIT-TEST0001", "T", "B.", nil, nil)
	tests := []External{
		{Tracker: "github", Repo: "owner/repo", ID: 127, URL: "https://forge.example/owner/repo/issues/127"},
		{Tracker: "gitea", Repo: "owner", ID: 127, URL: "https://forge.example/owner/repo/issues/127"},
		{Tracker: "gitea", Repo: "owner/repo", ID: 0, URL: "https://forge.example/owner/repo/issues/127"},
		{Tracker: "gitea", Repo: "owner/repo", ID: 127, URL: "https://user:pass@forge.example/owner/repo/issues/127"},
		{Tracker: "gitea", Repo: "owner/repo", ID: 127, URL: "https://forge.example/owner/repo/issues/127?x=1"},
		{Tracker: "gitea", Repo: "owner/repo", ID: 127, URL: "https://forge.example/owner/repo/issues/127#frag"},
		{Tracker: "gitea", Repo: "owner/repo", ID: 127, URL: "https://forge.example/owner/other/issues/127"},
		{Tracker: "gitea", Repo: "owner/repo", ID: 127, URL: "https://forge.example/owner/repo/issues/127/extra"},
		{Tracker: "gitea", Repo: "owner/repo", ID: 127, URL: "https://forge.example/owner/repo/../owner/repo/issues/127"},
		{Tracker: "gitea", Repo: "owner/repo", ID: 127, URL: "https://forge.example/owner/repo%2fissues/127"},
		{Tracker: "gitea", Repo: "own er/repo", ID: 127, URL: "https://forge.example/own%20er/repo/issues/127"},
		{Tracker: "gitea", Repo: ".", ID: 127, URL: "https://forge.example/./issues/127"},
		{Tracker: "gitea", Repo: "owner/..", ID: 127, URL: "https://forge.example/owner/../issues/127"},
	}
	for i, ext := range tests {
		if err := it.SetExternal(&ext); err == nil {
			t.Fatalf("case %d: SetExternal(%+v) succeeded", i, ext)
		}
		if it.External != nil {
			t.Fatalf("case %d: External mutated to %+v", i, it.External)
		}
	}
}

func TestSetExternalIdenticalMappingNoop(t *testing.T) {
	raw := []byte(`---
id: AWIT-TEST0001
title: T
status: open
external:
  tracker: gitea
  repo: owner/repo
  id: 127
  url: https://forge.example/owner/repo/issues/127
---
body
`)
	it, err := Parse("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	ext := validExternal()
	if err := it.SetExternal(&ext); err != nil {
		t.Fatal(err)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("identical mapping rewrote bytes:\n%s", got)
	}
}

func TestStatusOnlyEditPreservesExternalSubkeysAndBody(t *testing.T) {
	raw := []byte(`---
id: AWIT-TEST0001
title: T
status: open
external:
  tracker: gitea
  repo: owner/repo
  id: 127
  url: https://forge.example/owner/repo/issues/127
  extra: keep-me
---
raw body
`)
	it, err := Parse("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	it.SetStatus(StatusInProgress)
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("status: in_progress")) {
		t.Fatalf("status not updated:\n%s", got)
	}
	if !bytes.Contains(got, []byte("extra: keep-me")) {
		t.Fatalf("extra nested key dropped:\n%s", got)
	}
	if !bytes.Contains(got, []byte("url: https://forge.example/owner/repo/issues/127")) {
		t.Fatalf("url rewritten:\n%s", got)
	}
	if !bytes.HasSuffix(got, []byte("raw body\n")) {
		t.Fatalf("body not preserved:\n%s", got)
	}
}

func TestSetBodyOwnsCopy(t *testing.T) {
	it := New("AWIT-TEST0001", "T", "B.", nil, nil)
	body := []byte("hello\r\n")
	it.SetBody(body)
	body[0] = 'x'
	if !bytes.Equal(it.Body(), []byte("hello\r\n")) {
		t.Fatalf("SetBody did not own a copy: %q", it.Body())
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("hello\r\n")) {
		t.Fatalf("CRLF body normalized:\n%s", got)
	}
}

func TestValidateExternalPrefixURL(t *testing.T) {
	ext := validExternal()
	ext.URL = "https://forge.example/gitea/owner/repo/issues/127"
	if err := ValidateExternal(ext); err != nil {
		t.Fatalf("prefix URL rejected: %v", err)
	}
}

func validGitLabExternal() External {
	return External{
		Tracker: "gitlab",
		Repo:    "group/sub/project",
		ID:      127,
		URL:     "https://forge.example/apps/gitlab/group/sub/project/-/work_items/127",
	}
}

const validGitLabDoc = `---
id: AWIT-TEST0001
title: T
status: open
external:
  tracker: gitlab
  repo: group/sub/project
  id: 127
  url: https://forge.example/apps/gitlab/group/sub/project/-/work_items/127
---
body
`

func TestExternalGitLabValidation(t *testing.T) {
	e := External{
		Tracker: "gitlab", Repo: "group/sub/project", ID: 127,
		URL: "https://forge.example/apps/gitlab/group/sub/project/-/work_items/127",
	}
	if err := ValidateExternal(e); err != nil {
		t.Fatalf("valid GitLab issue mapping: %v", err)
	}

	valid := []External{
		validGitLabExternal(),
		{Tracker: "gitlab", Repo: "group/project", ID: 42, URL: "https://gitlab.example/group/project/-/issues/42"},
		{Tracker: "gitlab", Repo: "group/project", ID: 42, URL: "http://gitlab.example/gitlab/group/project/-/issues/42"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/-/issues/127"},
		{Tracker: "gitlab", Repo: "group/my.project", ID: 7, URL: "https://forge.example/group/my.project/-/work_items/7"},
		validExternal(),
		{Tracker: "gitea", Repo: "owner/repo", ID: 127, URL: "https://forge.example/gitea/owner/repo/issues/127"},
	}
	for i, ext := range valid {
		if err := ValidateExternal(ext); err != nil {
			t.Fatalf("valid[%d] %+v: %v", i, ext, err)
		}
	}

	invalid := []External{
		{Tracker: "github", Repo: "group/project", ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "GitLab", Repo: "group/project", ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "project", ID: 1, URL: "https://forge.example/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "/group/project", ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "group/project/", ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "group//project", ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "group/./project", ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "group/../project", ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "group/pro ject", ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "group/pro\u00a0ject", ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "gitlab", Repo: `group\project`, ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "group/pro%ject", ID: 1, URL: "https://forge.example/group/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 0, URL: "https://forge.example/group/sub/project/-/issues/1"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: -3, URL: "https://forge.example/group/sub/project/-/issues/3"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://user:pass@forge.example/group/sub/project/-/issues/127"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/-/issues/127?x=1"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/-/issues/127?"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/-/issues/127#frag"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/-/issues/127#"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/other/project/-/issues/127"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/-/issues/128"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/-/issues/127/extra"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/issues/127"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group%2Fsub/project/-/issues/127"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project%2f-/issues/127"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/-/issues/127%5c"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/%2e%2e/project/-/issues/127"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/apps//gitlab/group/sub/project/-/issues/127"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/apps/../group/sub/project/-/issues/127"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/-/issues/127/"},
		{Tracker: "gitlab", Repo: "group/sub/project", ID: 127, URL: "ftp://forge.example/group/sub/project/-/issues/127"},
		{Tracker: "gitea", Repo: "group/sub/project", ID: 127, URL: "https://forge.example/group/sub/project/issues/127"},
	}
	for i, ext := range invalid {
		if err := ValidateExternal(ext); err == nil {
			t.Fatalf("invalid[%d] %+v: accepted", i, ext)
		}
	}
}

func TestExternalGitLabRoundTrip(t *testing.T) {
	raw := []byte(validGitLabDoc)
	it, err := Parse("x.md", raw)
	if err != nil {
		t.Fatal(err)
	}
	if it.External == nil {
		t.Fatal("External is nil")
	}
	want := validGitLabExternal()
	if *it.External != want {
		t.Fatalf("External = %+v, want %+v", *it.External, want)
	}
	if it.ExternalProblem != "" {
		t.Fatalf("ExternalProblem = %q, want empty", it.ExternalProblem)
	}
	got, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, raw) {
		t.Fatalf("GitLab mapping rewritten:\n%s", got)
	}

	gitea, err := Parse("x.md", []byte(validExternalDoc))
	if err != nil {
		t.Fatal(err)
	}
	got, err = gitea.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, []byte(validExternalDoc)) {
		t.Fatalf("Gitea mapping rewritten:\n%s", got)
	}

	extra := []byte(`---
id: AWIT-TEST0001
title: T
status: open
external:
  tracker: gitlab
  repo: group/sub/project
  id: 127
  url: https://forge.example/apps/gitlab/group/sub/project/-/work_items/127
  extra: keep-me
---
raw body
`)
	it, err = Parse("x.md", extra)
	if err != nil {
		t.Fatal(err)
	}
	got, err = it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, extra) {
		t.Fatalf("extra nested keys changed source bytes:\n%s", got)
	}
	it.SetStatus(StatusInProgress)
	got, err = it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(got, []byte("extra: keep-me")) {
		t.Fatalf("extra nested key dropped:\n%s", got)
	}
	if !bytes.Contains(got, []byte("url: https://forge.example/apps/gitlab/group/sub/project/-/work_items/127")) {
		t.Fatalf("url rewritten:\n%s", got)
	}
	if !bytes.HasSuffix(got, []byte("raw body\n")) {
		t.Fatalf("body not preserved:\n%s", got)
	}

	fresh := New("AWIT-TEST0001", "T", "B.", nil, nil)
	ext := validGitLabExternal()
	if err := fresh.SetExternal(&ext); err != nil {
		t.Fatal(err)
	}
	if err := fresh.SetExternal(&ext); err != nil {
		t.Fatal(err)
	}
	got, err = fresh.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Count(got, []byte("tracker: gitlab")) != 1 {
		t.Fatalf("identical SetExternal rewrote mapping:\n%s", got)
	}

	malformed := []string{
		"---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal:\n  tracker: [gitlab]\n  repo: group/project\n  id: 127\n  url: https://forge.example/group/project/-/issues/127\n---\n",
		"---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal:\n  tracker: gitlab\n  repo: group/project\n  id: 127.5\n  url: https://forge.example/group/project/-/issues/127\n---\n",
		"---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal:\n  tracker: gitlab\n  repo: group/project\n  id: 9223372036854775808\n  url: https://forge.example/group/project/-/issues/1\n---\n",
		"---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal:\n  tracker: gitlab\n  repo: group/project\n  id: 127\n---\n",
		"---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal:\n  tracker: gitlab\n  tracker: gitlab\n  repo: group/project\n  id: 127\n  url: https://forge.example/group/project/-/issues/127\n---\n",
		"---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nexternal: gitlab#42\n---\nbody\n",
	}
	for i, doc := range malformed {
		raw := []byte(doc)
		it, err := Parse("x.md", raw)
		if err != nil {
			t.Fatalf("malformed[%d] parse: %v", i, err)
		}
		if it.External != nil {
			t.Fatalf("malformed[%d] External = %+v, want nil", i, it.External)
		}
		if it.ExternalProblem == "" {
			t.Fatalf("malformed[%d] ExternalProblem empty", i)
		}
		got, err := it.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, raw) {
			t.Fatalf("malformed[%d] rewritten:\n%s", i, got)
		}
	}
}

func TestParseAlias(t *testing.T) {
	it, err := Parse("/abs/AWIT-TEST0001.md", []byte(`---
id: AWIT-TEST0001
title: T
status: open
alias: DTRM-F21
---

body
`))
	if err != nil {
		t.Fatal(err)
	}
	if it.Alias != "DTRM-F21" {
		t.Fatalf("Alias = %q", it.Alias)
	}
	it2, err := Parse("/abs/AWIT-TEST0002.md", []byte("---\nid: AWIT-TEST0002\ntitle: T\nstatus: open\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if it2.Alias != "" {
		t.Fatalf("missing alias must parse as empty, got %q", it2.Alias)
	}
}

func TestParseAliasNonScalar(t *testing.T) {
	_, err := Parse("/abs/AWIT-TEST0001.md", []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nalias: [a, b]\n---\n"))
	if err == nil || !strings.Contains(err.Error(), "alias") {
		t.Fatalf("err = %v, want alias must be a string", err)
	}
}

func TestSetAliasValidation(t *testing.T) {
	valid := []string{"DTRM-F21", "a", "a.b_c-d", "x" + strings.Repeat("y", 126)}
	for _, v := range valid {
		it := New("AWIT-TEST0001", "T", "B.", nil, nil)
		if err := it.SetAlias(v); err != nil {
			t.Fatalf("SetAlias(%q) err = %v", v, err)
		}
		if it.Alias != v {
			t.Fatalf("SetAlias(%q): Alias = %q", v, it.Alias)
		}
	}
	invalid := []string{
		"1starts-digit", "-starts-dash", ".starts-dot",
		"has space", "has/slash", "has#hash", "has\ttab",
		"x" + strings.Repeat("y", 128), // 129 chars
		"AWIT-TEST0001",                // canonical ID shape
		"awit-test0001",                // case-insensitive canonical ID shape
	}
	for _, v := range invalid {
		it := New("AWIT-TEST0001", "T", "B.", nil, nil)
		if err := it.SetAlias(v); err == nil {
			t.Fatalf("SetAlias(%q) must fail", v)
		}
		if it.Alias != "" {
			t.Fatalf("SetAlias(%q) failed but Alias = %q", v, it.Alias)
		}
	}
}

func TestSetAliasEmptyClears(t *testing.T) {
	it := New("AWIT-TEST0001", "T", "B.", nil, nil)
	if err := it.SetAlias("DTRM-F21"); err != nil {
		t.Fatal(err)
	}
	if err := it.SetAlias(""); err != nil {
		t.Fatal(err)
	}
	if it.Alias != "" {
		t.Fatalf("Alias = %q after clear", it.Alias)
	}
	b, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(b, []byte("alias")) {
		t.Fatalf("cleared alias still serialized:\n%s", b)
	}
}

func TestSetAliasSerialization(t *testing.T) {
	it := New("AWIT-TEST0001", "T", "B.", nil, nil)
	if err := it.SetAlias("DTRM-F21"); err != nil {
		t.Fatal(err)
	}
	b, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("alias: DTRM-F21\n")) {
		t.Fatalf("alias not serialized:\n%s", b)
	}
	back, err := Parse("/abs/AWIT-TEST0001.md", b)
	if err != nil {
		t.Fatal(err)
	}
	if back.Alias != "DTRM-F21" {
		t.Fatalf("round-trip Alias = %q", back.Alias)
	}
}

func TestValidateAlias(t *testing.T) {
	if err := ValidateAlias("DTRM-F21"); err != nil {
		t.Fatal(err)
	}
	for _, v := range []string{"", "1x", "a b", "AWIT-TEST0001"} {
		if err := ValidateAlias(v); err == nil {
			t.Fatalf("ValidateAlias(%q) must fail", v)
		}
	}
}

func TestBlockedReasonParseMalformed(t *testing.T) {
	t.Parallel()
	const badValue = "item: blocked_reason must be a non-empty, single-line string without control characters"
	malformed := map[string]string{
		"sequence type":    "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nblocked_reason: [waiting, vendor]\n---\n",
		"integer type":     "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nblocked_reason: 42\n---\n",
		"whitespace-only":  "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nblocked_reason: \"   \"\n---\n",
		"embedded tab":     "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nblocked_reason: \"a\\tb\"\n---\n",
		"embedded newline": "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nblocked_reason: \"a\\nb\"\n---\n",
	}
	for name, doc := range malformed {
		if _, err := Parse("/abs/AWIT-TEST0001.md", []byte(doc)); err == nil || err.Error() != badValue {
			t.Errorf("%s: err = %v, want exactly %q", name, err, badValue)
		}
	}
	dup := "---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nblocked_reason: first\nblocked_reason: second\n---\n"
	if _, err := Parse("/abs/AWIT-TEST0001.md", []byte(dup)); err == nil || err.Error() != "item: duplicate key blocked_reason" {
		t.Errorf("duplicate key: err = %v, want exactly %q", err, "item: duplicate key blocked_reason")
	}
}

func TestBlockedReasonParseValid(t *testing.T) {
	t.Parallel()
	it, err := Parse("/abs/AWIT-TEST0001.md", []byte("---\nid: AWIT-TEST0001\ntitle: T\nstatus: open\nblocked_reason: waiting on vendor\n---\n\nbody\n"))
	if err != nil {
		t.Fatal(err)
	}
	if it.BlockedReason != "waiting on vendor" {
		t.Fatalf("BlockedReason = %q, want %q", it.BlockedReason, "waiting on vendor")
	}
	padded, err := Parse("/abs/AWIT-TEST0002.md", []byte("---\nid: AWIT-TEST0002\ntitle: T\nstatus: open\nblocked_reason: \"  padded  \"\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if padded.BlockedReason != "padded" {
		t.Fatalf("BlockedReason = %q, want trimmed %q", padded.BlockedReason, "padded")
	}
	absent, err := Parse("/abs/AWIT-TEST0003.md", []byte("---\nid: AWIT-TEST0003\ntitle: T\nstatus: open\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if absent.BlockedReason != "" {
		t.Fatalf("missing blocked_reason must parse as empty, got %q", absent.BlockedReason)
	}
}

func TestBlockedReasonSetAndClear(t *testing.T) {
	t.Parallel()
	it := New("AWIT-TEST0001", "T", "B.", []string{"AWIT-TEST0002"}, []string{"auth"})
	beforeBody := append([]byte(nil), it.Body()...)
	if err := it.SetBlockedReason("  waiting on vendor  "); err != nil {
		t.Fatal(err)
	}
	if it.BlockedReason != "waiting on vendor" {
		t.Fatalf("BlockedReason = %q, want trimmed %q", it.BlockedReason, "waiting on vendor")
	}
	b, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(b, []byte("blocked_reason: waiting on vendor\n")) {
		t.Fatalf("blocked_reason not serialized:\n%s", b)
	}
	for _, want := range []string{"title: T", "AWIT-TEST0002", "auth"} {
		if !bytes.Contains(b, []byte(want)) {
			t.Fatalf("unrelated field %q lost after set:\n%s", want, b)
		}
	}
	back, err := Parse("/abs/AWIT-TEST0001.md", b)
	if err != nil {
		t.Fatal(err)
	}
	if back.BlockedReason != "waiting on vendor" {
		t.Fatalf("round-trip BlockedReason = %q", back.BlockedReason)
	}
	if string(back.Body()) != string(beforeBody) {
		t.Fatalf("body changed across set/round-trip: %q", back.Body())
	}
	if err := it.SetBlockedReason(""); err != nil {
		t.Fatal(err)
	}
	if it.BlockedReason != "" {
		t.Fatalf("BlockedReason = %q after clear", it.BlockedReason)
	}
	cleared, err := it.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(cleared, []byte("blocked_reason")) {
		t.Fatalf("cleared blocked_reason still serialized:\n%s", cleared)
	}
	if string(it.Body()) != string(beforeBody) {
		t.Fatalf("body changed across clear: %q", it.Body())
	}
}

func TestBlockedReasonSetInvalid(t *testing.T) {
	t.Parallel()
	for _, v := range []string{"   ", "a\nb", "a\rb", "a\tb", "a\x7fb"} {
		it := New("AWIT-TEST0001", "T", "B.", nil, nil)
		raw, err := it.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := it.SetBlockedReason(v); err == nil {
			t.Fatalf("SetBlockedReason(%q) must fail", v)
		} else if err.Error() != "item: blocked_reason must be a non-empty, single-line string without control characters" {
			t.Fatalf("SetBlockedReason(%q) err = %q, want reviewed wording", v, err)
		}
		if it.BlockedReason != "" {
			t.Fatalf("SetBlockedReason(%q) failed but BlockedReason = %q", v, it.BlockedReason)
		}
		after, err := it.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(raw, after) {
			t.Fatalf("SetBlockedReason(%q) failed but mutated bytes:\n%s", v, after)
		}
	}
}
