package migrate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

// Applied describes a successful on-disk migration. BackupDir is empty if
// the plan made no changes; otherwise it contains the persistent recovery copy.
type Applied struct {
	FilesWritten int
	BackupDir    string
}

// Apply writes the exact validated plan returned by Planner.Plan. It does not
// recompile the configuration, invoke Terraform, or change Terraform state.
//
// BackupRoot is an optional private directory outside the Terraform tree.
// When empty, backups are stored under ~/.fastly-tf-migrate. Every non-empty
// plan is backed up before the first Terraform file is replaced.
//
// A group of file renames is not a filesystem transaction. The rewrite engine
// attempts rollback on ordinary failures; persistent backups and a manifest
// permit recovery after a process crash, power failure, or failed rollback.
func (r Result) Apply(ctx context.Context, backupRoot string) (Applied, error) {
	if err := ctx.Err(); err != nil {
		return Applied{}, err
	}
	if r.Blocked() {
		return Applied{}, errors.New("cannot write a migration with unresolved blockers")
	}
	if r.rewritePlan == nil {
		return Applied{}, errors.New("migration write requires the original validated rewrite plan")
	}
	root := r.rewritePlan.Root()
	changes := r.rewritePlan.Changes()

	directory, err := resolveBackupRoot(root, backupRoot)
	if err != nil {
		return Applied{}, err
	}

	// Serializes cooperating writers and prevents silent retries after a
	// previous process was interrupted. The lock also prevents simultaneous
	// no-op runs from concealing an incomplete earlier migration.
	project, unlock, err := lockProject(directory, root)
	if err != nil {
		return Applied{}, err
	}
	defer unlock()

	if err := checkUnfinishedBackups(project, root); err != nil {
		return Applied{}, err
	}
	// Recheck every file inspected by the analyzer, not just the changed
	// files: even a non-target source can invalidate preflight assumptions.
	if err := checkSnapshots(root, r.snapshots, changes); err != nil {
		return Applied{}, fmt.Errorf("configuration changed after migration planning: %w", err)
	}
	if len(changes) == 0 {
		return Applied{}, nil
	}

	backup, err := prepareBackup(ctx, project, root, changes)
	if err != nil {
		return Applied{}, fmt.Errorf("create recovery backup before migration: %w", err)
	}
	outcome := Applied{BackupDir: backup.path}
	if err := r.rewritePlan.Apply(ctx); err != nil {
		// The rewrite engine attempts best-effort rollback. Verify that
		// all files returned to their original state before declaring it.
		status := backupRecoveryRequired
		if allOriginalsMatch(root, changes) {
			status = backupRolledBack
		}
		if recordErr := backup.updateStatus(status); recordErr != nil {
			err = errors.Join(err, fmt.Errorf("update recovery manifest: %w", recordErr))
		}
		return outcome, fmt.Errorf("migration write failed; recovery backup at %q: %w", backup.path, err)
	}
	outcome.FilesWritten = len(changes)
	if err := backup.updateStatus(backupApplied); err != nil {
		return outcome, fmt.Errorf("migration files were written, but recovery manifest update failed at %q: %w",
			backup.path, err)
	}
	return outcome, nil
}

func allOriginalsMatch(root string, changes []rewrite.Change) bool {
	for _, change := range changes {
		path := filepath.Join(root, filepath.FromSlash(change.Path))
		info, err := os.Lstat(path)
		if !change.Exists {
			if !errors.Is(err, fs.ErrNotExist) {
				return false
			}
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode().Perm() != change.Mode {
			return false
		}
		actual, err := os.ReadFile(path)
		if err != nil || !bytes.Equal(actual, change.Before) {
			return false
		}
	}
	return true
}
