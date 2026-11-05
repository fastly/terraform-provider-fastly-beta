package ngwafrule

import (
	"context"
	"fmt"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/ngwafquery"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	fastly "github.com/fastly/go-fastly/v19/fastly"
	"github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v19/fastly/ngwaf/v1/scope"
)

var (
	_ list.ListResource              = &ListResource{}
	_ list.ListResourceWithConfigure = &ListResource{}
)

// ListDefinition describes one concrete rule list resource.
type ListDefinition struct {
	// TypeName is the full Terraform resource type name, matching the managed
	// resource.
	TypeName string
	// RuleType is the API's rule `type` discriminator.
	RuleType string
	// Scope is the scope the managed resource is defined at.
	Scope scope.Type
	// Flatten converts an API rule to a pointer to the managed resource's
	// state model.
	Flatten func(ctx context.Context, rule *rules.Rule) (any, diag.Diagnostics)
}

// ListResource implements terraform query for one concrete rule resource.
type ListResource struct {
	client     *fastly.Client
	definition ListDefinition
}

func NewListResource(definition ListDefinition) list.ListResource {
	return &ListResource{definition: definition}
}

type ruleEntry struct {
	rule        *rules.Rule
	workspaceID string
}

func (l *ListResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + l.definition.TypeName
}

func (l *ListResource) ListResourceConfigSchema(_ context.Context, _ list.ListResourceSchemaRequest, resp *list.ListResourceSchemaResponse) {
	resp.Schema = listschema.Schema{
		Description: fmt.Sprintf("List all Fastly Next-Gen WAF `%s` rules defined at %s scope that are accessible to the API token.", l.definition.RuleType, l.definition.Scope),
	}
}

func (l *ListResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}

	l.client = data.Client
}

func (l *ListResource) List(ctx context.Context, req list.ListRequest, stream *list.ListResultsStream) {
	tflog.Debug(ctx, "Listing Fastly NGWAF rules", map[string]any{
		"type":  l.definition.RuleType,
		"scope": l.definition.Scope,
	})

	entries, diags := l.collect(ctx)
	if diags.HasError() {
		stream.Results = list.ListResultsStreamDiagnostics(diags)
		return
	}

	stream.Results = func(push func(list.ListResult) bool) {
		var count int64
		for _, e := range entries {
			if req.Limit > 0 && count >= req.Limit {
				return
			}
			count++

			result := req.NewListResult(ctx)
			result.DisplayName = service.ToGeneratedResourceName(e.rule.Description, e.rule.RuleID)

			if l.definition.Scope == scope.ScopeTypeAccount {
				result.Diagnostics.Append(result.Identity.Set(ctx, resourceidentity.NGWAFAccountObject(types.StringValue(e.rule.RuleID)))...)
			} else {
				result.Diagnostics.Append(result.Identity.Set(ctx, resourceidentity.NGWAFWorkspaceObject(types.StringValue(e.workspaceID), types.StringValue(e.rule.RuleID)))...)
			}

			if req.IncludeResource {
				model, flattenDiags := l.definition.Flatten(ctx, e.rule)
				result.Diagnostics.Append(flattenDiags...)
				if !flattenDiags.HasError() {
					result.Diagnostics.Append(result.Resource.Set(ctx, model)...)
				}
			}

			if !push(result) {
				return
			}
		}
	}
}

func (l *ListResource) collect(ctx context.Context) ([]ruleEntry, diag.Diagnostics) {
	var diags diag.Diagnostics

	if l.definition.Scope == scope.ScopeTypeAccount {
		found, err := l.listScope(ctx, &scope.Scope{Type: scope.ScopeTypeAccount})
		if err != nil {
			diags.AddError("Error listing NGWAF account rules", err.Error())
			return nil, diags
		}
		return wrapRules(found, ""), diags
	}

	workspaces, diags := ngwafquery.ListWorkspaces(ctx, l.client)
	if diags.HasError() {
		return nil, diags
	}

	var out []ruleEntry
	for _, w := range workspaces {
		found, err := l.listScope(ctx, WorkspaceScope(w.WorkspaceID))
		if err != nil {
			tflog.Warn(ctx, "Error listing NGWAF rules for workspace", map[string]any{
				"workspace_id": w.WorkspaceID,
				"error":        err.Error(),
			})
			continue
		}
		out = append(out, wrapRules(found, w.WorkspaceID)...)
	}
	return out, diags
}

// listScope returns the rules of this resource's type that the given scope
// owns. The type is also checked client-side so a response that ignores the
// filter can't surface another resource's rules.
func (l *ListResource) listScope(ctx context.Context, apiScope *scope.Scope) ([]*rules.Rule, error) {
	found, err := rules.List(ctx, l.client, &rules.ListInput{
		Scope: apiScope,
		Types: new(l.definition.RuleType),
	})
	if err != nil {
		return nil, err
	}

	var out []*rules.Rule
	for i := range found {
		rule := &found[i]
		if rule.RuleID == "" || rule.Type != l.definition.RuleType || scope.Type(rule.Scope.Type) != apiScope.Type {
			continue
		}
		out = append(out, rule)
	}
	return out, nil
}

func wrapRules(found []*rules.Rule, workspaceID string) []ruleEntry {
	out := make([]ruleEntry, 0, len(found))
	for _, rule := range found {
		out = append(out, ruleEntry{rule: rule, workspaceID: workspaceID})
	}
	return out
}
