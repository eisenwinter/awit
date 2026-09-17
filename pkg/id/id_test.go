package id

import (
	"errors"
	"os"
	"sort"
	"testing"
	"time"
)

func TestEncodeDecodeRoundTrip(t *testing.T) {
	cases := []struct {
		secs        uint32
		worker, rnd uint8
	}{
		{0, 0, 0},
		{22451400, 7, 0},
		{0x3FFFFFFF, 63, 15},
		{1, 1, 1},
		{100, 32, 8},
	}
	for _, tc := range cases {
		s, err := Encode(tc.secs, tc.worker, tc.rnd)
		if err != nil {
			t.Fatalf("Encode(%d,%d,%d): %v", tc.secs, tc.worker, tc.rnd, err)
		}
		if len(s) != Chars {
			t.Fatalf("Encode(%d,%d,%d) len = %d, want 8", tc.secs, tc.worker, tc.rnd, len(s))
		}
		secs, worker, rnd, err := Decode(s)
		if err != nil {
			t.Fatalf("Decode(%q): %v", s, err)
		}
		if secs != tc.secs || worker != tc.worker || rnd != tc.rnd {
			t.Fatalf("Decode(%q) = %d,%d,%d want %d,%d,%d", s, secs, worker, rnd, tc.secs, tc.worker, tc.rnd)
		}
	}
}

func TestEncodeWorkedExample(t *testing.T) {
	got, err := Encode(22451400, 7, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got != "0ND5683G" {
		t.Fatalf("Encode(22451400, 7, 0) = %q, want 0ND5683G", got)
	}
}

func TestEncodeRangeErrors(t *testing.T) {
	if _, err := Encode(1<<TimestampBits, 0, 0); err == nil {
		t.Fatal("Encode(1<<30, 0, 0) = nil, want error")
	}
	if _, err := Encode(0, 1<<WorkerBits, 0); err == nil {
		t.Fatal("Encode(0, 64, 0) = nil, want error")
	}
	if _, err := Encode(0, 0, 1<<RandomBits); err == nil {
		t.Fatal("Encode(0, 0, 16) = nil, want error")
	}
}

func TestDecodeRejectsAmbiguous(t *testing.T) {
	for _, s := range []string{"I", "L", "O", "U", "SHORT"} {
		if _, _, _, err := Decode(s); err == nil {
			t.Errorf("Decode(%q) = nil, want error", s)
		}
	}
	for _, s := range []string{"0ND5683I", "0ND5683L", "0ND5683O", "0ND5683U"} {
		if _, _, _, err := Decode(s); err == nil {
			t.Errorf("Decode(%q) = nil, want error (ambiguous letter in body)", s)
		}
	}
}

func TestDecodeAcceptsLowercase(t *testing.T) {
	secs, worker, rnd, err := Decode("0nd5683g")
	if err != nil {
		t.Fatal(err)
	}
	if secs != 22451400 || worker != 7 || rnd != 0 {
		t.Fatalf("Decode(lowercase) = %d,%d,%d want 22451400,7,0", secs, worker, rnd)
	}
}

func TestIDsSortByTime(t *testing.T) {
	ids := make([]string, 200)
	for i := 0; i < 200; i++ {
		s, err := Encode(uint32(i+1), 7, 0)
		if err != nil {
			t.Fatal(err)
		}
		ids[i] = s
	}
	if !sort.StringsAreSorted(ids) {
		t.Fatalf("IDs are not sorted by increasing seconds: first=%q last=%q", ids[0], ids[len(ids)-1])
	}
}

func TestSplitAndValid(t *testing.T) {
	id := Format("AWIT", "0ND5683G")
	if id != "AWIT-0ND5683G" {
		t.Fatalf("Format = %q, want AWIT-0ND5683G", id)
	}
	p, b, err := Split(id)
	if err != nil || p != "AWIT" || b != "0ND5683G" {
		t.Fatalf("Split(%q) = %q, %q, %v", id, p, b, err)
	}
	if !Valid("AWIT", id) {
		t.Fatal("Valid(AWIT, AWIT-0ND5683G) = false")
	}
	if Valid("AWIT", "OTHER-0ND5683G") {
		t.Fatal("Valid accepted the wrong prefix")
	}
	if Valid("AWIT", "AWIT-SHORT") {
		t.Fatal("Valid accepted a short body")
	}
	if Valid("AWIT", "AWIT-0ND5683I") {
		t.Fatal("Valid accepted I in the body")
	}
	if _, _, err := Split("NODASH"); err == nil {
		t.Fatal("Split(NODASH) = nil, want error")
	}
	if _, _, err := Split("AWIT-SHORT"); err == nil {
		t.Fatal("Split short body = nil, want error")
	}
}

func TestTime(t *testing.T) {
	got, err := Time("0ND5683G")
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 9, 17, 20, 30, 0, 0, time.UTC)
	if !got.Equal(want) {
		t.Fatalf("Time(0ND5683G) = %s, want %s", got.UTC().Format(time.RFC3339), want.Format(time.RFC3339))
	}
	if got.Location() != time.UTC {
		t.Fatalf("Time() location = %v, want UTC", got.Location())
	}
}

