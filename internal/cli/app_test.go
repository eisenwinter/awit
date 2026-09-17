package cli

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"
)

func TestReport(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantCode int
		wantOut  string
	}{
		{name: "nil is success", err: nil, wantCode: 0, wantOut: ""},
		{
			name:     "plain error gets the Error prefix",
			err:      errors.New("boom"),
			wantCode: 1,
			wantOut:  "Error: boom\n",
		},
		{
			name:     "wrapped error is unwrapped by the formatter",
			err:      fmt.Errorf("load: %w", errors.New("boom")),
			wantCode: 1,
			wantOut:  "Error: load: boom\n",
		},
		{
			name:     "exit coder keeps its code and prints bare",
			err:      cli.Exit("No ready items", 1),
			wantCode: 1,
			wantOut:  "No ready items\n",
		},
		{
			name:     "usage exit coder returns 2",
			err:      cli.Exit(`unknown command "nope"`, 2),
			wantCode: 2,
			wantOut:  "unknown command \"nope\"\n",
		},
		{
			name:     "empty exit coder message prints nothing",
			err:      cli.Exit("", 3),
			wantCode: 3,
			wantOut:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var buf bytes.Buffer
			got := report(&buf, tt.err)
			if got != tt.wantCode {
				t.Errorf("report code = %d, want %d", got, tt.wantCode)
			}
			if buf.String() != tt.wantOut {
				t.Errorf("report output = %q, want %q", buf.String(), tt.wantOut)
			}
		})
	}
}

func TestMainVersion(t *testing.T) {
	for _, arg := range []string{"--version", "-v"} {
		code, stdout, stderr := runMain(t, arg)
		if code != 0 {
			t.Fatalf("%s exit = %d, want 0 (stderr %q)", arg, code, stderr)
		}
		if !strings.Contains(stdout, "awit dev") {
			t.Errorf("%s stdout = %q, want it to contain %q", arg, stdout, "awit dev")
		}
		if stderr != "" {
			t.Errorf("%s stderr = %q, want empty", arg, stderr)
		}
	}
}

func TestMainHelp(t *testing.T) {
	code, stdout, stderr := runMain(t, "--help")
	if code != 0 {
		t.Fatalf("--help exit = %d, want 0 (stderr %q)", code, stderr)
	}
	for _, want := range []string{"USAGE", "awit", "--format", "--repo", "--no-color"} {
		if !strings.Contains(stdout, want) {
			t.Errorf("--help stdout is missing %q; got:\n%s", want, stdout)
		}
	}
}

func TestMainNoArgsPrintsHelp(t *testing.T) {
	code, stdout, stderr := runMain(t)
	if code != 0 {
		t.Fatalf("bare awit exit = %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "USAGE") {
		t.Errorf("bare awit stdout = %q, want it to contain USAGE", stdout)
	}
}

func TestMainUnknownCommand(t *testing.T) {
	code, _, stderr := runMain(t, "nope")
	if code != 2 {
		t.Fatalf("unknown command exit = %d, want 2 (stderr %q)", code, stderr)
	}
	if len(stderr) == 0 {
		t.Fatal("unknown command wrote nothing to stderr")
	}
	if !strings.Contains(stderr, "nope") {
		t.Errorf("stderr = %q, want it to name the unknown command", stderr)
	}
}

func TestMainUsageError(t *testing.T) {
	code, _, stderr := runMain(t, "--bogus")
	if code != 2 {
		t.Fatalf("bad flag exit = %d, want 2 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stderr, "bogus") {
		t.Errorf("stderr = %q, want it to name the offending flag", stderr)
	}
}

func TestMainGlobalFlagsAreAccepted(t *testing.T) {
	code, stdout, stderr := runMain(t, "--no-color", "--format", "json", "--repo", ".", "--help")
	if code != 0 {
		t.Fatalf("global flags exit = %d, want 0 (stderr %q)", code, stderr)
	}
	if !strings.Contains(stdout, "USAGE") {
		t.Errorf("stdout = %q, want help output", stdout)
	}
}

func TestSplitLabels(t *testing.T) {
	tests := []struct {
		name  string
		flags []string
		want  [][]string
	}{
		{
			name:  "one group per flag, comma splits within a flag",
			flags: []string{"p0,p1", "auth"},
			want:  [][]string{{"p0", "p1"}, {"auth"}},
		},
		{
			name:  "spaces trimmed and trailing empty dropped",
			flags: []string{" p0 , "},
			want:  [][]string{{"p0"}},
		},
		{
			name:  "empty flag produces no group",
			flags: []string{""},
			want:  nil,
		},
		{
			name:  "no flags",
			flags: nil,
			want:  nil,
		},
		{
			name:  "double comma collapses",
			flags: []string{"a,,b"},
			want:  [][]string{{"a", "b"}},
		},
		{
			name:  "three flags stay three groups",
			flags: []string{"p0", "auth,api", "db"},
			want:  [][]string{{"p0"}, {"auth", "api"}, {"db"}},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitLabels(tt.flags)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SplitLabels(%q) = %#v, want %#v", tt.flags, got, tt.want)
			}
		})
	}
}
