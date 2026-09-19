package cli

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/eisenwinter/awit/pkg/format"
)

const unknownLabelsAdvisorySuffix = " (declare them in .awit/config.yaml labels)\n"

func unknownLabelsAdvisory(list string) string {
	return "warning: unknown labels: " + list + unknownLabelsAdvisorySuffix
}

func writeVocab(t *testing.T, repo, yaml string) {
	t.Helper()
	writeDefaultLabels(t, repo, []byte(yaml))
}

func TestUnknownLabelsCreateDefaults(t *testing.T) {
	dir := initRepo(t)
	writeVocab(t, dir, "prefix: AWIT\ndefault_labels: [phase1]\nlabels: [p1]\nstale_claim: 2h\n")
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "A test item.", "Defaults")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stderr != unknownLabelsAdvisory("phase1") {
		t.Fatalf("stderr = %q, want %q", stderr, unknownLabelsAdvisory("phase1"))
	}
	it := onlyItem(t, dir)
	if strings.Join(it.Labels, ",") != "phase1" {
		t.Fatalf("labels = %v, want [phase1] stored", it.Labels)
	}
}

func TestUnknownLabelsCreateJSONStdout(t *testing.T) {
	dir := initRepo(t)
	writeVocab(t, dir, "prefix: AWIT\nlabels: [p1]\nstale_claim: 2h\n")
	code, stdout, stderr := run(t, "--repo", dir, "--format", "json", "create", "--brief", "A test item.", "-l", "typo", "Test")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stderr != unknownLabelsAdvisory("typo") {
		t.Fatalf("stderr = %q, want %q", stderr, unknownLabelsAdvisory("typo"))
	}
	var e format.Entry
	if err := json.Unmarshal([]byte(stdout), &e); err != nil {
		t.Fatalf("json stdout corrupted: %v (stdout %q)", err, stdout)
	}
	if strings.Join(e.Labels, ",") != "typo" {
		t.Fatalf("entry labels = %v", e.Labels)
	}
	it := onlyItem(t, dir)
	if strings.Join(it.Labels, ",") != "typo" {
		t.Fatalf("stored labels = %v, want [typo]", it.Labels)
	}
}

func TestUnknownLabelsUpdateNewAdditions(t *testing.T) {
	dir := initRepo(t)
	writeVocab(t, dir, "prefix: AWIT\nlabels: [p1]\nstale_claim: 2h\n")
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", []string{"p1"})
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "-l", "auth")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stderr != unknownLabelsAdvisory("auth") {
		t.Fatalf("stderr = %q, want %q", stderr, unknownLabelsAdvisory("auth"))
	}
	if g := strings.Join(readItem(t, dir, "AWIT-TEST0001").Labels, ","); g != "p1,auth" {
		t.Fatalf("labels = %s", g)
	}
}

func TestUnknownLabelsUpdateAddThenRemove(t *testing.T) {
	dir := initRepo(t)
	writeVocab(t, dir, "prefix: AWIT\nlabels: [p1]\nstale_claim: 2h\n")
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", nil)
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "-l", "auth", "--unlabel", "auth")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "unknown labels") {
		t.Fatalf("add-then-remove warned: %q", stderr)
	}
	if g := strings.Join(readItem(t, dir, "AWIT-TEST0001").Labels, ","); g != "" {
		t.Fatalf("labels = %q, want empty", g)
	}
}

func TestUnknownLabelsUpdateRepeatedExisting(t *testing.T) {
	dir := initRepo(t)
	writeVocab(t, dir, "prefix: AWIT\nlabels: [p1]\nstale_claim: 2h\n")
	seedItem(t, dir, "AWIT-TEST0001", "T", "B.", []string{"typo"})
	code, _, stderr := run(t, "--repo", dir, "update", "AWIT-TEST0001", "--title", "New")
	if code != 0 {
		t.Fatalf("title exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "unknown labels") {
		t.Fatalf("unrelated update warned: %q", stderr)
	}
	code, _, stderr = run(t, "--repo", dir, "update", "AWIT-TEST0001", "-l", "typo")
	if code != 0 {
		t.Fatalf("re-add exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "unknown labels") {
		t.Fatalf("re-adding existing unknown label warned: %q", stderr)
	}
}

func TestUnknownLabelsDisabledVocabulary(t *testing.T) {
	dir := initRepo(t)
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "A test item.", "-l", "typo", "Test")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "unknown labels") {
		t.Fatalf("disabled vocabulary warned: %q", stderr)
	}
	if g := strings.Join(onlyItem(t, dir).Labels, ","); g != "typo" {
		t.Fatalf("labels = %s", g)
	}
}

func TestUnknownLabelsExactCase(t *testing.T) {
	dir := initRepo(t)
	writeVocab(t, dir, "prefix: AWIT\nlabels: [p1]\nstale_claim: 2h\n")
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "A test item.", "-l", "P1", "Case")
	if code != 0 {
		t.Fatalf("exit %d stderr %q", code, stderr)
	}
	if stderr != unknownLabelsAdvisory("P1") {
		t.Fatalf("stderr = %q, want %q", stderr, unknownLabelsAdvisory("P1"))
	}
	if g := strings.Join(onlyItem(t, dir).Labels, ","); g != "P1" {
		t.Fatalf("labels = %s, want P1 stored as-is", g)
	}
}

func TestUnknownLabelsFailedMutation(t *testing.T) {
	dir := initRepo(t)
	writeVocab(t, dir, "prefix: AWIT\nlabels: [p1]\nstale_claim: 2h\n")
	code, _, stderr := run(t, "--repo", dir, "create", "--brief", "A test item.", "-l", "typo")
	if code != 1 {
		t.Fatalf("exit %d, want 1 stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "unknown labels") {
		t.Fatalf("failed create warned: %q", stderr)
	}
	code, _, stderr = run(t, "--repo", dir, "update", "AWIT-TEST0001", "-l", "typo")
	if code != 1 {
		t.Fatalf("update exit %d, want 1 stderr %q", code, stderr)
	}
	if strings.Contains(stderr, "unknown labels") {
		t.Fatalf("failed update warned: %q", stderr)
	}
}
