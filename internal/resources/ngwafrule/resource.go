package ngwafrule

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"

	"github.com/fastly/go-fastly/v17/fastly/ngwaf/v1/rules"
)

// ImportState populates workspace_id and id from a "workspace_id/rule_id"
// import identifier.
func ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, "/", 2)
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
