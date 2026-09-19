// Command glabstub is a fake `glab` CLI used by awit's tests. It never talks
// to a network; every response is scripted through files in GLAB_STUB_DIR:
//
//	config-<key>                  value printed by `config get <key> --host <h>`
//	<endpoint>.status             HTTP status code for an api call (default 200)
//	<endpoint>.json               body for an api call (default {})
//
// <endpoint> is the last api argument with "/" replaced by "_", e.g.
// `user` or `projects_group%2Fsub%2Fproject_issues_127`. The project
// parameter is kept verbatim: the stub never decodes %2F, so tests prove
// the wrapper sends one encoded project segment. Every invocation appends
// one JSON array of its argv to GLAB_STUB_DIR/argv.log.
//
// Like real glab 1.118.0, the --include status block goes to stdout and the
// bare response body follows it. HTTP failures exit nonzero by default. A
// PUT with `-F description=@<file>` stores the raw file bytes (no LF
// adaptation) into <endpoint>.json like GitLab would, and answers with the
// patched issue. A PUT with `-f state_event=<close|reopen>` stores the
// decoded state the same way.
//
// Environment knobs:
//
//	GLAB_STUB_CONFIG_EXIT=N      force exit code N for config get calls
//	GLAB_STUB_API_EXIT=N         force exit code N for api calls (after printing)
//	GLAB_STUB_API_STDERR=s       print s on stderr for api calls
//	GLAB_STUB_PATCH_NO_STORE=1   acknowledge the PUT but never store the change
//	GLAB_STUB_PATCH_KEEP_RESPONSE=1  keep a pre-scripted <endpoint>.patch-response
//	GLAB_STUB_PATCH_OMIT_STATE=1 answer the PUT without the state field
//	GLAB_STUB_PATCH_WRONG_IID=1  answer the PUT with iid+1
//	GLAB_STUB_NO_INCLUDE=1       print the body without the --include status block
package main

import (
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func main() {
	dir := os.Getenv("GLAB_STUB_DIR")
	args := os.Args[1:]
	if dir != "" {
		logArgv(dir, args)
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "stub: no command")
		os.Exit(1)
	}
	switch args[0] {
	case "config":
		configCmd(dir, args[1:])
	case "api":
		api(dir, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "stub: unknown command %q\n", args[0])
		os.Exit(1)
	}
}

// configCmd implements `config get <key> --host <h>`: it prints the scripted
// value for key, honouring GITLAB_SUBFOLDER for subfolder like the pinned
// glab release does. An unset key prints nothing with exit 0.
func configCmd(dir string, args []string) {
	if s := os.Getenv("GLAB_STUB_CONFIG_EXIT"); s != "" {
		if n, err := strconv.Atoi(s); err == nil {
			os.Exit(n)
		}
		os.Exit(1)
	}
	if len(args) < 2 || args[0] != "get" {
		fmt.Fprintln(os.Stderr, "stub: config needs `get <key>`")
		os.Exit(1)
	}
	key := args[1]
	if key == "subfolder" {
		if v := os.Getenv("GITLAB_SUBFOLDER"); v != "" {
			fmt.Print(v)
			return
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "config-"+key)); err == nil {
		fmt.Print(string(b))
	}
}

func api(dir string, args []string) {
	hostname := ""
	method := "GET"
	var fileFields []string
	var plainFields []string
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--hostname" && i+1 < len(args):
			hostname = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--hostname="):
			hostname = strings.TrimPrefix(args[i], "--hostname=")
		case args[i] == "--method" && i+1 < len(args):
			method = args[i+1]
			i++
		case strings.HasPrefix(args[i], "--method="):
			method = strings.TrimPrefix(args[i], "--method=")
		case args[i] == "-X" && i+1 < len(args):
			method = args[i+1]
			i++
		case args[i] == "-F" && i+1 < len(args):
			fileFields = append(fileFields, args[i+1])
			i++
		case args[i] == "-f" && i+1 < len(args):
			plainFields = append(plainFields, args[i+1])
			i++
		}
	}
	_ = hostname
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "stub: api needs an endpoint")
		os.Exit(1)
	}
	endpoint := args[len(args)-1]
	key := strings.ReplaceAll(endpoint, "/", "_")
	status := "200"
	if b, err := os.ReadFile(filepath.Join(dir, key+".status")); err == nil {
		status = strings.TrimSpace(string(b))
	}
	if s := os.Getenv("GLAB_STUB_API_EXIT"); s != "" {
		// A forced process failure never reaches HTTP: no status block,
		// only diagnostics, like a glab that cannot reach its server.
		if es := os.Getenv("GLAB_STUB_API_STDERR"); es != "" {
			fmt.Fprintln(os.Stderr, es)
		}
		n, err := strconv.Atoi(s)
		if err != nil {
			n = 1
		}
		os.Exit(n)
	}
	if strings.HasPrefix(status, "2") {
		for _, f := range fileFields {
			k, v, ok := strings.Cut(f, "=")
			if !ok || k != "description" || !strings.HasPrefix(v, "@") {
				continue
			}
			data, err := os.ReadFile(v[1:])
			if err != nil {
				fmt.Fprintf(os.Stderr, "stub: read %s: %v\n", v, err)
				os.Exit(1)
			}
			// glab sends -F file values byte-exact: no adaptation.
			applyDescriptionPatch(dir, key, endpoint, string(data))
		}
		for _, f := range plainFields {
			k, v, ok := strings.Cut(f, "=")
			if !ok || k != "state_event" {
				continue
			}
			applyStatePatch(dir, key, endpoint, v)
		}
	}
	body := "{}"
	if b, err := os.ReadFile(filepath.Join(dir, key+".json")); err == nil {
		body = string(b)
	}
	if method == "PUT" {
		if b, err := os.ReadFile(filepath.Join(dir, key+".patch-response")); err == nil {
			body = string(b)
		}
	}
	if os.Getenv("GLAB_STUB_NO_INCLUDE") == "" {
		fmt.Fprintf(os.Stdout, "HTTP/1.1 %s STUB\nContent-Type: application/json; charset=utf-8\n\n", status)
	}
	if s := os.Getenv("GLAB_STUB_API_STDERR"); s != "" {
		fmt.Fprintln(os.Stderr, s)
	}
	fmt.Print(body)
	if !strings.HasPrefix(status, "2") {
		fmt.Fprintf(os.Stderr, "glab: stub request failed for %s (HTTP %s)\n", endpoint, status)
		os.Exit(1)
	}
}

