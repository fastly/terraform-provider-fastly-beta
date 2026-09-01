package ngwafrule

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/scope"
)

// ImportWorkspaceState populates workspace_id and id from a
// "workspace_id/rule_id" import identifier. Account-scoped rules import from a
// bare rule ID instead, since their endpoint has no workspace path segment.
func ImportWorkspaceState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.Split(req.ID, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		resp.Diagnostics.AddError(
			"Unexpected import identifier",
			fmt.Sprintf("Expected import identifier of the form \"workspace_id/rule_id\", got: %q", req.ID),
		)
		return
	}

	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("workspace_id"), parts[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), parts[1])...)
}

// CheckType errors when a rule read back from the API isn't the type the
// calling resource manages. A rule ID doesn't encode its type, so an import
// can name a rule belonging to one of the sibling resources.
func CheckType(want string, rule *rules.Rule, diags *diag.Diagnostics) {
	if rule.Type == want {
		return
	}

	diags.AddError(
		"Wrong rule type",
		fmt.Sprintf("Rule %s is a %q rule, not %q; use the resource for %q rules instead.", rule.RuleID, rule.Type, want, rule.Type),
	)
}

// CheckAccountScope guards that a rule read back from the account endpoint is
// account-scoped before its scope is written into applies_to. The endpoint
// itself already partitions by scope - a workspace rule's ID 404s here rather
// than resolving - so this catches a response whose scope is missing or
// unexpected, mirroring the scope check ngwaflist makes on read.
func CheckAccountScope(rule *rules.Rule, diags *diag.Diagnostics) {
	if scope.Type(rule.Scope.Type) == scope.ScopeTypeAccount {
		return
	}

	diags.AddError(
		"Wrong rule scope",
		fmt.Sprintf("Rule %s is %q-scoped, not %q; use the fastly_ngwaf_workspace_* rule resources for workspace-scoped rules.", rule.RuleID, rule.Scope.Type, scope.ScopeTypeAccount),
	)
}
