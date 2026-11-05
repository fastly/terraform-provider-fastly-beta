package ngwafworkspacestringlist

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/list"
)

func NewListResource() list.ListResource {
	return ngwaflist.NewWorkspaceListResource(
		listType,
		"string",
		"Manages a Fastly Next-Gen WAF `string` list scoped to a single workspace. String lists hold exact string values used by NGWAF list conditions.",
	)
}
