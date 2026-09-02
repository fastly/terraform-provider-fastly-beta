package tlssubscriptionvalidation

import (
	"context"
	"fmt"
	"time"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
)

const (
	subscriptionStateIssued = "issued"
	createTimeout           = 45 * time.Minute
	pollInterval            = 10 * time.Second
)

var (
	_ resource.Resource              = &Resource{}
	_ resource.ResourceWithConfigure = &Resource{}
)

type Resource struct {
	client *fastly.Client
}

func NewResource() resource.Resource {
	return &Resource{}
}

func (r *Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_subscription_validation"
}

func (r *Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "This resource represents a successful validation of a Fastly TLS Subscription in concert with other resources. " +
			"Most commonly, this resource is used together with a resource for a DNS record and `fastly_tls_subscription` to request " +
			"a DNS validated certificate, deploy the required validation records and wait for validation to complete.\n\n" +
			"This resource implements a part of the validation workflow. It does not represent a real-world entity in Fastly, " +
			"therefore changing or deleting this resource on its own has no immediate effect. Waits up to 45 minutes for the " +
			"subscription to reach the `issued` state.",
		Attributes: ResourceAttributes(),
	}
}

func (r *Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}
	r.client = data.Client
}

func (r *Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := service.StringValue(plan.SubscriptionID)
	tflog.Debug(ctx, "Waiting for Fastly TLS subscription to be validated", map[string]any{"subscription_id": subscriptionID})

	if err := waitForIssued(ctx, r.getSubscription, subscriptionID, createTimeout, pollInterval); err != nil {
		resp.Diagnostics.AddError("Error waiting for TLS subscription validation", err.Error())
		return
	}

	subscription, err := r.client.GetTLSSubscription(ctx, &fastly.GetTLSSubscriptionInput{ID: subscriptionID})
	if err != nil {
		resp.Diagnostics.AddError("Error reading TLS subscription", err.Error())
		return
	}

	newState := flattenToModel(subscription, subscriptionID)
	if newState.CertificateID.IsNull() {
		resp.Diagnostics.AddError("Error reading TLS subscription",
			fmt.Sprintf("subscription %s reached state %q but has no issued certificate", subscriptionID, subscription.State))
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

func (r *Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state Model
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := service.StringValue(state.SubscriptionID)
	tflog.Debug(ctx, "Reading Fastly TLS subscription", map[string]any{"subscription_id": subscriptionID})

	subscription, err := r.client.GetTLSSubscription(ctx, &fastly.GetTLSSubscriptionInput{ID: subscriptionID})
	if err != nil {
		resp.Diagnostics.AddError("Error reading TLS subscription", err.Error())
		return
	}

	newState := flattenToModel(subscription, subscriptionID)
	if newState.CertificateID.IsNull() {
		tflog.Warn(ctx, "TLS subscription no longer has an issued certificate, removing from state", map[string]any{"subscription_id": subscriptionID})
		resp.State.RemoveResource(ctx)
		return
	}

	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Update is unreachable in practice: subscription_id is the only configurable attribute and
// always forces replacement, so Terraform never calls Update without also replacing the resource.
// It's implemented to satisfy resource.Resource and to keep state fresh defensively.
func (r *Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan Model
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	subscriptionID := service.StringValue(plan.SubscriptionID)
	subscription, err := r.client.GetTLSSubscription(ctx, &fastly.GetTLSSubscriptionInput{ID: subscriptionID})
	if err != nil {
		resp.Diagnostics.AddError("Error reading TLS subscription", err.Error())
		return
	}

	newState := flattenToModel(subscription, subscriptionID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &newState)...)
}

// Delete is a no-op: this is a virtual resource with no upstream entity to remove.
func (r *Resource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *Resource) getSubscription(ctx context.Context, id string) (*fastly.TLSSubscription, error) {
	return r.client.GetTLSSubscription(ctx, &fastly.GetTLSSubscriptionInput{ID: id})
}

// waitForIssued polls get until the subscription reaches the "issued" state or timeout elapses.
// get is a parameter (rather than a direct API call) so the poll/timeout logic can be unit
// tested without a live client.
func waitForIssued(ctx context.Context, get func(ctx context.Context, id string) (*fastly.TLSSubscription, error), subscriptionID string, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		subscription, err := get(ctx, subscriptionID)
		if err != nil {
			return err
		}
		if subscription.State == subscriptionStateIssued {
			return nil
		}
		if !time.Now().Before(deadline) {
			return fmt.Errorf("expected subscription state to be %s but it was %s after waiting %s", subscriptionStateIssued, subscription.State, timeout)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
