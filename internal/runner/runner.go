package runner

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"oseram/internal/actions"
	"oseram/internal/config"
)

type Runner struct {
	Config  *config.Config
	Actions actions.Registry
	Verbose bool
	DryRun  bool
}

type taskStatus int

const (
	statusWaiting taskStatus = iota
	statusRunning
	statusSuccess
	statusFailure
)

var (
	waitingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
	runningStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("33"))
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("42"))
	failureStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	titleStyle   = lipgloss.NewStyle().Bold(true)
	outputStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("244"))
)

func (s taskStatus) icon() string {
	switch s {
	case statusRunning:
		return runningStyle.Render("▶")
	case statusSuccess:
		return successStyle.Render("✓")
	case statusFailure:
		return failureStyle.Render("✗")
	default:
		return waitingStyle.Render("○")
	}
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
	initialModel := newModel(ctx, cancel, r, selected)
	program := tea.NewProgram(initialModel)
	final, err := program.Run()
	if err != nil {
		return err
	}
	m, ok := final.(*model)
	if !ok {
		return nil
	}
	return m.firstErr
}

type model struct {
	ctx         context.Context
	cancel      context.CancelFunc
	runner      Runner
	selected    []string
	statuses    map[string]taskStatus
	remaining   map[string]int
	dependents  map[string][]string
	ready       []string
	total       int
	completed   int
	running     int
	failed      bool
	firstErr    error
	output      []string
	initialized bool
}

func newModel(ctx context.Context, cancel context.CancelFunc, r Runner, selected []string) *model {
	m := &model{
		ctx:        ctx,
		cancel:     cancel,
		runner:     r,
		selected:   selected,
		statuses:   map[string]taskStatus{},
		remaining:  map[string]int{},
		dependents: map[string][]string{},
		total:      len(selected),
	}
	selectedSet := map[string]bool{}
	for _, name := range selected {
		selectedSet[name] = true
		m.statuses[name] = statusWaiting
		m.remaining[name] = 0
	}
	for _, name := range selected {
		for _, dep := range r.Config.Tasks[name].DependsOn {
			if selectedSet[dep] {
				m.remaining[name]++
				m.dependents[dep] = append(m.dependents[dep], name)
			}
		}
	}
	for name := range m.dependents {
		sort.Strings(m.dependents[name])
	}
	for name, n := range m.remaining {
		if n == 0 {
			m.ready = append(m.ready, name)
		}
	}
	sort.Strings(m.ready)
	return m
}

func (m *model) Init() tea.Cmd {
	return m.startReady()
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "ctrl+c" || msg.String() == "q" {
			m.cancel()
			m.firstErr = fmt.Errorf("build cancelled")
			return m, tea.Quit
		}
	case taskResult:
		m.running--
		m.completed++
		verbose := m.runner.Verbose || m.runner.Config.Global.Verbose
		if verbose || msg.Err != nil {
			m.appendOutput(msg)
		}
		if msg.Err != nil && !m.failed {
			m.statuses[msg.Name] = statusFailure
			m.failed = true
			m.firstErr = msg.Err
			m.cancel()
			return m, tea.Quit
		}
		m.statuses[msg.Name] = statusSuccess
		if m.completed == m.total {
			return m, tea.Quit
		}
		if !m.failed {
			for _, dependent := range m.dependents[msg.Name] {
				m.remaining[dependent]--
				if m.remaining[dependent] == 0 {
					m.ready = append(m.ready, dependent)
				}
			}
			sort.Strings(m.ready)
			return m, m.startReady()
		}
	}
	return m, nil
}

func (m *model) View() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Tasks"))
	b.WriteString("\n")
	for _, name := range m.selected {
		b.WriteString(m.statuses[name].icon())
		b.WriteString(" ")
		b.WriteString(name)
		b.WriteString(" (")
		b.WriteString(m.runner.Config.Tasks[name].ActionName)
		b.WriteString(")")
		b.WriteString("\n")
	}
	if len(m.output) > 0 {
		b.WriteString("\n")
		b.WriteString(outputStyle.Render(strings.Join(m.output, "\n")))
		b.WriteString("\n")
	}
	return b.String()
}

func (m *model) startReady() tea.Cmd {
	if m.failed || len(m.ready) == 0 {
		return nil
	}
	cmds := make([]tea.Cmd, 0, len(m.ready))
	for len(m.ready) > 0 {
		name := m.ready[0]
		m.ready = m.ready[1:]
		m.statuses[name] = statusRunning
		m.running++
		cmds = append(cmds, m.runTaskCmd(name))
	}
	return tea.Batch(cmds...)
}

func (m *model) runTaskCmd(name string) tea.Cmd {
	return func() tea.Msg {
		return m.runner.runTask(m.ctx, name)
	}
}

func (m *model) appendOutput(res taskResult) {
	if res.Output.Stdout != "" {
		m.output = append(m.output, fmt.Sprintf("[%s stdout]\n%s", res.Name, strings.TrimRight(res.Output.Stdout, "\n")))
	}
	if res.Output.Stderr != "" {
		m.output = append(m.output, fmt.Sprintf("[%s stderr]\n%s", res.Name, strings.TrimRight(res.Output.Stderr, "\n")))
	}
}

type taskResult struct {
	Name   string
	Output actions.Output
	Err    error
}

func (r Runner) runTask(ctx context.Context, name string) taskResult {
	task := r.Config.Tasks[name]
	action := r.Actions[task.ActionName]
	workDir := r.resolveWorkDir(task)
	env := actions.MergeEnv(actions.CurrentEnv(), r.Config.Global.Environment)
	env = actions.MergeEnv(env, task.Environment)
	taskCtx := actions.TaskContext{Name: name, BaseDir: r.Config.BaseDir, WorkDir: workDir, Env: env}
	out, err := action.Run(ctx, taskCtx, task.Action)
	if err != nil {
		return taskResult{Name: name, Output: out, Err: fmt.Errorf("task %q failed: %w", name, err)}
	}
	return taskResult{Name: name, Output: out}
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
