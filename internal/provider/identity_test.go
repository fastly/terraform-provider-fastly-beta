package provider

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

func TestListResourcesHaveStableManagedResourceIdentity(t *testing.T) {
	ctx := context.Background()
	p := &fastlyProvider{}

	managedByType := make(map[string]resource.Resource)
	for _, newResource := range p.Resources(ctx) {
		r := newResource()
		var metadata resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "fastly"}, &metadata)
		managedByType[metadata.TypeName] = r
	}

	listResources := p.ListResources(ctx)
	if len(listResources) == 0 {
		t.Fatal("provider registered no ListResources")
	}

	for _, newListResource := range listResources {
		lr := newListResource()

		var metadata resource.MetadataResponse
		lr.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: "fastly"}, &metadata)

		t.Run(metadata.TypeName, func(t *testing.T) {
			managed, ok := managedByType[metadata.TypeName]
			if !ok {
				t.Fatalf("registered ListResource %q has no matching managed resource", metadata.TypeName)
			}

			withIdentity, ok := managed.(resource.ResourceWithIdentity)
			if !ok {
				t.Fatalf("managed resource %q does not implement resource.ResourceWithIdentity", metadata.TypeName)
			}

			var identityResp resource.IdentitySchemaResponse
			withIdentity.IdentitySchema(ctx, resource.IdentitySchemaRequest{}, &identityResp)

			if identityResp.Diagnostics.HasError() {
				t.Fatalf("identity schema returned diagnostics: %v", identityResp.Diagnostics)
			}
			if len(identityResp.IdentitySchema.Attributes) == 0 {
				t.Fatal("identity schema must not be empty")
			}
			if _, ok := identityResp.IdentitySchema.Attributes["version"]; ok {
				t.Fatal("mutable service version must not be part of resource identity")
			}
		})
	}
}
