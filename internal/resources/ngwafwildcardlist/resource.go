package ngwafwildcardlist

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwaflist"

	"github.com/hashicorp/terraform-plugin-framework/resource"
)

const listType = "wildcard"

func NewResource() resource.Resource {
	return ngwaflist.NewAccountResource(
		listType,
		"wildcard",
		"Manages a Fastly Next-Gen WAF `wildcard` list defined at account scope. Wildcard lists hold wildcard string patterns used by NGWAF list conditions.",
	)
}
