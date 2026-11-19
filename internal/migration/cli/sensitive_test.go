package cli

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMigrateSensitiveBackendTransaction(t *testing.T) {
	t.Parallel()
	for _, blocked := range []bool{false, true} {
		name := "supported"
		if blocked {
			name = "blocked"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir, backups := t.TempDir(), privateBackupDir(t)
			source := `resource "fastly_service_vcl" "demo" {
  name = "demo"
  backend {
    name = "origin"
    address = "example.com"
    ssl_client_cert = file("nonexistent-client-certificate.pem")
    ssl_client_key = <<-PEM
      SYNTHETIC_PRIVATE_KEY_MARKER
    PEM
`
			if blocked {
				source += "    ssl_client_secrets = { ssl_client_key = \"SYNTHETIC_CONFLICT_MARKER\" }\n"
			}
			source += "  }\n}\n"
			main := filepath.Join(dir, "main.tf")
			if err := writeConfigFile(main, []byte(source)); err != nil {
				t.Fatal(err)
			}
			refs := `locals {
  secret = "SYNTHETIC_LOCAL_MARKER"
}
output "id" {
  value = fastly_service_vcl.demo.id
}
`
			refFile := filepath.Join(dir, "references.tf")
			if err := writeConfigFile(refFile, []byte(refs)); err != nil {
				t.Fatal(err)
			}
			lock := filepath.Join(dir, ".terraform.lock.hcl")
			lockBefore, err := os.ReadFile(lock)
			if err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{
				{"analyze"},
				{"migrate", "--dry-run"},
				{"migrate", "--write", "--backup-dir", backups},
			} {
				var stdout, stderr bytes.Buffer
				cmd := NewCommand(Config{Writer: &stdout, ErrWriter: &stderr})
				err := cmd.Run(context.Background(), append(append([]string{commandName}, args...), dir))
				if blocked {
					var code interface{ ExitCode() int }
					if !errors.As(err, &code) || code.ExitCode() != exitCodeAnalysisBlocked {
						t.Fatal("unsupported sensitive migration did not block")
					}
				} else if err != nil {
					t.Fatal("supported sensitive migration failed")
				}
				if strings.Contains(stdout.String()+stderr.String(), "SYNTHETIC_") ||
					(err != nil && strings.Contains(err.Error(), "SYNTHETIC_")) {
					t.Fatal("secret material appeared in command output")
				}
				if !blocked && args[0] == "migrate" && !strings.Contains(stdout.String(), "HCL diff withheld") {
					t.Fatal("missing sensitive preview notice")
				}
				if blocked || args[0] == "analyze" || args[1] == "--dry-run" {
					for path, want := range map[string]string{main: source, refFile: refs} {
						got, readErr := os.ReadFile(path) // #nosec G304 G703 -- test fixture paths
						if readErr != nil || string(got) != want {
							t.Fatal("preflight modified source")
						}
					}
					if _, statErr := os.Stat(filepath.Join(dir, "migration.tf")); !os.IsNotExist(statErr) {
						t.Fatal("preflight created a migration file")
					}
				}
			}
			lockAfter, err := os.ReadFile(lock)
			if err != nil || !bytes.Equal(lockBefore, lockAfter) {
				t.Fatal("migration modified provider lockfile")
			}
			if blocked {
				entries, err := os.ReadDir(backups)
				if err != nil || len(entries) != 0 {
					t.Fatal("blocked migration created recovery artifacts")
				}
				return
			}
			got, err := os.ReadFile(main)
			if err != nil ||
				!bytes.Contains(got, []byte("ssl_client_secrets = {")) ||
				!bytes.Contains(got, []byte("file(\"nonexistent-client-certificate.pem\")")) ||
				!bytes.Contains(got, []byte("      SYNTHETIC_PRIVATE_KEY_MARKER\n    PEM")) {
				t.Fatal("written plan lost a credential expression")
			}
			afterRefs, err := os.ReadFile(refFile)
			if err != nil || string(afterRefs) != strings.ReplaceAll(refs, "fastly_service_vcl.", "fastly_service_cdn_auto.") {
				t.Fatal("written plan lost a dependent reference")
			}
		})
	}
}

func TestMigrateStandaloneSensitiveResourcesRemainBlocked(t *testing.T) {
	t.Parallel()
	for _, typ := range []string{"fastly_tls_private_key", "fastly_integration"} {
		t.Run(typ, func(t *testing.T) {
			t.Parallel()
			dir, backups := t.TempDir(), privateBackupDir(t)
			source := []byte("resource \"" + typ + "\" \"demo\" {\n  name = \"demo\"\n}\n")
			path := filepath.Join(dir, "main.tf")
			if err := writeConfigFile(path, source); err != nil {
				t.Fatal(err)
			}
			var stdout bytes.Buffer
			cmd := NewCommand(Config{Writer: &stdout, ErrWriter: &stdout})
			err := cmd.Run(context.Background(), []string{commandName, "migrate", "--write", "--backup-dir", backups, dir})
			var code interface{ ExitCode() int }
			if !errors.As(err, &code) || code.ExitCode() != exitCodeAnalysisBlocked ||
				!strings.Contains(stdout.String(), typ) {
				t.Fatal("standalone migration was not explicitly blocked")
			}
			got, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(got, source) {
				t.Fatal("unsupported standalone configuration changed")
			}
		})
	}
}
