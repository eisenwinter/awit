package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/eisenwinter/awit/internal/ops"
	"github.com/eisenwinter/awit/pkg/format"
	"github.com/urfave/cli/v3"
)

var commentCmd = &cli.Command{
	Name:      "comment",
	Usage:     "Write a timestamped comment or attach a file to an item",
	ArgsUsage: "<id> [text...]",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "file", Usage: "copy this file into the item's comments instead of writing text"},
		&cli.StringFlag{Name: "author", Usage: "author (default: AWIT_AGENT, config agent_id, git user.name)"},
	},
	Action: commentAction,
}

// commentJSON is the --format json shape: the item that received the
// comment and the forward-slash ref that was appended to its refs.
type commentJSON struct {
	ID  string `json:"id"`
	Ref string `json:"ref"`
}

func commentAction(_ context.Context, cmd *cli.Command) error {
	id := cmd.Args().First()
	if id == "" {
		return fmt.Errorf("comment needs an item id")
	}
	text := strings.Join(cmd.Args().Tail(), " ")
	file := cmd.String("file")
	if file != "" && text != "" {
		return fmt.Errorf("pass either text or --file")
	}
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	noteWalkedUp(cmd, s)
	release, err := s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer release()
	it, err := ops.LoadItem(s, id)
	if err != nil {
		return err
	}
	author, err := ops.ResolveAuthor(cmd.String("author"), s.Root, s.Config)
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	var ref string
	if file != "" {
		ref, err = s.AttachFile(it, author, now, file)
	} else {
		if text == "" {
			text, err = readStdinText(cmd.Root().Reader)
			if err != nil {
				return err
			}
		}
		if strings.TrimSpace(text) == "" {
			return fmt.Errorf("empty comment")
		}
		ref, err = s.AddComment(it, author, now, text)
	}
	if err != nil {
		return err
	}
	f, err := detectFormat(cmd)
	if err != nil {
		return err
	}
	if f == format.JSON {
		enc := json.NewEncoder(cmd.Root().Writer)
		enc.SetIndent("", "  ")
		return enc.Encode(commentJSON{ID: it.ID, Ref: ref})
	}
	_, err = fmt.Fprintln(cmd.Root().Writer, ref)
	return err
}

// readStdinText returns everything on r, or "" when r is an interactive
// terminal (so a bare "awit comment <id>" in a shell fails with "empty
// comment" instead of hanging on a read nobody knows is pending).
func readStdinText(r io.Reader) (string, error) {
	if r == nil {
		return "", nil
	}
	if f, ok := r.(*os.File); ok && format.IsTerminal(f) {
		return "", nil
	}
	b, err := io.ReadAll(r)
	if err != nil {
		return "", fmt.Errorf("read stdin: %w", err)
	}
	return string(b), nil
}