func TestWorkerForStable(t *testing.T) {
	a := WorkerFor("host", "tree", "main")
	b := WorkerFor("host", "tree", "main")
	if a != b {
		t.Fatalf("WorkerFor not stable: %d vs %d", a, b)
	}
	if a > 63 {
		t.Fatalf("WorkerFor = %d, want 0..63", a)
	}
	// FNV-1a 32 of host+"\x00"+tree+"\x00"+main, then % 64. Pinning this
	// value catches a missing NUL separator (plain concatenation hashes
	// to a different bucket).
	if a != 22 {
		t.Fatalf("WorkerFor(host, tree, main) = %d, want 22", a)
	}
}

func TestWorkerEnvOverride(t *testing.T) {
	host, err := os.Hostname()
	if err != nil {
		t.Fatal(err)
	}
	fallback := WorkerFor(host, "/wt", "main")

	t.Setenv("AWIT_WORKER", "9")
	if got := Worker("/wt", "main"); got != 9 {
		t.Fatalf("AWIT_WORKER=9: Worker() = %d, want 9", got)
	}

	t.Setenv("AWIT_WORKER", "99")
	if got := Worker("/wt", "main"); got != fallback {
		t.Fatalf("AWIT_WORKER=99: Worker() = %d, want fallback %d", got, fallback)
	}

	t.Setenv("AWIT_WORKER", "abc")
	if got := Worker("/wt", "main"); got != fallback {
		t.Fatalf("AWIT_WORKER=abc: Worker() = %d, want fallback %d", got, fallback)
	}
}

func TestMintRetriesOnCollision(t *testing.T) {
	n := 0
	exists := func(string) bool {
		n++
		return n <= 3
	}
	now := time.Date(2026, 9, 17, 20, 30, 0, 123, time.UTC)
	id, err := Mint("AWIT", now, 7, exists)
	if err != nil {
		t.Fatalf("Mint() error = %v", err)
	}
	if n != 4 {
		t.Fatalf("exists called %d times, want 4 (true, true, true, false)", n)
	}
	if !Valid("AWIT", id) {
		t.Fatalf("Mint() = %q, not a valid AWIT id", id)
	}
}

func TestMintExhausted(t *testing.T) {
	n := 0
	exists := func(string) bool {
		n++
		return true
	}
	_, err := Mint("AWIT", time.Date(2026, 9, 17, 20, 30, 0, 0, time.UTC), 1, exists)
	if !errors.Is(err, ErrExhausted) {
		t.Fatalf("Mint() error = %v, want ErrExhausted", err)
	}
	if n != 16 {
		t.Fatalf("exists called %d times, want 16", n)
	}
}
