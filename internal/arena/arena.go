// Package arena bounds the blast radius of everything the agent does.
//
// The agent performs its own mutations, so safety here is not a deterministic
// actuator but a bounded, reversible region: a controller-owned worktree on a
// feature branch, never the default branch, never a force-push, never a merge
// or deploy. Anything the agent can reach is something a human can undo.
package arena

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"
)

// Arena is one controller-owned lane the agent may mutate.
type Arena struct {
	Root          string
	Owner         string
	Repository    string
	Lane          string
	Branch        string
	DefaultBranch string
}

var (
	// ErrDefaultBranch is returned when the lane sits on the protected base.
	ErrDefaultBranch = errors.New("arena may not operate on the default branch")
	// ErrOutsideArena is returned for a path beyond the arena root.
	ErrOutsideArena = errors.New("path is outside the arena root")
	// ErrIncomplete is returned when the arena is not fully specified.
	ErrIncomplete = errors.New("arena requires a root, branch, and default branch")
	// ErrNotCanonical is returned when the root is not a canonical lane path.
	ErrNotCanonical = errors.New("arena root is not a canonical worktree lane")
)

// Validate reports whether the arena may be used at all.
//
// The default-branch check is the load-bearing one. Every other guard limits
// what happens inside a feature branch; this one keeps the controller off the
// branch a human would have to force-push to recover.
func (a Arena) Validate() error {
	if strings.TrimSpace(a.Root) == "" || strings.TrimSpace(a.Branch) == "" ||
		strings.TrimSpace(a.DefaultBranch) == "" {
		return ErrIncomplete
	}
	if a.Branch == a.DefaultBranch {
		return fmt.Errorf("%w: branch %q", ErrDefaultBranch, a.Branch)
	}
	if !filepath.IsAbs(a.Root) {
		return fmt.Errorf("%w: %q is not absolute", ErrNotCanonical, a.Root)
	}
	return nil
}

// CanonicalRoot returns the lane path an arena is required to occupy.
func CanonicalRoot(home, owner, repository, lane string) string {
	return filepath.Join(home, "worktrees", owner, repository, lane)
}

// IsCanonical reports whether the root matches the canonical lane path for its
// own coordinates, so a lane cannot be quietly relocated somewhere unbounded.
func (a Arena) IsCanonical(home string) bool {
	want := CanonicalRoot(home, a.Owner, a.Repository, a.Lane)
	return filepath.Clean(a.Root) == filepath.Clean(want)
}

// Contains reports whether a path lies inside the arena root.
//
// The path is cleaned before comparison so that traversal through a parent
// cannot escape, and a path equal to the root itself counts as inside.
func (a Arena) Contains(path string) bool {
	root := filepath.Clean(a.Root)
	target := filepath.Clean(path)
	if !filepath.IsAbs(target) {
		target = filepath.Clean(filepath.Join(root, target))
	}
	if target == root {
		return true
	}
	relative, err := filepath.Rel(root, target)
	if err != nil {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// CheckPath returns an error when a path is not inside the arena.
func (a Arena) CheckPath(path string) error {
	if !a.Contains(path) {
		return fmt.Errorf("%w: %q not under %q", ErrOutsideArena, path, a.Root)
	}
	return nil
}
