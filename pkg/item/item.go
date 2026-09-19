package item

import (
	"bytes"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

// Item is one Markdown work item under .awit/items/.
//
// Known frontmatter keys map to the exported fields below. Unknown keys are
// kept on the unexported YAML mapping node and survive Bytes() after any
// setter. Optional `external` is a Gitea issue mapping; invalid or legacy
// scalar values populate ExternalProblem and leave the YAML intact.
type Item struct {
	ID              string
	Title           string
	Brief           string
	Status          Status
	Deps            []string
	Labels          []string
	Assignee        string
	ClaimedAt       *time.Time
	Refs            []string
	RefsBase        string // "repo" or empty (historical .awit/items/ base)
	Path            string
	External        *External
	ExternalProblem string // derived diagnostic; never serialized

	doc   *yaml.Node
	body  []byte
	raw   []byte
	dirty bool
}

// External is a Gitea issue linked from an item. ID is the repository
// issue number, not Gitea's database-wide issue id.
type External struct {
	Tracker string `json:"tracker"`
	Repo    string `json:"repo"`
	ID      int64  `json:"id"`
	URL     string `json:"url"`
}

func seqStrings(v *yaml.Node) []string {
	if v.Kind != yaml.SequenceNode {
		return nil
	}
	out := make([]string, 0, len(v.Content))
	for _, c := range v.Content {
		out = append(out, c.Value)
	}
	return out
}

// Parse decodes an item. path is stored on the Item; the caller checks ID
// vs filename. Unknown keys are kept in the underlying mapping node so that
// Bytes preserves them.
func Parse(path string, data []byte) (*Item, error) {
	front, body, err := Split(data)
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(front, &root); err != nil {
		return nil, fmt.Errorf("item: parse frontmatter: %w", err)
	}
	if len(root.Content) == 0 || root.Content[0].Kind != yaml.MappingNode {
		return nil, fmt.Errorf("item: frontmatter is not a mapping")
	}
	doc := root.Content[0]
	it := &Item{Path: path, doc: doc, body: append([]byte(nil), body...), raw: append([]byte(nil), data...)}
	var (
		haveID, haveTitle, haveStatus bool
		statusRaw                     string
		claimedRaw                    string
		haveClaimed                   bool
	)
	for i := 0; i+1 < len(doc.Content); i += 2 {
		k := doc.Content[i]
		v := doc.Content[i+1]
		switch k.Value {
		case "id":
			it.ID = v.Value
			haveID = true
		case "title":
			it.Title = v.Value
			haveTitle = true
		case "brief":
			it.Brief = v.Value
		case "status":
			statusRaw = v.Value
			haveStatus = true
		case "deps":
			it.Deps = seqStrings(v)
		case "labels":
			it.Labels = seqStrings(v)
		case "assignee":
			it.Assignee = v.Value
		case "claimed_at":
			claimedRaw = v.Value
			haveClaimed = true
		case "refs_base":
			if v.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("item: refs_base must be a string")
			}
			switch v.Value {
			case "", "repo":
				it.RefsBase = v.Value
			default:
				return nil, fmt.Errorf(`item: refs_base must be "repo" or empty`)
			}
		case "refs":
			it.Refs = seqStrings(v)
		case "alias":
			if v.Kind != yaml.ScalarNode {
				return nil, fmt.Errorf("item: alias must be a string")
			}
			it.Alias = v.Value
		}
	}
	for _, req := range []struct {
		name string
		have bool
	}{
		{"id", haveID},
		{"title", haveTitle},
		{"status", haveStatus},
	} {
		if !req.have {
			return nil, fmt.Errorf("item: missing required key %q", req.name)
		}
	}
	st, err := ParseStatus(statusRaw)
	if err != nil {
		return nil, err
	}
	it.Status = st
	if haveClaimed {
		tm, err := time.Parse(time.RFC3339, claimedRaw)
		if err != nil {
			return nil, fmt.Errorf("item: claimed_at: %w", err)
		}
		it.ClaimedAt = &tm
	}
	if _, v, idx := it.findKey("external"); idx >= 0 {
		ext, problem := parseExternalNode(v)
		it.External = ext
		it.ExternalProblem = problem
	}
	return it, nil
}

