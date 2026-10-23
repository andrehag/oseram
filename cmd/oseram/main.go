package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/andrehag/oseram/internal/actions"
	"github.com/andrehag/oseram/internal/config"
	"github.com/andrehag/oseram/internal/graph"
	"github.com/andrehag/oseram/internal/quotes"
	"github.com/andrehag/oseram/internal/runner"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	flag.Usage = printHelp
	if len(os.Args) > 1 && os.Args[1] == "init" {
		return runInit(os.Args[2:])
	}
	file := flag.String("file", "build.yml", "build file to read")
	dryRun := flag.Bool("dry-run", false, "print execution plan without running tasks")
	list := flag.Bool("list", false, "list tasks")
	verbose := flag.Bool("verbose", false, "print captured stdout/stderr")
	nonInteractive := flag.Bool("non-interactive", false, "simplified output for non-interactive terminals")
	flag.Parse()

	registry := actions.Builtins()
	cfg, err := config.Load(*file, registry.Names())
	if err != nil {
		return err
	}
	for _, task := range cfg.Tasks {
		action := registry[task.ActionName]
		if err := action.Validate(task.Action); err != nil {
			return fmt.Errorf("task %q: %w", task.Name, err)
		}
	}
	if err := graph.Validate(cfg.Tasks); err != nil {
		return err
	}
	if cfg.Global.Default != "" {
		if _, ok := cfg.Tasks[cfg.Global.Default]; !ok {
			return fmt.Errorf("global.default references unknown task %q", cfg.Global.Default)
		}
	}

	if *list {
		for _, name := range config.TaskNames(cfg.Tasks) {
			fmt.Println(name)
		}
		return nil
	}

	var targets []string
	if flag.NArg() > 0 {
		targets = flag.Args()
	} else if cfg.Global.Default != "" {
		targets = []string{cfg.Global.Default}
	} else {
		finals := graph.FinalTasks(cfg.Tasks)
		if len(finals) == 0 {
			return fmt.Errorf("no root/final task found")
		}
		if len(finals) > 1 {
			return fmt.Errorf("multiple root/final tasks found (%v); specify a task or set global.default", finals)
		}
		targets = []string{finals[0]}
	}

	selected, err := graph.Select(cfg.Tasks, targets)
	if err != nil {
		return err
	}
	order, err := graph.Order(cfg.Tasks, selected)
	if err != nil {
		return err
	}
	r := runner.Runner{Config: cfg, Actions: registry, Verbose: *verbose, DryRun: *dryRun, NonInteractive: *nonInteractive || !termIsInteractive(os.Stdout)}
	if err := r.Run(context.Background(), order); err != nil {
		return err
	}
	if !*dryRun && !cfg.Global.NoQuote && !r.NonInteractive {
		quote, err := quotes.Random()
		if err != nil {
			return err
		}
		fmt.Println(quotes.Format(quote))
	}
	return nil
}

// termIsInteractive reports whether the given file is a terminal. When stdout
// is not a terminal (for example when oseram runs as part of an automated
// build or its output is piped), Oseram automatically switches to simplified,
// non-interactive output.
func termIsInteractive(f *os.File) bool {
	fi, err := f.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}

func printHelp() {
	fmt.Fprintf(flag.CommandLine.Output(), `Oseram - run build.yml tasks in dependency order.

Usage:
  oseram [flags] [task]
       oseram init [flags]

Subcommands:
  init            Create a suggested build.yml for the current folder

Flags:
  --file <path>  Build file to read (default: build.yml)
  --dry-run      Print execution plan without running tasks
  --list         List available tasks and exit
  --verbose      Print captured stdout/stderr for successful tasks
  --non-interactive
                 Use simplified line output instead of the interactive TUI
  --help         Show this help text

Arguments:
  task           One or more tasks to run with their dependencies. If omitted,
                 global.default is used when set; otherwise the single final
                 task is inferred. Multiple independent targets run in parallel.
`)
}
