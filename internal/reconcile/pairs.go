package reconcile

// ThreePassUpdate reconciles a parent/dependent pair referenced by name (e.g. director ->
// backend): parent create/update must precede the dependent, but parent deletes must wait until
// the dependent no longer references them, or the API rejects the delete. If dependentPresent is
// false it skips straight to parentReconcile (delete-before-create), avoiding a transient item
// that could exceed a service-side count limit.
func ThreePassUpdate(
	dependentPresent bool,
	parentReconcile func() error,
	parentCreateOrUpdate func() error,
	dependentReconcile func() error,
	parentDeleteRemoved func() error,
) error {
	if !dependentPresent {
		if err := parentReconcile(); err != nil {
			return err
		}
		return dependentReconcile()
	}

	if err := parentCreateOrUpdate(); err != nil {
		return err
	}
	if err := dependentReconcile(); err != nil {
		return err
	}
	return parentDeleteRemoved()
}