func (it *Item) Bytes() ([]byte, error) {
	if !it.dirty && it.raw != nil {
		out := make([]byte, len(it.raw))
		copy(out, it.raw)
		return out, nil
	}
	var yb bytes.Buffer
	enc := yaml.NewEncoder(&yb)
	enc.SetIndent(2)
	if err := enc.Encode(it.doc); err != nil {
		return nil, err
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteString("---\n")
	buf.Write(yb.Bytes())
	buf.WriteString("---\n")
	buf.Write(it.body)
	return buf.Bytes(), nil
}

func New(id, title, brief string, deps, labels []string) *Item {
	depsCopy := append([]string(nil), deps...)
	labelsCopy := append([]string(nil), labels...)
	briefNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: brief}
	if strings.Contains(brief, "\n") || len(brief) > 80 {
		briefNode.Style = yaml.FoldedStyle
	}
	doc := &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: id},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "title"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: title},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "brief"},
			briefNode,
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "status"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: string(StatusOpen)},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "deps"},
			seqNode(depsCopy, yaml.FlowStyle),
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "labels"},
			seqNode(labelsCopy, yaml.FlowStyle),
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "refs_base"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "repo"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "refs"},
			seqNode(nil, 0),
		},
	}
	return &Item{
		ID:       id,
		Title:    title,
		Brief:    brief,
		Status:   StatusOpen,
		Deps:     depsCopy,
		Labels:   labelsCopy,
		Refs:     []string{},
		RefsBase: "repo",
		doc:      doc,
		body:     []byte("\n## Summary\n\n## Acceptance Criteria\n\n"),
		dirty:    true,
	}
}

func seqNode(values []string, style yaml.Style) *yaml.Node {
	n := &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: style}
	for _, v := range values {
		n.Content = append(n.Content, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v})
	}
	return n
}

func (it *Item) findKey(key string) (k, v *yaml.Node, idx int) {
	if it.doc == nil {
		return nil, nil, -1
	}
	for i := 0; i+1 < len(it.doc.Content); i += 2 {
		if it.doc.Content[i].Value == key {
			return it.doc.Content[i], it.doc.Content[i+1], i
		}
	}
	return nil, nil, -1
}

func (it *Item) setScalar(key, value string) {
	_, val, idx := it.findKey(key)
	if idx >= 0 {
		val.Kind = yaml.ScalarNode
		val.Tag = "!!str"
		val.Value = value
		return
	}
	it.doc.Content = append(it.doc.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}

func (it *Item) setScalarBefore(before, key, value string) {
	_, val, idx := it.findKey(key)
	if idx >= 0 {
		val.Kind = yaml.ScalarNode
		val.Tag = "!!str"
		val.Value = value
		val.Content = nil
		return
	}
	pair := []*yaml.Node{
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	}
	_, _, beforeIdx := it.findKey(before)
	if beforeIdx < 0 {
		it.doc.Content = append(it.doc.Content, pair...)
		return
	}
	it.doc.Content = append(it.doc.Content[:beforeIdx:beforeIdx], append(pair, it.doc.Content[beforeIdx:]...)...)
}

func (it *Item) deleteKey(key string) {
	_, _, idx := it.findKey(key)
	if idx < 0 {
		return
	}
	it.doc.Content = append(it.doc.Content[:idx], it.doc.Content[idx+2:]...)
}

func (it *Item) setSeq(key string, values []string, defaultStyle yaml.Style) {
	nodes := make([]*yaml.Node, 0, len(values))
	for _, v := range values {
		nodes = append(nodes, &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: v})
	}
	_, val, idx := it.findKey(key)
	if idx >= 0 {
		val.Kind = yaml.SequenceNode
		val.Tag = "!!seq"
		val.Value = ""
		val.Content = nodes
		return
	}
	it.doc.Content = append(it.doc.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq", Style: defaultStyle, Content: nodes},
	)
}

func (it *Item) markDirty() {
	it.dirty = true
}

func (it *Item) SetTitle(s string) {
	it.Title = s
	it.setScalar("title", s)
	it.markDirty()
}

func (it *Item) SetBrief(s string) {
	it.Brief = s
	it.setScalar("brief", s)
	_, val, _ := it.findKey("brief")
	if strings.Contains(s, "\n") || len(s) > 80 {
		val.Style = yaml.FoldedStyle
	} else {
		val.Style = 0
	}
	it.markDirty()
}

func (it *Item) SetStatus(s Status) {
	it.Status = s
	it.setScalar("status", string(s))
	it.markDirty()
}

func (it *Item) SetAssignee(s string) {
	it.Assignee = s
	if s == "" {
		it.deleteKey("assignee")
		it.markDirty()
		return
	}
	it.setScalar("assignee", s)
	it.markDirty()
}

func (it *Item) SetClaimedAt(t *time.Time) {
	if t == nil {
		it.ClaimedAt = nil
		it.deleteKey("claimed_at")
		it.markDirty()
		return
	}
	cp := t.UTC().Truncate(time.Second)
	it.ClaimedAt = &cp
	it.setScalar("claimed_at", cp.Format(time.RFC3339))
	it.markDirty()
}

