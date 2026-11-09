package rewrite

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApplyRollsBackPreviousFileAfterInstallFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	mainPath := filepath.Join(dir, "main.tf")
	before := []byte(`resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
	if err := os.WriteFile(mainPath, before, 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Build(context.Background(), dir,
		RenameResource{File: "main.tf", FromType: "fastly_service_vcl", ToType: "fastly_service_cdn_auto", Name: "demo"},
		AddMoved{File: "migration.tf", From: "fastly_service_vcl.demo", To: "fastly_service_cdn_auto.demo"},
	)
	if err != nil {
		t.Fatal(err)
	}
	injected := errors.New("injected second install failure")
	err = plan.applyWithInstall(context.Background(), func(src, dst string) error {
		if filepath.Base(dst) == "migration.tf" {
			return injected
		}
		return os.Rename(src, dst)
	})
	if !errors.Is(err, injected) {
		t.Fatalf("Apply error = %v; want injected install failure", err)
	}
	current, err := os.ReadFile(mainPath)
	if err != nil || !bytes.Equal(current, before) {
		t.Fatalf("previously installed source was not rolled back: %v\n%s", err, current)
	}
	if _, err := os.Stat(filepath.Join(dir, "migration.tf")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("failed install left generated HCL behind: %v", err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".fastly-tf-migrate-") {
			t.Fatalf("temporary staging file remained after rollback: %q", entry.Name())
		}
	}
}
