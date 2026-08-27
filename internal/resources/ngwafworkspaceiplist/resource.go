package ngwafworkspaceiplist

import (
	"github.com/fastly/terraform-provider-fastly/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const listType = "ip"

func NewResource() resource.Resource {
	return ngwaflist.NewWorkspaceResource(
		listType,
		"ip",
		"Manages a Fastly Next-Gen WAF `ip` list scoped to a single workspace. IP lists hold IP address or CIDR values used by NGWAF list conditions.",
	)
}
