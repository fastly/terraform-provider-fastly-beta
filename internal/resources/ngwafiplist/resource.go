package ngwafiplist

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const listType = "ip"

func NewResource() resource.Resource {
	return ngwaflist.NewAccountResource(
		listType,
		"ip",
		"Manages a Fastly Next-Gen WAF `ip` list defined at account scope. IP lists hold IP address or CIDR values used by NGWAF list conditions.",
	)
}