// applyDescriptionPatch mimics a GitLab issue PUT: the raw decoded
// description becomes the stored issue description (<key>.json) and the PUT
// response (<key>.patch-response), unless a knob says otherwise.
func applyDescriptionPatch(dir, key, endpoint, decoded string) {
	stored := map[string]any{}
	if b, err := os.ReadFile(filepath.Join(dir, key+".json")); err == nil {
		if err := json.Unmarshal(b, &stored); err != nil {
			fmt.Fprintf(os.Stderr, "stub: %s.json: %v\n", key, err)
			os.Exit(1)
		}
	}
	if _, ok := stored["iid"]; !ok {
		tail := endpoint[strings.LastIndex(endpoint, "/")+1:]
		n, err := strconv.Atoi(tail)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stub: no issue iid in %q\n", endpoint)
			os.Exit(1)
		}
		stored["iid"] = float64(n)
	}
	patched := map[string]any{}
	for k, v := range stored {
		patched[k] = v
	}
	patched["description"] = decoded
	resp := patched
	if os.Getenv("GLAB_STUB_PATCH_NO_STORE") == "" {
		stored = patched
	} else {
		// The server acknowledges but keeps the old description.
		resp = stored
	}
	if os.Getenv("GLAB_STUB_PATCH_WRONG_IID") == "1" {
		resp = maps.Clone(resp)
		if n, ok := stored["iid"].(float64); ok {
			resp["iid"] = n + 1
		}
	}
	if os.Getenv("GLAB_STUB_PATCH_OMIT_DESCRIPTION") == "1" {
		resp = maps.Clone(resp)
		delete(resp, "description")
	}
	if os.Getenv("GLAB_STUB_PATCH_KEEP_RESPONSE") == "" {
		writeJSONFile(dir, key+".patch-response", resp)
	}
	writeJSONFile(dir, key+".json", stored)
}

// applyStatePatch mimics a GitLab issue state-event PUT: the decoded event
// maps to the stored issue state (<key>.json) and the PUT response
// (<key>.patch-response), unless a knob says otherwise.
func applyStatePatch(dir, key, endpoint, event string) {
	state := event
	switch event {
	case "close":
		state = "closed"
	case "reopen":
		state = "opened"
	}
	stored := map[string]any{}
	if b, err := os.ReadFile(filepath.Join(dir, key+".json")); err == nil {
		if err := json.Unmarshal(b, &stored); err != nil {
			fmt.Fprintf(os.Stderr, "stub: %s.json: %v\n", key, err)
			os.Exit(1)
		}
	}
	if _, ok := stored["iid"]; !ok {
		tail := endpoint[strings.LastIndex(endpoint, "/")+1:]
		n, err := strconv.Atoi(tail)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stub: no issue iid in %q\n", endpoint)
			os.Exit(1)
		}
		stored["iid"] = float64(n)
	}
	patched := map[string]any{}
	for k, v := range stored {
		patched[k] = v
	}
	patched["state"] = state
	resp := patched
	if os.Getenv("GLAB_STUB_PATCH_NO_STORE") == "" {
		stored = patched
	} else {
		// The server acknowledges but keeps the old state.
		resp = stored
	}
	if os.Getenv("GLAB_STUB_PATCH_WRONG_IID") == "1" {
		resp = maps.Clone(resp)
		if n, ok := stored["iid"].(float64); ok {
			resp["iid"] = n + 1
		}
	}
	if os.Getenv("GLAB_STUB_PATCH_OMIT_STATE") == "1" {
		resp = maps.Clone(resp)
		delete(resp, "state")
	}
	if os.Getenv("GLAB_STUB_PATCH_KEEP_RESPONSE") == "" {
		writeJSONFile(dir, key+".patch-response", resp)
	}
	writeJSONFile(dir, key+".json", stored)
}

func writeJSONFile(dir, name string, v map[string]any) {
	b, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stub: marshal %s: %v\n", name, err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "stub: %v\n", err)
		os.Exit(1)
	}
}

func logArgv(dir string, args []string) {
	f, err := os.OpenFile(filepath.Join(dir, "argv.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	b, err := json.Marshal(args)
	if err != nil {
		return
	}
	fmt.Fprintln(f, string(b))
}
