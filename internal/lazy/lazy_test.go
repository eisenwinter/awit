package lazy

import (
	"context"
	"errors"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/graph"
	"github.com/eisenwinter/awit/pkg/item"
)

var update = flag.Bool("update", false, "update golden files")

func TestMain(m *testing.M) {
	os.Setenv("NO_COLOR", "1")
	os.Exit(m.Run())
}

// errTest is the canned Ops failure; setting f.fail makes every mutator
// return it so refusal paths can be tested.
var errTest = errors.New("boom")

// fakeOps is a TTY-free Ops over in-memory items.
type fakeOps struct {
	items    map[string]*item.Item
	archive  []*item.Item
	loadErr  error
	fail     error
	calls    []string
	external string
	cfg      config.Config
	cfgErr   error
	saved    []config.Config
}

func (f *fakeOps) Load() (*graph.Graph, error) {
	if f.loadErr != nil {
		return nil, f.loadErr
	}
	items := make([]*item.Item, 0, len(f.items))
	for _, it := range f.items {
		items = append(items, it)
	}
	return graph.Build(items, nil), nil
}

func (f *fakeOps) LoadArchive() ([]*item.Item, error) {
	f.calls = append(f.calls, "LoadArchive")
	return f.archive, nil
}

func (f *fakeOps) Line(n *graph.Node) string {
	return "[" + n.Item.ID + "] " + string(n.Item.Status) + " " + n.Item.Title
}

func (f *fakeOps) ArchiveLine(it *item.Item) string {
	return "[" + it.ID + "] archived " + it.Title
}

func (f *fakeOps) Detail(g *graph.Graph, id string) string {
	return "detail of " + id + "\n"
}

func (f *fakeOps) ArchiveDetail(id string) (string, error) {
	return "archive of " + id + "\n", nil
}

func (f *fakeOps) find(id string) *item.Item {
	return f.items[id]
}

func (f *fakeOps) Close(id, reason string) error {
	f.calls = append(f.calls, "Close "+id+" "+reason)
	if f.fail != nil {
		return f.fail
	}
	if it := f.find(id); it != nil {
		it.SetStatus(item.StatusClosed)
	}
	return nil
}
func (f *fakeOps) Block(id, reason string) error {
	f.calls = append(f.calls, "Block "+id+" "+reason)
	return f.fail
}
func (f *fakeOps) Unblock(id string) error {
	f.calls = append(f.calls, "Unblock "+id)
	return f.fail
}
func (f *fakeOps) Comment(id, text string) error {
	f.calls = append(f.calls, "Comment "+id+" "+text)
	return f.fail
}
func (f *fakeOps) Claim(id string) error {
	f.calls = append(f.calls, "Claim "+id)
	if f.fail != nil {
		return f.fail
	}
	if it := f.find(id); it != nil {
		it.SetStatus(item.StatusInProgress)
	}
	return nil
}
func (f *fakeOps) Release(id string) error {
	f.calls = append(f.calls, "Release "+id)
	if f.fail != nil {
		return f.fail
	}
	if it := f.find(id); it != nil {
		it.SetStatus(item.StatusOpen)
	}
	return nil
}
func (f *fakeOps) Validate(g *graph.Graph) string { return "ok: no faults" }
func (f *fakeOps) ExternalCheck(ctx context.Context, id string) string {
	f.calls = append(f.calls, "ExternalCheck "+id)
	if f.external != "" {
		return f.external
	}
	return "MATCH " + id
}
func (f *fakeOps) Config() (config.Config, error) {
	if f.cfgErr != nil {
		return config.Config{}, f.cfgErr
	}
	return f.cfg, nil
}
func (f *fakeOps) SaveConfig(c config.Config) error {
	f.calls = append(f.calls, "SaveConfig")
	if f.fail != nil {
		return f.fail
	}
	f.cfg = c
	f.saved = append(f.saved, c)
	return nil
}

func mk(id, title string, st item.Status, deps, labels []string) *item.Item {
	it := item.New(id, title, "Brief for "+title+".", deps, labels)
	it.SetStatus(st)
	return it
}

// newFixture: L1 ready (unblocks L3,L4), L2 ready, L3 blocked by L1,
// L4 blocked by L3, L5 closed, L6 in_progress claimed, L7 held,
// L8 quarantined (dangling dep). Archive: A1, A2.
func newFixture() *fakeOps {
	now := time.Date(2026, 9, 24, 9, 0, 0, 0, time.UTC)
	l6 := mk("AWIT-LAZY0006", "Rotate keys", item.StatusInProgress, nil, nil)
	l6.SetAssignee("agent/x")
	l6.SetClaimedAt(&now)
	l7 := mk("AWIT-LAZY0007", "Vendor contract", item.StatusOpen, nil, []string{"p0"})
	_ = l7.SetBlockedReason("waiting on vendor")
	items := []*item.Item{
		mk("AWIT-LAZY0001", "Token extraction", item.StatusOpen, nil, []string{"auth", "p1"}),
		mk("AWIT-LAZY0002", "Migration scripts", item.StatusOpen, nil, []string{"db"}),
		mk("AWIT-LAZY0003", "E2E auth tests", item.StatusOpen, []string{"AWIT-LAZY0001"}, nil),
		mk("AWIT-LAZY0004", "Rotate API tokens", item.StatusOpen, []string{"AWIT-LAZY0003"}, []string{"p0"}),
		mk("AWIT-LAZY0005", "Middleware spec", item.StatusClosed, nil, nil),
		l6, l7,
		mk("AWIT-LAZY0008", "Dangling", item.StatusOpen, []string{"AWIT-LAZY0999"}, nil),
	}
	byID := make(map[string]*item.Item, len(items))
	for _, it := range items {
		byID[it.ID] = it
	}
	return &fakeOps{
		items: byID,
		archive: []*item.Item{
			mk("AWIT-LAZY0101", "Old thing one", item.StatusClosed, nil, []string{"auth"}),
			mk("AWIT-LAZY0102", "Old thing two", item.StatusClosed, nil, nil),
		},
	}
}

func newModel(t *testing.T, f *fakeOps) Model {
	t.Helper()
	m := New(f, context.Background(), Options{})
	if m.fatal != "" {
		t.Fatalf("fatal: %s", m.fatal)
	}
	return resize(m, 100, 30)
}

func resize(m Model, w, h int) Model {
	next, _ := m.Update(tea.WindowSizeMsg{Width: w, Height: h})
	return next.(Model)
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

func press(m Model, keys ...string) (Model, tea.Cmd) {
	var cmd tea.Cmd
	for _, k := range keys {
		var next tea.Model
		next, cmd = m.Update(keyMsg(k))
		m = next.(Model)
	}
	return m, cmd
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }

func countCalls(f *fakeOps, prefix string) int {
	n := 0
	for _, c := range f.calls {
		if strings.HasPrefix(c, prefix) {
			n++
		}
	}
	return n
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
