package graph

import (
	"fmt"
	"sort"

	"github.com/andrehag/oseram/internal/config"
)

func Validate(tasks map[string]*config.Task) error {
	for name, task := range tasks {
		seen := map[string]bool{}
		for _, dep := range task.DependsOn {
			if dep == name {
				return fmt.Errorf("task %q depends on itself", name)
			}
			if seen[dep] {
				return fmt.Errorf("task %q has duplicate dependency %q", name, dep)
			}
			seen[dep] = true
			if _, ok := tasks[dep]; !ok {
				return fmt.Errorf("task %q depends on unknown task %q", name, dep)
			}
		}
	}
	_, err := Order(tasks, config.TaskNames(tasks))
	return err
}

func FinalTasks(tasks map[string]*config.Task) []string {
	dependedOn := map[string]bool{}
	for _, task := range tasks {
		for _, dep := range task.DependsOn {
			dependedOn[dep] = true
		}
	}
	var finals []string
	for name := range tasks {
		if !dependedOn[name] {
			finals = append(finals, name)
		}
	}
	sort.Strings(finals)
	return finals
}

func Closure(tasks map[string]*config.Task, target string) ([]string, error) {
	if _, ok := tasks[target]; !ok {
		return nil, fmt.Errorf("unknown task %q", target)
	}
	seen := map[string]bool{}
	var visit func(string)
	visit = func(name string) {
		if seen[name] {
			return
		}
		seen[name] = true
		for _, dep := range tasks[name].DependsOn {
			visit(dep)
		}
	}
	visit(target)
	selected := make([]string, 0, len(seen))
	for name := range seen {
		selected = append(selected, name)
	}
	sort.Strings(selected)
	return selected, nil
}

func Order(tasks map[string]*config.Task, selected []string) ([]string, error) {
	selectedSet := map[string]bool{}
	for _, name := range selected {
		selectedSet[name] = true
	}
	indegree := map[string]int{}
	dependents := map[string][]string{}
	for _, name := range selected {
		indegree[name] = 0
	}
	for _, name := range selected {
		for _, dep := range tasks[name].DependsOn {
			if selectedSet[dep] {
				indegree[name]++
				dependents[dep] = append(dependents[dep], name)
			}
		}
	}
	ready := make([]string, 0)
	for name, n := range indegree {
		if n == 0 {
			ready = append(ready, name)
		}
	}
	sort.Strings(ready)
	var order []string
	for len(ready) > 0 {
		name := ready[0]
		ready = ready[1:]
		order = append(order, name)
		sort.Strings(dependents[name])
		for _, dep := range dependents[name] {
			indegree[dep]--
			if indegree[dep] == 0 {
				ready = append(ready, dep)
				sort.Strings(ready)
			}
		}
	}
	if len(order) != len(selected) {
		return nil, fmt.Errorf("dependency graph contains a cycle")
	}
	return order, nil
}
