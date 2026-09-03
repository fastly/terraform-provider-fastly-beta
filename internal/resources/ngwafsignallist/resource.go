package ngwafsignallist

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const listType = "signal"

func NewResource() resource.Resource {
	return ngwaflist.NewAccountResource(
		listType,
		"signal",
		"Manages a Fastly Next-Gen WAF `signal` list defined at account scope. Signal lists hold signal reference IDs used by NGWAF list conditions.",
	)
}
