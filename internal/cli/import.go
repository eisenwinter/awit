package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/eisenwinter/awit/internal/glabx"
	"github.com/eisenwinter/awit/pkg/format"
	"github.com/eisenwinter/awit/pkg/item"
	"github.com/urfave/cli/v3"
)

var importCmd = &cli.Command{
	Name:      "import",
	Usage:     "Mint a local item from an existing Gitea or GitLab issue (one-time snapshot)",
	ArgsUsage: "<issue-url>",
	Description: `Import copies labels once as metadata. Labels are not synchronised
afterwards, and a label named "blocked" does not pause work. Use
awit block <id> --reason "..." for a local manual block.`,
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "brief", Usage: "one to three sentences (default: derived from the remote title or body)"},
		&cli.StringFlag{Name: "alias", Usage: "short human alias for the new item"},
		&cli.StringFlag{Name: "tea-login", Usage: "tea login name for a Gitea issue; ignored for GitLab"},
	},
	Action: importAction,
}

// parseIssueURL turns an issue URL into a Gitea or GitLab external mapping.
// GitLab is recognized only by the explicit /-/issues/ or /-/work_items/
// path shape — never by host — and other /-/ resources are refused before
// the Gitea branch. GitLab prefix resolution is delegated to glabx.
func parseIssueURL(ctx context.Context, raw string) (item.External, error) {
	bad := func() (item.External, error) {
		return item.External{}, cli.Exit(fmt.Sprintf("Incorrect usage: issue URL %q must look like https://host/owner/repo/issues/123 or https://host/group/project/-/issues/123 (run \"awit --help\")", raw), 2)
	}
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() || (u.Scheme != "http" && u.Scheme != "https") {
		return bad()
	}
	segs := strings.Split(strings.Trim(u.Path, "/"), "/")
	for i := range segs {
		if segs[i] != "-" {
			continue
		}
		if i+2 == len(segs)-1 && (segs[i+1] == "issues" || segs[i+1] == "work_items") {
			if n, err := strconv.ParseInt(segs[i+2], 10, 64); err != nil || n <= 0 {
				return bad()
			}
			ext, err := glabx.ParseIssueURL(ctx, raw)
			if err != nil {
				return item.External{}, err
			}
			return ext, nil
		}
		return bad()
	}
	if len(segs) < 4 || segs[len(segs)-2] != "issues" {
		return bad()
	}
	n, err := strconv.ParseInt(segs[len(segs)-1], 10, 64)
	if err != nil {
		return bad()
	}
	ext := item.External{
		Tracker: "gitea",
		Repo:    segs[len(segs)-4] + "/" + segs[len(segs)-3],
		ID:      n,
		URL:     raw,
	}
	if err := item.ValidateExternal(ext); err != nil {
		return item.External{}, cli.Exit(err.Error(), 2)
	}
	return ext, nil
}

