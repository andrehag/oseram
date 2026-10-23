package runner

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/andrehag/oseram/internal/actions"
)

type nStatus int

const (
	nWaiting nStatus = iota
	nRunning
	nDone
)

// runNonInteractive executes tasks with simple, line-oriented output for
// non-interactive terminals (CI, scripts, redirected output). Independent
// tasks still run in parallel; results are reported one line at a time as
// they complete. No colors, no redraws, no decorative quote.
func (r Runner) runNonInteractive(ctx context.Context, selected []string) error {
	inSelected := map[string]bool{}
	for _, name := range selected {
		inSelected[name] = true
	}
	remaining := map[string]int{}
	dependents := map[string][]string{}
	for _, name := range selected {
		for _, dep := range r.Config.Tasks[name].DependsOn {
			if inSelected[dep] {
				remaining[name]++
				dependents[dep] = append(dependents[dep], name)
			}
		}
	}

	status := map[string]nStatus{}
	for _, name := range selected {
		status[name] = nWaiting
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	results := make(chan taskResult, len(selected))
	verbose := r.Verbose || r.Config.Global.Verbose
	started := 0
	failed := false

	run := func(name string) {
		if status[name] != nWaiting || remaining[name] != 0 || failed {
			return
		}
		status[name] = nRunning
		started++
		if verbose {
			fmt.Printf("oseram: running %s %s\n", name, r.plainTaskLine(name))
		}
		go func(n string) {
			results <- r.runTask(ctx, n)
		}(name)
	}

	for _, name := range selected {
		run(name)
	}

	completed := 0
	var firstErr error
	for completed < started {
		res := <-results
		completed++
		status[res.Name] = nDone

		if res.Err != nil {
			if firstErr == nil {
				firstErr = res.Err
				fmt.Fprintf(os.Stderr, "[FAIL] %s %s\n", res.Name, r.plainTaskLine(res.Name))
				fmt.Fprint(os.Stderr, formatCaptured(res.Name, res.Output))
				cancel()
				failed = true
				// In-flight tasks still report; they are drained and ignored so
				// the failed build's exit code reflects the first error.
			}
			continue
		}

		if verbose {
			fmt.Printf("[ok] %s %s\n", res.Name, r.plainTaskLine(res.Name))
			fmt.Fprint(os.Stdout, formatCaptured(res.Name, res.Output))
		} else {
			fmt.Printf("[ok] %s %s\n", res.Name, r.plainTaskLine(res.Name))
		}

		for _, dependent := range dependents[res.Name] {
			remaining[dependent]--
			if remaining[dependent] == 0 {
				run(dependent)
			}
		}
	}
	return firstErr
}

// formatCaptured renders a task's captured stdout/stderr with a simple header,
// matching the layout the TUI mode shows in its output strip.
func formatCaptured(name string, out actions.Output) string {
	var b strings.Builder
	if out.Stdout != "" {
		fmt.Fprintf(&b, "[%s stdout]\n%s\n", name, strings.TrimRight(out.Stdout, "\n"))
	}
	if out.Stderr != "" {
		fmt.Fprintf(&b, "[%s stderr]\n%s\n", name, strings.TrimRight(out.Stderr, "\n"))
	}
	return b.String()
}
