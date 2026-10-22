package main

import (
	"errors"
	"flag"
	"fmt"
	"io"

	"github.com/andrehag/oseram/internal/initgen"
)

// runInit handles "oseram init", which generates a suggested build.yml by
// scanning a working directory for Go entry points and package.json files.
// It uses its own FlagSet so the subcommand's flags do not interfere with
// the top-level oseram flags.
func runInit(args []string) error {
	fs := flag.NewFlagSet("oseram init", flag.ContinueOnError)
	workingDirectory := fs.String("working-directory", ".", "directory to scan and write build.yml into")
	fs.Usage = func() { printInitHelp(fs.Output()) }
	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("oseram init does not accept positional arguments")
	}

	msg, err := initgen.Run(*workingDirectory)
	if err != nil {
		return err
	}
	fmt.Println(msg)
	return nil
}

func printInitHelp(out io.Writer) {
	fmt.Fprintf(out, `Oseram Init - create a suggested build.yml for the current folder.

Usage:
  oseram init [flags]

Flags:
  --working-directory <path>  Directory to scan and write build.yml into (default: .)
  --help                      Show this help text

Scans for Go entry points and package.json files. If build.yml already exists,
it is left unchanged.
`)
}
