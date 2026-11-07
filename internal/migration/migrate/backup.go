package migrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

const (
	backupManifestName     = "manifest.json"
	backupPrepared         = "prepared"
	backupApplied          = "applied"
	backupRolledBack       = "rolled-back"
	backupRecoveryRequired = "recovery-required"
)

// A manifest describes both modified and newly created paths. Existing files
// are saved in originals/<relative-path> with mode 0600 regardless of their
// original permissions (Terraform files can contain sensitive data).
type recoveryManifest struct {
	Version int          `json:"version"`
	Root    string       `json:"root"`
	Status  string       `json:"status"`
	Files   []backupFile `json:"files"`
}

type backupFile struct {
	Path       string `json:"path"`
	Existed    bool   `json:"existed"`
	Mode       uint32 `json:"mode"`
	BeforeHash string `json:"before_sha256,omitempty"`
	AfterHash  string `json:"after_sha256"`
}

type preparedBackup struct {
	path     string
	manifest recoveryManifest
}

// resolveBackupRoot rejects backups inside the Terraform tree. By default
// backup data lives in the user's home, never inside a version-controlled
// Terraform module.
func resolveBackupRoot(root, requested string) (string, error) {
	if requested == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory for migration backups: %w", err)
		}
		requested = filepath.Join(home, ".fastly-tf-migrate")
	}
	path, err := filepath.Abs(requested)
	if err != nil {
		return "", fmt.Errorf("resolve backup directory: %w", err)
	}
	// Resolve existing symlinks even when the requested backup leaf does not
	// exist yet. A symlinked parent must never redirect backups into a module.
	path, err = resolveExistingParents(path)
	if err != nil {
		return "", fmt.Errorf("resolve backup directory: %w", err)
	}
	if isWithinPath(root, path) {
		return "", fmt.Errorf("backup directory %q must be outside the Terraform root %q", path, root)
	}
	return path, nil
}

func resolveExistingParents(path string) (string, error) {
	var suffix []string
	current := path
	for {
		_, err := os.Lstat(current)
		switch {
		case err == nil:
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", err
			}
			for _, name := range suffix {
				resolved = filepath.Join(resolved, name)
			}
			return resolved, nil
		case errors.Is(err, fs.ErrNotExist):
			parent := filepath.Dir(current)
			if parent == current {
				return "", fmt.Errorf("cannot find an existing backup directory ancestor of %q", path)
			}
			suffix = append([]string{filepath.Base(current)}, suffix...)
			current = parent
		default:
			return "", err
		}
	}
}

func isWithinPath(parent, path string) bool {
	relative, err := filepath.Rel(parent, path)
	return err == nil && relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

func ensurePrivateDirectory(path string) error {
	if err := os.MkdirAll(path, 0o700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("backup directory %q is not a real directory", path)
	}
	// Windows does not expose POSIX group/other bits in the same way.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o077 != 0 {
		return fmt.Errorf("backup directory %q must not be accessible to other users (chmod 700)", path)
	}
	return nil
}

func projectName(root string) string {
	sum := sha256.Sum256([]byte(root))
	return hex.EncodeToString(sum[:])
}

// lockProject creates a private project directory and acquires an exclusive
// cross-process lock. A hard kill leaves the lock in place intentionally,
// requiring a user to inspect backups before authorizing another write.
func lockProject(base, root string) (string, func(), error) {
	if err := ensurePrivateDirectory(base); err != nil {
		return "", nil, fmt.Errorf("initialize backup directory: %w", err)
	}
	project := filepath.Join(base, "backups", projectName(root))
	for _, dir := range []string{filepath.Join(base, "backups"), project} {
		if err := ensurePrivateDirectory(dir); err != nil {
			return "", nil, fmt.Errorf("initialize project backups: %w", err)
		}
	}
	lockPath := filepath.Join(project, "migration.lock")
	lock, err := os.OpenFile(lockPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		if errors.Is(err, fs.ErrExist) {
			return "", nil, fmt.Errorf("migration lock %q already exists; verify no migration is running and inspect backups before removing a stale lock", lockPath)
		}
		return "", nil, fmt.Errorf("create migration lock: %w", err)
	}
	release := func() { _ = os.Remove(lockPath) }
	_, writeErr := fmt.Fprintf(lock, "root: %s\npid: %d\nstarted: %s\n",
		root, os.Getpid(), time.Now().UTC().Format(time.RFC3339))
	if writeErr == nil {
		writeErr = lock.Sync()
	}
	closeErr := lock.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		release()
		return "", nil, fmt.Errorf("write migration lock: %w", err)
	}
	return project, release, nil
}

