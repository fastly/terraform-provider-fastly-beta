package rewrite

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

type stagedFile struct {
	target         string
	after          string
	backup         string
	change         Change
	installed      bool
	preserveBackup bool
}

// Apply stages complete replacements beside their destinations, verifies that
// inputs still match the Build snapshot, and renames each staged file into
// place. Existing modes are preserved. If a later rename fails, previously
// installed files are restored from staged backups (best-effort rollback).
//
// On Unix-like local filesystems individual replacement renames are usually
// atomic; Go does not guarantee atomic Rename on Windows. Multi-file
// replacements are NOT a single filesystem transaction: interruptions and
// concurrent writers still require recovery or a higher-level lock.
func (p *Plan) Apply(ctx context.Context) error {
	if p == nil {
		return errors.New("nil rewrite plan")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(p.changes) == 0 {
		return nil
	}

	staged := make([]stagedFile, 0, len(p.changes))
	defer func() {
		for _, entry := range staged {
			if entry.after != "" {
				_ = os.Remove(entry.after)
			}
			if entry.backup != "" && !entry.preserveBackup {
				_ = os.Remove(entry.backup)
			}
		}
	}()

	// Check every original before creating any output file.
	for _, change := range p.changes {
		_, target, err := resolveFile(p.root, change.Path)
		if err != nil {
			return err
		}
		if err := matchesSnapshot(target, change); err != nil {
			return err
		}
	}

	// Stage all replacements and rollback copies before the first rename.
	for _, change := range p.changes {
		if err := ctx.Err(); err != nil {
			return err
		}
		_, target, err := resolveFile(p.root, change.Path)
		if err != nil {
			return err
		}
		entry := stagedFile{target: target, change: change}
		stageErr := error(nil)
		entry.after, stageErr = stageContents(filepath.Dir(target), change.After, change.Mode)
		if stageErr == nil && change.Exists {
			entry.backup, stageErr = stageContents(filepath.Dir(target), change.Before, change.Mode)
		}
		staged = append(staged, entry)
		if stageErr != nil {
			return fmt.Errorf("stage %q: %w", change.Path, stageErr)
		}
	}

	for i := range staged {
		if err := ctx.Err(); err != nil {
			return rollback(staged, err)
		}
		entry := &staged[i]
		if err := matchesSnapshot(entry.target, entry.change); err != nil {
			return rollback(staged, err)
		}
		// Go's os.Rename replaces an existing regular file on Windows
		// (MoveFileEx with MOVEFILE_REPLACE_EXISTING) as well as on Unix.
		// A locked Windows destination can still reject replacement.
		if err := os.Rename(entry.after, entry.target); err != nil {
			return rollback(staged, fmt.Errorf("install %q: %w", entry.change.Path, err))
		}
		entry.after = ""
		entry.installed = true
	}
	return nil
}

func matchesSnapshot(path string, change Change) error {
	info, err := os.Lstat(path)
	if !change.Exists {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		if err != nil {
			return fmt.Errorf("check new file %q: %w", change.Path, err)
		}
		return fmt.Errorf("file %q appeared after planning", change.Path)
	}
	if err != nil {
		return fmt.Errorf("check original %q: %w", change.Path, err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm() != change.Mode {
		return fmt.Errorf("original file %q changed mode or type since planning", change.Path)
	}
	current, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read original %q: %w", change.Path, err)
	}
	if !bytes.Equal(current, change.Before) {
		return fmt.Errorf("original file %q changed since planning", change.Path)
	}
	return nil
}

func stageContents(directory string, contents []byte, mode fs.FileMode) (string, error) {
	file, err := os.CreateTemp(directory, ".fastly-tf-migrate-*")
	if err != nil {
		return "", err
	}
	path := file.Name()
	_, writeErr := file.Write(contents)
	if writeErr == nil {
		writeErr = file.Chmod(mode)
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	closeErr := file.Close()
	if writeErr != nil {
		_ = os.Remove(path)
		return "", writeErr
	}
	if closeErr != nil {
		_ = os.Remove(path)
		return "", closeErr
	}
	return path, nil
}

func rollback(entries []stagedFile, cause error) error {
	var failures []error
	for i := len(entries) - 1; i >= 0; i-- {
		entry := entries[i]
		if !entry.installed {
			continue
		}
		if entry.change.Exists {
			if err := os.Rename(entry.backup, entry.target); err != nil {
				failures = append(failures, fmt.Errorf("restore %q: %w", entry.change.Path, err))
			}
		} else if err := os.Remove(entry.target); err != nil {
			failures = append(failures, fmt.Errorf("remove generated %q: %w", entry.change.Path, err))
		}
	}
	if len(failures) == 0 {
		return cause
	}
	// The backup paths must survive cleanup if rollback fails; callers need
	// those copies to restore their files manually.
	for i := range entries {
		entry := &entries[i]
		if !entry.installed || !entry.change.Exists {
			continue
		}
		if _, err := os.Stat(entry.backup); err == nil {
			entry.preserveBackup = true
			failures = append(failures, fmt.Errorf("recovery copy preserved at %q", entry.backup))
		}
	}
	return errors.Join(append([]error{cause}, failures...)...)
}
