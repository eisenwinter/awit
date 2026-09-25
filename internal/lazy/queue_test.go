package lazy

import (
	"strings"
	"testing"
)

func TestQueueRowsAreReadyOrder(t *testing.T) {
	f := newFixture()
	m := newModel(t, f)
	m, _ = press(m, "3")
	var ids []string
	for _, r := range m.queue.list.rows {
		ids = append(ids, r.id)
		if !r.selectable {
			t.Fatalf("queue row unselectable: %+v", r)
		}
	}
	if got := strings.Join(ids, ","); got != "AWIT-LAZY0001,AWIT-LAZY0002,AWIT-LAZY0006" {
		t.Fatalf("queue = %s", got)
	}
	if !contains(m.View(), "why: AWIT-LAZY0001; unblocks=2; critical-path=yes; selection=max-unblocks; tie-break=none") {
		t.Fatalf("why line missing:\n%s", m.View())
	}
	golden(t, "queue", m.View())
	m, _ = press(m, "j")
	if !contains(m.View(), "why: AWIT-LAZY0002; unblocks=0; critical-path=no; selection=ranked; tie-break=none") {
		t.Fatalf("why line for second row:\n%s", m.View())
	}
	if !contains(m.View(), "detail of AWIT-LAZY0002") {
		t.Fatal("detail not pinned to queue selection")
	}
}

func TestQueueClaimAndRelease(t *testing.T) {
	f := newFixture()
	m := newModel(t, f)
	m, cmd := press(m, "3", "space")
	if countCalls(f, "Claim AWIT-LAZY0001") != 1 || m.toast != "claimed AWIT-LAZY0001" {
		t.Fatalf("claim: calls=%v toast=%q", f.calls, m.toast)
	}
	if cmd == nil {
		t.Fatal("space must dispatch an async reload")
	}
	next, _ := m.Update(cmd())
	m = next.(Model)
	if m.selectedID() != "AWIT-LAZY0001" || !contains(m.queue.list.rows[0].text, "in_progress") {
		t.Fatalf("after claim: sel=%s rows=%+v", m.selectedID(), m.queue.list.rows)
	}
	golden(t, "queue_after_claim", m.View())
	m, cmd = press(m, "r")
	if countCalls(f, "Release AWIT-LAZY0001") != 1 || m.toast != "reopened AWIT-LAZY0001" {
		t.Fatalf("release: calls=%v toast=%q", f.calls, m.toast)
	}
	if cmd == nil {
		t.Fatal("r must dispatch an async reload")
	}
	next, _ = m.Update(cmd())
	m = next.(Model)
	if !contains(m.queue.list.rows[0].text, "open") {
		t.Fatalf("release rows: %+v", m.queue.list.rows)
	}
}

func TestQueueRefusalKeepsSelection(t *testing.T) {
	f := newFixture()
	m := newModel(t, f)
	f.fail = errTest
	m, _ = press(m, "3", "j", "space")
	if m.toast != "error: boom" || m.selectedID() != "AWIT-LAZY0002" {
		t.Fatalf("toast=%q sel=%s", m.toast, m.selectedID())
	}
}

func TestQueueEmpty(t *testing.T) {
	f := newFixture()
	for _, id := range []string{"AWIT-LAZY0001", "AWIT-LAZY0002", "AWIT-LAZY0003", "AWIT-LAZY0004", "AWIT-LAZY0006"} {
		_ = f.Close(id, "")
	}
	m := newModel(t, f)
	m, _ = press(m, "3", "space")
	if m.toast != "error: no item selected" || !contains(m.View(), "No ready items") {
		t.Fatalf("empty queue: toast=%q\n%s", m.toast, m.View())
	}
}
