package ngwafworkspacewildcardlist

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const listType = "wildcard"

func NewResource() resource.Resource {
	return ngwaflist.NewWorkspaceResource(
		listType,
		"wildcard",
		"Manages a Fastly Next-Gen WAF `wildcard` list scoped to a single workspace. Wildcard lists hold wildcard string patterns used by NGWAF list conditions.",
	)
}
