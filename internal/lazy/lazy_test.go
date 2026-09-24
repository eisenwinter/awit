package lazy

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

var update = flag.Bool("update", false, "update golden files")

func TestMain(m *testing.M) {
	os.Setenv("NO_COLOR", "1")
	os.Exit(m.Run())
}

// fakeOps is a TTY-free Ops over in-memory items.
type fakeOps struct {
	items   []*item.Item
	archive []*item.Item
	loadErr error
	calls   []string
}

func (f *fakeOps) Load() (*graph.Graph, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	return graph.Build(f.items, nil), nil
}

func (f *fakeOps) LoadArchive() ([]*item.Item, error) { return f.archive, nil }

func (f *fakeOps) Line(n *graph.Node) string {
	return "[" + n.Item.ID + "] " + string(n.Item.Status) + " " + n.Item.Title
}

func (f *fakeOps) ArchiveLine(it *item.Item) string {
	return "[" + it.ID + "] archived " + it.Title
}

func (f *fakeOps) Detail(g *graph.Graph, id string) string {
	n := g.Nodes[id]
	if n == nil {
		return "unknown item " + id
	}
	return "== " + n.Item.ID + " ==\n" + n.Item.Title + "\nstatus: " + string(n.Item.Status) + "\n"
}

func (f *fakeOps) ArchiveDetail(id string) (string, error) {
	for _, it := range f.archive {
		if it.ID == id {
			return "== " + it.ID + " (archive) ==\n" + it.Title + "\n", nil
		}
	}
	return "", os.ErrNotExist
}

func (f *fakeOps) Close(id, reason string) error { f.calls = append(f.calls, "close "+id); return nil }
func (f *fakeOps) Block(id, reason string) error { f.calls = append(f.calls, "block "+id); return nil }
func (f *fakeOps) Unblock(id string) error       { f.calls = append(f.calls, "unblock "+id); return nil }
func (f *fakeOps) Comment(id, text string) error {
	f.calls = append(f.calls, "comment "+id)
	return nil
}
func (f *fakeOps) Claim(id string) error          { f.calls = append(f.calls, "claim "+id); return nil }
func (f *fakeOps) Release(id string) error        { f.calls = append(f.calls, "release "+id); return nil }
func (f *fakeOps) Validate(g *graph.Graph) string { return "ok: no faults" }
func (f *fakeOps) ExternalCheck(ctx context.Context, id string) string {
	return "MATCH " + id
}

func mustItem(id, title string, deps, labels []string) *item.Item {
	return item.New(id, title, "brief for "+title, deps, labels)
}

// newFixture builds a fake over ready, blocked-on-dep, held, in-progress,
// closed, and quarantined-by-dangling-dep items, plus two archive items.
func newFixture() *fakeOps {
	ready := mustItem("AWIT-00000001", "first", nil, []string{"auth"})
	blocked := mustItem("AWIT-00000002", "second", []string{"AWIT-00000001"}, nil)
	held := mustItem("AWIT-00000003", "third", nil, nil)
	if err := held.SetBlockedReason("waiting on keys"); err != nil {
		panic(err)
	}
	prog := mustItem("AWIT-00000004", "fourth", nil, nil)
	prog.SetStatus(item.StatusInProgress)
	closed := mustItem("AWIT-00000005", "fifth", nil, nil)
	closed.SetStatus(item.StatusClosed)
	dangling := mustItem("AWIT-00000006", "sixth", []string{"AWIT-9ZZZZZZZ"}, nil)
	arch1 := mustItem("AWIT-000000A1", "archived one", nil, nil)
	arch1.SetStatus(item.StatusClosed)
	arch2 := mustItem("AWIT-000000A2", "archived two", nil, nil)
	arch2.SetStatus(item.StatusClosed)
	return &fakeOps{
		items:   []*item.Item{ready, blocked, held, prog, closed, dangling},
		archive: []*item.Item{arch1, arch2},
	}
}

func newModel() Model {
	m := New(newFixture(), context.Background(), Options{})
	um, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	return um.(Model)
}

func keyMsg(s string) tea.Msg {
	switch s {
	case "enter":
		return tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		return tea.KeyMsg{Type: tea.KeyEsc}
	case "tab":
		return tea.KeyMsg{Type: tea.KeyTab}
	case "up":
		return tea.KeyMsg{Type: tea.KeyUp}
	case "down":
		return tea.KeyMsg{Type: tea.KeyDown}
	case "left":
		return tea.KeyMsg{Type: tea.KeyLeft}
	case "right":
		return tea.KeyMsg{Type: tea.KeyRight}
	case " ", "space":
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")}
	case "ctrl+c":
		return tea.KeyMsg{Type: tea.KeyCtrlC}
	case "pgup":
		return tea.KeyMsg{Type: tea.KeyPgUp}
	case "pgdown":
		return tea.KeyMsg{Type: tea.KeyPgDown}
	default:
		return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
	}
}

func press(m Model, keys ...string) Model {
	for _, k := range keys {
		um, _ := m.Update(keyMsg(k))
		m = um.(Model)
	}
	return m
}

func golden(t *testing.T, name, got string) {
	t.Helper()
	if strings.ContainsRune(got, '\x1b') {
		t.Fatalf("%s: output contains escape byte", name)
	}
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll("testdata", 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%s: reading golden: %v (run with -update)", name, err)
	}
	if got != string(want) {
		t.Fatalf("%s: mismatch:\n--- got ---\n%s\n--- want ---\n%s", name, got, want)
	}
}
