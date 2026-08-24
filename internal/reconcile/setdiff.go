package reconcile

import (
	"github.com/fastly/terraform-provider-fastly/internal/errors"
)

// DiffSet reconciles a many-to-many association keyed by plain strings (e.g. a director's
// backend membership), removing keys missing from wanted then adding keys missing from current.
// A remove error satisfying errors.IsNotFound is tolerated - the association is already gone.
func DiffSet(current, wanted []string, add, remove func(key string) error) error {
	currentSet := make(map[string]struct{}, len(current))
	for _, k := range current {
		currentSet[k] = struct{}{}
	}

	wantedSet := make(map[string]struct{}, len(wanted))
	for _, k := range wanted {
		wantedSet[k] = struct{}{}
	}

	for _, k := range current {
		if _, ok := wantedSet[k]; ok {
			continue
		}
		if err := remove(k); err != nil && !errors.IsNotFound(err) {
			return err
		}
	}

	for _, k := range wanted {
		if _, ok := currentSet[k]; ok {
			continue
		}
		if err := add(k); err != nil {
			return err
		}
	}

	return nil
}
