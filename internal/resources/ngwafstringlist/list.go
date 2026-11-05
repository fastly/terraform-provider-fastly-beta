package ngwafstringlist

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/list"
)

func NewListResource() list.ListResource {
	return ngwaflist.NewAccountListResource(
		listType,
		"string",
		"Manages a Fastly Next-Gen WAF `string` list defined at account scope. String lists hold exact string values used by NGWAF list conditions.",
	)
}
