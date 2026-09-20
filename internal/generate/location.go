package generate

import (
	"os"
	"path/filepath"
)

// Location says where a folder sits with respect to a git repository, which
// decides whether GitHub will ever read the workflow written there.
type Location int

const (
	// AtRepositoryRoot is the only place GitHub reads workflows from.
	AtRepositoryRoot Location = iota
	// InsideRepository means the folder belongs to a repository but is not
	// its root. A workflow written there runs locally and is ignored by
	// GitHub — a green pipeline that never executes.
	InsideRepository
	// OutsideRepository means no repository was found above the folder.
	OutsideRepository
)

// RepositoryRoot walks up from dir looking for the repository it belongs to,
// and reports the root it found.
//
// It looks for a .git entry without caring whether it is a directory or a
// file: .git is a file inside a worktree and inside a submodule, and both are
// real repositories.
//
// It does not call git. The answer must be the same on a machine where git is
// not installed, and a function that shells out cannot be tested on three
// systems without building repositories first.
func RepositoryRoot(dir string) (string, bool) {
	current, err := filepath.Abs(dir)
	if err != nil {
		return "", false
	}

	for {
		if _, err := os.Lstat(filepath.Join(current, ".git")); err == nil {
			return current, true
		}

		parent := filepath.Dir(current)
		if parent == current {
			// Reached the top of the filesystem.
			return "", false
		}
		current = parent
	}
}

// Where reports how dir sits with respect to its repository, and the root
// when there is one.
func Where(dir string) (Location, string) {
	root, found := RepositoryRoot(dir)
	if !found {
		return OutsideRepository, ""
	}

	absolute, err := filepath.Abs(dir)
	if err != nil || absolute != root {
		return InsideRepository, root
	}
	return AtRepositoryRoot, root
}