func (it *Item) SetDeps(v []string) {
	it.Deps = append([]string(nil), v...)
	it.setSeq("deps", v, yaml.FlowStyle)
	it.markDirty()
}

func (it *Item) SetLabels(v []string) {
	it.Labels = append([]string(nil), v...)
	it.setSeq("labels", v, yaml.FlowStyle)
	it.markDirty()
}

func (it *Item) SetRefs(v []string) {
	it.Refs = append([]string(nil), v...)
	it.setSeq("refs", v, 0)
	it.markDirty()
}

// SetRefsBase records whether Refs are repo-root relative ("repo") or
// historical items-relative (empty). Only those two values are allowed.
func (it *Item) SetRefsBase(base string) error {
	if base != "" && base != "repo" {
		return fmt.Errorf(`item: refs_base must be "repo" or empty`)
	}
	if it.RefsBase == base {
		_, _, idx := it.findKey("refs_base")
		if base == "" && idx < 0 {
			return nil
		}
		if base == "repo" && idx >= 0 {
			return nil
		}
	}
	it.RefsBase = base
	if base == "" {
		it.deleteKey("refs_base")
	} else {
		it.setScalarBefore("refs", "refs_base", base)
	}
	it.markDirty()
	return nil
}

func (it *Item) HasLabel(l string) bool {
	for _, e := range it.Labels {
		if e == l {
			return true
		}
	}
	return false
}

func (it *Item) Body() []byte {
	return it.body
}

// SetBody replaces the markdown body with a copy of body. No newline
// conversion or other normalization is applied.
func (it *Item) SetBody(body []byte) {
	it.body = append([]byte(nil), body...)
	it.markDirty()
}

func invalidExternal(reason string) error {
	return fmt.Errorf("invalid external: %s", reason)
}

func validRepoSegment(s string) error {
	if s == "" || s == "." || s == ".." {
		return invalidExternal("repo must have two nonempty path segments")
	}
	for _, r := range s {
		if r <= 0x20 || r == 0x7F || unicode.IsControl(r) {
			return invalidExternal("repo contains whitespace or control characters")
		}
		if strings.ContainsRune(`\/:?#@[]%`, r) {
			return invalidExternal("repo contains a URL delimiter")
		}
	}
	return nil
}

func parseRepo(repo string) (owner, name string, err error) {
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || strings.Contains(name, "/") {
		return "", "", invalidExternal("repo must have two nonempty path segments")
	}
	if err := validRepoSegment(owner); err != nil {
		return "", "", err
	}
	if err := validRepoSegment(name); err != nil {
		return "", "", err
	}
	return owner, name, nil
}

func validateExternalURL(raw, owner, name string, id int64) error {
	u, err := url.Parse(raw)
	if err != nil || !u.IsAbs() {
		return invalidExternal("url must be an absolute HTTP(S) URL")
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return invalidExternal("url must be an absolute HTTP(S) URL")
	}
	if u.Host == "" {
		return invalidExternal("url must include a host")
	}
	if u.User != nil {
		return invalidExternal("url must not include userinfo")
	}
	if u.RawQuery != "" || u.ForceQuery {
		return invalidExternal("url must not include a query")
	}
	if u.Fragment != "" || strings.Contains(raw, "#") {
		return invalidExternal("url must not include a fragment")
	}
	escaped := u.EscapedPath()
	lower := strings.ToLower(escaped)
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") {
		return invalidExternal("url path contains an encoded separator")
	}
	if strings.Contains(lower, "%2e") {
		return invalidExternal("url path contains an encoded dot")
	}
	path := u.Path
	if path == "" || path[0] != '/' {
		return invalidExternal("url path must end in /owner/repo/issues/id")
	}
	segs := strings.Split(path, "/")
	for _, seg := range segs {
		if seg == "." || seg == ".." {
			return invalidExternal("url path contains a dot segment")
		}
	}
	if segs[len(segs)-1] == "" || len(segs) < 5 {
		return invalidExternal("url path must end in /owner/repo/issues/id")
	}
	want := []string{owner, name, "issues", strconv.FormatInt(id, 10)}
	got := segs[len(segs)-4:]
	for i := range want {
		if got[i] != want[i] {
			return invalidExternal("url path must end in /owner/repo/issues/id")
		}
	}
	return nil
}

// ValidateExternal reports whether e is a complete, well-formed Gitea mapping.
func ValidateExternal(e External) error {
	if e.Tracker != "gitea" {
		return invalidExternal("tracker must be gitea")
	}
	owner, name, err := parseRepo(e.Repo)
	if err != nil {
		return err
	}
	if e.ID <= 0 {
		return invalidExternal("id must be a positive integer")
	}
	return validateExternalURL(e.URL, owner, name, e.ID)
}

