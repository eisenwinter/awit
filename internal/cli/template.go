package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"unicode/utf8"

	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var templateCmd = &cli.Command{
	Name:  "template",
	Usage: "Print the work item body template",
	Description: `Writes the body create would use: the file named by config.yaml
template:, or the built-in skeleton when none is set. Fill it in and pass it
back with awit create --body-file, or awit update <id> --body-file.

Builds no graph, so it never reports quarantined items. Ignores --format.`,
	Action: func(ctx context.Context, cmd *cli.Command) error {
		s, err := openStore(cmd)
		if err != nil {
			return err
		}
		body, err := readTemplateBody(s.Root, s.Config.Template)
		if err != nil {
			return err
		}
		if body == nil {
			body = []byte(item.DefaultBody)
		}
		_, err = cmd.Root().Writer.Write(body)
		return err
	},
}

// validateBodyBytes rejects bodies awit cannot store: invalid UTF-8, and Git
// conflict markers that would quarantine the item on its next load. source
// names the origin for the message ("template docs/t.md", "--body").
func validateBodyBytes(source string, data []byte) error {
	if !utf8.Valid(data) {
		return fmt.Errorf("%s: not valid UTF-8", source)
	}
	if item.HasConflictMarkers(data) {
		return fmt.Errorf("%s: conflict markers", source)
	}
	return nil
}

// readTemplateBody reads the config.yaml template: file. An empty rel means
// no template is configured and returns (nil, nil), which leaves the caller
// on item.DefaultBody. Shared by create and template.
func readTemplateBody(root, rel string) ([]byte, error) {
	if rel == "" {
		return nil, nil
	}
	cleaned := path.Clean(rel)
	abs := filepath.Join(root, filepath.FromSlash(cleaned))
	fi, err := os.Stat(abs)
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", rel, err)
	}
	if fi.IsDir() {
		return nil, fmt.Errorf("template %s: is a directory", rel)
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, fmt.Errorf("template %s: %w", rel, err)
	}
	if err := validateBodyBytes("template "+rel, data); err != nil {
		return nil, err
	}
	return data, nil
}

// resolveBodyFlags returns the body named by --body or --body-file, or (nil, nil)
// when neither is set. "-" reads stdin. The two are mutually exclusive. Callers
// must invoke it before any mutation: a usage error must not leave a minted ID
// or a half-written item behind.
//
// --body "" is indistinguishable from unset, the same tri-state limit the reused
// command tree forces on every string flag. An empty body is reachable only
// through --body-file naming an empty file.
func resolveBodyFlags(cmd *cli.Command) ([]byte, error) {
	text := cmd.String("body")
	file := cmd.String("body-file")
	switch {
	case text != "" && file != "":
		return nil, cli.Exit("Incorrect usage: --body and --body-file cannot be combined", 2)
	case text != "":
		if err := validateBodyBytes("--body", []byte(text)); err != nil {
			return nil, err
		}
		return []byte(text), nil
	case file == "":
		return nil, nil
	}
	var (
		data []byte
		err  error
	)
	if file == "-" {
		data, err = io.ReadAll(cmd.Root().Reader)
	} else {
		data, err = os.ReadFile(file)
	}
	if err != nil {
		return nil, fmt.Errorf("--body-file %s: %w", file, err)
	}
	if err := validateBodyBytes("--body-file "+file, data); err != nil {
		return nil, err
	}
	return data, nil
}
