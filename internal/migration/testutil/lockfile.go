// Package testutil holds helpers shared by the migration packages' tests.
package testutil

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

const legacyLockfile = `provider "registry.terraform.io/fastly/fastly" {
  version = "9.1.0"
}
`

// SeedLegacyLockfile keeps tests unrelated to the source-version gate from being blocked by it.
func SeedLegacyLockfile(dir string) error {
	path := filepath.Join(dir, ".terraform.lock.hcl")
	_, err := os.Stat(path)
	if err == nil {
		return nil
	}
	if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return os.WriteFile(path, []byte(legacyLockfile), 0o600)
}
