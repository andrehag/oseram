package runner

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"osseram/internal/actions"
	"osseram/internal/config"
)

type Runner struct {
	Config  *config.Config
	Actions actions.Registry
	Verbose bool
	DryRun  bool
}

func (r Runner) Run(ctx context.Context, selected []string) error {
	if r.DryRun {
		fmt.Println("execution plan:")
		for _, name := range selected {
			fmt.Printf("- %s\n", name)
		}
		return nil
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	selectedSet := map[string]bool{}
	for _, name := range selected {
		selectedSet[name] = true
	}
	remainingDeps := map[string]int{}
	dependents := map[string][]string{}
	for _, name := range selected {
		remainingDeps[name] = 0
	}
	for _, name := range selected {
		for _, dep := range r.Config.Tasks[name].DependsOn {
			if selectedSet[dep] {
				remainingDeps[name]++
				dependents[dep] = append(dependents[dep], name)
			}
		}
	}
	for name := range dependents {
		sort.Strings(dependents[name])
	}

	ready := []string{}
	for name, n := range remainingDeps {
		if n == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)
	total := len(selected)
	completed := 0
	running := 0
	failed := false
	var firstErr error
	results := make(chan taskResult)

	start := func(name string) {
		running++
		go func() { results <- r.runTask(ctx, name) }()
	}
	for !failed && len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		start(name)
	}
	for completed < total && running > 0 {
		res := <-results
		running--
		completed++
		if res.Err != nil && !failed {
			failed = true
			firstErr = res.Err
			cancel()
		}
		if !failed {
			for _, dependent := range dependents[res.Name] {
				remainingDeps[dependent]--
				if remainingDeps[dependent] == 0 {
					ready = append(ready, dependent)
				}
			}
			sort.Strings(ready)
			for len(ready) > 0 {
				name := ready[0]
				ready = ready[1:]
				start(name)
			}
		}
	}
	if firstErr != nil {
		return firstErr
	}
	return nil
}

type taskResult struct {
	Name string
	Err  error
}

var printMu sync.Mutex

func (r Runner) runTask(ctx context.Context, name string) taskResult {
	task := r.Config.Tasks[name]
	action := r.Actions[task.ActionName]
	fmt.Printf("running %s\n", name)
	workDir := r.resolveWorkDir(task)
	env := actions.MergeEnv(actions.CurrentEnv(), r.Config.Global.Environment)
	env = actions.MergeEnv(env, task.Environment)
	taskCtx := actions.TaskContext{Name: name, BaseDir: r.Config.BaseDir, WorkDir: workDir, Env: env}
	out, err := action.Run(ctx, taskCtx, task.Action)
	verbose := r.Verbose || r.Config.Global.Verbose
	if verbose || err != nil {
		printMu.Lock()
		defer printMu.Unlock()
		if out.Stdout != "" {
			fmt.Printf("[%s stdout]\n%s", name, out.Stdout)
		}
		if out.Stderr != "" {
			fmt.Fprintf(os.Stderr, "[%s stderr]\n%s", name, out.Stderr)
		}
	}
	if err != nil {
		return taskResult{Name: name, Err: fmt.Errorf("task %q failed: %w", name, err)}
	}
	fmt.Printf("finished %s\n", name)
	return taskResult{Name: name}
}

func (r Runner) resolveWorkDir(task *config.Task) string {
	wd := r.Config.Global.WorkingDirectory
	if task.WorkingDirectory != "" {
		wd = task.WorkingDirectory
	}
	if wd == "" {
		return r.Config.BaseDir
	}
	if filepath.IsAbs(wd) {
		return wd
	}
	return filepath.Join(r.Config.BaseDir, wd)
}