func scalarString(n *yaml.Node, name string) (string, error) {
	if n.Kind != yaml.ScalarNode {
		return "", invalidExternal(name + " must be a scalar")
	}
	switch n.ShortTag() {
	case "!!int", "!!float", "!!bool", "!!null", "!!seq", "!!map":
		return "", invalidExternal(name + " must be a string")
	}
	return n.Value, nil
}

func parseExternalID(n *yaml.Node) (int64, error) {
	if n.Kind != yaml.ScalarNode {
		return 0, invalidExternal("id must be a scalar")
	}
	switch n.ShortTag() {
	case "!!float", "!!bool", "!!null", "!!seq", "!!map":
		return 0, invalidExternal("id must be a positive integer")
	}
	if strings.ContainsAny(n.Value, ".eE+") {
		return 0, invalidExternal("id must be a positive integer")
	}
	id, err := strconv.ParseInt(n.Value, 10, 64)
	if err != nil {
		return 0, invalidExternal("id must be a positive integer")
	}
	return id, nil
}

func parseExternalNode(n *yaml.Node) (*External, string) {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil, invalidExternal("expected a mapping").Error()
	}
	type hit struct {
		node *yaml.Node
		n    int
	}
	seen := map[string]*hit{}
	for i := 0; i+1 < len(n.Content); i += 2 {
		k := n.Content[i].Value
		h := seen[k]
		if h == nil {
			h = &hit{}
			seen[k] = h
		}
		h.node = n.Content[i+1]
		h.n++
	}
	for _, name := range []string{"tracker", "repo", "id", "url"} {
		h := seen[name]
		if h == nil {
			return nil, invalidExternal("missing " + name).Error()
		}
		if h.n > 1 {
			return nil, invalidExternal("duplicate key " + name).Error()
		}
	}
	tracker, err := scalarString(seen["tracker"].node, "tracker")
	if err != nil {
		return nil, err.Error()
	}
	repo, err := scalarString(seen["repo"].node, "repo")
	if err != nil {
		return nil, err.Error()
	}
	id, err := parseExternalID(seen["id"].node)
	if err != nil {
		return nil, err.Error()
	}
	u, err := scalarString(seen["url"].node, "url")
	if err != nil {
		return nil, err.Error()
	}
	ext := External{Tracker: tracker, Repo: repo, ID: id, URL: u}
	if err := ValidateExternal(ext); err != nil {
		return nil, err.Error()
	}
	cp := ext
	return &cp, ""
}

func newExternalMapping(e *External) *yaml.Node {
	return &yaml.Node{
		Kind: yaml.MappingNode,
		Tag:  "!!map",
		Content: []*yaml.Node{
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "tracker"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: e.Tracker},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "repo"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: e.Repo},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "id"},
			{Kind: yaml.ScalarNode, Tag: "!!int", Value: strconv.FormatInt(e.ID, 10)},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "url"},
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: e.URL},
		},
	}
}

func (it *Item) setExternalNode(e *External) {
	_, val, idx := it.findKey("external")
	if idx < 0 {
		it.doc.Content = append(it.doc.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: "external"},
			newExternalMapping(e),
		)
		return
	}
	if val.Kind != yaml.MappingNode {
		*val = *newExternalMapping(e)
		return
	}
	setOwned := func(key, tag, value string) {
		for i := 0; i+1 < len(val.Content); i += 2 {
			if val.Content[i].Value == key {
				n := val.Content[i+1]
				n.Kind = yaml.ScalarNode
				n.Tag = tag
				n.Value = value
				n.Content = nil
				return
			}
		}
		val.Content = append(val.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
			&yaml.Node{Kind: yaml.ScalarNode, Tag: tag, Value: value},
		)
	}
	setOwned("tracker", "!!str", e.Tracker)
	setOwned("repo", "!!str", e.Repo)
	setOwned("id", "!!int", strconv.FormatInt(e.ID, 10))
	setOwned("url", "!!str", e.URL)
}

// SetExternal writes a validated Gitea mapping, or removes the field when e
// is nil. An identical mapping is a no-op. Extra nested keys are retained.
func (it *Item) SetExternal(e *External) error {
	if e == nil {
		_, _, idx := it.findKey("external")
		if idx < 0 && it.External == nil && it.ExternalProblem == "" {
			return nil
		}
		it.External = nil
		it.ExternalProblem = ""
		it.deleteKey("external")
		it.markDirty()
		return nil
	}
	if err := ValidateExternal(*e); err != nil {
		return err
	}
	if it.External != nil && *it.External == *e {
		return nil
	}
	cp := *e
	it.External = &cp
	it.ExternalProblem = ""
	it.setExternalNode(&cp)
	it.markDirty()
	return nil
}
