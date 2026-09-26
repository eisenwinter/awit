package lazy

import (
	"strings"
	"testing"
)

func TestCloseFlowResortsQueue(t *testing.T) {
	f := newFixture()
	m := newModel(t, f)
	m, _ = press(m, "3", "c")
	if m.mode != modeInput || m.inputTarget != "AWIT-LAZY0001" {
		t.Fatalf("c: mode=%d target=%s", m.mode, m.inputTarget)
	}
	golden(t, "prompt_close", m.View())
	m, cmd := press(m, "d", "o", "n", "e", "enter")
	if countCalls(f, "Close AWIT-LAZY0001 done") != 1 || m.toast != "closed AWIT-LAZY0001" || m.mode != modeNormal {
		t.Fatalf("submit: calls=%v toast=%q mode=%d", f.calls, m.toast, m.mode)
	}
	if cmd == nil {
		t.Fatal("close submit must dispatch an async reload")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	var ids []string
	for _, r := range m.queue.list.rows {
		ids = append(ids, r.id)
	}
	// L1 closed → L3 becomes ready (U1) and outranks L2/L6 (U0).
	if got := strings.Join(ids, ","); got != "AWIT-LAZY0003,AWIT-LAZY0002,AWIT-LAZY0006" {
		t.Fatalf("queue after close = %s", got)
	}
	if m.selectedID() != "AWIT-LAZY0003" {
		t.Fatalf("cursor after close = %s, want first row", m.selectedID())
	}
}

func TestBlockRequiresReasonAndCommentRequiresText(t *testing.T) {
	f := newFixture()
	m := newModel(t, f)
	m, _ = press(m, "b")
	golden(t, "prompt_block", m.View())
	m, _ = press(m, "enter")
	if m.toast != "error: block requires a non-empty reason" || countCalls(f, "Block") != 0 {
		t.Fatalf("empty block: toast=%q calls=%v", m.toast, f.calls)
	}
	m, cmd := press(m, "b", "v", "e", "n", "d", "o", "r", "enter")
	if countCalls(f, "Block AWIT-LAZY0001 vendor") != 1 || m.toast != "blocked AWIT-LAZY0001: vendor" {
		t.Fatalf("block: calls=%v toast=%q", f.calls, m.toast)
	}
	if cmd == nil {
		t.Fatal("block submit must dispatch an async reload")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	m, _ = press(m, "m")
	golden(t, "prompt_comment", m.View())
	m, _ = press(m, "esc")
	if m.mode != modeNormal || countCalls(f, "Comment") != 0 {
		t.Fatal("esc must cancel the comment prompt")
	}
	m, _ = press(m, "m", "enter")
	if m.toast != "error: empty comment" {
		t.Fatalf("empty comment toast = %q", m.toast)
	}
	m, _ = press(m, "m", "h", "i", "enter")
	if countCalls(f, "Comment AWIT-LAZY0001 hi") != 1 || m.toast != "commented AWIT-LAZY0001" {
		t.Fatalf("comment: calls=%v toast=%q", f.calls, m.toast)
	}
}

func TestUnblockAndErrorToastKeepsSelection(t *testing.T) {
	f := newFixture()
	m := newModel(t, f)
	for m.selectedID() != "AWIT-LAZY0007" {
		m, _ = press(m, "j")
	}
	m, _ = press(m, "u")
	if countCalls(f, "Unblock AWIT-LAZY0007") != 1 || m.toast != "unblocked AWIT-LAZY0007" || m.selectedID() != "AWIT-LAZY0007" {
		t.Fatalf("unblock: calls=%v toast=%q sel=%s", f.calls, m.toast, m.selectedID())
	}
	f.fail = errTest
	m, _ = press(m, "u")
	if m.toast != "error: boom" || m.selectedID() != "AWIT-LAZY0007" {
		t.Fatalf("error: toast=%q sel=%s", m.toast, m.selectedID())
	}
}

func TestArchiveAndGraphHeaderRefusals(t *testing.T) {
	f := newFixture()
	m := newModel(t, f)
	m, _ = press(m, "o", "c")
	if m.mode != modeNormal || m.toast != "error: archived items are read-only" {
		t.Fatalf("archive c: mode=%d toast=%q", m.mode, m.toast)
	}
	m, _ = press(m, "o", "2", "tab", "c") // focused tree: cursor lands on the root node row → prompt opens
	if m.mode != modeInput {
		t.Fatalf("graph c on node row: mode=%d toast=%q", m.mode, m.toast)
	}
}

func TestExternalCheckAsyncToast(t *testing.T) {
	f := newFixture()
	f.external = "MATCH AWIT-LAZY0001 https://example/1"
	m := newModel(t, f)
	m, cmd := press(m, "P")
	if cmd == nil || countCalls(f, "ExternalCheck") != 0 {
		t.Fatal("P must return a cmd and not call Ops synchronously")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if countCalls(f, "ExternalCheck AWIT-LAZY0001") != 1 || m.toast != f.external {
		t.Fatalf("external: calls=%v toast=%q", f.calls, m.toast)
	}
}

func TestQuarantineFooterTracksSnapshot(t *testing.T) {
	f := newFixture()
	m := newModel(t, f)
	if !contains(m.View(), "warning: 1 item(s) quarantined, run awit validate") {
		t.Fatalf("footer missing:\n%s", m.View())
	}
	delete(f.items, "AWIT-LAZY0008")
	m, cmd := press(m, "R")
	if cmd == nil {
		t.Fatal("R must dispatch an async reload")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if contains(m.View(), "quarantined, run awit validate") {
		t.Fatal("footer must disappear after reload")
	}
}