func importAction(ctx context.Context, cmd *cli.Command) error {
	if cmd.Args().Len() != 1 {
		return cli.Exit("import needs exactly one issue URL", 2)
	}
	ext, err := parseIssueURL(ctx, cmd.Args().First())
	if err != nil {
		return err
	}
	alias := cmd.String("alias")
	if alias != "" {
		if err := item.ValidateAlias(alias); err != nil {
			return cli.Exit(err.Error(), 2)
		}
	}
	s, err := openStore(cmd)
	if err != nil {
		return err
	}
	noteWalkedUp(cmd, s)
	// Preflight and fetch happen before the mutation lock: no local state
	// is touched while the network is involved. --tea-login is Gitea-only.
	issue, err := getExternalIssue(ctx, ext, cmd.String("tea-login"))
	if err != nil {
		return err
	}
	if issue.Number != ext.ID {
		return fmt.Errorf("issue number mismatch: the URL says %d but the server returned number %d", ext.ID, issue.Number)
	}
	var status item.Status
	switch issue.State {
	case "open":
		status = item.StatusOpen
	case "closed":
		status = item.StatusClosed
	default:
		return fmt.Errorf("unsupported issue state %q for %s#%d (only open and closed can be imported)", issue.State, ext.Repo, ext.ID)
	}
	// An explicit --brief is used verbatim (no normalization, truncation or
	// sentence extraction); --brief="" is the unset value and derives. The
	// stored title keeps the exact remote bytes either way.
	brief := cmd.String("brief")
	if brief == "" {
		brief = deriveImportBrief(issue.Title, issue.Body)
		if brief == "" {
			return fmt.Errorf("cannot derive import brief: remote title and body are empty; pass --brief")
		}
	}
	// Labels are the exact remote label names, first-seen deduplicated.
	// Local default labels are NOT merged: an import is a historical
	// snapshot, not a fresh create.
	labels := []string{}
	seen := map[string]bool{}
	for _, l := range issue.Labels {
		if l == "" || seen[l] {
			continue
		}
		seen[l] = true
		labels = append(labels, l)
	}
	release, err := s.Lock(5 * time.Second)
	if err != nil {
		return err
	}
	defer release()
	if err := refuseDuplicateImport(s, ext); err != nil {
		return err
	}
	newID, err := s.Mint(time.Now())
	if err != nil {
		return err
	}
	it := item.New(newID, issue.Title, brief, nil, labels)
	it.SetStatus(status)
	ext.ID = issue.Number // the repository issue number, never the database id
	if err := it.SetExternal(&ext); err != nil {
		return err
	}
	if alias != "" {
		if err := it.SetAlias(alias); err != nil {
			return cli.Exit(err.Error(), 2)
		}
	}
	it.SetBody(issue.Body)
	if err := validateImportCandidate(it); err != nil {
		return err
	}
	if err := s.Save(it); err != nil {
		return err
	}
	warnBlockedLabel(cmd, it, labels)
	f, err := detectFormat(cmd)
	if err != nil {
		return err
	}
	state := "ready"
	if status == item.StatusClosed {
		state = "closed"
	}
	return format.WriteOne(cmd.Root().Writer, f, format.Entry{
		ID:       it.ID,
		Title:    it.Title,
		Brief:    it.Brief,
		Status:   string(it.Status),
		State:    state,
		Labels:   it.Labels,
		Deps:     []string{},
		Alias:    it.Alias,
		Unblocks: 0,
		External: it.External,
	})
}

// maxDerivedBriefRunes caps automatically derived import briefs in Unicode
// code points, not bytes.
const maxDerivedBriefRunes = 240

// deriveImportBrief derives the brief for an import without an explicit
// --brief. A title holding any non-whitespace rune wins and is used whole;
// otherwise the body's first sentence is used, stopping at the first '.',
// '!' or '?' immediately followed by whitespace or end-of-source (the same
// boundary convention as sentenceCount). The chosen source is trimmed of
// leading/trailing Unicode whitespace with each internal run collapsed to
// one ASCII space; Markdown, case and punctuation are untouched. Derived
// values are capped at maxDerivedBriefRunes code points. Both sources
// normalizing to empty yields "". The scan accumulates at most the capped
// candidate plus one rune of lookahead, so an arbitrarily long remote body
// is never copied or normalized in full.
func deriveImportBrief(title string, body []byte) string {
	useTitle := false
	for _, r := range title {
		if !unicode.IsSpace(r) {
			useTitle = true
			break
		}
	}
	// next yields source runes one at a time without copying the body.
	pos := 0
	next := func() (rune, bool) {
		if useTitle {
			if pos >= len(title) {
				return 0, false
			}
			r, w := utf8.DecodeRuneInString(title[pos:])
			pos += w
			return r, true
		}
		if pos >= len(body) {
			return 0, false
		}
		r, w := utf8.DecodeRune(body[pos:])
		pos += w
		return r, true
	}
	var out []rune
	pendingSpace := false
	var pendingPunct rune // body '.', '!' or '?' awaiting its follower
	// emit appends one normalized rune, collapsing a pending whitespace run
	// to a single ASCII space. It reports true when non-space content lies
	// beyond the cap, in which case out is left at the cap for truncation.
	emit := func(r rune) bool {
		if pendingSpace && len(out) > 0 {
			if len(out) == maxDerivedBriefRunes {
				return true
			}
			out = append(out, ' ')
		}
		pendingSpace = false
		if len(out) == maxDerivedBriefRunes {
			return true
		}
		out = append(out, r)
		return false
	}
	for {
		r, ok := next()
		if !ok {
			break
		}
		if unicode.IsSpace(r) {
			if pendingPunct != 0 {
				// Punctuation followed by whitespace ends the body
				// sentence; the punctuation is kept, with any pending
				// space flushed before it like any other rune.
				if emit(pendingPunct) {
					return truncateDerivedBrief(out)
				}
				return string(out)
			}
			pendingSpace = true
			continue
		}
		if pendingPunct != 0 {
			if emit(pendingPunct) {
				return truncateDerivedBrief(out)
			}
			pendingPunct = 0
		}
		if !useTitle && (r == '.' || r == '!' || r == '?') {
			pendingPunct = r
			continue
		}
		if emit(r) {
			return truncateDerivedBrief(out)
		}
	}
	if pendingPunct != 0 {
		// Punctuation at end-of-source ends the sentence; it is kept,
		// with any pending space flushed before it like any other rune.
		if emit(pendingPunct) {
			return truncateDerivedBrief(out)
		}
	}
	return string(out)
}

