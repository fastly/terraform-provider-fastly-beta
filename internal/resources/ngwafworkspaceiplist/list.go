package ngwafworkspaceiplist

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/list"
)

func NewListResource() list.ListResource {
	return ngwaflist.NewWorkspaceListResource(
		listType,
		"ip",
		"Manages a Fastly Next-Gen WAF `ip` list scoped to a single workspace. IP lists hold IP address or CIDR values used by NGWAF list conditions.",
	)
}
