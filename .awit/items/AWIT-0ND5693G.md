---
id: AWIT-0ND5693G
title: 'pkg/id: Crockford snowflake IDs'
brief: >-
  Implement pkg/id: 40-bit Crockford snowflake encode/decode, Format/Split/Valid/Time,
  FNV-1a worker hashing with AWIT_WORKER override, and Mint with a 4-bit crypto/rand
  nibble and 16 collision retries.
status: open
deps: []
labels: [phase0, p0]
refs:
  - ../../plan/implementation-guide.md
  - ../../plan/awit-implementation-plan.md
---

## Summary
After this ticket `pkg/id/id.go` exists with every signature in guide §4.1 copied verbatim. IDs are 8 uppercase Crockford chars packing 30-bit seconds-since-epoch, 6-bit worker and 4-bit random, MSB first. `Encode(22451400, 7, 0)` is `0ND5683G` and `Time` of that body is `2026-09-17T20:30:00Z`. `Mint` retries 16 times then returns `ErrExhausted`. No CLI, no store, no YAML.

## Context (read first)
- Guide §4.1 `pkg/id` — copy the signatures; do not rename. `Alphabet`, `Epoch`, bit-width constants, `ErrExhausted` and every func listed there are the whole public surface.
- Guide §1: stdlib only (`errors`, `fmt`, `os`, `strconv`, `strings`, `time`, `hash/fnv`, `crypto/rand`). Module `github.com/eisenwinter/awit`. Tabs, `gofmt`. Tests: `testing` only, table-driven, no golden files.
- Guide §2 decision 2: worker hash input is **hostname + worktree absolute path + branch name**, FNV-1a 32-bit, `% 64`. `AWIT_WORKER` (0–63) overrides. Branch missing → empty string, still hashed. Join the three strings with a NUL byte between them: `hostname+"\x00"+worktree+"\x00"+branch`. Do **not** concatenate without NULs.
- Guide §2 decision 6: epoch `2026-01-01T00:00:00Z`, 30-bit seconds, 6-bit worker, 4-bit random, 8 Crockford chars. `Encode` errors if any field exceeds its width (timestamp included).
- Spec `plan/awit-implementation-plan.md` §Decisions → ID scheme and §ID layout: snowflake-like, `PREFIX-` + 8 Crockford chars, time-sortable; random nibble re-rolls on local collision; `crypto/rand`; a per-process sequence counter is meaningless for a one-shot CLI.
- Spec Phase 0: `pkg/id`: base32 encode/decode, worker hash, minting; property test that IDs sort by creation time — that is `TestIDsSortByTime`.
- Crockford alphabet is **exactly** `0123456789ABCDEFGHJKMNPQRSTVWXYZ` (no I, L, O, U). Decode must **reject** those letters, not map them. Accept lowercase via `strings.ToUpper` before lookup.
- Packing: `v := uint64(secs)<<10 | uint64(worker)<<4 | uint64(rnd)`. Emit `Alphabet[(v>>(5*i))&31]` for `i := 7; i >= 0; i--` (MSB first, 8 chars).
- `Time(body)` is `Epoch.Add(time.Duration(secs)*time.Second)` after `Decode`. UTC.
- `Mint`: `now` truncated to seconds then converted to UTC; 16 attempts; each attempt a fresh 4-bit nibble from `crypto/rand` (`buf[0]&0x0F`); `exists` is called with the **full** formatted id (`Format(prefix, body)`); after 16 hits return `ErrExhausted`.
- `Worker`: if `AWIT_WORKER` is set and `strconv.Atoi` yields 0..63 inclusive, use that; otherwise `WorkerFor(os.Hostname(), worktree, branch)`. Hostname error → empty hostname, still hashed. Values 99 and `abc` are **not** overrides.

## Files
- Create: `pkg/id/id.go`
- Create: `pkg/id/id_test.go`
- Modify: none. Do not touch `go.mod`. This package is stdlib-only.
- Fixtures/golden: none.

## Interfaces
- Consumes: nothing. No other awit package.
- Produces (verbatim from guide §4.1):

```go
package id

const Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ" // Crockford, uppercase
var Epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
const (
    TimestampBits = 30
    WorkerBits    = 6
    RandomBits    = 4
    Chars         = 8 // 40 bits / 5
)

func Encode(secs uint32, worker uint8, rnd uint8) (string, error)
func Decode(s string) (secs uint32, worker uint8, rnd uint8, err error)
func Format(prefix, body string) string
func Split(s string) (prefix, body string, err error)
func Valid(prefix, s string) bool
func Time(body string) (time.Time, error)
func WorkerFor(hostname, worktree, branch string) uint8
func Worker(worktree, branch string) uint8
func Mint(prefix string, now time.Time, worker uint8, exists func(string) bool) (string, error)

var ErrExhausted = errors.New("id: could not mint unique id after 16 attempts")
```

