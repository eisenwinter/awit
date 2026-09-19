// Command teastub is a fake `tea` CLI used by awit's tests. It never talks
// to a network; every response is scripted through files in TEA_STUB_DIR:
//
//	logins.json                     body printed by `login list --output json`
//	<endpoint>.status               HTTP status code for an api call (default 200)
//	<endpoint>.json                 body for an api call (default {})
//
// <endpoint> is the last api argument with "/" replaced by "_", e.g.
// `user` or `repos_owner_repo_issues_127`. Every invocation appends one
// JSON array of its argv to TEA_STUB_DIR/argv.log.
//
// Like real tea, the --include status block goes to stderr and the bare
// response body to stdout. A PATCH with `-F body=@<file>` mimics tea's
// reader (strips exactly one terminal LF), stores the decoded body into
// <endpoint>.json like Gitea would, and answers with the patched issue. A
// PATCH with `-f state=<open|closed>` stores the decoded state the same
// way.
//
// Environment knobs:
//
//	TEA_STUB_NO_API=1           behave like a tea too old for the api subcommand
//	TEA_STUB_API_EXIT=N         force exit code N for api calls (after printing)
//	TEA_STUB_API_STDERR=s       print s on stderr for api calls
//	TEA_STUB_PATCH_NO_STORE=1   acknowledge the PATCH but never store the body/state
//	TEA_STUB_PATCH_OMIT_BODY=1  answer the PATCH without the body field
//	TEA_STUB_PATCH_OMIT_STATE=1 answer the PATCH without the state field
//	TEA_STUB_PATCH_WRONG_NUMBER=1  answer the PATCH with issue number+1
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
	dir := os.Getenv("TEA_STUB_DIR")
	args := os.Args[1:]
	if dir != "" {
		logArgv(dir, args)
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "stub: no command")
		os.Exit(1)
	}
	switch args[0] {
	case "login":
		data, err := os.ReadFile(filepath.Join(dir, "logins.json"))
		if err != nil {
			fmt.Fprintln(os.Stderr, "stub: logins.json: not configured")
			os.Exit(1)
		}
		fmt.Print(string(data))
	case "api":
		api(dir, args[1:])
	default:
		fmt.Fprintf(os.Stderr, "stub: unknown command %q\n", args[0])
		os.Exit(1)
	}
}

func api(dir string, args []string) {
	if os.Getenv("TEA_STUB_NO_API") == "1" {
		fmt.Fprintln(os.Stderr, "No help topic for 'api'")
		os.Exit(1)
	}
	for _, a := range args {
		if a == "--help" || a == "-h" {
			fmt.Println("Query the Gitea API")
			return
		}
	}
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "stub: api needs an endpoint")
		os.Exit(1)
	}
	method := "GET"
	var typedFields []string
	var plainFields []string
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-X":
			if i+1 < len(args) {
				method = args[i+1]
				i++
			}
		case "-F":
			if i+1 < len(args) {
				typedFields = append(typedFields, args[i+1])
				i++
			}
		case "-f":
			if i+1 < len(args) {
				plainFields = append(plainFields, args[i+1])
				i++
			}
		}
	}
	endpoint := args[len(args)-1]
	key := strings.ReplaceAll(endpoint, "/", "_")
	status := "200"
	if b, err := os.ReadFile(filepath.Join(dir, key+".status")); err == nil {
		status = strings.TrimSpace(string(b))
	}
	if strings.HasPrefix(status, "2") {
		for _, f := range typedFields {
			k, v, ok := strings.Cut(f, "=")
			if !ok || k != "body" || !strings.HasPrefix(v, "@") {
				continue
			}
			data, err := os.ReadFile(v[1:])
			if err != nil {
				fmt.Fprintf(os.Stderr, "stub: read %s: %v\n", v, err)
				os.Exit(1)
			}
			// tea's -F key=@file reader strips exactly one terminal LF.
			applyPatch(dir, key, endpoint, strings.TrimSuffix(string(data), "\n"))
		}
		for _, f := range plainFields {
			k, v, ok := strings.Cut(f, "=")
			if !ok || k != "state" {
				continue
			}
			applyStatePatch(dir, key, endpoint, v)
		}
	}
	body := "{}"
	if b, err := os.ReadFile(filepath.Join(dir, key+".json")); err == nil {
		body = string(b)
	}
	if method == "PATCH" {
		if b, err := os.ReadFile(filepath.Join(dir, key+".patch-response")); err == nil {
			body = string(b)
		}
	}
	fmt.Fprintf(os.Stderr, "HTTP/1.1 %s STUB\nContent-Type: application/json; charset=utf-8\n\n", status)
	if s := os.Getenv("TEA_STUB_API_STDERR"); s != "" {
		fmt.Fprintln(os.Stderr, s)
	}
	fmt.Print(body)
	if s := os.Getenv("TEA_STUB_API_EXIT"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			n = 1
		}
		os.Exit(n)
	}
}

