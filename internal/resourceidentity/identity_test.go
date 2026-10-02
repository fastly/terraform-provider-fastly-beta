package resourceidentity

import (
	"testing"

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
