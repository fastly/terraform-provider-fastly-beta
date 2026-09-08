package ngwafworkspacealertdatadogintegrations

import (
	"github.com/fastly/terraform-provider-fastly-beta/internal/datasources/ngwafalertintegrations"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resources/ngwafworkspacealertdatadogintegration"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
)

func NewDataSource() datasource.DataSource {
	return ngwafalertintegrations.NewWorkspaceDataSource(ngwafworkspacealertdatadogintegration.DefinitionForDataSource())
}