- Future consumers (**not** implemented here): `pkg/item.Store.Mint` (`Worker`, `Mint`, `Exists`), `pkg/item.Store.LoadAll` (`Valid` / `Split`), `awit create --id`.

## Steps

- [ ] **Step 1: Write the failing tests for Encode, Decode, Format, Split, Valid, Time, sort, Worker and Mint.**
  Create `pkg/id/id_test.go` with every test named in Acceptance. Do not create `id.go` yet.

```go
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
		secs          uint32
		worker, rnd   uint8
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
```

- [ ] **Step 2: Run it, see it fail to compile.**
  ```bash
  go test ./pkg/id -run TestEncode -v
  ```
  Expected failure (`id.go` does not exist, so `Encode`/`Decode`/`Chars` are undefined):
  ```text
  # github.com/eisenwinter/awit/pkg/id [github.com/eisenwinter/awit/pkg/id.test]
  pkg/id/id_test.go: undefined: Encode
  FAIL	github.com/eisenwinter/awit/pkg/id [build failed]
  ```
  The compiler will list several undefined names (`Encode`, `Decode`, `Chars`, `TimestampBits`, …). That is the red step. Do not skip it.

- [ ] **Step 3: Implement `pkg/id/id.go`.**
  Create `pkg/id/id.go` with the complete package. Keep the packing formula, the `i := 7; i >= 0; i--` emit loop, NUL-separated FNV input, 16-attempt Mint, and the exact `ErrExhausted` string.

```go
package id

import (
	"crypto/rand"
	"errors"
	"fmt"
	"hash/fnv"
	"os"
	"strconv"
	"strings"
	"time"
)

const Alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ" // Crockford, uppercase

var Epoch = time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)

const (
	TimestampBits = 30
	WorkerBits    = 6
	RandomBits    = 4
	Chars         = 8 // 40 bits / 5
)

var ErrExhausted = errors.New("id: could not mint unique id after 16 attempts")

func Encode(secs uint32, worker uint8, rnd uint8) (string, error) {
	if secs >= 1<<TimestampBits {
		return "", fmt.Errorf("id: timestamp %d exceeds %d bits", secs, TimestampBits)
	}
	if worker >= 1<<WorkerBits {
		return "", fmt.Errorf("id: worker %d exceeds %d bits", worker, WorkerBits)
	}
	if rnd >= 1<<RandomBits {
		return "", fmt.Errorf("id: random %d exceeds %d bits", rnd, RandomBits)
	}
	v := uint64(secs)<<10 | uint64(worker)<<4 | uint64(rnd)
	var b [Chars]byte
	for i := 7; i >= 0; i-- {
		b[7-i] = Alphabet[(v>>(5*i))&31]
	}
	return string(b[:]), nil
}

func Decode(s string) (secs uint32, worker uint8, rnd uint8, err error) {
	if len(s) != Chars {
		return 0, 0, 0, fmt.Errorf("id: length %d, want %d", len(s), Chars)
	}
	s = strings.ToUpper(s)
	var v uint64
	for i := 0; i < Chars; i++ {
		idx := strings.IndexByte(Alphabet, s[i])
		if idx < 0 {
			return 0, 0, 0, fmt.Errorf("id: invalid character %q", s[i])
		}
		v = v<<5 | uint64(idx)
	}
	secs = uint32(v >> 10)
	worker = uint8((v >> 4) & 63)
	rnd = uint8(v & 15)
	return secs, worker, rnd, nil
}

func Format(prefix, body string) string {
	return prefix + "-" + body
}

func Split(s string) (prefix, body string, err error) {
	prefix, body, ok := strings.Cut(s, "-")
	if !ok || len(body) != Chars {
		return "", "", fmt.Errorf("id: invalid id %q", s)
	}
	return prefix, body, nil
}

func Valid(prefix, s string) bool {
	p, body, err := Split(s)
	if err != nil || p != prefix {
		return false
	}
	_, _, _, err = Decode(body)
	return err == nil
}

func Time(body string) (time.Time, error) {
	secs, _, _, err := Decode(body)
	if err != nil {
		return time.Time{}, err
	}
	return Epoch.Add(time.Duration(secs) * time.Second), nil
}

func WorkerFor(hostname, worktree, branch string) uint8 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(hostname + "\x00" + worktree + "\x00" + branch))
	return uint8(h.Sum32() % 64)
}

func Worker(worktree, branch string) uint8 {
	if s := os.Getenv("AWIT_WORKER"); s != "" {
		n, err := strconv.Atoi(s)
		if err == nil && n >= 0 && n <= 63 {
			return uint8(n)
		}
	}
	host, _ := os.Hostname()
	return WorkerFor(host, worktree, branch)
}

func Mint(prefix string, now time.Time, worker uint8, exists func(string) bool) (string, error) {
	now = now.UTC().Truncate(time.Second)
	secs := uint32(now.Sub(Epoch) / time.Second)
	for range 16 {
		var buf [1]byte
		if _, err := rand.Read(buf[:]); err != nil {
			return "", err
		}
		body, err := Encode(secs, worker, buf[0]&0x0F)
		if err != nil {
			return "", err
		}
		id := Format(prefix, body)
		if !exists(id) {
			return id, nil
		}
	}
	return "", ErrExhausted
}
```

  Notes that must survive `gofmt`:
  - `v := uint64(secs)<<10 | uint64(worker)<<4 | uint64(rnd)` is the only packing. Do not shift by `TimestampBits`/`WorkerBits` names in a different order.
  - Emit loop is `for i := 7; i >= 0; i--` writing `b[7-i]`. Reversing the index produces a different string and fails the worked example.
  - `Decode` uppercases **before** the alphabet lookup so `0nd5683g` works and `i`/`l`/`o`/`u` become `I`/`L`/`O`/`U` which are **not** in `Alphabet`.
  - Do not implement Crockford's traditional I/L→1, O→0 folding. Reject.
  - `Split` uses the **first** dash (`strings.Cut`). Body length must be exactly 8; Crockford validity is `Valid`/`Decode`, not `Split`.
  - `Mint` calls `exists` with `Format(prefix, body)`, not the bare body. Truncate `now` to seconds **after** `UTC()`. `for range 16` is 16 attempts (Go 1.22+). A 4-bit nibble is `buf[0]&0x0F`, not a decimal 0–9.
  - `WorkerFor` MUST use `hash/fnv` `New32a` (FNV-1a, not FNV-1). `% 64` after `Sum32`.

