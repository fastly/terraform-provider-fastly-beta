package resourceidentity

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
)

func TestIdentitySchemasAreStable(t *testing.T) {
	tests := map[string]struct {
		want   map[string]bool
		schema identityschema.Schema
	}{
		"service": {
			want:   map[string]bool{"service_id": true},
			schema: ServiceSchema(),
		},
		"named versioned": {
			want:   map[string]bool{"service_id": true, "name": true},
			schema: NamedVersionedSchema(),
		},
		"service scoped versioned": {
			want:   map[string]bool{"service_id": true},
			schema: ServiceScopedVersionedSchema(),
		},
		"resource link": {
			want:   map[string]bool{"service_id": true, "resource_id": true},
			schema: ResourceLinkSchema(),
		},
		"ACL collection": {
			want:   map[string]bool{"service_id": true, "acl_id": true},
			schema: ACLCollectionSchema(),
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if len(tt.schema.Attributes) != len(tt.want) {
				t.Fatalf("got %d identity attributes, want %d", len(tt.schema.Attributes), len(tt.want))
			}
			if _, ok := tt.schema.Attributes["version"]; ok {
				t.Fatal("version must never be part of resource identity")
			}
			for attr := range tt.schema.Attributes {
				if !tt.want[attr] {
					t.Errorf("unexpected identity attribute %q", attr)
				}
			}
			for attr := range tt.want {
				if _, ok := tt.schema.Attributes[attr]; !ok {
					t.Errorf("missing identity attribute %q", attr)
				}
			}
		})
	}
}

func TestImportHelpersRejectMissingIdentity(t *testing.T) {
	ctx := context.Background()
	req := resource.ImportStateRequest{}

	tests := map[string]func(*resource.ImportStateResponse){
		"service": func(resp *resource.ImportStateResponse) {
			ImportService(ctx, req, resp)
		},
		"named versioned": func(resp *resource.ImportStateResponse) {
			ImportNamedVersioned(ctx, nil, req, resp)
		},
		"service scoped versioned": func(resp *resource.ImportStateResponse) {
			ImportServiceScopedVersioned(ctx, nil, req, resp)
		},
		"resource link": func(resp *resource.ImportStateResponse) {
			ImportResourceLink(ctx, nil, req, resp)
		},
		"ACL collection": func(resp *resource.ImportStateResponse) {
			ImportACLCollection(ctx, req, resp)
		},
	}

	for name, importFn := range tests {
		t.Run(name, func(t *testing.T) {
			resp := &resource.ImportStateResponse{}
			importFn(resp)

			if !resp.Diagnostics.HasError() {
				t.Fatal("expected missing resource identity diagnostic")
			}
		})
	}
}
