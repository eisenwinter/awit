// Package skill renders the driving-awit skill for the agent tools awit
// knows how to seed, and reports which of them a repository uses.
//
// The body is one embedded asset shared by every tool; only the frontmatter
// is per-target, because that is the only part the tools disagree about.
// Today they do not disagree at all (see Targets), but the seam is kept so a
// future dialect costs one string instead of a second copy of the skill.
package skill

import (
	_ "embed"
	"os"
	"path/filepath"
)

// Name is the skill's directory name. Every tool below requires the `name`
// in the frontmatter to equal the directory holding SKILL.md, so this value
// is both.
const Name = "driving-awit"

//go:embed assets/driving-awit.body.md
var body string

//go:embed assets/driving-awit.frontmatter.yaml
var defaultFrontmatter string

// Target is one agent tool awit knows how to seed. Dir is repo-root
// relative; Path is relative to Dir. Frontmatter is the complete YAML block
// including both --- fences, because the dialect is the part that differs
// per tool.
type Target struct {
	Dir         string
	Path        string
	Frontmatter string
}

// Targets returns every known target in a fixed order. The order is fixed so
// that prompts, output and tests are deterministic.
//
// All five currently take the same frontmatter: each requires `name` and
// `description` and ignores unknown keys, so one block satisfies them all.
// That is a finding, not an assumption - it was checked against each tool's
// own documentation, and Frontmatter stays per-target so a divergence later
// is a one-line change.
//
// Note these tools overlap on purpose: opencode also reads .claude/skills
// and .agents/skills, and pi also reads .agents/skills. Seeding two
// directories in one repository can therefore hand the same tool the same
// skill twice. awit asks per directory rather than guessing, so the choice
// stays with whoever runs init.
func Targets() []Target {
	rel := filepath.Join("skills", Name, "SKILL.md")
	dirs := []string{".claude", ".omp", ".opencode", ".agents", ".pi"}
	out := make([]Target, 0, len(dirs))
	for _, d := range dirs {
		out = append(out, Target{Dir: d, Path: rel, Frontmatter: defaultFrontmatter})
	}
	return out
}

// Detect returns the targets whose Dir exists under repoRoot as a directory,
// in Targets order. A plain file sharing the name is not a match.
func Detect(repoRoot string) []Target {
	var out []Target
	for _, t := range Targets() {
		fi, err := os.Stat(filepath.Join(repoRoot, t.Dir))
		if err != nil || !fi.IsDir() {
			continue
		}
		out = append(out, t)
	}
	return out
}

// Render returns the full SKILL.md bytes for t: its frontmatter block
// followed by the shared body. Deterministic.
func Render(t Target) []byte {
	return []byte(t.Frontmatter + body)
}
