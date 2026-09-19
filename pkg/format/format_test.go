package format

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/item"
)

// nonTTY returns an open handle to a regular file inside t.TempDir().
//
// Why not os.DevNull: on Linux and macOS /dev/null IS a character device, so
// os.ModeCharDevice is set and IsTerminal would report true.
// Why not os.Stdout: under `go test` it is a pipe, but when the compiled test
// binary is run directly from a shell it is a terminal, so the expected value
// would depend on how the test was started.
func nonTTY(t *testing.T) *os.File {
	t.Helper()
	f, err := os.Create(filepath.Join(t.TempDir(), "out.txt"))
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return f
}

func TestDetect(t *testing.T) {
	tests := []struct {
		name    string
		flag    string
		want    Format
		wantErr string
	}{
		{name: "flag compact", flag: "compact", want: Compact},
		{name: "flag table", flag: "table", want: Table},
		{name: "flag json", flag: "json", want: JSON},
		{name: "unknown flag", flag: "yaml", wantErr: `format: unknown format "yaml" (compact|table|json)`},
		{name: "empty flag, no tty", flag: "", want: Compact},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Detect(tt.flag, nonTTY(t))
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("Detect(%q) = %q, want error", tt.flag, got)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("error = %q, want %q", err.Error(), tt.wantErr)
				}
				if got != "" {
					t.Fatalf("format on error = %q, want empty", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Detect(%q): unexpected error %v", tt.flag, err)
			}
			if got != tt.want {
				t.Fatalf("Detect(%q) = %q, want %q", tt.flag, got, tt.want)
			}
		})
	}
}

func TestIsTerminalFalseForFile(t *testing.T) {
	if IsTerminal(nonTTY(t)) {
		t.Fatal("IsTerminal(regular file) = true, want false")
	}
	if IsTerminal(nil) {
		t.Fatal("IsTerminal(nil) = true, want false")
	}
	closed, err := os.Create(filepath.Join(t.TempDir(), "closed.txt"))
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if err := closed.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if IsTerminal(closed) {
		t.Fatal("IsTerminal(closed file) = true, want false (Stat fails)")
	}
}

// sampleEntries is the fixed input behind every golden file in this package.
// Entry B deliberately leaves Labels nil to exercise the JSON normalisation.
func sampleEntries() []Entry {
	return []Entry{
		{
			ID:       "AWIT-TEST0001",
			Title:    "Implement OAuth2 token extraction",
			Brief:    "Fix header parsing.",
			Status:   "open",
			State:    "ready",
			Labels:   []string{"auth", "p1"},
			Deps:     []string{},
			Unblocks: 2,
		},
		{
			ID:       "AWIT-TEST0003",
			Title:    "Add E2E auth tests",
			Status:   "open",
			State:    "blocked",
			Deps:     []string{"AWIT-TEST0001"},
			Unblocks: 0,
		},
		{
			ID:       "AWIT-TEST0009",
			Title:    "Rotate tokens",
			Status:   "open",
			State:    "quarantined",
			Labels:   []string{"p0"},
			Deps:     []string{"AWIT-TEST0009"},
			Unblocks: -1,
			Faults:   []string{"[CYCLE] AWIT-TEST0009 -> AWIT-TEST0009"},
		},
	}
}

func TestLine(t *testing.T) {
	entries := sampleEntries()
	tests := []struct {
		name string
		in   Entry
		want string
	}{
		{
			name: "labels and unblocks",
			in:   entries[0],
			want: "[AWIT-TEST0001] open Implement OAuth2 token extraction | auth,p1 | Unblocks: 2",
		},
		{
			name: "no labels renders dash",
			in:   entries[1],
			want: "[AWIT-TEST0003] open Add E2E auth tests | - | Unblocks: 0",
		},
		{
			name: "quarantined suffix and negative unblocks",
			in:   entries[2],
			want: "[AWIT-TEST0009] open Rotate tokens | p0 | Unblocks: -1 | QUARANTINED",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Line(tt.in); got != tt.want {
				t.Fatalf("Line() =\n%q\nwant\n%q", got, tt.want)
			}
		})
	}
}

func TestWriteCompactGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, Compact, sampleEntries()); err != nil {
		t.Fatalf("Write compact: %v", err)
	}
	golden(t, "format_compact.golden", buf.Bytes())
}

func TestWriteTableGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, Table, sampleEntries()); err != nil {
		t.Fatalf("Write table: %v", err)
	}
	golden(t, "format_table.golden", buf.Bytes())
}

func TestWriteJSONGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, JSON, sampleEntries()); err != nil {
		t.Fatalf("Write json: %v", err)
	}
	golden(t, "format_json.golden", buf.Bytes())

	var back []Entry
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("json round-trip: %v", err)
	}
	if len(back) != 3 {
		t.Fatalf("decoded %d entries, want 3", len(back))
	}
	if back[1].Labels == nil || len(back[1].Labels) != 0 {
		t.Fatalf("entry B labels = %#v, want empty non-nil slice", back[1].Labels)
	}
	if len(back[0].Deps) != 0 {
		t.Fatalf("entry A deps = %#v, want empty", back[0].Deps)
	}
	if back[2].Unblocks != -1 {
		t.Fatalf("entry C unblocks = %d, want -1", back[2].Unblocks)
	}
}

