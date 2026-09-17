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
