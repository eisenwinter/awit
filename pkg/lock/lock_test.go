package lock

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	rel, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("lock file missing after Acquire: %v", err)
	}
	if err := rel(); err != nil {
		t.Fatal(err)
	}
	rel2, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatalf("re-acquire after release: %v", err)
	}
	if err := rel2(); err != nil {
		t.Fatal(err)
	}
}

func TestSecondAcquireBlocksUntilRelease(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	rel, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		close(started)
		rel2, err := Acquire(path, 3*time.Second)
		if err != nil {
			done <- err
			return
		}
		done <- rel2()
	}()
	<-started
	select {
	case err := <-done:
		t.Fatalf("second Acquire returned before release: %v", err)
	case <-time.After(150 * time.Millisecond):
	}
	if err := rel(); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("second Acquire: %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("second Acquire did not unblock after release")
	}
}

func TestAcquireTimeout(t *testing.T) {
	path := filepath.Join(t.TempDir(), "lock")
	rel, err := Acquire(path, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer rel()
	start := time.Now()
	_, err = Acquire(path, 200*time.Millisecond)
	elapsed := time.Since(start)
	if !errors.Is(err, ErrTimeout) {
		t.Fatalf("err = %v, want ErrTimeout", err)
	}
	if elapsed < 150*time.Millisecond || elapsed > 2*time.Second {
		t.Fatalf("timeout elapsed %v, want ~200ms", elapsed)
	}
}
