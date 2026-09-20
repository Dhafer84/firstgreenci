package generate_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/Dhafer84/firstgreenci/internal/generate"
)

func TestWhere(t *testing.T) {
	// A repository whose .git is a directory, the ordinary case.
	ordinary := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ordinary, ".git"), 0o755); err != nil {
		t.Fatalf("build the repository: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(ordinary, "backend", "src"), 0o755); err != nil {
		t.Fatalf("build the subfolders: %v", err)
	}

	// A repository whose .git is a file, as in a worktree or a submodule.
	asFile := t.TempDir()
	if err := os.WriteFile(filepath.Join(asFile, ".git"), []byte("gitdir: /elsewhere\n"), 0o644); err != nil {
		t.Fatalf("build the worktree: %v", err)
	}

	// A folder belonging to no repository at all.
	orphan := t.TempDir()

	tests := []struct {
		name     string
		dir      string
		want     generate.Location
		wantRoot string
	}{
		{
			name:     "the root itself",
			dir:      ordinary,
			want:     generate.AtRepositoryRoot,
			wantRoot: ordinary,
		},
		{
			name:     "one folder below the root",
			dir:      filepath.Join(ordinary, "backend"),
			want:     generate.InsideRepository,
			wantRoot: ordinary,
		},
		{
			name:     "two folders below the root",
			dir:      filepath.Join(ordinary, "backend", "src"),
			want:     generate.InsideRepository,
			wantRoot: ordinary,
		},
		{
			name:     "a worktree, where .git is a file",
			dir:      asFile,
			want:     generate.AtRepositoryRoot,
			wantRoot: asFile,
		},
		{
			name: "no repository anywhere above",
			dir:  orphan,
			want: generate.OutsideRepository,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, root := generate.Where(test.dir)
			if got != test.want {
				t.Errorf("Where(%s) = %v, want %v", test.dir, got, test.want)
			}
			if test.wantRoot != "" && root != test.wantRoot {
				t.Errorf("root = %q, want %q", root, test.wantRoot)
			}
		})
	}
}
