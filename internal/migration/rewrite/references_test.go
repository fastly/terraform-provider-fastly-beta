package rewrite

import (
	"bytes"
	"strings"
	"testing"

	"github.com/fastly/terraform-provider-fastly-beta/internal/migration/compatibility"
)

func referenceTestEdit() References {
	return References{
		File:     "references.tf",
		Registry: compatibility.Default(),
		Routes: map[string]ReferenceTarget{
			"fastly_service_vcl.demo":     {To: "fastly_service_cdn_auto.demo", Indexed: true, Settings: true},
			"fastly_service_vcl.value":    {To: "fastly_service_cdn_auto.value"},
			"fastly_service_compute.demo": {To: "fastly_service_compute_auto.demo", Indexed: true},
		},
		AllRoutes: map[string]bool{
			"fastly_service_vcl.demo": true, "fastly_service_cdn_auto.demo": true,
			"fastly_service_compute.demo": true, "fastly_service_compute_auto.demo": true,
		},
	}
}

func TestRewriteReferencesExpressions(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, before, after string
	}{
		{"direct", `fastly_service_vcl.demo.id`, `fastly_service_cdn_auto.demo.id`},
		{"explicit resource namespace", `resource.fastly_service_vcl.demo.id`, `resource.fastly_service_cdn_auto.demo.id`},
		{"static index", `fastly_service_vcl.demo[0].active_version`, `fastly_service_cdn_auto.demo[0].active_version`},
		{"string key", `fastly_service_compute.demo["prod"].id`, `fastly_service_compute_auto.demo["prod"].id`},
		{"dynamic index", `fastly_service_vcl.demo[var.index].id`, `fastly_service_cdn_auto.demo[var.index].id`},
		{"each", `fastly_service_compute.demo[each.key].name`, `fastly_service_compute_auto.demo[each.key].name`},
		{"count", `fastly_service_vcl.demo[count.index].id`, `fastly_service_cdn_auto.demo[count.index].id`},
		{"index has reference", `fastly_service_compute.demo[fastly_service_vcl.demo[0].name].id`, `fastly_service_compute_auto.demo[fastly_service_cdn_auto.demo[0].name].id`},
		{"modern splat", `fastly_service_vcl.demo[*].id`, `fastly_service_cdn_auto.demo[*].id`},
		{"legacy splat", `fastly_service_vcl.demo.*.active_version`, `fastly_service_cdn_auto.demo.*.active_version`},
		{"conditional", `var.use_vcl ? fastly_service_vcl.demo[0].id : fastly_service_compute.demo["p"].id`, `var.use_vcl ? fastly_service_cdn_auto.demo[0].id : fastly_service_compute_auto.demo["p"].id`},
		{"function", `join(",", [fastly_service_vcl.demo[0].id, fastly_service_compute.demo["p"].id])`, `join(",", [fastly_service_cdn_auto.demo[0].id, fastly_service_compute_auto.demo["p"].id])`},
		{"parentheses", `(fastly_service_vcl.demo[0]).id`, `(fastly_service_cdn_auto.demo[0]).id`},
		{"interpolation", `"svc:${fastly_service_compute.demo["prod"].id}"`, `"svc:${fastly_service_compute_auto.demo["prod"].id}"`},
		{"heredoc", "<<EOT\nservice=${fastly_service_vcl.demo[0].id}\nEOT\n", "<<EOT\nservice=${fastly_service_cdn_auto.demo[0].id}\nEOT\n"},
		{"escaped interpolation", `"$${fastly_service_vcl.demo.id}"`, `"$${fastly_service_vcl.demo.id}"`},
		{"literal string", `"fastly_service_vcl.demo.id"`, `"fastly_service_vcl.demo.id"`},
		{"object key", `{ "fastly_service_vcl.demo.id" = fastly_service_vcl.demo[0].id }`, `{ "fastly_service_vcl.demo.id" = fastly_service_cdn_auto.demo[0].id }`},
		{"computed key", `{ (fastly_service_vcl.demo[0].id) = 1 }`, `{ (fastly_service_cdn_auto.demo[0].id) = 1 }`},
		{"data source", `data.fastly_service_vcl.demo.id`, `data.fastly_service_vcl.demo.id`},
		{"unmapped resource", `fastly_service_vcl.other.id`, `fastly_service_vcl.other.id`},
		{"module outputs", `module.edge["prod"].service_id`, `module.edge["prod"].service_id`},
		{"lexical shadow", `[for fastly_service_vcl in var.objects : fastly_service_vcl.demo.id]`, `[for fastly_service_vcl in var.objects : fastly_service_vcl.demo.id]`},
		{"shadowed collection and external value", `[for fastly_service_vcl in var.objects : "${fastly_service_vcl.demo.id}:${fastly_service_compute.demo["prod"].id}"]`, `[for fastly_service_vcl in var.objects : "${fastly_service_vcl.demo.id}:${fastly_service_compute_auto.demo["prod"].id}"]`},
		{"for scalar", `[for k in var.keys : fastly_service_compute.demo[k].id]`, `[for k in var.keys : fastly_service_compute_auto.demo[k].id]`},
		{"for instance collection", `{for key, svc in fastly_service_compute.demo : key => svc.id}`, `{for key, svc in fastly_service_compute_auto.demo : key => svc.id}`},
		{"count projection", `[for svc in fastly_service_vcl.demo : svc.id]`, `[for svc in fastly_service_cdn_auto.demo : svc.id]`},
		{"ACL ID map", `{for acl in fastly_service_vcl.demo[0].acl : acl.name => acl.acl_id}`, `{for acl in fastly_service_cdn_auto.demo[0].acl : acl.name => acl.acl_id}`},
		{"dictionary ID map", `lookup({for d in fastly_service_compute.demo["prod"].dictionary : d.name => d.dictionary_id}, "config")`, `lookup({for d in fastly_service_compute_auto.demo["prod"].dictionary : d.name => d.dictionary_id}, "config")`},
		{"snippet rename", `{for s in fastly_service_vcl.demo[0].dynamicsnippet : s.name => s.snippet_id}`, `{for s in fastly_service_cdn_auto.demo[0].dynamic_snippet : s.name => s.snippet_id}`},
		{"rate limiter ID rename", `{for r in fastly_service_vcl.demo[0].rate_limiter : r.name => r.ratelimiter_id}`, `{for r in fastly_service_cdn_auto.demo[0].rate_limiter : r.name => r.rate_limiter_id}`},
		{"settings path", `fastly_service_vcl.demo[0].default_ttl`, `fastly_service_cdn_auto.demo[0].settings[0].default_ttl`},
		{"settings dynamic index", `fastly_service_vcl.demo[var.idx].http3`, `fastly_service_cdn_auto.demo[var.idx].settings[0].http3`},
		{"multiple references", `[fastly_service_vcl.demo[0].id, fastly_service_vcl.demo[1].id]`, `[fastly_service_cdn_auto.demo[0].id, fastly_service_cdn_auto.demo[1].id]`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			prefix := "# fastly_service_vcl.demo.id is documentation, not an expression\nlocals {\n  value = "
			suffix := "\n}\n"
			edit := referenceTestEdit()
			before := []byte(prefix + tc.before + suffix)
			want := []byte(prefix + tc.after + suffix)
			got, err := edit.apply(before, edit.File)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("got:\n%s\nwant:\n%s", got, want)
			}
			// No changes to comments or strings on a second pass.
			again, err := edit.apply(got, edit.File)
			if err != nil || !bytes.Equal(got, again) {
				t.Fatalf("second pass not idempotent: %v\n%s", err, again)
			}
		})
	}
}

