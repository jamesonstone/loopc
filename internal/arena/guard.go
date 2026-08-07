package arena

import (
	"errors"
	"fmt"
	"path"
	"sort"
	"strings"
)

// GuardedPaths are the parts of the controller the controller may not change.
//
// Self-improvement does not include self-modification. A loop permitted to edit
// its own arena or control law could widen its authority by editing the code
// that limits it, so a change touching either is refused outright — the one
// refusal that cannot be reasoned around, since the reasoning would be done by
// the thing being constrained.
var GuardedPaths = []string{
	"internal/arena",
	"internal/policy",
}

// ErrSelfModification is returned for a change touching the guarded paths.
var ErrSelfModification = errors.New("change touches the controller's own arena or control law")

// Refusal explains why a change was refused.
type Refusal struct {
	Reason string
	Paths  []string
}

// Error renders the refusal for logs and journal records.
func (r Refusal) Error() string {
	return fmt.Sprintf("%s: %s", r.Reason, strings.Join(r.Paths, ", "))
}

// InspectDiff reports whether a set of changed paths may be acted on.
//
// Paths are compared as whole segments after cleaning, so a directory named
// internal/policyholder does not match internal/policy, and a traversal cannot
// disguise a guarded path as an innocent one.
func InspectDiff(changed []string) (Refusal, bool) {
	hits := make([]string, 0)
	for _, raw := range changed {
		candidate := normalise(raw)
		if candidate == "" {
			continue
		}
		for _, guarded := range GuardedPaths {
			if candidate == guarded || strings.HasPrefix(candidate, guarded+"/") {
				hits = append(hits, candidate)
				break
			}
		}
	}
	if len(hits) == 0 {
		return Refusal{}, false
	}
	sort.Strings(hits)
	return Refusal{Reason: ErrSelfModification.Error(), Paths: dedupe(hits)}, true
}

// normalise reduces a repository-relative path to clean forward-slash form and
// rejects anything escaping the repository root.
func normalise(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	cleaned := path.Clean(strings.ReplaceAll(trimmed, "\\", "/"))
	cleaned = strings.TrimPrefix(cleaned, "./")
	if cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return ""
	}
	return strings.TrimPrefix(cleaned, "/")
}

func dedupe(values []string) []string {
	out := make([]string, 0, len(values))
	var previous string
	for i, v := range values {
		if i > 0 && v == previous {
			continue
		}
		out = append(out, v)
		previous = v
	}
	return out
}

// CheckDiff returns an error when a change may not be acted on.
func CheckDiff(changed []string) error {
	if refusal, refused := InspectDiff(changed); refused {
		return fmt.Errorf("%w: %s", ErrSelfModification, strings.Join(refusal.Paths, ", "))
	}
	return nil
}
