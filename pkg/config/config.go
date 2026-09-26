package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
	"unicode"

	"gopkg.in/yaml.v3"
)

const FileName = "config.yaml"

type Config struct {
	Prefix        string   `yaml:"prefix"`
	DefaultLabels []string `yaml:"default_labels,omitempty"`
	Labels        []string `yaml:"labels,omitempty"` // advisory vocabulary; empty disables warnings
	StaleClaim    Duration `yaml:"stale_claim"`      // default 2h
	AgentID       string   `yaml:"agent_id,omitempty"`
	// Commit is the repository default for `next --claim` git commits.
	// nil (the key is absent) means the documented default: commit. Only
	// next --claim reads it; no other command's behaviour depends on it.
	Commit *bool `yaml:"commit,omitempty"`
	// ExternalPush is the repository default for automatic linked-issue
	// state pushes from close, release, and explicit update --status.
	// nil (the key is absent) means the documented default: push. It is
	// independent of Commit and does not govern push-body, import, or check.
	ExternalPush *bool `yaml:"external_push,omitempty"`
	// Template is a repo-root-relative forward-slash path to a body-only
	// file used by create. Empty means the built-in skeleton. Only create
	// reads the file.
	Template string `yaml:"template,omitempty"`
}

type Duration time.Duration

// String renders d the way Write emits it: "0s", whole hours as "2h",
// whole minutes as "90m", otherwise time.Duration's form ("1m30s").
func (d Duration) String() string {
	td := time.Duration(d)
	switch {
	case td == 0:
		return "0s"
	case td%time.Hour == 0:
		return fmt.Sprintf("%dh", td/time.Hour)
	case td%time.Minute == 0:
		return fmt.Sprintf("%dm", td/time.Minute)
	default:
		return td.String()
	}
}

func (d Duration) MarshalYAML() (any, error) { return d.String(), nil }

var prefixRE = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,7}$`)

// ValidPrefix reports whether p matches the id prefix grammar
// ^[A-Z][A-Z0-9]{1,7}$ that awit init enforces (2-8 uppercase
// alphanumerics starting with a letter). Load does not call it: existing
// repositories keep loading whatever prefix they were initialised with.
func ValidPrefix(p string) bool { return prefixRE.MatchString(p) }

func (d *Duration) UnmarshalYAML(n *yaml.Node) error {
	var s string
	if err := n.Decode(&s); err != nil {
		return err
	}
	parsed, err := time.ParseDuration(s)
	if err != nil {
		return err
	}
	*d = Duration(parsed)
	return nil
}

func Default(prefix string) Config {
	return Config{
		Prefix:     prefix,
		StaleClaim: Duration(2 * time.Hour),
	}
}

// Normalize applies Load's post-decode rules to a copy of c and returns
// it: prefix required, zero stale_claim → 2h, template path checks, labels
// entry rules with in-memory dedupe. default_labels are not checked.
func (c Config) Normalize() (Config, error) {
	if c.Prefix == "" {
		return Config{}, errors.New("config: prefix is required")
	}
	if c.StaleClaim == 0 {
		c.StaleClaim = Duration(2 * time.Hour)
	}
	if err := validateTemplatePath(c.Template); err != nil {
		return Config{}, err
	}
	labels, err := normalizeLabels(c.Labels)
	if err != nil {
		return Config{}, err
	}
	c.Labels = labels
	return c, nil
}

// Load reads awitDir/config.yaml: yaml.Unmarshal followed by Normalize.
func Load(awitDir string) (Config, error) {
	data, err := os.ReadFile(filepath.Join(awitDir, FileName))
	if err != nil {
		return Config{}, err
	}
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return Config{}, err
	}
	return c.Normalize()
}

func (c Config) Write(awitDir string) error {
	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(c); err != nil {
		return err
	}
	if err := enc.Close(); err != nil {
		return err
	}
	return WriteAtomic(filepath.Join(awitDir, FileName), buf.Bytes())
}

func (c Config) Agent(flag string) string {
	if flag != "" {
		return flag
	}
	if v := os.Getenv("AWIT_AGENT"); v != "" {
		return v
	}
	return c.AgentID
}

// ShouldCommit reports the config-only answer to "does a claim commit?":
// an absent commit key commits, only an explicit commit: false skips.
// Per-invocation flags override this in the claim path; the policy never
// controls pushing or any other command.
func (c Config) ShouldCommit() bool {
	return c.Commit == nil || *c.Commit
}

// ShouldPushExternal reports the config-only answer to "does a status
// mutation push to the linked issue?": an absent external_push key
// pushes, only an explicit external_push: false skips. Per-invocation
// --push / a true --no-push override this in close, release, and
// update --status; the policy never controls claim commits, push-body,
// import, or check.
func (c Config) ShouldPushExternal() bool {
	return c.ExternalPush == nil || *c.ExternalPush
}

func validateTemplatePath(p string) error {
	if p == "" {
		return nil
	}
	if strings.Contains(p, "\\") || path.IsAbs(p) || windowsAbs(p) {
		return errors.New("config: template must be a repo-root-relative path")
	}
	cleaned := path.Clean(p)
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return errors.New("config: template escapes repository root")
	}
	return nil
}

func windowsAbs(p string) bool {
	if strings.HasPrefix(p, "//") {
		return true
	}
	if len(p) < 2 || p[1] != ':' {
		return false
	}
	c := p[0]
	if (c < 'A' || c > 'Z') && (c < 'a' || c > 'z') {
		return false
	}
	return len(p) == 2 || p[2] == '/'
}

func normalizeLabels(in []string) ([]string, error) {
	if len(in) == 0 {
		return in, nil
	}
	seen := make(map[string]bool, len(in))
	out := make([]string, 0, len(in))
	for _, s := range in {
		if s == "" {
			return nil, errors.New("config: labels entry must be nonempty")
		}
		if strings.TrimSpace(s) != s {
			return nil, errors.New("config: labels entry must not have leading or trailing whitespace")
		}
		for _, r := range s {
			if unicode.IsControl(r) {
				return nil, errors.New("config: labels entry must not contain control characters")
			}
		}
		if seen[s] {
			continue
		}
		seen[s] = true
		out = append(out, s)
	}
	return out, nil
}
