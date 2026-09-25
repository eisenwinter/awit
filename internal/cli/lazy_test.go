package cli

import (
	"bytes"
	"strings"
	"testing"
	"time"
)

func TestLazyHumanRegisteredAndQuits(t *testing.T) {
	code, stdout, _ := run(t, "lazy-human", "--help")
	if code != 0 || !strings.Contains(stdout, "--agent") {
		t.Fatalf("help: exit %d\n%s", code, stdout)
	}
	dir := copyFixture(t, "clean")
	done := make(chan int, 1)
	go func() {
		var out, errb bytes.Buffer
		done <- Main([]string{"--repo", dir, "lazy-human"}, strings.NewReader("q"), &out, &errb)
	}()
	select {
	case code := <-done:
		if code != 0 {
			t.Fatalf("exit %d", code)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("lazy-human did not exit on q within 10s")
	}
}
