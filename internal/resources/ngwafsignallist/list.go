package ngwafsignallist

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/list"
)

func NewListResource() list.ListResource {
	return ngwaflist.NewAccountListResource(
		listType,
		"signal",
		"Manages a Fastly Next-Gen WAF `signal` list defined at account scope. Signal lists hold signal reference IDs used by NGWAF list conditions.",
	)
}