// truncateDerivedBrief caps an over-long derived candidate at
// maxDerivedBriefRunes code points: the first 239 runes minus any trailing
// normalized space, plus U+2026. The result holds no space before the
// ellipsis and is at most 240 runes of valid UTF-8.
func truncateDerivedBrief(out []rune) string {
	prefix := out
	if len(prefix) > maxDerivedBriefRunes {
		prefix = prefix[:maxDerivedBriefRunes]
	}
	if len(prefix) == maxDerivedBriefRunes {
		prefix = prefix[:maxDerivedBriefRunes-1]
	}
	for len(prefix) > 0 && prefix[len(prefix)-1] == ' ' {
		prefix = prefix[:len(prefix)-1]
	}
	return string(append(prefix, '…'))
}

// validateImportCandidate serializes the would-be item and refuses anything
// that would immediately quarantine. The remote body is never altered: an
// issue carrying conflict markers must be reconciled before import.
func validateImportCandidate(it *item.Item) error {
	data, err := it.Bytes()
	if err != nil {
		return err
	}
	if item.HasConflictMarkers(data) {
		return fmt.Errorf("the issue body contains conflict markers; reconcile it on the issue before importing (local bytes are never silently altered)")
	}
	if _, err := item.Parse(it.ID+".md", data); err != nil {
		return fmt.Errorf("the imported item would not parse: %w", err)
	}
	return nil
}

// refuseDuplicateImport enforces import identity under the store lock: the
// same tracker + normalized installation base + repo + issue number may
// exist at most once across active items and archived item files. A file
// that cannot be inspected — including invalid external metadata — refuses
// the import instead of claiming uniqueness. Missing external metadata is
// not a match. Archived items remain excluded from the graph; this scan is
// an import-identity guard only.
func refuseDuplicateImport(s *item.Store, ext item.External) error {
	base, err := externalBase(ext)
	if err != nil {
		return err
	}
	items, broken, err := s.LoadAll()
	if err != nil {
		return err
	}
	for _, b := range broken {
		return fmt.Errorf("cannot verify import uniqueness: %s is broken (%s); fix it first", b.Path, b.Reason)
	}
	for _, it := range items {
		if it.ExternalProblem != "" {
			return fmt.Errorf("cannot verify import uniqueness: %s has invalid external metadata (%s); fix it first", it.Path, it.ExternalProblem)
		}
		if sameImportIdentity(base, ext, it.External) {
			return fmt.Errorf("issue %s#%d was already imported as %s (%s); import is a one-time snapshot, not an update", ext.Repo, ext.ID, it.ID, it.Path)
		}
	}
	ents, err := os.ReadDir(s.ArchiveDir())
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".md" {
			continue
		}
		p := filepath.Join(s.ArchiveDir(), e.Name())
		data, err := os.ReadFile(p)
		if err != nil {
			return fmt.Errorf("cannot verify import uniqueness: %s: %w", p, err)
		}
		if item.HasConflictMarkers(data) {
			return fmt.Errorf("cannot verify import uniqueness: %s contains conflict markers", p)
		}
		arch, err := item.Parse(p, data)
		if err != nil {
			return fmt.Errorf("cannot verify import uniqueness: %s does not parse: %w", p, err)
		}
		if arch.ExternalProblem != "" {
			return fmt.Errorf("cannot verify import uniqueness: %s has invalid external metadata (%s); fix it first", p, arch.ExternalProblem)
		}
		if sameImportIdentity(base, ext, arch.External) {
			return fmt.Errorf("issue %s#%d was already imported and archived as %s; import is a one-time snapshot, not an update", ext.Repo, ext.ID, p)
		}
	}
	return nil
}

func sameImportIdentity(base string, want item.External, have *item.External) bool {
	if have == nil || have.Tracker != want.Tracker || have.Repo != want.Repo || have.ID != want.ID {
		return false
	}
	hb, err := externalBase(*have)
	return err == nil && hb == base
}
