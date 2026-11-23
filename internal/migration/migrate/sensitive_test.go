package migrate

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPlanSensitiveBackendObjects(t *testing.T) {
	t.Parallel()
	for _, family := range []struct{ legacy, target string }{
		{"fastly_service_vcl", "fastly_service_cdn_auto"},
		{"fastly_service_compute", "fastly_service_compute_auto"},
	} {
		t.Run(family.legacy, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := fmt.Sprintf(`resource %q "demo" {
  name = "demo"
  backend {
    name = "first"
    address = "example.com"
    ssl_client_cert = file(var.cert)
    ssl_client_key = local.secret
  }
  backend {
    name = "second"
    address = "example.net"
    ssl_client_key = "SYNTHETIC_LITERAL_KEY"
  }
  backend {
    name = "third"
    address = "example.org"
  }
}
`, family.legacy)
			// The same marker may appear in another modified file, or in
			// diff context. Entire-tree suppression must cover those too.
			references := fmt.Sprintf(`locals {
  secret = "SYNTHETIC_LOCAL_KEY"
}
# SYNTHETIC_COMMENT_KEY
output "id" {
  value = %s.demo.id
}
`, family.legacy)
			writeConfig(t, dir, "main.tf", source)
			writeConfig(t, dir, "references.tf", references)
			result := newPlan(t, dir)
			if result.Blocked() || len(result.Moves) != 1 || len(result.Changes) != 3 {
				t.Fatalf("sensitive migration blocked: %v %v", result.Issues, result.Analysis.Findings)
			}
			after := changedFile(t, result, "main.tf")
			for _, want := range []string{
				`resource "` + family.target + `" "demo"`,
				"ssl_client_secrets = {\n      ssl_client_cert = file(var.cert)\n      ssl_client_key = local.secret\n    }",
				"ssl_client_secrets = {\n      ssl_client_key = \"SYNTHETIC_LITERAL_KEY\"\n    }",
				"    name = \"third\"\n    address = \"example.org\"\n  }",
			} {
				if !strings.Contains(after, want) {
					t.Fatal("missing expected sensitive object structure")
				}
			}
			wantRefs := strings.ReplaceAll(references, family.legacy+".demo", family.target+".demo")
			if changedFile(t, result, "references.tf") != wantRefs {
				t.Fatal("reference transformation changed unrelated source")
			}
			for _, render := range []func(io.Writer, Result) error{WriteText, WriteApplyPreview} {
				var output bytes.Buffer
				if err := render(&output, result); err != nil {
					t.Fatal(err)
				}
				if strings.Contains(output.String(), "SYNTHETIC_") ||
					strings.Contains(output.String(), "+++ b/") ||
					!strings.Contains(output.String(), "HCL diff withheld") {
					t.Fatal("sensitive preview emitted source or omitted its safety notice")
				}
			}
			before, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(before) != source {
				t.Fatal("plan modified input")
			}
			applied, err := result.Apply(context.Background(), privateBackupDir(t))
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(got) != after {
				t.Fatal("write did not use original unredacted plan")
			}
			backup, err := os.ReadFile(filepath.Join(applied.BackupDir, "originals", "main.tf"))
			if err != nil || string(backup) != source {
				t.Fatal("recovery backup lost original expressions")
			}
			info, err := os.Stat(filepath.Join(applied.BackupDir, "originals", "main.tf"))
			if err != nil || (runtime.GOOS != "windows" && info.Mode().Perm() != 0o600) {
				t.Fatal("sensitive backup is not private")
			}
			again := newPlan(t, dir)
			if again.Blocked() || len(again.Changes) != 0 {
				t.Fatal("repeated migration was not a no-op")
			}
		})
	}
}

func TestPlanSensitiveBackendRejectsMixedAndUnsupportedShapes(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, body, want string }{
		{"object conflict", "    ssl_client_key = \"SYNTHETIC_SECRET\"\n    ssl_client_secrets = local.other\n", "already contains target ssl_client_secrets"},
		{"target only", "    ssl_client_secrets = { ssl_client_key = \"SYNTHETIC_SECRET\" }\n", "already contains target ssl_client_secrets"},
		{"block conflict", "    ssl_client_key = \"SYNTHETIC_SECRET\"\n    ssl_client_secrets {}\n", "no service-block compatibility rule"},
		{"invalid syntax", "    ssl_client_key = \"SYNTHETIC_SECRET\"\n    ssl_client_key = \"SECOND_SECRET\"\n", "Invalid native Terraform syntax"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := "resource \"fastly_service_vcl\" \"demo\" {\n  name = \"demo\"\n  backend {\n    name = \"origin\"\n    address = \"example.com\"\n" + tc.body + "  }\n}\n"
			writeConfig(t, dir, "main.tf", source)
			result := newPlan(t, dir)
			if !result.Blocked() || len(result.Changes) != 0 || len(result.Moves) != 0 {
				t.Fatal("migration accepted mixed or invalid sensitive HCL")
			}
			var output bytes.Buffer
			if err := WriteText(&output, result); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(output.String(), "SYNTHETIC_SECRET") ||
				strings.Contains(output.String(), "SECOND_SECRET") ||
				!strings.Contains(output.String(), tc.want) {
				t.Fatal("missing safe diagnostic or source material exposed")
			}
			if _, err := result.Apply(context.Background(), privateBackupDir(t)); err == nil {
				t.Fatal("blocked plan could be written")
			}
			got, err := os.ReadFile(filepath.Join(dir, "main.tf"))
			if err != nil || string(got) != source {
				t.Fatal("blocked migration changed source")
			}
		})
	}
}

func TestPlanSensitiveProviderCredentialsWithholdDiff(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeConfig(t, dir, "main.tf", `provider "fastly" {
  api_key = "SYNTHETIC_PROVIDER_CREDENTIAL"
}
resource "fastly_service_vcl" "demo" {
  name = "demo"
}
`)
	result := newPlan(t, dir)
	if result.Blocked() {
		t.Fatal("provider credential expression migration unexpectedly blocked")
	}
	var out bytes.Buffer
	if err := WriteText(&out, result); err != nil {
		t.Fatal(err)
	}
	if strings.Contains(out.String(), "SYNTHETIC_PROVIDER_CREDENTIAL") ||
		!strings.Contains(out.String(), "HCL diff withheld") {
		t.Fatal("provider credential was not protected")
	}
	if !strings.Contains(changedFile(t, result, "main.tf"), `api_token = "SYNTHETIC_PROVIDER_CREDENTIAL"`) {
		t.Fatal("provider credential expression not preserved")
	}
}
