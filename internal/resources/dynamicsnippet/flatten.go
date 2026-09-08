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

	return NestedModel{
		Name:      types.StringValue(fastly.ToValue(api.Name)),
		Type:      types.StringValue(string(fastly.ToValue(api.Type))),
		Priority:  types.Int64Value(priority),
		SnippetID: types.StringValue(fastly.ToValue(api.SnippetID)),
	}, nil
}
