package logging

import (
	"context"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/resourceidentity"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"
	"github.com/fastly/terraform-provider-fastly-beta/internal/validation"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// PrepareWrite runs the checks shared by every standalone logging resource's
// Create and Update. It returns false when a diagnostic was added.
func PrepareWrite(ctx context.Context, data *fastlyclient.Data, resourceName, serviceID string, version int, cfg tfsdk.Config, diags *diag.Diagnostics) (string, bool) {
	if err := validation.EnsureServiceTypeSupported(ctx, data.TypeChecker, serviceID, resourceName, service.TypeVCL, service.TypeCompute); err != nil {
		diags.AddError("Unsupported service type", err.Error())
		return "", false
	}

	serviceType, err := data.TypeChecker.GetType(ctx, serviceID)
	if err != nil {
		diags.AddError("Error determining service type", err.Error())
		return "", false
	}
	if serviceType == service.TypeCompute {
		diags.Append(ValidateNoVCLOnlyAttributesForCompute(ctx, cfg)...)
		if diags.HasError() {
			return "", false
		}
	}

	diags.Append(data.VersionChecker.EnsureMutable(ctx, serviceID, version)...)
	if diags.HasError() {
		return "", false
	}
	return serviceType, true
}

// PrepareDelete returns false when the caller must stop: a diagnostic was added,
// or the service or version is already gone.
func PrepareDelete(ctx context.Context, data *fastlyclient.Data, resourceName, serviceID string, version int, diags *diag.Diagnostics) bool {
	if err := validation.EnsureServiceTypeSupported(ctx, data.TypeChecker, serviceID, resourceName, service.TypeVCL, service.TypeCompute); err != nil {
		diags.AddError("Unsupported service type", err.Error())
		return false
	}

	notFound, d := data.VersionChecker.EnsureMutableForDelete(ctx, serviceID, version)
	diags.Append(d...)
	return !notFound && !diags.HasError()
}

func SetIdentity(ctx context.Context, identity *tfsdk.ResourceIdentity, serviceID, name types.String, diags *diag.Diagnostics) {
	if !diags.HasError() && identity != nil {
		diags.Append(identity.Set(ctx, resourceidentity.NamedVersioned(serviceID, name))...)
	}
}