- [ ] **Step 4: Run the Encode tests, see them pass.**
  ```bash
  go test ./pkg/id -run 'TestEncode|TestDecode' -v
  ```
  Expected:
  ```text
  === RUN   TestEncodeDecodeRoundTrip
  --- PASS: TestEncodeDecodeRoundTrip (0.00s)
  === RUN   TestEncodeWorkedExample
  --- PASS: TestEncodeWorkedExample (0.00s)
  === RUN   TestEncodeRangeErrors
  --- PASS: TestEncodeRangeErrors (0.00s)
  === RUN   TestDecodeRejectsAmbiguous
  --- PASS: TestDecodeRejectsAmbiguous (0.00s)
  === RUN   TestDecodeAcceptsLowercase
  --- PASS: TestDecodeAcceptsLowercase (0.00s)
  PASS
  ok  	github.com/eisenwinter/awit/pkg/id	0.00s
  ```
  Commit:
  ```bash
  gofmt -l pkg/id
  git add pkg/id
  git commit -m "id: add base32 codec"
  ```
  (`gofmt -l` must print nothing.)

- [ ] **Step 5: Run Format/Split/Valid/Time/sort, see them pass.**
  ```bash
  go test ./pkg/id -run 'TestSplitAndValid|TestTime|TestIDsSortByTime' -v
  ```
  Expected:
  ```text
  === RUN   TestIDsSortByTime
  --- PASS: TestIDsSortByTime (0.00s)
  === RUN   TestSplitAndValid
  --- PASS: TestSplitAndValid (0.00s)
  === RUN   TestTime
  --- PASS: TestTime (0.00s)
  PASS
  ok  	github.com/eisenwinter/awit/pkg/id	0.00s
  ```
  `TestTime` must print `2026-09-17T20:30:00Z` (UTC, seconds precision). If it is off by the local zone you forgot `Epoch` is UTC and `Time` must return a UTC `time.Time`.
  Commit:
  ```bash
  git add pkg/id
  git commit -m "id: Format Split Valid Time"
  ```

- [ ] **Step 6: Run Worker and Mint tests, see them pass.**
  ```bash
  go test ./pkg/id -run 'TestWorker|TestMint' -v
  ```
  Expected:
  ```text
  === RUN   TestWorkerForStable
  --- PASS: TestWorkerForStable (0.00s)
  === RUN   TestWorkerEnvOverride
  --- PASS: TestWorkerEnvOverride (0.00s)
  === RUN   TestMintRetriesOnCollision
  --- PASS: TestMintRetriesOnCollision (0.00s)
  === RUN   TestMintExhausted
  --- PASS: TestMintExhausted (0.00s)
  PASS
  ok  	github.com/eisenwinter/awit/pkg/id	0.00s
  ```
  If `TestWorkerForStable` gets a value other than 22, the NUL separators are missing or FNV-1 (not FNV-1a) was used. If `TestMintExhausted` sees `n != 16`, the retry loop is wrong. If `TestMintRetriesOnCollision` sees `n != 4`, `exists` is not being called per attempt.
  Commit:
  ```bash
  git add pkg/id
  git commit -m "id: worker hash and mint retries"
  ```