func TestRewriteReferencesMetaArgumentsAndDynamicScope(t *testing.T) {
	t.Parallel()
	source := `resource "Fastly_not_migrated" "consumer" {
  provider = fastly.edge
  depends_on = [fastly_service_vcl.demo]
  lifecycle {
    replace_triggered_by = [fastly_service_vcl.demo[0].id]
    ignore_changes = [fastly_service_vcl.demo]
  }
  dynamic "example" {
    iterator = fastly_service_vcl
    for_each = [fastly_service_vcl.value.id]
    content {
      name = fastly_service_vcl.value.id
      id = fastly_service_compute.demo["p"].id
    }
  }
}
module "consumer" {
  source = "./consumer"
  providers = { fastly = fastly.edge }
  service_id = fastly_service_vcl.demo[0].id
}
`
	want := strings.NewReplacer(
		"depends_on = [fastly_service_vcl.demo]", "depends_on = [fastly_service_cdn_auto.demo]",
		"replace_triggered_by = [fastly_service_vcl.demo[0].id]", "replace_triggered_by = [fastly_service_cdn_auto.demo[0].id]",
		"for_each = [fastly_service_vcl.value.id]", "for_each = [fastly_service_cdn_auto.value.id]",
		`id = fastly_service_compute.demo["p"].id`, `id = fastly_service_compute_auto.demo["p"].id`,
		"service_id = fastly_service_vcl.demo[0].id", "service_id = fastly_service_cdn_auto.demo[0].id",
	).Replace(source)
	edit := referenceTestEdit()
	got, err := edit.apply([]byte(source), edit.File)
	if err != nil || string(got) != want {
		t.Fatalf("meta/dynamic rewrite: %v\n%s\nwant:\n%s", err, got, want)
	}
}

