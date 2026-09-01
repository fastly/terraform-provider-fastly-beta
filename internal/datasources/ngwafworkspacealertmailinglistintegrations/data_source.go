package ngwafworkspacealertmailinglistintegrations

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafalertintegrations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacealertmailinglistintegration"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func NewDataSource() datasource.DataSource {
	return ngwafalertintegrations.NewWorkspaceDataSource(ngwafworkspacealertmailinglistintegration.DefinitionForDataSource())
}
