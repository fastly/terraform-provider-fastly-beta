package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/list"
	listschema "github.com/hashicorp/terraform-plugin-framework/list/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"

	"github.com/fastly/go-fastly/v17/fastly"
)

const serviceIDFilterDescription = "Optional Fastly service ID to restrict discovery to a single service. When omitted, all services accessible to the API token are considered."

type queryListConfig struct {
	ServiceID types.String `tfsdk:"service_id"`
}

func QueryListResourceConfigSchema(description string) listschema.Schema {
	return listschema.Schema{
		Description: description,
		Attributes: map[string]listschema.Attribute{
			"service_id": listschema.StringAttribute{
				Optional:    true,
				Description: serviceIDFilterDescription,
			},
		},
	}
}

// ListServicesForQuery returns either the single service selected by the
// optional service_id list configuration or, when no filter is configured, all
// services visible to the API token.
//
// Using GetService for a configured service_id avoids account-wide service
// enumeration and ensures child-resource discovery only issues API requests for
// the requested service.
func ListServicesForQuery(ctx context.Context, client *fastly.Client, req list.ListRequest) ([]*fastly.Service, diag.Diagnostics) {
	var config queryListConfig
	var diags diag.Diagnostics

	diags.Append(req.Config.Get(ctx, &config)...)
	if diags.HasError() {
		return nil, diags
	}

	services, serviceDiags := listServicesForServiceID(ctx, client, config.ServiceID)
	diags.Append(serviceDiags...)
	return services, diags
}

func listServicesForServiceID(ctx context.Context, client *fastly.Client, serviceIDValue types.String) ([]*fastly.Service, diag.Diagnostics) {
	var diags diag.Diagnostics

	if serviceIDValue.IsUnknown() {
		diags.AddError(
			"Unknown Fastly service ID",
			"The list resource service_id filter must be known before Terraform can query Fastly resources.",
		)
		return nil, diags
	}

	if serviceIDValue.IsNull() {
		services, err := client.ListServices(ctx, &fastly.ListServicesInput{})
		if err != nil {
			diags.AddError("Error listing Fastly services", err.Error())
			return nil, diags
		}
		return services, diags
	}

	serviceID := strings.TrimSpace(serviceIDValue.ValueString())
	if serviceID == "" {
		diags.AddError(
			"Invalid Fastly service ID",
			"The list resource service_id filter must not be empty.",
		)
		return nil, diags
	}

	svc, err := client.GetService(ctx, &fastly.GetServiceInput{
		ServiceID: serviceID,
	})
	if err != nil {
		diags.AddError(
			"Error reading Fastly service",
			fmt.Sprintf("Unable to read Fastly service %q for query filtering: %s", serviceID, err),
		)
		return nil, diags
	}
	if svc == nil {
		diags.AddError(
			"Error reading Fastly service",
			fmt.Sprintf("Fastly returned no service for query filter %q.", serviceID),
		)
		return nil, diags
	}

	return []*fastly.Service{svc}, diags
}