func TestWriteJSONEmpty(t *testing.T) {
	for _, in := range [][]Entry{nil, {}} {
		var buf bytes.Buffer
		if err := Write(&buf, JSON, in); err != nil {
			t.Fatalf("Write json: %v", err)
		}
		if got := buf.String(); got != "[]\n" {
			t.Fatalf("Write(json, %#v) = %q, want %q", in, got, "[]\n")
		}
	}
}

func TestWriteCompactEmpty(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, Compact, nil); err != nil {
		t.Fatalf("Write compact: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("Write(compact, nil) = %q, want empty", buf.String())
	}
}

func TestWriteJSONDoesNotMutateInput(t *testing.T) {
	entries := sampleEntries()
	var buf bytes.Buffer
	if err := Write(&buf, JSON, entries); err != nil {
		t.Fatalf("Write json: %v", err)
	}
	if entries[1].Labels != nil {
		t.Fatalf("input entry mutated: labels = %#v, want nil", entries[1].Labels)
	}
}

func TestWriteOneTableGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, Table, sampleEntries()[0]); err != nil {
		t.Fatalf("WriteOne table: %v", err)
	}
	golden(t, "format_one_table.golden", buf.Bytes())
}

func TestWriteOneTableSkipsEmptyAndKeepsUnblocks(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, Table, sampleEntries()[1]); err != nil {
		t.Fatalf("WriteOne table: %v", err)
	}
	got := buf.String()
	want := "ID: AWIT-TEST0003\nTitle: Add E2E auth tests\nStatus: open\nState: blocked\nDeps: AWIT-TEST0001\nUnblocks: 0\n"
	if got != want {
		t.Fatalf("WriteOne table =\n%q\nwant\n%q", got, want)
	}
}

func TestWriteOneTableFaults(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, Table, sampleEntries()[2]); err != nil {
		t.Fatalf("WriteOne table: %v", err)
	}
	got := buf.String()
	want := "ID: AWIT-TEST0009\nTitle: Rotate tokens\nStatus: open\nState: quarantined\n" +
		"Labels: p0\nDeps: AWIT-TEST0009\nUnblocks: -1\nFaults: [CYCLE] AWIT-TEST0009 -> AWIT-TEST0009\n"
	if got != want {
		t.Fatalf("WriteOne table =\n%q\nwant\n%q", got, want)
	}
}

func TestWriteOneCompact(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, Compact, sampleEntries()[0]); err != nil {
		t.Fatalf("WriteOne compact: %v", err)
	}
	want := "[AWIT-TEST0001] open Implement OAuth2 token extraction | auth,p1 | Unblocks: 2\n"
	if got := buf.String(); got != want {
		t.Fatalf("WriteOne compact = %q, want %q", got, want)
	}
}

func TestWriteOneJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteOne(&buf, JSON, sampleEntries()[1]); err != nil {
		t.Fatalf("WriteOne json: %v", err)
	}
	if !bytes.HasPrefix(buf.Bytes(), []byte("{\n")) {
		t.Fatalf("WriteOne json = %q, want a single object", buf.String())
	}
	if !bytes.HasSuffix(buf.Bytes(), []byte("}\n")) {
		t.Fatalf("WriteOne json = %q, want trailing newline after object", buf.String())
	}
	var back Entry
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.ID != "AWIT-TEST0003" || back.Labels == nil || len(back.Labels) != 0 {
		t.Fatalf("decoded = %#v, want id AWIT-TEST0003 and empty non-nil labels", back)
	}
}

func sampleLabels() []LabelCount {
	return []LabelCount{{Label: "auth", Count: 3}, {Label: "p1", Count: 3}, {Label: "db", Count: 1}}
}

func TestWriteLabelsGolden(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, Table, sampleLabels()); err != nil {
		t.Fatalf("WriteLabels table: %v", err)
	}
	golden(t, "format_labels_table.golden", buf.Bytes())
}

func TestWriteLabelsCompact(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, Compact, sampleLabels()); err != nil {
		t.Fatalf("WriteLabels compact: %v", err)
	}
	want := "auth 3\np1 3\ndb 1\n"
	if got := buf.String(); got != want {
		t.Fatalf("WriteLabels compact = %q, want %q", got, want)
	}
}

func TestWriteLabelsJSON(t *testing.T) {
	var buf bytes.Buffer
	if err := WriteLabels(&buf, JSON, sampleLabels()); err != nil {
		t.Fatalf("WriteLabels json: %v", err)
	}
	want := "[\n  {\n    \"label\": \"auth\",\n    \"count\": 3\n  },\n  {\n    \"label\": \"p1\",\n    \"count\": 3\n  },\n  {\n    \"label\": \"db\",\n    \"count\": 1\n  }\n]\n"
	if got := buf.String(); got != want {
		t.Fatalf("WriteLabels json =\n%q\nwant\n%q", got, want)
	}
}

