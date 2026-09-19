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
// Environment knobs:
//
//	TEA_STUB_NO_API=1      behave like a tea too old for the api subcommand
//	TEA_STUB_API_EXIT=N    force exit code N for api calls (after printing)
//	TEA_STUB_API_STDERR=s  print s on stderr for api calls
package main

import (
	"encoding/json"
	"fmt"
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
	if s := os.Getenv("TEA_STUB_API_STDERR"); s != "" {
		fmt.Fprintln(os.Stderr, s)
	}
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
	body := "{}"
	if b, err := os.ReadFile(filepath.Join(dir, key+".json")); err == nil {
		body = string(b)
	}
	fmt.Printf("HTTP/1.1 %s STUB\nContent-Type: application/json; charset=utf-8\n\n%s", status, body)
	if s := os.Getenv("TEA_STUB_API_EXIT"); s != "" {
		n, err := strconv.Atoi(s)
		if err != nil {
			n = 1
		}
		os.Exit(n)
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
