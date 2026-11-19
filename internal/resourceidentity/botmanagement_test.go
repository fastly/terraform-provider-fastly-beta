package resourceidentity

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

func TestImportBotManagementCopiesIdentityToState(t *testing.T) {
	tests := map[string]struct {
		schema   identityschema.Schema
		values   map[string]string
		importFn func(context.Context, resource.ImportStateRequest, *resource.ImportStateResponse)
	}{
		"workspace": {
			schema:   BotManagementWorkspaceSchema(),
			values:   map[string]string{"workspace_id": "ws"},
			importFn: ImportBotManagementWorkspace,
		},
		"category": {
			schema:   BotManagementCategorySchema(),
			values:   map[string]string{"workspace_id": "ws", "category_id": "cat_ai_crawler"},
			importFn: ImportBotManagementCategory,
		},
		"bot": {
			schema:   BotManagementBotSchema(),
			values:   map[string]string{"workspace_id": "ws", "category_id": "cat_ai_crawler", "bot_id": "bot_gptbot"},
			importFn: ImportBotManagementBot,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			req := resource.ImportStateRequest{Identity: identityForTest(ctx, tt.schema, tt.values)}
			resp := &resource.ImportStateResponse{
				State:    stateForTest(ctx, tt.values),
				Identity: identityForTest(ctx, tt.schema, nil),
			}

			tt.importFn(ctx, req, resp)
			if resp.Diagnostics.HasError() {
				t.Fatal(resp.Diagnostics)
			}

			for attr, want := range tt.values {
				var got types.String
				if diags := resp.State.GetAttribute(ctx, path.Root(attr), &got); diags.HasError() {
					t.Fatal(diags)
				}
				if got.ValueString() != want {
					t.Errorf("state %s = %q, want %q", attr, got.ValueString(), want)
				}
			}
		})
	}
}

func TestImportBotManagementRejectsEmptyAttribute(t *testing.T) {
	ctx := context.Background()
	values := map[string]string{"workspace_id": "ws", "category_id": "", "bot_id": "bot_gptbot"}
	req := resource.ImportStateRequest{Identity: identityForTest(ctx, BotManagementBotSchema(), values)}
	resp := &resource.ImportStateResponse{State: stateForTest(ctx, values)}

	ImportBotManagementBot(ctx, req, resp)
	if !resp.Diagnostics.HasError() {
		t.Fatal("expected an empty category_id to be rejected")
	}
}

func identityForTest(ctx context.Context, s identityschema.Schema, values map[string]string) *tfsdk.ResourceIdentity {
	typ := s.Type().TerraformType(ctx)
	raw := tftypes.NewValue(typ, nil)
	if values != nil {
		vals := map[string]tftypes.Value{}
		for attr := range s.Attributes {
			vals[attr] = tftypes.NewValue(tftypes.String, values[attr])
		}
		raw = tftypes.NewValue(typ, vals)
	}
	return &tfsdk.ResourceIdentity{Schema: s, Raw: raw}
}

func stateForTest(ctx context.Context, values map[string]string) tfsdk.State {
	attrs := map[string]schema.Attribute{}
	for attr := range values {
		attrs[attr] = schema.StringAttribute{Optional: true}
	}
	s := schema.Schema{Attributes: attrs}
	return tfsdk.State{Schema: s, Raw: tftypes.NewValue(s.Type().TerraformType(ctx), nil)}
}
