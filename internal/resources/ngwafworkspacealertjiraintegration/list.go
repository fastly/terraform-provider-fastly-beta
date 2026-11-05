package ngwafworkspacealertjiraintegration

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafalertintegration"

	"github.com/hashicorp/terraform-plugin-framework/list"
)

func NewListResource() list.ListResource {
	return ngwafalertintegration.NewWorkspaceListResource(definition())
}
