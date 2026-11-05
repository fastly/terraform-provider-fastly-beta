package ngwafworkspacecountrylist

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/list"
)

func NewListResource() list.ListResource {
	return ngwaflist.NewWorkspaceListResource(
		listType,
		"country",
		"Manages a Fastly Next-Gen WAF `country` list scoped to a single workspace. Country lists hold country codes used by NGWAF list conditions.",
	)
}
