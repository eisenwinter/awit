package item

import (
	"bytes"
	"errors"
	"strings"
	"testing"
)

func TestSplit(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		in      []byte
		front   []byte
		body    []byte
		wantErr error
	}{
		{
			name:  "LF",
			in:    []byte("---\nid: x\n---\nBODY"),
			front: []byte("id: x\n"),
			body:  []byte("BODY"),
		},
		{
			name:  "CRLF",
			in:    []byte("---\r\nid: x\r\n---\r\nBODY"),
			front: []byte("id: x\r\n"),
			body:  []byte("BODY"),
		},
		{
			name:    "no fence",
			in:      []byte("id: x\n---\nBODY"),
			wantErr: ErrNoFrontmatter,
		},
		{
			name:    "unterminated",
			in:      []byte("---\nid: x\n"),
			wantErr: ErrUnterminatedFrontmatter,
		},
		{
			name:    "opening fence without newline",
			in:      []byte("---"),
			wantErr: ErrNoFrontmatter,
		},
		{
			name:  "closing fence trimmed",
			in:    []byte("---\nid: x\n---  \nBODY"),
			front: []byte("id: x\n"),
			body:  []byte("BODY"),
		},
		{
			name:  "empty body",
			in:    []byte("---\nid: x\n---\n"),
			front: []byte("id: x\n"),
			body:  []byte(""),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			front, body, err := Split(tt.in)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("Split() err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Split() err = %v", err)
			}
			if !bytes.Equal(front, tt.front) {
				t.Fatalf("front = %q, want %q", front, tt.front)
			}
			if !bytes.Equal(body, tt.body) {
				t.Fatalf("body = %q, want %q", body, tt.body)
			}
		})
	}
}

func TestHasConflictMarkers(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		in   string
		want bool
	}{
		{"head", "<<<<<<< HEAD\n", true},
		{"equals7", "=======\n", true},
		{"equals8", "========\n", true},
		{"equals6", "======\n", false},
		{"tail", ">>>>>>> branch\n", true},
		{"show delimiter", "===== REF 1/1 =====\n", false},
		{"trimmed head", "  <<<<<<< HEAD  \n", true},
		{"no space after chevrons", "<<<<<<<HEAD\n", false},
		{"clean", "---\nid: x\n---\nbody\n", false},
		{"empty", "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := HasConflictMarkers([]byte(tt.in)); got != tt.want {
				t.Fatalf("HasConflictMarkers(%q) = %v, want %v", tt.in, got, tt.want)
			}
		})
	}
}

func FuzzSplit(f *testing.F) {
	f.Add([]byte("---\nid: x\n---\nbody"))
	f.Add([]byte("---\r\nid: x\r\n---\r\nbody"))
	f.Add([]byte("nope"))
	f.Add([]byte("---\n"))
	f.Add([]byte(""))
	f.Add([]byte("---\n---\n"))
	f.Fuzz(func(t *testing.T, data []byte) {
		front, body, err := Split(data)
		if err != nil {
			return
		}
		if len(front)+len(body) > len(data) {
			t.Fatalf("len(front)+len(body)=%d > len(data)=%d", len(front)+len(body), len(data))
		}
	})
}

func TestSplitBodyMayContainFence(t *testing.T) {
	front, body, err := Split([]byte("---\nid: x\n---\n---\n"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(front, []byte("id: x\n")) {
		t.Fatalf("front = %q", front)
	}
	if !bytes.Equal(body, []byte("---\n")) {
		t.Fatalf("body = %q", body)
	}
	_ = strings.Count
}
