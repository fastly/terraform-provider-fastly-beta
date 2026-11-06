package migrate

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/rewrite"
)

func TestWriteDiffGolden(t *testing.T) {
	t.Parallel()
	changes := []rewrite.Change{
		{
			Path:  "migration.tf",
			After: []byte("moved {\n  from = fastly_service_vcl.demo\n  to   = fastly_service_cdn_auto.demo\n}\n"),
		},
		{
			Path:   "main.tf",
			Exists: true,
			Before: []byte("resource \"fastly_service_vcl\" \"demo\" {\n  name = var.name\n}\n"),
			After:  []byte("resource \"fastly_service_cdn_auto\" \"demo\" {\n  name = var.name\n}\n"),
		},
	}
	var buf bytes.Buffer
	if err := WriteDiff(&buf, changes); err != nil {
		t.Fatal(err)
	}
	want := `--- a/main.tf
+++ b/main.tf
@@ -1,3 +1,3 @@
-resource "fastly_service_vcl" "demo" {
+resource "fastly_service_cdn_auto" "demo" {
   name = var.name
 }
--- /dev/null
+++ b/migration.tf
@@ -0,0 +1,4 @@
+moved {
+  from = fastly_service_vcl.demo
+  to   = fastly_service_cdn_auto.demo
+}
`
	if got := buf.String(); got != want {
		t.Fatalf("diff:\n%s\nwant:\n%s", got, want)
	}
}

func TestWriteDiffMultipleHunksAndNoTrailingNewline(t *testing.T) {
	t.Parallel()
	var before, after strings.Builder
	for i := 0; i < 40; i++ {
		line := "line " + string(rune('A'+i))
		if i == 2 || i == 37 {
			before.WriteString(line + " old\n")
			after.WriteString(line + " new\n")
		} else {
			before.WriteString(line + "\n")
			after.WriteString(line + "\n")
		}
	}
	var buf bytes.Buffer
	if err := WriteDiff(&buf, []rewrite.Change{{
		Path: "large.tf", Exists: true,
		Before: []byte(before.String()), After: []byte(after.String()),
	}}); err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(buf.String(), "@@ -"); got != 2 {
		t.Fatalf("expected two separate hunks, got %d:\n%s", got, buf.String())
	}
	buf.Reset()
	if err := WriteDiff(&buf, []rewrite.Change{{
		Path: "main.tf", Exists: true,
		Before: []byte("name = old"), After: []byte("name = new"),
	}}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\\ No newline at end of file") {
		t.Fatalf("missing no-newline marker:\n%s", buf.String())
	}
}

func TestWriteDiffHandlesLargeUnchangedFileWithLocalizedEdit(t *testing.T) {
	t.Parallel()
	var before strings.Builder
	for i := 0; i < 5000; i++ {
		before.WriteString("value = var.value\n")
	}
	source := before.String()
	after := strings.Replace(source, "value = var.value\n",
		"value = local.value\n", 1)
	steps := diffLines(splitLines(source), splitLines(after))
	edits := 0
	for _, step := range steps {
		if step.kind != ' ' {
			edits++
		}
	}
	if edits != 2 {
		t.Fatalf("large file localized edit yielded %d changes, want 2", edits)
	}
}

// TestWriteDiffSkipsUnchangedBytes verifies that byte-identical file contents
// produce no diff even when held in separately allocated byte slices.
func TestWriteDiffSkipsUnchangedBytes(t *testing.T) {
	t.Parallel()

	before := []byte("resource \"fastly_service_vcl\" \"demo\" {}\n")
	after := bytes.Clone(before)
	var output bytes.Buffer
	err := WriteDiff(&output, []rewrite.Change{
		{Path: "main.tf", Exists: true, Before: before, After: after},
	})
	if err != nil {
		t.Fatal(err)
	}
	if output.Len() != 0 {
		t.Fatalf("unchanged bytes produced a diff:\n%s", output.String())
	}
}
