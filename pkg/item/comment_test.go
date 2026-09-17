package item

import (
	"testing"
	"time"
)

func TestSanitizeAuthor(t *testing.T) {
	t.Parallel()
	tests := []struct {
		in, want string
	}{
		{"agent/Claude Opus", "claude-opus"},
		{"agent/", "anon"},
		{"Jan", "jan"},
		{"Foo_Bar.baz", "foo_bar.baz"},
		{"@@@", "anon"},
		{"", "anon"},
		{"agent/agent/x", "agent-x"},
		{"--ok--", "ok"},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			if got := SanitizeAuthor(tt.in); got != tt.want {
				t.Fatalf("SanitizeAuthor(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

func TestCommentFileName(t *testing.T) {
	now := time.Date(2026, 9, 17, 14, 32, 5, 0, time.UTC)
	got := CommentFileName(now, "agent/Claude Opus", ".md")
	want := "20260917T143205Z-claude-opus.md"
	if got != want {
		t.Fatalf("CommentFileName = %q, want %q", got, want)
	}
}
