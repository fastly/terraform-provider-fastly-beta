package resourceidentity

import (
	"context"
	"fmt"
	"maps"
	"slices"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/identityschema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// BotManagementWorkspaceModel identifies a Bot Management workspace by its
// workspace ID.
type BotManagementWorkspaceModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
}

// BotManagementCategoryModel identifies a Fastly-defined bot category within
// a Bot Management workspace.
type BotManagementCategoryModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	CategoryID  types.String `tfsdk:"category_id"`
}

// BotManagementBotModel identifies a Fastly-defined bot within a category of
// a Bot Management workspace.
type BotManagementBotModel struct {
	WorkspaceID types.String `tfsdk:"workspace_id"`
	CategoryID  types.String `tfsdk:"category_id"`
	BotID       types.String `tfsdk:"bot_id"`
}

var botManagementIdentityDescriptions = map[string]string{
	"workspace_id": "Fastly Bot Management workspace ID.",
	"category_id":  "Fastly Bot Management category ID.",
	"bot_id":       "Fastly Bot Management bot ID.",
}

func botManagementSchema(attrs ...string) identityschema.Schema {
	out := make(map[string]identityschema.Attribute, len(attrs))
	for _, a := range attrs {
		out[a] = identityschema.StringAttribute{
			RequiredForImport: true,
			Description:       botManagementIdentityDescriptions[a],
		}
	}
	return identityschema.Schema{Attributes: out}
}

func BotManagementWorkspaceSchema() identityschema.Schema {
	return botManagementSchema("workspace_id")
}

func BotManagementCategorySchema() identityschema.Schema {
	return botManagementSchema("workspace_id", "category_id")
}

func BotManagementBotSchema() identityschema.Schema {
	return botManagementSchema("workspace_id", "category_id", "bot_id")
}

func BotManagementWorkspace(workspaceID types.String) *BotManagementWorkspaceModel {
	return &BotManagementWorkspaceModel{WorkspaceID: workspaceID}
}

func BotManagementCategory(workspaceID, categoryID types.String) *BotManagementCategoryModel {
	return &BotManagementCategoryModel{WorkspaceID: workspaceID, CategoryID: categoryID}
}

func BotManagementBot(workspaceID, categoryID, botID types.String) *BotManagementBotModel {
	return &BotManagementBotModel{WorkspaceID: workspaceID, CategoryID: categoryID, BotID: botID}
}

// ImportBotManagementWorkspace seeds workspace_id from an identity-based
// import so the resource's normal Read can populate the remaining state.
func ImportBotManagementWorkspace(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var identity BotManagementWorkspaceModel
	importBotManagement(ctx, req, resp, &identity, func() map[string]types.String {
		return map[string]types.String{"workspace_id": identity.WorkspaceID}
	})
}

// ImportBotManagementCategory seeds workspace_id and category_id from an
// identity-based import.
func ImportBotManagementCategory(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var identity BotManagementCategoryModel
	importBotManagement(ctx, req, resp, &identity, func() map[string]types.String {
		return map[string]types.String{
			"workspace_id": identity.WorkspaceID,
			"category_id":  identity.CategoryID,
		}
	})
}

// ImportBotManagementBot seeds workspace_id, category_id and bot_id from an
// identity-based import.
func ImportBotManagementBot(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	var identity BotManagementBotModel
	importBotManagement(ctx, req, resp, &identity, func() map[string]types.String {
		return map[string]types.String{
			"workspace_id": identity.WorkspaceID,
			"category_id":  identity.CategoryID,
			"bot_id":       identity.BotID,
		}
	})
}

// importBotManagement decodes identity, then copies each attribute returned by
// fields into the state attribute of the same name. fields is a closure so it
// observes identity after decoding.
func importBotManagement(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse, identity any, fields func() map[string]types.String) {
	if !requireImportIdentity(req, resp) {
		return
	}

	resp.Diagnostics.Append(req.Identity.Get(ctx, identity)...)
	if resp.Diagnostics.HasError() {
		return
	}

	values := fields()
	for _, name := range slices.Sorted(maps.Keys(values)) {
		v := values[name]
		if v.IsNull() || v.IsUnknown() || v.ValueString() == "" {
			resp.Diagnostics.AddError("Invalid resource identity", fmt.Sprintf("The resource identity must contain a non-empty %s.", name))
			return
		}
		resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(name), v)...)
	}
	if resp.Diagnostics.HasError() || resp.Identity == nil {
		return
	}
	resp.Diagnostics.Append(resp.Identity.Set(ctx, identity)...)
}
