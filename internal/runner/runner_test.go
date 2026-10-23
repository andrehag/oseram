package runner

import (
	"context"
	"sync"
	"testing"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/andrehag/oseram/internal/actions"
	"github.com/andrehag/oseram/internal/config"
)

// blockAction signals entry through starts and waits on release, letting a
// test control exactly when a task finishes and observe when it runs.
type blockAction struct {
	starts  chan string
	release chan struct{}
	once    sync.Once
}

func (b *blockAction) Name() string { return "gate" }
func (b *blockAction) Validate(raw yaml.Node) error {
	return nil
}
func (b *blockAction) Run(ctx context.Context, task actions.TaskContext, raw yaml.Node) (actions.Output, error) {
	select {
	case b.starts <- task.Name:
	case <-ctx.Done():
		return actions.Output{}, ctx.Err()
	}
	select {
	case <-b.release:
		return actions.Output{}, nil
	case <-ctx.Done():
		return actions.Output{}, ctx.Err()
	}
}

func newTestConfig(t *testing.T) *config.Config {
	t.Helper()
	cfg := &config.Config{BaseDir: t.TempDir(), Tasks: map[string]*config.Task{}}
	addTask := func(name string, deps ...string) {
		task := &config.Task{
			Name:       name,
			DependsOn:  deps,
			ActionName: "gate",
			Action:     yaml.Node{Kind: yaml.MappingNode},
		}
		cfg.Tasks[name] = task
	}
	addTask("a")
	addTask("b", "a")
	addTask("c", "a")
	return cfg
}

// TestNonInteractiveRunsDependenciesSequentially ensures the plain runner
// starts independent tasks in parallel and only releases a dependent once its
// dependencies have finished.
func TestNonInteractiveRunsDependenciesSequentially(t *testing.T) {
	gate := &blockAction{starts: make(chan string, 4), release: make(chan struct{})}
	r := Runner{
		Config:         newTestConfig(t),
		Actions:        actions.Registry{"gate": gate},
		NonInteractive: true,
	}
	done := make(chan struct{})
	go func() {
		err := r.runNonInteractive(context.Background(), []string{"a", "b", "c"})
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		close(done)
	}()

	// a must start first; b and c must not start until a finishes.
	first := <-gate.starts
	if first != "a" {
		t.Fatalf("expected task a to start first, got %q", first)
	}
	select {
	case started := <-gate.starts:
		t.Fatalf("b/c started before a finished: %q", started)
	case <-time.After(50 * time.Millisecond):
	}

	close(gate.release)
	started := map[string]bool{}
	for i := 0; i < 2; i++ {
		name := <-gate.starts
		started[name] = true
	}
	if !started["b"] || !started["c"] {
		t.Fatalf("expected b and c to both start after a, got %v", started)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runner did not finish in time")
	}
}

// errAction reports the given error from Run without blocking.
type errAction struct {
	starts chan string
}

func (f *errAction) Name() string { return "gate" }
func (f *errAction) Validate(raw yaml.Node) error {
	return nil
}
func (f *errAction) Run(ctx context.Context, task actions.TaskContext, raw yaml.Node) (actions.Output, error) {
	select {
	case f.starts <- task.Name:
	case <-ctx.Done():
		return actions.Output{}, ctx.Err()
	}
	return actions.Output{}, context.Canceled
}

// TestNonInteractiveFailsStopsDependents ensures a task failure is returned
// and no dependent task is ever started.
func TestNonInteractiveFailsStopsDependents(t *testing.T) {
	gate := &errAction{starts: make(chan string, 4)}
	cfg := newTestConfig(t)
	for _, name := range []string{"a", "b", "c"} {
		cfg.Tasks[name].ActionName = "gate"
	}
	r := Runner{
		Config:         cfg,
		Actions:        actions.Registry{"gate": gate},
		NonInteractive: true,
	}

	go func() {
		// a is the only independent task; it fails, so b and c must not start.
		if err := r.runNonInteractive(context.Background(), []string{"a", "b", "c"}); err == nil {
			t.Error("expected an error from the failed run")
		}
	}()

	if name := <-gate.starts; name != "a" {
		t.Fatalf("expected a to start, got %q", name)
	}
	select {
	case started := <-gate.starts:
		t.Fatalf("a dependent started after a failed: %q", started)
	case <-time.After(100 * time.Millisecond):
	}
}
