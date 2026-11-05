package ngwafcountrylist

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/list"
)

func NewListResource() list.ListResource {
	return ngwaflist.NewAccountListResource(
		listType,
		"country",
		"Manages a Fastly Next-Gen WAF `country` list defined at account scope. Country lists hold country codes used by NGWAF list conditions.",
	)
}
