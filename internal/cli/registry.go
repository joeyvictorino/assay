// Package cli is the subcommand registry for the assay binary.
//
// Streams add one cmd_<name>.go file each and register from an init func.
// Only the lead edits this file.
package cli

import (
	"fmt"
	"io"
	"sort"
)

// Exit codes (mirroring tasia): 0 pass, 1 blocked, 2 tool/config error.
const (
	ExitPass    = 0
	ExitBlocked = 1
	ExitError   = 2
)

// Runner executes a subcommand with its args and returns an exit code.
type Runner func(args []string, stdout, stderr io.Writer) int

type command struct {
	name, summary string
	run           Runner
}

var registry = map[string]command{}

// Register adds a subcommand. Duplicate names panic at init time.
func Register(name, summary string, run Runner) {
	if _, dup := registry[name]; dup {
		panic("cli: duplicate subcommand " + name)
	}
	registry[name] = command{name: name, summary: summary, run: run}
}

// Main dispatches to a registered subcommand.
func Main(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "-h" || args[0] == "--help" {
		usage(stdout)
		return ExitPass
	}
	c, ok := registry[args[0]]
	if !ok {
		fmt.Fprintf(stderr, "assay: unknown subcommand %q\n\n", args[0])
		usage(stderr)
		return ExitError
	}
	return c.run(args[1:], stdout, stderr)
}

func usage(w io.Writer) {
	fmt.Fprintln(w, "usage: assay <subcommand> [flags]")
	fmt.Fprintln(w)
	names := make([]string, 0, len(registry))
	for n := range registry {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		fmt.Fprintf(w, "  %-12s %s\n", n, registry[n].summary)
	}
}
