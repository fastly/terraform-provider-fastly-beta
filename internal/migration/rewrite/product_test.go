package rewrite

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRemoveProductEnablementAndGenerateImports(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	source := `resource "fastly_service_vcl" "demo" {
  name = "demo"
  # This comment is outside the removed product block.
  product_enablement {
    domain_inspector = true
  }
  force_destroy = true
}
`
	if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(source), 0o600); err != nil {
		t.Fatal(err)
	}
	plan, err := Build(context.Background(), dir,
		RemoveProductEnablement{File: "main.tf", ResourceName: "demo"},
		RenameResource{File: "main.tf", FromType: "fastly_service_vcl", ToType: "fastly_service_cdn_auto", Name: "demo"},
		AddSimpleProduct{
			File: "migration.tf", TypeName: "fastly_service_product_domain_inspector",
			Name: "demo", ServiceName: "demo", Enabled: true, ProviderRef: "fastly.edge",
		},
		AddImport{
			File: "migration.tf", To: "fastly_service_product_domain_inspector.demo",
			IDExpression: "fastly_service_cdn_auto.demo.id",
		},
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(plan.Changes()) != 2 {
		t.Fatalf("expected main.tf and migration.tf, got %#v", plan.Changes())
	}
	for _, change := range plan.Changes() {
		after := string(change.After)
		if change.Path == "main.tf" {
			if strings.Contains(after, "product_enablement") {
				t.Error("removed block remains")
			}
			if !strings.Contains(after, "# This comment is outside") {
				t.Error("external comment lost")
			}
			if !strings.Contains(after, `fastly_service_cdn_auto" "demo`) {
				t.Error("service was not renamed")
			}
		} else {
			for _, expected := range []string{
				`resource "fastly_service_product_domain_inspector" "demo"`,
				`service_id = fastly_service_cdn_auto.demo.id`,
				`provider   = fastly.edge`,
				`enabled    = true`,
				`id = fastly_service_cdn_auto.demo.id`,
			} {
				if !strings.Contains(after, expected) {
					t.Errorf("missing %q: %s", expected, after)
				}
			}
		}
	}
}

func TestRemoveProductEnablementRefusesCommentsAndTrailingContent(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, inner string }{
		{"comment inside", `product_enablement {
  # important
  domain_inspector = true
}`},
		{"trailing comment", `product_enablement { domain_inspector = true } # important`},
		{"block on attribute line", `name = "demo" product_enablement { domain_inspector = true }`},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := "resource \"fastly_service_vcl\" \"demo\" {\n" + test.inner + "\n}\n"
			if err := os.WriteFile(filepath.Join(dir, "main.tf"), []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			_, err := Build(context.Background(), dir, RemoveProductEnablement{File: "main.tf", ResourceName: "demo"})
			if err == nil {
				t.Fatal("expected removal to refuse unsafe block layout")
			}
			got, errRead := os.ReadFile(filepath.Join(dir, "main.tf"))
			if errRead != nil || string(got) != source {
				t.Fatalf("source modified: %v", errRead)
			}
		})
	}
}