- [ ] **Step 7: Run the whole package, build and vet.**
  ```bash
  go test ./pkg/id -v
  go build ./...
  go vet ./pkg/id
  gofmt -l pkg/id
  ```
  Expected: twelve tests PASS (`TestEncodeDecodeRoundTrip`, `TestEncodeWorkedExample`, `TestEncodeRangeErrors`, `TestDecodeRejectsAmbiguous`, `TestDecodeAcceptsLowercase`, `TestIDsSortByTime`, `TestSplitAndValid`, `TestTime`, `TestWorkerForStable`, `TestWorkerEnvOverride`, `TestMintRetriesOnCollision`, `TestMintExhausted`), then `ok  	github.com/eisenwinter/awit/pkg/id`; `go build` and `go vet` print nothing and exit `0`; `gofmt -l` prints nothing.

- [ ] **Step 8: Close ticket.**
  - Set `status: closed` in the frontmatter of `.awit/items/AWIT-0ND5693G.md`.
  - Create `.awit/comments/AWIT-0ND5693G/<YYYYMMDDTHHMMSSZ>-<author>.md` (UTC stamp):
    ```markdown
    ---
    author: agent/claude
    created: 2026-09-17T15:22:33Z
    ---

    Acceptance output:

    $ go test ./pkg/id -v
    (all twelve tests PASS; paste the real output here)

    $ go build ./...
    (no output, exit 0)

    $ go vet ./pkg/id
    (no output, exit 0)

    $ gofmt -l pkg/id
    (no output)
    ```
  - Append the ref `../comments/AWIT-0ND5693G/<file>.md` to this ticket's `refs` list (forward slashes, block style, after the two plan refs).
  - Commit:
    ```bash
    git add .awit/items/AWIT-0ND5693G.md .awit/comments/AWIT-0ND5693G
    git commit -m "tickets: close AWIT-0ND5693G"
    ```

## Acceptance Criteria
- `go test ./pkg/id -v` → all twelve tests `PASS`, final line `ok  	github.com/eisenwinter/awit/pkg/id`, exit code `0`.
- `go test ./pkg/id -run TestEncodeWorkedExample -v` → `PASS`; `Encode(22451400, 7, 0) == "0ND5683G"`.
- `go test ./pkg/id -run TestTime -v` → `PASS`; `Time("0ND5683G")` is `2026-09-17T20:30:00Z`.
- `go test ./pkg/id -run TestIDsSortByTime -v` → `PASS`; 200 IDs with increasing seconds, same worker/rnd, `sort.StringsAreSorted`.
- `go test ./pkg/id -run TestDecodeRejectsAmbiguous -v` → `PASS` for `I`, `L`, `O`, `U`, `SHORT` and the four 8-char bodies ending in those letters.
- `go test ./pkg/id -run TestWorkerEnvOverride -v` → `PASS` for `t.Setenv` values `9` (override), `99` (fallback), `abc` (fallback).
- `go test ./pkg/id -run TestMintExhausted -v` → `PASS`; `errors.Is(err, ErrExhausted)` and `exists` called 16 times.
- `go build ./...` → no output, exit `0`.
- `go vet ./pkg/id` → no output, exit `0`.
- `gofmt -l pkg/id` → no output.
- `pkg/id/id.go` imports no third-party module: `grep -c 'urfave\|yaml' pkg/id/id.go` → `0` with exit code `1`.

## Out of scope
- `pkg/item.Store.Mint` / `Store.Exists` / filename allocation — `AWIT-0ND56E3G`. This ticket only provides `id.Mint` and `id.Worker`.
- `gitx.Branch` — `AWIT-0ND56C3G`. `Worker` takes `worktree` and `branch` as plain strings; it does not call git.
- `awit create`, `--id` override, CLI wiring — `AWIT-0ND56H3G`.
- Prefix configuration (`config.yaml` `prefix`) — `AWIT-0ND56A3G`. `Format`/`Mint` take `prefix` as an argument.
- Crockford checksum chars, hyphens inside the body, or fuzzy I/L/O/U mapping.
- A package-level `rand` seam or injectable clock. Tests inject `exists` and pass `now`; `crypto/rand` stays real.
- Sequential counters, UUID, ULID, or any ID width other than 8 chars / 40 bits.