// Never silently resume an interrupted multi-file replacement: the user must
// inspect the recovery manifest and reconcile the filesystem first.
func checkUnfinishedBackups(project, root string) error {
	entries, err := os.ReadDir(project)
	if err != nil {
		return fmt.Errorf("inspect migration backups: %w", err)
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		manifestPath := filepath.Join(project, entry.Name(), backupManifestName)
		data, err := os.ReadFile(manifestPath)
		if err != nil {
			return fmt.Errorf("incomplete or unreadable migration backup %q: %w; inspect before another write",
				filepath.Dir(manifestPath), err)
		}
		var manifest recoveryManifest
		if err := json.Unmarshal(data, &manifest); err != nil {
			return fmt.Errorf("invalid migration manifest %q: %w", manifestPath, err)
		}
		if manifest.Version != 1 {
			return fmt.Errorf("unsupported migration manifest version %d in %q", manifest.Version, manifestPath)
		}
		if manifest.Root != root {
			return fmt.Errorf("migration backup %q belongs to %q, not %q", manifestPath, manifest.Root, root)
		}
		switch manifest.Status {
		case backupApplied, backupRolledBack:
			continue
		case backupPrepared, backupRecoveryRequired:
			return fmt.Errorf("unfinished migration backup %q has status %q; restore/reconcile it before another write",
				filepath.Dir(manifestPath), manifest.Status)
		default:
			return fmt.Errorf("unknown migration backup status %q in %q", manifest.Status, manifestPath)
		}
	}
	return nil
}

func prepareBackup(ctx context.Context, project, root string, changes []rewrite.Change) (preparedBackup, error) {
	if err := ctx.Err(); err != nil {
		return preparedBackup{}, err
	}
	dir, err := os.MkdirTemp(project, "backup-*")
	if err != nil {
		return preparedBackup{}, err
	}
	complete := false
	defer func() {
		if !complete {
			_ = os.RemoveAll(dir) // No target Terraform files have been touched.
		}
	}()
	if err := os.Mkdir(filepath.Join(dir, "originals"), 0o700); err != nil {
		return preparedBackup{}, err
	}
	manifest := recoveryManifest{Version: 1, Root: root, Status: backupPrepared}
	for _, change := range changes {
		if err := ctx.Err(); err != nil {
			return preparedBackup{}, err
		}
		// The immutable rewrite plan supplies paths originating from the
		// inspected module tree. Reject any path that cannot be safely saved
		// as a relative backup path, even if that invariant changes later.
		relative := filepath.Clean(filepath.FromSlash(change.Path))
		if relative == "." || filepath.IsAbs(relative) ||
			relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
			return preparedBackup{}, fmt.Errorf("unsafe backup path %q", change.Path)
		}
		record := backupFile{
			Path: change.Path, Existed: change.Exists,
			Mode: uint32(change.Mode.Perm()), AfterHash: sha256String(change.After),
		}
		if change.Exists {
			record.BeforeHash = sha256String(change.Before)
			backupPath := filepath.Join(dir, "originals", relative)
			if err := os.MkdirAll(filepath.Dir(backupPath), 0o700); err != nil {
				return preparedBackup{}, err
			}
			if err := writePrivateFile(backupPath, change.Before); err != nil {
				return preparedBackup{}, fmt.Errorf("save original %q: %w", change.Path, err)
			}
		}
		manifest.Files = append(manifest.Files, record)
	}
	backup := preparedBackup{path: dir, manifest: manifest}
	if err := backup.updateStatus(backupPrepared); err != nil {
		return preparedBackup{}, err
	}
	// Persist the new backup directory entry before allowing Terraform
	// replacements to begin (best effort across supported filesystems).
	if err := syncDirectory(project); err != nil {
		return preparedBackup{}, fmt.Errorf("sync backup directory: %w", err)
	}
	complete = true
	return backup, nil
}

func sha256String(contents []byte) string {
	sum := sha256.Sum256(contents)
	return hex.EncodeToString(sum[:])
}

func writePrivateFile(path string, contents []byte) error {
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	n, writeErr := file.Write(contents)
	if writeErr == nil && n != len(contents) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = file.Sync()
	}
	return errors.Join(writeErr, file.Close())
}

func (b *preparedBackup) updateStatus(status string) error {
	b.manifest.Status = status
	data, err := json.MarshalIndent(b.manifest, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	// Write a complete, synced manifest and replace it in one rename.
	temp, err := os.CreateTemp(b.path, ".manifest-*.tmp")
	if err != nil {
		return err
	}
	path := temp.Name()
	n, writeErr := temp.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = io.ErrShortWrite
	}
	if writeErr == nil {
		writeErr = temp.Sync()
	}
	closeErr := temp.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		_ = os.Remove(path)
		return err
	}
	if err := os.Rename(path, filepath.Join(b.path, backupManifestName)); err != nil {
		_ = os.Remove(path)
		return err
	}
	return syncDirectory(b.path)
}

// Directory entry sync improves the chance that backup names and manifest
// replacements survive a sudden power loss. It cannot make a multi-file
// operation transactional. Windows does not provide the same directory-Sync
// semantics through os.File.
func syncDirectory(path string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	dir, err := os.Open(path)
	if err != nil {
		return err
	}
	return errors.Join(dir.Sync(), dir.Close())
}
