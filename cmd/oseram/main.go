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
	file := flag.String("file", "build.yml", "build file to read")
	dryRun := flag.Bool("dry-run", false, "print execution plan without running tasks")
	list := flag.Bool("list", false, "list tasks")
	verbose := flag.Bool("verbose", false, "print captured stdout/stderr")
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

	var target string
	if flag.NArg() > 1 {
		return fmt.Errorf("expected zero or one task argument")
	}
	if flag.NArg() == 1 {
		target = flag.Arg(0)
	} else if cfg.Global.Default != "" {
		target = cfg.Global.Default
	} else {
		finals := graph.FinalTasks(cfg.Tasks)
		if len(finals) == 0 {
			return fmt.Errorf("no root/final task found")
		}
		if len(finals) > 1 {
			return fmt.Errorf("multiple root/final tasks found (%v); specify a task or set global.default", finals)
		}
		target = finals[0]
	}

	selected, err := graph.Closure(cfg.Tasks, target)
	if err != nil {
		return err
	}
	order, err := graph.Order(cfg.Tasks, selected)
	if err != nil {
		return err
	}
	r := runner.Runner{Config: cfg, Actions: registry, Verbose: *verbose, DryRun: *dryRun}
	if err := r.Run(context.Background(), order); err != nil {
		return err
	}
	if !*dryRun {
		quote, err := quotes.Random()
		if err != nil {
			return err
		}
		fmt.Println(quotes.Format(quote))
	}
	return nil
}

func printHelp() {
	fmt.Fprintf(flag.CommandLine.Output(), `Oseram - run build.yml tasks in dependency order.

Usage:
  oseram [flags] [task]

Flags:
  --file <path>  Build file to read (default: build.yml)
  --dry-run      Print execution plan without running tasks
  --list         List available tasks and exit
  --verbose      Print captured stdout/stderr for successful tasks
  --help         Show this help text

Arguments:
  task           Optional task to run with its dependencies. If omitted,
                 global.default is used when set; otherwise the single final
                 task is inferred.
`)
}
