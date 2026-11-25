package provider

import (
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
)

// These tests drive the MoveResourceState RPC with SDKv2-shaped JSON state for
// fastly_compute_acl.
const legacyComputeACLID = "4Gp9l2sRnzbfn6Ym0ZgK3v"

func requireComputeACLMoveRejected(t *testing.T, version int64, raw, contains string) {
	t.Helper()
	resp, err := moveDomainRPC(domainTestServer(t), "fastly_compute_acl", "fastly_acl", version, raw)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov6.DiagnosticSeverityError && d.Summary == "Cannot migrate fastly_compute_acl" &&
			strings.Contains(d.Detail, contains) {
			return
		}
	}
	t.Fatalf("expected an error containing %q, got %v", contains, resp.Diagnostics)
}

func TestLegacyComputeACLMoveState(t *testing.T) {
	server := domainTestServer(t)
	resp, err := moveDomainRPC(server, "fastly_compute_acl", "fastly_acl", 0, `{"id":"`+legacyComputeACLID+`","name":"blocklist"}`)
	if err != nil || rpcHasError(resp.Diagnostics) {
		t.Fatalf("MoveResourceState: %v %v", err, resp.Diagnostics)
	}
	attrs := targetAttrs(t, server, "fastly_acl", resp.TargetState)
	for name, want := range map[string]string{"id": legacyComputeACLID, "name": "blocklist"} {
		if got, _ := stringAttr(t, attrs, name); got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
}

func TestLegacyComputeACLMoveStateRejectsUnsupportedState(t *testing.T) {
	for name, tc := range map[string]struct {
		version       int64
		raw, contains string
	}{
		"future schema version": {1, `{"id":"` + legacyComputeACLID + `","name":"blocklist"}`, "This `fastly_compute_acl` state was saved by an unsupported provider release, so it can't be moved."},
		"missing id":            {0, `{"id":null,"name":"blocklist"}`, "has no `id`, so it can't be moved. Instead, remove it from state with a `removed` block (`lifecycle { destroy = false }`) or `terraform state rm`, then import it as `fastly_acl` with the ACL ID."},
		"empty id":              {0, `{"id":"","name":"blocklist"}`, "has no `id`"},
		"missing name":          {0, `{"id":"` + legacyComputeACLID + `","name":""}`, "has no `name`, so it can't be moved."},
		"missing name key":      {0, `{"id":"` + legacyComputeACLID + `"}`, "then import it as `fastly_acl` with the ID `" + legacyComputeACLID + "`."},
	} {
		t.Run(name, func(t *testing.T) {
			requireComputeACLMoveRejected(t, tc.version, tc.raw, tc.contains)
		})
	}
}

func TestLegacyComputeACLMoveStateIgnoresOtherSources(t *testing.T) {
	// A mover that does not recognize the source must not claim the state.
	for _, source := range []string{"fastly_compute_acl_entries", "fastly_compute_acl_v2"} {
		t.Run(source, func(t *testing.T) {
			resp, err := moveDomainRPC(domainTestServer(t), source, "fastly_acl", 0, `{"id":"x","name":"y"}`)
			if err == nil && !rpcHasError(resp.Diagnostics) {
				t.Fatal("expected an unsupported-source error")
			}
		})
	}
}

// This opt-in test compares the shape the fastly_compute_acl mover decodes
// with the schemas exported from the published registry binaries.
func TestLegacyComputeACLMoveStatePublishedSchemas(t *testing.T) {
	t.Parallel()

	dir := os.Getenv("FASTLY_LEGACY_SCHEMA_DIR")
	if dir == "" {
		t.Skip("run scripts/test-migration-state-schemas/run.sh to capture published schemas")
	}
	want := legacyServiceBlock{Attributes: map[string]any{"id": "string", "name": "string"}}
	for _, release := range []string{"9.1.0", "9.5.0", "9.8.0", "10.2.0", "10.3.0"} {
		t.Run(release, func(t *testing.T) {
			t.Parallel()
			actual := readPublishedLegacySchema(t, dir, release, "fastly_compute_acl")
			if actual.Version != 0 {
				t.Fatalf("fastly_compute_acl published schema version = %d, want 0", actual.Version)
			}
			comparePublishedLegacyBlock(t, "fastly_compute_acl", actual.Block, want)
		})
	}
}