func TestRewriteReferencesLifecycleRelativePaths(t *testing.T) {
	t.Parallel()
	edit := referenceTestEdit()
	source := `resource "fastly_service_vcl" "demo" {
  name = "test"
  default_ttl = 3600
  lifecycle {
    ignore_changes = [comment, default_ttl]
    postcondition {
      condition = self.default_ttl == 3600 && self.id != ""
      error_message = "expected service"
    }
  }
}
`
	want := strings.NewReplacer("comment, default_ttl", "comment, settings[0].default_ttl",
		"self.default_ttl", "self.settings[0].default_ttl").Replace(source)
	got, err := edit.apply([]byte(source), edit.File)
	if err != nil || string(got) != want {
		t.Fatalf("relative paths: %v\n%s\nwant:\n%s", err, got, want)
	}
}

func TestRewriteReferencesRejectsUnverifiedExpressions(t *testing.T) {
	t.Parallel()
	for _, value := range []string{
		`fastly_service_vcl.demo`,
		`fastly_service_vcl.demo[0]`,
		`fastly_service_vcl.demo[0].activate`,
		`fastly_service_vcl.demo[var.idx].cloned_version`,
		`fastly_service_compute.demo[each.key].staged_version`,
		`fastly_service_compute.demo[*].force_refresh`,
		`fastly_service_vcl.demo.*.default_ttl`,
		`fastly_service_vcl.demo[0].backend`,
		`fastly_service_vcl.demo[0].backend[0].address`,
		`fastly_service_vcl.demo[0].product_enablement`,
		`[for acl in fastly_service_vcl.demo[0].acl : acl.acl_id]`,
		`{for i, acl in fastly_service_vcl.demo[0].acl : i => acl.acl_id}`,
		`{for acl in fastly_service_vcl.demo[0].acl : acl.name => acl}`,
		`{for acl in fastly_service_vcl.demo[0].acl : acl.name => acl.acl_id if var.enabled}`,
		`{for k, svc in fastly_service_compute.demo : k => svc}`,
		`[for svc in fastly_service_compute.demo : svc.cloned_version]`,
		`fastly_service_vcl.value.default_ttl`,
		`fastly_service_vcl.demo[*].acl`,
		`jsonencode(fastly_service_compute.demo["prod"])`,
	} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()
			edit := referenceTestEdit()
			source := []byte("locals {\n  value = " + value + "\n  good = fastly_service_vcl.demo[0].id\n}\n")
			got, err := edit.apply(source, edit.File)
			if err == nil || !strings.Contains(err.Error(), "reference") || got != nil {
				t.Fatalf("expected whole edit to fail closed: got=%s err=%v", got, err)
			}
		})
	}
}

