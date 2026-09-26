package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/eisenwinter/awit/internal/skill"
	"github.com/eisenwinter/awit/pkg/config"
	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var initCmd = &cli.Command{
	Name:  "init",
	Usage: "Create a .awit directory",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "prefix",
			Value: "AWIT",
			Usage: "item id prefix (2-8 uppercase alphanumerics starting with a letter)",
		},
		&cli.BoolFlag{
			Name:  "skills",
			Usage: "seed the driving-awit skill into every detected agent directory without asking",
		},
		&cli.BoolFlag{
			Name:  "no-skills",
			Usage: "do not look for agent directories and do not ask",
		},
		&cli.BoolFlag{
			Name:  "force",
			Usage: "overwrite an existing skill file instead of keeping it",
		},
	},
	Action: initAction,
}

func initAction(_ context.Context, cmd *cli.Command) error {
	prefix := cmd.String("prefix")
	if !config.ValidPrefix(prefix) {
		return fmt.Errorf("prefix must be 2-8 uppercase alphanumerics starting with a letter")
	}
	if cmd.Bool("skills") && cmd.Bool("no-skills") {
		return cli.Exit("Error: pass either --skills or --no-skills", 2)
	}
	root := cmd.Root().String("repo")
	var err error
	if root == "" {
		root, err = os.Getwd()
	} else {
		root, err = filepath.Abs(root)
	}
	if err != nil {
		return err
	}
	if _, err := item.Init(root, prefix); err != nil {
		return err
	}
	fmt.Fprintf(cmd.Root().Writer, "Initialized .awit in %s (prefix %s)\n", root, prefix)

	if !cmd.Bool("no-skills") {
		seedSkills(cmd, root)
	}
	return nil
}

// seedSkills offers the driving-awit skill to every agent directory the repo
// already has. It never fails init: .awit exists by the time it runs, so a
// write error is a warning, not a reason to leave a half-made repository.
func seedSkills(cmd *cli.Command, root string) {
	targets := skill.Detect(root)
	if len(targets) == 0 {
		return
	}
	out := cmd.Root().Writer
	ask := !cmd.Bool("skills")
	force := cmd.Bool("force")

	// One reader for every prompt: a fresh bufio.Reader per question would
	// drop whatever the previous one buffered past its newline.
	var in *bufio.Reader
	if ask && cmd.Root().Reader != nil {
		in = bufio.NewReader(cmd.Root().Reader)
	}
	// On a terminal the user's Enter ends the prompt line. Anywhere else
	// nothing echoes, so without this the answer and the next message run
	// together. Display only - the prompt is asked either way.
	//
	// format.IsTerminal really answers "is a character device", so stdin
	// from /dev/null counts as a terminal here and those prompts do run
	// together. Left alone: an EOF stdin seeds nothing, so the only output
	// affected is prompts nobody answered.
	echo := true
	if f, ok := cmd.Root().Reader.(*os.File); ok && format.IsTerminal(f) {
		echo = false
	}

	for _, t := range targets {
		dest := filepath.Join(root, t.Dir, t.Path)
		shown := filepath.ToSlash(filepath.Join(t.Dir, t.Path))

		if _, err := os.Stat(dest); err == nil && !force {
			fmt.Fprintf(out, "  %s exists, keeping it (--force overwrites)\n", shown)
			continue
		}
		if ask && !confirm(out, in, t.Dir, echo) {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			fmt.Fprintf(cmd.Root().ErrWriter, "Warning: %s: %v\n", shown, err)
			continue
		}
		if err := config.WriteAtomic(dest, skill.Render(t)); err != nil {
			fmt.Fprintf(cmd.Root().ErrWriter, "Warning: %s: %v\n", shown, err)
			continue
		}
		fmt.Fprintf(out, "  seeded %s\n", shown)
	}
}

// confirm asks once and reads a single line. Reading one line is safe on a
// terminal in a way that reading to EOF is not, so there is no need to detect
// one; a closed or exhausted stdin simply reads EOF, which means no.
func confirm(out io.Writer, in *bufio.Reader, dir string, echo bool) bool {
	fmt.Fprintf(out, "seed the %s skill into %s/skills/? [y/N] ", skill.Name, dir)
	if in == nil {
		fmt.Fprintln(out)
		return false
	}
	line, err := in.ReadString('\n')
	if echo {
		fmt.Fprintln(out)
	}
	if err != nil && line == "" {
		return false
	}
	switch strings.ToLower(strings.TrimSpace(line)) {
	case "y", "yes":
		return true
	default:
		return false
	}
}
