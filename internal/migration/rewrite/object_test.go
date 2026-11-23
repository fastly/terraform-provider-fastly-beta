package rewrite

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/hcl/v2/hclsyntax"
)

func TestNestObjectPreservesExpressionsAndParentScope(t *testing.T) {
	t.Parallel()
	source := `resource "example" "demo" {
  backend {
    name = "one"
    # first credential
    old_cert = file(var.paths[each.key]) # keep this comment
    address = "unchanged"
    old_key = (
      var.enabled ? local.key : null
    )
  }
  backend {
    name = "two"
    old_key = <<-PEM
      synthetic-key
        second line
    PEM
  }
  backend {
    name = "three"
    address = "untouched"
  }
}
resource "example" "other" {
  backend {
    old_key = "not part of this edit"
  }
}
`
	want := `resource "example" "demo" {
  backend {
    name = "one"
    secrets = {
    # first credential
      cert = file(var.paths[each.key]) # keep this comment
      key = (
      var.enabled ? local.key : null
    )
    }
    address = "unchanged"
  }
  backend {
    name = "two"
    secrets = {
      key = <<-PEM
      synthetic-key
        second line
    PEM
    }
  }
  backend {
    name = "three"
    address = "untouched"
  }
}
resource "example" "other" {
  backend {
    old_key = "not part of this edit"
  }
}
`
	edit := NestObject{
		File: "main.tf", ResourceType: "example", ResourceName: "demo",
		Parents: []string{"backend"}, Target: "secrets",
		Fields: []ObjectField{{From: "old_key", To: "key"}, {From: "old_cert", To: "cert"}},
	}
	got, err := edit.apply([]byte(source), "main.tf")
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != want {
		t.Fatalf("object rewrite:\n%s\nwant:\n%s", got, want)
	}
	// Parse the rendered object, not just a string containing a brace.
	before, err := requireResource([]byte(source), "main.tf", "example", "demo")
	if err != nil {
		t.Fatal(err)
	}
	after, err := requireResource(got, "main.tf", "example", "demo")
	if err != nil {
		t.Fatal(err)
	}
	for i := range 2 {
		previous := before.Body.Blocks[i].Body
		current := after.Body.Blocks[i].Body
		object, ok := current.Attributes["secrets"].Expr.(*hclsyntax.ObjectConsExpr)
		if !ok {
			t.Fatal("target is not an object expression")
		}
		for _, field := range object.Items {
			// Only tests evaluate synthetic object keys, never the migrator.
			key, diags := field.KeyExpr.Value(nil)
			if diags.HasErrors() {
				t.Fatal("invalid object key")
			}
			original := previous.Attributes["old_"+key.AsString()]
			r1, r2 := original.Expr.Range(), field.ValueExpr.Range()
			if !bytes.Equal([]byte(source)[r1.Start.Byte:r1.End.Byte], got[r2.Start.Byte:r2.End.Byte]) {
				t.Fatal("expression bytes changed")
			}
		}
	}
	again, err := edit.apply(got, "main.tf")
	if err != nil || !bytes.Equal(got, again) {
		t.Fatal("object writer is not idempotent")
	}
}

func TestNestObjectTopLevelAndLiteralNull(t *testing.T) {
	t.Parallel()
	source := "resource \"example\" \"demo\" {\r\n  old = null\r\n  other = \"literal\"\r\n}\r\n"
	want := "resource \"example\" \"demo\" {\r\n  authentication = {\r\n    new_key = null\r\n  }\r\n  other = \"literal\"\r\n}\r\n"
	edit := NestObject{
		ResourceType: "example", ResourceName: "demo", Target: "authentication",
		Fields: []ObjectField{{From: "old", To: "new_key"}},
	}
	got, err := edit.apply([]byte(source), "main.tf")
	if err != nil || string(got) != want {
		t.Fatalf("top-level object: %v\n%s", err, got)
	}
	if _, err := parseBody(got, "main.tf"); err != nil {
		t.Fatal(err)
	}
}

func TestNestObjectRejectsAmbiguousLayoutsWithoutWriting(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name, body, want string
		fields           []ObjectField
	}{
		{"existing object", "  old = \"synthetic-marker\"\n  secrets = local.credentials\n", "already exists", nil},
		{"existing block", "  old = \"synthetic-marker\"\n  secrets {}\n", "conflicts", nil},
		{"duplicate target", "  old = \"synthetic-marker\"\n", "ambiguous", []ObjectField{{"old", "same"}, {"another", "same"}}},
		{"duplicate source", "  old = \"synthetic-marker\"\n", "ambiguous", []ObjectField{{"old", "one"}, {"old", "two"}}},
		{"inline attribute", " old = \"synthetic-marker\" ", "own line", nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			source := "resource \"example\" \"demo\" {\n" + tc.body + "}\n"
			if tc.name == "inline attribute" {
				source = "resource \"example\" \"demo\" {" + tc.body + "}\n"
			}
			path := filepath.Join(dir, "main.tf")
			if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
				t.Fatal(err)
			}
			fields := tc.fields
			if fields == nil {
				fields = []ObjectField{{"old", "key"}}
			}
			plan, err := Build(context.Background(), dir, NestObject{
				File: "main.tf", ResourceType: "example", ResourceName: "demo", Target: "secrets", Fields: fields,
			})
			if err == nil || plan != nil || !strings.Contains(err.Error(), tc.want) ||
				strings.Contains(err.Error(), "synthetic-marker") {
				t.Fatal("unsafe or revealing object rewrite")
			}
			got, err := os.ReadFile(path)
			if err != nil || string(got) != source {
				t.Fatal("failed edit modified configuration")
			}
		})
	}
}