func TestWriteLabelsEmptyJSON(t *testing.T) {
	for _, in := range [][]LabelCount{nil, {}} {
		var buf bytes.Buffer
		if err := WriteLabels(&buf, JSON, in); err != nil {
			t.Fatalf("WriteLabels json: %v", err)
		}
		if got := buf.String(); got != "[]\n" {
			t.Fatalf("WriteLabels(json, %#v) = %q, want %q", in, got, "[]\n")
		}
	}
}

func TestWriteUnknownFormat(t *testing.T) {
	const want = `format: unknown format "yaml" (compact|table|json)`
	var buf bytes.Buffer
	if err := Write(&buf, Format("yaml"), sampleEntries()); err == nil || err.Error() != want {
		t.Fatalf("Write error = %v, want %q", err, want)
	}
	if err := WriteOne(&buf, Format("yaml"), sampleEntries()[0]); err == nil || err.Error() != want {
		t.Fatalf("WriteOne error = %v, want %q", err, want)
	}
	if err := WriteLabels(&buf, Format("yaml"), sampleLabels()); err == nil || err.Error() != want {
		t.Fatalf("WriteLabels error = %v, want %q", err, want)
	}
}

func sampleExternal() *item.External {
	return &item.External{
		Tracker: "gitea",
		Repo:    "owner/repo",
		ID:      127,
		URL:     "https://forge.example/owner/repo/issues/127",
	}
}

func TestLineExternal(t *testing.T) {
	e := sampleEntries()[0]
	e.External = sampleExternal()
	want := "[AWIT-TEST0001] open Implement OAuth2 token extraction | auth,p1 | Unblocks: 2 | External: gitea owner/repo#127"
	if got := Line(e); got != want {
		t.Fatalf("Line() =\n%q\nwant\n%q", got, want)
	}
}

func TestWriteTableExternalColumn(t *testing.T) {
	entries := sampleEntries()
	entries[0].External = sampleExternal()
	var buf bytes.Buffer
	if err := Write(&buf, Table, entries); err != nil {
		t.Fatal(err)
	}
	got := buf.String()
	if !strings.Contains(got, "EXTERNAL") {
		t.Fatalf("missing EXTERNAL column:\n%s", got)
	}
	if !strings.Contains(got, "gitea owner/repo#127") {
		t.Fatalf("missing link:\n%s", got)
	}
}

func TestWriteTableOmitsExternalColumnWhenUnused(t *testing.T) {
	var buf bytes.Buffer
	if err := Write(&buf, Table, sampleEntries()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(buf.String(), "EXTERNAL") {
		t.Fatalf("unexpected EXTERNAL column:\n%s", buf.String())
	}
}

func TestWriteJSONExternal(t *testing.T) {
	e := sampleEntries()[0]
	e.External = sampleExternal()
	var buf bytes.Buffer
	if err := Write(&buf, JSON, []Entry{e}); err != nil {
		t.Fatal(err)
	}
	var back []Entry
	if err := json.Unmarshal(buf.Bytes(), &back); err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, buf.Bytes())
	}
	if len(back) != 1 || back[0].External == nil {
		t.Fatalf("decoded = %#v", back)
	}
	if *back[0].External != *e.External {
		t.Fatalf("external = %+v, want %+v", back[0].External, e.External)
	}
}

func TestWriteOneTableExternal(t *testing.T) {
	e := sampleEntries()[1]
	e.External = sampleExternal()
	var buf bytes.Buffer
	if err := WriteOne(&buf, Table, e); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "External: gitea owner/repo#127") {
		t.Fatalf("WriteOne table =\n%s", buf.String())
	}
}

func TestLineAlias(t *testing.T) {
	e := sampleEntries()[0]
	e.Alias = "DTRM-F21"
	want := "[AWIT-TEST0001] open Implement OAuth2 token extraction | auth,p1 | Unblocks: 2 | Alias: DTRM-F21"
	if got := Line(e); got != want {
		t.Fatalf("Line() =\n%q\nwant\n%q", got, want)
	}
}

func TestWriteJSONAliasField(t *testing.T) {
	e := sampleEntries()[0]
	e.Alias = "DTRM-F21"
	var buf bytes.Buffer
	if err := Write(&buf, JSON, []Entry{e}); err != nil {
		t.Fatal(err)
	}
	var rows []map[string]any
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatal(err)
	}
	if rows[0]["alias"] != "DTRM-F21" {
		t.Fatalf("json = %s", buf.String())
	}
	var bare bytes.Buffer
	if err := Write(&bare, JSON, sampleEntries()); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(bare.String(), "alias") {
		t.Fatalf("alias must be omitted when empty:\n%s", bare.String())
	}
}
