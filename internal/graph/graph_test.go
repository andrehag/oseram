package graph

import (
	"reflect"
	"testing"

	"github.com/andrehag/oseram/internal/config"
)

func TestSelect(t *testing.T) {
	tasks := map[string]*config.Task{
		"frontend": {Name: "frontend"},
		"server":   {Name: "server", DependsOn: []string{"frontend"}},
		"lint":     {Name: "lint"},
		"package":  {Name: "package", DependsOn: []string{"server", "lint"}},
	}

	tests := []struct {
		name    string
		targets []string
		want    []string
		wantErr bool
	}{
		{
			name:    "single target",
			targets: []string{"server"},
			want:    []string{"frontend", "server"},
		},
		{
			name:    "parallel targets",
			targets: []string{"lint", "server"},
			want:    []string{"frontend", "lint", "server"},
		},
		{
			name:    "dedup shared dependencies",
			targets: []string{"server", "package"},
			want:    []string{"frontend", "lint", "package", "server"},
		},
		{
			name:    "target that is also a dependency",
			targets: []string{"frontend", "server"},
			want:    []string{"frontend", "server"},
		},
		{
			name:    "empty",
			targets: nil,
			want:    []string{},
		},
		{
			name:    "unknown target",
			targets: []string{"nope"},
			wantErr: true,
		},
		{
			name:    "unknown target among valid",
			targets: []string{"server", "nope"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Select(tasks, tt.targets)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Select(%v) expected error, got %v", tt.targets, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("Select(%v) unexpected error: %v", tt.targets, err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Select(%v) = %v, want %v", tt.targets, got, tt.want)
			}
		})
	}
}
