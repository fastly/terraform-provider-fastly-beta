package ngwafworkspacecountrylist

import (
	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const listType = "country"

func NewResource() resource.Resource {
	return ngwaflist.NewWorkspaceResource(
		listType,
		"country",
		"Manages a Fastly Next-Gen WAF `country` list scoped to a single workspace. Country lists hold country codes used by NGWAF list conditions.",
	)
}
