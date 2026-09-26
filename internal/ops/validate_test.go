package ops

import (
	"testing"
	"unicode"
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

func TestSentenceCountTerminatorsNeedBoundary(t *testing.T) {
	if unicode.IsSpace(' ') != true {
		t.Fatal("sanity")
	}
	if sentenceCount("Hello.World") != 1 {
		t.Fatalf("sentenceCount(Hello.World) = %d, want 1 (dot not at a boundary)", sentenceCount("Hello.World"))
	}
}