func TestRewriteReferencesMigrationConstructs(t *testing.T) {
	t.Parallel()
	edit := referenceTestEdit()
	exact := `moved {
  from = fastly_service_vcl.demo
  to = fastly_service_cdn_auto.demo
}
`
	got, err := edit.apply([]byte(exact), edit.File)
	if err != nil || string(got) != exact {
		t.Fatalf("historic move changed: %v\n%s", err, got)
	}
	for _, source := range []string{
		"moved {\n  from = fastly_service_vcl.demo[0]\n  to = fastly_service_cdn_auto.demo[0]\n}\n",
		"moved {\n  from = fastly_service_vcl.demo\n  to = fastly_service_cdn_auto.other\n}\n",
		"moved {\n  from = module.edge[0].fastly_service_vcl.demo\n  to = module.edge[0].fastly_service_cdn_auto.demo\n}\n",
		"import {\n  to = fastly_service_vcl.demo[0]\n  id = \"test\"\n}\n",
		"import {\n  for_each = var.keys\n  to = module.edge[each.key].fastly_service_compute.demo[each.key]\n  id = each.value\n}\n",
		"removed {\n  from = fastly_service_vcl.demo\n}\n",
	} {
		got, err := edit.apply([]byte(source), edit.File)
		if err == nil || got != nil {
			t.Fatalf("ambiguous state construct accepted: %v\n%s", err, source)
		}
	}
	// References in the import ID, not the address, are ordinary expressions.
	source := "import {\n  to = terraform_data.other\n  id = fastly_service_vcl.demo[0].id\n}\n"
	got, err = edit.apply([]byte(source), edit.File)
	if err != nil || string(got) != strings.ReplaceAll(source, "fastly_service_vcl.demo", "fastly_service_cdn_auto.demo") {
		t.Fatalf("import ID rewrite: %v\n%s", err, got)
	}
}

func TestRewriteReferencesAppliedRenameToReferenceOnlyRoute(t *testing.T) {
	t.Parallel()
	edit := References{
		File:     "references.tf",
		Registry: compatibility.Default(),
		Routes: map[string]ReferenceTarget{
			"fastly_domain.example":              {To: "fastly_domain.example"},
			"fastly_domain_service_link.example": {To: "fastly_domain_service_link.example"},
		},
		AllRoutes: map[string]bool{"fastly_domain.example": true, "fastly_domain_service_link.example": true},
	}
	for _, source := range []string{
		"moved {\n  from = fastly_domain_v1.example\n  to = fastly_domain.example\n}\n",
		"moved {\n  from = fastly_domain_v1_service_link.example\n  to = fastly_domain_service_link.example\n}\n",
	} {
		got, err := edit.apply([]byte(source), edit.File)
		if err != nil || string(got) != source {
			t.Fatalf("applied rename rejected or changed: %v\n%s", err, got)
		}
	}
	for name, source := range map[string]string{
		"different name":          "moved {\n  from = fastly_domain_v1.other\n  to = fastly_domain.example\n}\n",
		"unrelated source type":   "moved {\n  from = fastly_dns_zone.example\n  to = fastly_domain.example\n}\n",
		"leaves the route":        "moved {\n  from = fastly_domain.example\n  to = fastly_domain_v1.example\n}\n",
		"indexed destination":     "moved {\n  from = fastly_domain_v1.example\n  to = fastly_domain.example[0]\n}\n",
		"wrong target type":       "moved {\n  from = fastly_domain_v1.example\n  to = fastly_domain_service_link.example\n}\n",
		"removed at route":        "removed {\n  from = fastly_domain.example\n}\n",
		"import into route":       "import {\n  to = fastly_domain.example\n  id = \"test\"\n}\n",
		"module-qualified rename": "moved {\n  from = module.edge.fastly_domain_v1.example\n  to = module.edge.fastly_domain.example\n}\n",
	} {
		if got, err := edit.apply([]byte(source), edit.File); err == nil || got != nil {
			t.Errorf("%s: conflicting state construct accepted:\n%s", name, source)
		}
	}
}
