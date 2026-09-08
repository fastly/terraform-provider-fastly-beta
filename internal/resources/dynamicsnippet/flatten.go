package dynamicsnippet

import (
	"fmt"

	regularsnippet "github.com/fastly/terraform-provider-fastly-beta/internal/resources/snippet"

	"github.com/hashicorp/terraform-plugin-framework/types"

	fastly "github.com/fastly/go-fastly/v17/fastly"
)

func FlattenToNestedModel(api *fastly.Snippet) (NestedModel, error) {
	if api == nil {
		return NestedModel{}, nil
	}

	if !regularsnippet.IsDynamic(api) {
		return NestedModel{}, fmt.Errorf("VCL snippet %q is regular; expected dynamic snippet", fastly.ToValue(api.Name))
	}

	priority, err := parsePriority(api.Priority)
	if err != nil {
		return NestedModel{}, err
	}

	// Content is deliberately left unset here: it's versionless, and can be written by
	// fastly_service_dynamic_snippet_content outside of this snippet's own reconcile. Reading it
	// back from a per-version ListSnippets call would surface whatever's currently live instead of
	// what this model's own config/plan says, which either fights with that resource for ownership
	// (when configured here) or shows spurious drift against an unset config (when not).
	// See MatchOrderPreserveContent and MatchOrderPreservePlanFields, which restore it from
	// plan/previous state instead.
	return NestedModel{
		Name:      types.StringValue(fastly.ToValue(api.Name)),
		Type:      types.StringValue(string(fastly.ToValue(api.Type))),
		Priority:  types.Int64Value(priority),
		SnippetID: types.StringValue(fastly.ToValue(api.SnippetID)),
	}, nil
}
