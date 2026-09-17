// Command awit turns Markdown files under .awit/ into a dependency graph.
package main

import (
	"os"

	"github.com/eisenwinter/awit/internal/cli"
)

func main() {
	os.Exit(cli.Main(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}