// applyPatch mimics a Gitea issue PATCH: the decoded body becomes the
// stored issue body (<key>.json) and the PATCH response
// (<key>.patch-response), unless a knob says otherwise.
func applyPatch(dir, key, endpoint, decoded string) {
	stored := map[string]any{}
	if b, err := os.ReadFile(filepath.Join(dir, key+".json")); err == nil {
		if err := json.Unmarshal(b, &stored); err != nil {
			fmt.Fprintf(os.Stderr, "stub: %s.json: %v\n", key, err)
			os.Exit(1)
		}
	}
	if _, ok := stored["number"]; !ok {
		tail := endpoint[strings.LastIndex(endpoint, "/")+1:]
		n, err := strconv.Atoi(tail)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stub: no issue number in %q\n", endpoint)
			os.Exit(1)
		}
		stored["number"] = float64(n)
	}
	patched := map[string]any{}
	for k, v := range stored {
		patched[k] = v
	}
	patched["body"] = decoded
	resp := patched
	if os.Getenv("TEA_STUB_PATCH_NO_STORE") == "" {
		stored = patched
	} else {
		// The server acknowledges but keeps the old body.
		resp = stored
	}
	if os.Getenv("TEA_STUB_PATCH_WRONG_NUMBER") == "1" {
		resp = maps.Clone(resp)
		resp["number"] = stored["number"].(float64) + 1
	}
	if os.Getenv("TEA_STUB_PATCH_OMIT_BODY") == "1" {
		resp = maps.Clone(resp)
		delete(resp, "body")
	}
	b, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stub: marshal patch response: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(dir, key+".patch-response"), b, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "stub: %v\n", err)
		os.Exit(1)
	}
	b, err = json.Marshal(stored)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stub: marshal stored: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(dir, key+".json"), b, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "stub: %v\n", err)
		os.Exit(1)
	}
}

// applyStatePatch mimics a Gitea issue state PATCH: the decoded state
// becomes the stored issue state (<key>.json) and the PATCH response
// (<key>.patch-response), unless a knob says otherwise.
func applyStatePatch(dir, key, endpoint, state string) {
	stored := map[string]any{}
	if b, err := os.ReadFile(filepath.Join(dir, key+".json")); err == nil {
		if err := json.Unmarshal(b, &stored); err != nil {
			fmt.Fprintf(os.Stderr, "stub: %s.json: %v\n", key, err)
			os.Exit(1)
		}
	}
	if _, ok := stored["number"]; !ok {
		tail := endpoint[strings.LastIndex(endpoint, "/")+1:]
		n, err := strconv.Atoi(tail)
		if err != nil {
			fmt.Fprintf(os.Stderr, "stub: no issue number in %q\n", endpoint)
			os.Exit(1)
		}
		stored["number"] = float64(n)
	}
	patched := map[string]any{}
	for k, v := range stored {
		patched[k] = v
	}
	patched["state"] = state
	resp := patched
	if os.Getenv("TEA_STUB_PATCH_NO_STORE") == "" {
		stored = patched
	} else {
		// The server acknowledges but keeps the old state.
		resp = stored
	}
	if os.Getenv("TEA_STUB_PATCH_WRONG_NUMBER") == "1" {
		resp = maps.Clone(resp)
		resp["number"] = stored["number"].(float64) + 1
	}
	if os.Getenv("TEA_STUB_PATCH_OMIT_STATE") == "1" {
		resp = maps.Clone(resp)
		delete(resp, "state")
	}
	b, err := json.Marshal(resp)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stub: marshal patch response: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(dir, key+".patch-response"), b, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "stub: %v\n", err)
		os.Exit(1)
	}
	b, err = json.Marshal(stored)
	if err != nil {
		fmt.Fprintf(os.Stderr, "stub: marshal stored: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(filepath.Join(dir, key+".json"), b, 0o644); err != nil {
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
