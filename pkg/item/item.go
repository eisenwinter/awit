package item

import (
	"bytes"
	"fmt"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// Item is one Markdown work item under .awit/items/.
//
// Known frontmatter keys map to the exported fields below. Unknown keys are
// kept on the unexported YAML mapping node and survive Bytes() after any
// setter. The key "external" is reserved for a future GitLab/GitHub mirror
// (value form "external: <provider>#<number>", e.g. gitlab#42) and is never
// read in v1 — do not add an External field.
type Item struct {
	ID        string
	Title     string
	Brief     string
	Status    Status
	Deps      []string
	Labels    []string
	Assignee  string
	ClaimedAt *time.Time
	Refs      []string
	Path      string

	doc  *yaml.Node
	body []byte
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
	it := &Item{Path: path, doc: doc, body: body}
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
		case "refs":
			it.Refs = seqStrings(v)
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
	return it, nil
}

func (it *Item) Bytes() ([]byte, error) {
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
			{Kind: yaml.ScalarNode, Tag: "!!str", Value: "refs"},
			seqNode(nil, 0),
		},
	}
	return &Item{
		ID:     id,
		Title:  title,
		Brief:  brief,
		Status: StatusOpen,
		Deps:   depsCopy,
		Labels: labelsCopy,
		Refs:   []string{},
		doc:    doc,
		body:   []byte("\n## Summary\n\n## Acceptance Criteria\n\n"),
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

func (it *Item) SetTitle(s string) {
	it.Title = s
	it.setScalar("title", s)
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
}

func (it *Item) SetStatus(s Status) {
	it.Status = s
	it.setScalar("status", string(s))
}

func (it *Item) SetAssignee(s string) {
	it.Assignee = s
	if s == "" {
		it.deleteKey("assignee")
		return
	}
	it.setScalar("assignee", s)
}

func (it *Item) SetClaimedAt(t *time.Time) {
	if t == nil {
		it.ClaimedAt = nil
		it.deleteKey("claimed_at")
		return
	}
	cp := t.UTC().Truncate(time.Second)
	it.ClaimedAt = &cp
	it.setScalar("claimed_at", cp.Format(time.RFC3339))
}

func (it *Item) SetDeps(v []string) {
	it.Deps = append([]string(nil), v...)
	it.setSeq("deps", v, yaml.FlowStyle)
}

func (it *Item) SetLabels(v []string) {
	it.Labels = append([]string(nil), v...)
	it.setSeq("labels", v, yaml.FlowStyle)
}

func (it *Item) SetRefs(v []string) {
	it.Refs = append([]string(nil), v...)
	it.setSeq("refs", v, 0)
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
