package logginggooglepubsub

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fastly "github.com/fastly/go-fastly/v17/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
)

// Test helpers

func defaultNestedModel() NestedModel {
	return NestedModel{
		commonModel:       defaultCommonModel(),
		Format:            types.StringValue(constants.LoggingGooglePubSubDefaultFormat),
		FormatVersion:     types.Int64Value(DefaultFormatVersion),
		Placement:         types.StringNull(),
		ResponseCondition: types.StringValue(DefaultResponseCondition),
	}
}

func defaultCommonModel() commonModel {
	return commonModel{
		Name:             types.StringValue(""),
		ProjectID:        types.StringValue(""),
		Topic:            types.StringValue(""),
		Authentication:   NewAuthenticationObject(types.StringValue(""), types.StringValue(""), types.StringValue("")),
		ProcessingRegion: types.StringValue(DefaultProcessingRegion),
	}
}

func fullNestedModel() NestedModel {
	m := defaultNestedModel()
	m.Name = types.StringValue("test-pubsub")
	m.ProjectID = types.StringValue("example-fastly-log")
	m.Topic = types.StringValue("fastly-logs-topic")
	m.Authentication = NewAuthenticationObject(
		types.StringValue("service-account"),
		types.StringValue("fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com"),
		types.StringValue("test-secret-key"),
	)
	m.ProcessingRegion = types.StringValue("eu")
	m.Format = types.StringValue("%h %l %u")
	m.FormatVersion = types.Int64Value(1)
	m.Placement = types.StringValue("none")
	m.ResponseCondition = types.StringValue("response-condition-1")
	return m
}

func minimalNestedModel() NestedModel {
	m := defaultNestedModel()
	m.Name = types.StringValue("test-pubsub")
	m.ProjectID = types.StringValue("example-fastly-log")
	m.Topic = types.StringValue("fastly-logs-topic")
	m.Authentication = NewAuthenticationObject(
		types.StringValue(""),
		types.StringValue("fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com"),
		types.StringValue("test-secret-key"),
	)
	return m
}

func fullComputeNestedModel() ComputeNestedModel {
	return ComputeNestedModel{commonModel: fullNestedModel().commonModel}
}

// Tests for flatten.go

func TestFlattenToNestedModel(t *testing.T) {
	tests := []struct {
		name     string
		api      *fastly.Pubsub
		expected NestedModel
	}{
		{
			name:     "nil returns empty model",
			api:      nil,
			expected: NestedModel{},
		},
		{
			name: "only required fields uses defaults",
			api: &fastly.Pubsub{
				Name:      new("test-pubsub"),
				ProjectID: new("example-fastly-log"),
				Topic:     new("fastly-logs-topic"),
				User:      new("fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com"),
				SecretKey: new("test-secret-key"),
			},
			expected: minimalNestedModel(),
		},
		{
			name: "all fields populated",
			api: &fastly.Pubsub{
				Name:              new("test-pubsub"),
				ProjectID:         new("example-fastly-log"),
				Topic:             new("fastly-logs-topic"),
				AccountName:       new("service-account"),
				User:              new("fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com"),
				SecretKey:         new("test-secret-key"),
				ProcessingRegion:  new("eu"),
				Format:            new("%h %l %u"),
				FormatVersion:     new(1),
				Placement:         new("none"),
				ResponseCondition: new("response-condition-1"),
			},
			expected: fullNestedModel(),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := FlattenToNestedModel(tt.api)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestFlattenToComputeNestedModel(t *testing.T) {
	api := &fastly.Pubsub{
		Name:             new("test-pubsub"),
		ProjectID:        new("example-fastly-log"),
		Topic:            new("fastly-logs-topic"),
		AccountName:      new("service-account"),
		User:             new("fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com"),
		SecretKey:        new("test-secret-key"),
		ProcessingRegion: new("eu"),
		// VCL-only fields must be ignored by the Compute flatten.
		Format:            new("%h %l %u"),
		FormatVersion:     new(1),
		Placement:         new("none"),
		ResponseCondition: new("response-condition-1"),
	}

	result := FlattenToComputeNestedModel(api)
	assert.Equal(t, fullComputeNestedModel(), result)
}

func TestFlatten(t *testing.T) {
	tests := []struct {
		name     string
		api      *fastly.Pubsub
		validate func(t *testing.T, m *Model)
	}{
		{
			name: "nil leaves model untouched",
			api:  nil,
			validate: func(t *testing.T, m *Model) {
				assert.Equal(t, types.String{}, m.ID)
				assert.Equal(t, types.String{}, m.Service)
				assert.Equal(t, types.Int64{}, m.Version)
			},
		},
		{
			name: "service metadata builds composite ID",
			api: &fastly.Pubsub{
				ServiceID:      new("service-123"),
				ServiceVersion: new(5),
				Name:           new("test-pubsub"),
				ProjectID:      new("example-fastly-log"),
				Topic:          new("fastly-logs-topic"),
			},
			validate: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-test-pubsub"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("test-pubsub"), m.Name)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			m := &Model{}
			flatten(ctx, tt.api, m)
			tt.validate(t, m)
		})
	}
}

// Tests for expand.go

func TestBuildCreateInput(t *testing.T) {
	tests := []struct {
		name      string
		serviceID string
		version   int
		model     NestedModel
		validate  func(t *testing.T, input *fastly.CreatePubsubInput)
	}{
		{
			name:      "minimal model",
			serviceID: "service-123",
			version:   5,
			model:     minimalNestedModel(),
			validate: func(t *testing.T, input *fastly.CreatePubsubInput) {
				assert.Equal(t, "service-123", input.ServiceID)
				assert.Equal(t, 5, input.ServiceVersion)
				assert.Equal(t, "test-pubsub", *input.Name)
				assert.Equal(t, "example-fastly-log", *input.ProjectID)
				assert.Equal(t, "fastly-logs-topic", *input.Topic)
				assert.Equal(t, "fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com", *input.User)
				assert.Equal(t, "test-secret-key", *input.SecretKey)
				assert.Nil(t, input.AccountName, "account_name left unset must not be sent")
				assert.Equal(t, constants.LoggingGooglePubSubDefaultFormat, *input.Format)
				assert.Nil(t, input.Placement, "unset placement must not be sent as \"none\" — the API treats them differently")
			},
		},
		{
			name:      "fully populated model",
			serviceID: "service-456",
			version:   10,
			model:     fullNestedModel(),
			validate: func(t *testing.T, input *fastly.CreatePubsubInput) {
				assert.Equal(t, "test-pubsub", *input.Name)
				assert.Equal(t, "example-fastly-log", *input.ProjectID)
				assert.Equal(t, "fastly-logs-topic", *input.Topic)
				assert.Equal(t, "service-account", *input.AccountName)
				assert.Equal(t, "fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com", *input.User)
				assert.Equal(t, "test-secret-key", *input.SecretKey)
				assert.Equal(t, "eu", *input.ProcessingRegion)
				assert.Equal(t, "%h %l %u", *input.Format)
				assert.Equal(t, 1, *input.FormatVersion)
				assert.Equal(t, "none", *input.Placement)
				assert.Equal(t, "response-condition-1", *input.ResponseCondition)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			input := BuildCreateInput(tt.serviceID, tt.version, tt.model)
			tt.validate(t, input)
		})
	}
}

func TestBuildComputeCreateInput(t *testing.T) {
	input := BuildComputeCreateInput("service-456", 10, fullComputeNestedModel())

	assert.Equal(t, "service-456", input.ServiceID)
	assert.Equal(t, 10, input.ServiceVersion)
	assert.Equal(t, "test-pubsub", *input.Name)
	assert.Equal(t, "example-fastly-log", *input.ProjectID)
	assert.Equal(t, "fastly-logs-topic", *input.Topic)
	assert.Equal(t, "service-account", *input.AccountName)
	assert.Equal(t, "fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com", *input.User)
	assert.Equal(t, "test-secret-key", *input.SecretKey)
	assert.Equal(t, "eu", *input.ProcessingRegion)
	assert.Nil(t, input.Format, "VCL-only fields must never be set for Compute")
	assert.Nil(t, input.FormatVersion)
	assert.Nil(t, input.Placement)
	assert.Nil(t, input.ResponseCondition)
}

func TestBuildUpdateInput(t *testing.T) {
	input := BuildUpdateInput("service-456", 10, fullNestedModel())

	assert.Equal(t, "service-456", input.ServiceID)
	assert.Equal(t, 10, input.ServiceVersion)
	assert.Equal(t, "test-pubsub", input.Name)
	assert.Equal(t, "test-pubsub", *input.NewName)
	assert.Equal(t, "example-fastly-log", *input.ProjectID)
	assert.Equal(t, "fastly-logs-topic", *input.Topic)
	assert.Equal(t, "service-account", *input.AccountName)
	assert.Equal(t, "fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com", *input.User)
	assert.Equal(t, "test-secret-key", *input.SecretKey)
	assert.Equal(t, "eu", *input.ProcessingRegion)
	assert.Equal(t, "%h %l %u", *input.Format)
	assert.Equal(t, 1, *input.FormatVersion)
	assert.Equal(t, fastly.NewNullable("none"), input.Placement)
	assert.Equal(t, "response-condition-1", *input.ResponseCondition)
}

// TestBuildUpdateInputClearsClearableFields verifies that response_condition
// and the credential fields are always sent as concrete values on update —
// even when empty — so clearing them actually reaches the API rather than
// being omitted (which would leave a previously-set value in place).
// placement is cleared the same way, but as an explicit JSON null rather than
// an empty string — see BuildUpdateInput. account_name is the opposite case:
// the API rejects an explicit empty string, so it must be omitted rather than
// sent when unset.
func TestBuildUpdateInputClearsClearableFields(t *testing.T) {
	input := BuildUpdateInput("service-1", 1, minimalNestedModel())

	assert.Nil(t, input.AccountName, "unset account_name must be omitted, not sent as an empty string the API rejects")
	assert.NotNil(t, input.ResponseCondition, "response_condition must be sent even when empty")
	assert.Equal(t, "", *input.ResponseCondition)
	assert.NotNil(t, input.Placement, "unset placement must be sent as an explicit null, not omitted (omitting leaves a previously-set \"none\" in place)")
	assert.Equal(t, fastly.NullValue[string](), input.Placement)
}

func TestBuildComputeUpdateInput(t *testing.T) {
	input := BuildComputeUpdateInput("service-456", 10, fullComputeNestedModel())

	assert.Equal(t, "service-456", input.ServiceID)
	assert.Equal(t, 10, input.ServiceVersion)
	assert.Equal(t, "test-pubsub", input.Name)
	assert.Equal(t, "test-pubsub", *input.NewName)
	assert.Equal(t, "service-account", *input.AccountName)
	assert.Equal(t, "eu", *input.ProcessingRegion)
	assert.Nil(t, input.Format)
	assert.Nil(t, input.FormatVersion)
	assert.Nil(t, input.Placement)
	assert.Nil(t, input.ResponseCondition)
}

// TestBuildComputeUpdateInputOmitsUnsetAccountName is the Compute counterpart
// to TestBuildUpdateInputClearsClearableFields' account_name assertion: the
// API rejects an explicit empty account_name, so BuildComputeUpdateInput must
// omit it the same way BuildUpdateInput does.
func TestBuildComputeUpdateInputOmitsUnsetAccountName(t *testing.T) {
	model := ComputeNestedModel{commonModel: minimalNestedModel().commonModel}
	input := BuildComputeUpdateInput("service-1", 1, model)

	assert.Nil(t, input.AccountName, "unset account_name must be omitted, not sent as an empty string the API rejects")
}

func TestClearVCLOnlyCreateFields(t *testing.T) {
	input := &fastly.CreatePubsubInput{
		Format:            new("some-format"),
		FormatVersion:     new(2),
		Placement:         new("none"),
		ResponseCondition: new("cond"),
	}

	ClearVCLOnlyCreateFields(input)

	assert.Nil(t, input.Format)
	assert.Nil(t, input.FormatVersion)
	assert.Nil(t, input.Placement)
	assert.Nil(t, input.ResponseCondition)
}

func TestClearVCLOnlyUpdateFields(t *testing.T) {
	input := &fastly.UpdatePubsubInput{
		Format:            new("some-format"),
		FormatVersion:     new(2),
		Placement:         fastly.NewNullable("none"),
		ResponseCondition: new("cond"),
	}

	ClearVCLOnlyUpdateFields(input)

	assert.Nil(t, input.Format)
	assert.Nil(t, input.FormatVersion)
	assert.Nil(t, input.Placement)
	assert.Nil(t, input.ResponseCondition)
}

// TestResetVCLOnlyToDefaults covers the Compute read-back path. On a Compute
// service the VCL-only fields are never sent, so the API reports its own
// server-side values. Adopting those breaks consistency-after-apply, so they
// must be reset to exactly the values a plan produces.
func TestResetVCLOnlyToDefaults(t *testing.T) {
	// What the API actually reports back for a Compute service.
	m := FlattenToNestedModel(&fastly.Pubsub{
		Name:              new("test-pubsub"),
		ProjectID:         new("example-fastly-log"),
		Topic:             new("fastly-logs-topic"),
		User:              new("fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com"),
		SecretKey:         new("test-secret-key"),
		ProcessingRegion:  new("none"),
		Format:            new("{\n  \"foo\": \"bar\"\n}\n"),
		FormatVersion:     new(1),
		Placement:         new("none"),
		ResponseCondition: new("some-condition"),
	})

	ResetVCLOnlyToDefaults(&m)

	assert.Equal(t, constants.LoggingGooglePubSubDefaultFormat, m.Format.ValueString())
	assert.Equal(t, int64(DefaultFormatVersion), m.FormatVersion.ValueInt64())
	assert.True(t, m.Placement.IsNull(), "placement must go back to unset, not the API's forced \"none\"")
	assert.Equal(t, DefaultResponseCondition, m.ResponseCondition.ValueString())

	// Non-VCL-only fields must survive untouched.
	assert.Equal(t, "test-pubsub", m.Name.ValueString())
	assert.Equal(t, "fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com", m.Email().ValueString())
	assert.Equal(t, "none", m.ProcessingRegion.ValueString())
}

// TestResetVCLOnlyToDefaultsMatchesPlannedDefaults ties the reset to the schema
// itself: the values it writes must equal the schema's declared defaults, or
// Create/Update would still disagree with the plan.
func TestResetVCLOnlyToDefaultsMatchesPlannedDefaults(t *testing.T) {
	var m NestedModel
	ResetVCLOnlyToDefaults(&m)

	attrs := CommonAttributes()

	format := attrs["format"].(schema.StringAttribute)
	var fResp defaults.StringResponse
	format.Default.DefaultString(context.Background(), defaults.StringRequest{}, &fResp)
	assert.Equal(t, fResp.PlanValue, m.Format, "format must match its schema default")

	formatVersion := attrs["format_version"].(schema.Int64Attribute)
	var fvResp defaults.Int64Response
	formatVersion.Default.DefaultInt64(context.Background(), defaults.Int64Request{}, &fvResp)
	assert.Equal(t, fvResp.PlanValue, m.FormatVersion, "format_version must match its schema default")

	responseCondition := attrs["response_condition"].(schema.StringAttribute)
	var rcResp defaults.StringResponse
	responseCondition.Default.DefaultString(context.Background(), defaults.StringRequest{}, &rcResp)
	assert.Equal(t, rcResp.PlanValue, m.ResponseCondition, "response_condition must match its schema default")

	// placement is Optional-only with no Default, so an absent config value plans
	// as null — the reset has to produce null, not "".
	assert.Nil(t, attrs["placement"].(schema.StringAttribute).Default)
	assert.True(t, m.Placement.IsNull())
}

// Tests for schema.go

func TestModelsEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        NestedModel
		b        NestedModel
		expected bool
	}{
		{
			name:     "identical models",
			a:        fullNestedModel(),
			b:        fullNestedModel(),
			expected: true,
		},
		{
			name:     "default models",
			a:        defaultNestedModel(),
			b:        defaultNestedModel(),
			expected: true,
		},
		{
			name: "different secret_key",
			a: func() NestedModel {
				m := minimalNestedModel()
				m.Authentication = NewAuthenticationObject(types.StringValue(""), types.StringValue("email"), types.StringValue("key-1"))
				return m
			}(),
			b: func() NestedModel {
				m := minimalNestedModel()
				m.Authentication = NewAuthenticationObject(types.StringValue(""), types.StringValue("email"), types.StringValue("key-2"))
				return m
			}(),
			expected: false,
		},
		{
			name: "different format only affects NestedModel equality",
			a: func() NestedModel {
				m := minimalNestedModel()
				m.Format = types.StringValue("format-a")
				return m
			}(),
			b: func() NestedModel {
				m := minimalNestedModel()
				m.Format = types.StringValue("format-b")
				return m
			}(),
			expected: false,
		},
		{
			name: "unset placement differs from explicit none",
			a: func() NestedModel {
				m := minimalNestedModel()
				m.Placement = types.StringNull()
				return m
			}(),
			b: func() NestedModel {
				m := minimalNestedModel()
				m.Placement = types.StringValue("none")
				return m
			}(),
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.a.ModelsEqual(tt.b))
		})
	}
}

func TestComputeModelsEqual(t *testing.T) {
	a := fullComputeNestedModel()
	b := fullComputeNestedModel()
	assert.True(t, a.ModelsEqual(b))

	b.ProcessingRegion = types.StringValue("us")
	assert.False(t, a.ModelsEqual(b))
}

// TestComputeModelsEqualIgnoresVCLOnlyFields verifies that a Compute endpoint
// whose remote state carries VCL-only fields still compares equal to the
// desired Compute model — otherwise ComputeReconcile would issue a pointless
// update on every apply.
func TestComputeModelsEqualIgnoresVCLOnlyFields(t *testing.T) {
	desired := fullComputeNestedModel()

	remote := &fastly.Pubsub{
		Name:              new("test-pubsub"),
		ProjectID:         new("example-fastly-log"),
		Topic:             new("fastly-logs-topic"),
		AccountName:       new("service-account"),
		User:              new("fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com"),
		SecretKey:         new("test-secret-key"),
		ProcessingRegion:  new("eu"),
		Format:            new("something-else-entirely"),
		FormatVersion:     new(1),
		Placement:         new("none"),
		ResponseCondition: new("some-condition"),
	}

	assert.True(t, desired.ModelsEqual(FlattenToComputeNestedModel(remote)))
}

func TestEqual(t *testing.T) {
	tests := []struct {
		name     string
		a        []NestedModel
		b        []NestedModel
		expected bool
	}{
		{
			name:     "both empty",
			a:        []NestedModel{},
			b:        []NestedModel{},
			expected: true,
		},
		{
			name: "different order but same content matches by name",
			a: []NestedModel{
				func() NestedModel { m := minimalNestedModel(); m.Name = types.StringValue("b"); return m }(),
				func() NestedModel { m := minimalNestedModel(); m.Name = types.StringValue("a"); return m }(),
			},
			b: []NestedModel{
				func() NestedModel { m := minimalNestedModel(); m.Name = types.StringValue("a"); return m }(),
				func() NestedModel { m := minimalNestedModel(); m.Name = types.StringValue("b"); return m }(),
			},
			expected: true,
		},
		{
			name: "different content",
			a: []NestedModel{
				func() NestedModel { m := minimalNestedModel(); m.Name = types.StringValue("a"); return m }(),
			},
			b: []NestedModel{
				func() NestedModel {
					m := minimalNestedModel()
					m.Name = types.StringValue("a")
					m.ProcessingRegion = types.StringValue("eu")
					return m
				}(),
			},
			expected: false,
		},
		{
			name: "different length",
			a: []NestedModel{
				func() NestedModel { m := minimalNestedModel(); m.Name = types.StringValue("a"); return m }(),
			},
			b: []NestedModel{
				func() NestedModel { m := minimalNestedModel(); m.Name = types.StringValue("a"); return m }(),
				func() NestedModel { m := minimalNestedModel(); m.Name = types.StringValue("b"); return m }(),
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, Equal(tt.a, tt.b))
		})
	}
}

func TestComputeEqual(t *testing.T) {
	a := []ComputeNestedModel{fullComputeNestedModel()}
	b := []ComputeNestedModel{fullComputeNestedModel()}
	assert.True(t, ComputeEqual(a, b))

	b[0].ProcessingRegion = types.StringValue("us")
	assert.False(t, ComputeEqual(a, b))
}

func TestMatchOrder(t *testing.T) {
	itemA := func() NestedModel { m := minimalNestedModel(); m.Name = types.StringValue("a"); return m }()
	itemB := func() NestedModel { m := minimalNestedModel(); m.Name = types.StringValue("b"); return m }()
	items := []NestedModel{itemB, itemA}

	orderA := minimalNestedModel()
	orderA.Name = types.StringValue("a")
	orderB := minimalNestedModel()
	orderB.Name = types.StringValue("b")
	order := []NestedModel{orderA, orderB}

	result := MatchOrder(items, order)

	assert.Len(t, result, 2)
	assert.Equal(t, "a", result[0].Name.ValueString())
	assert.Equal(t, "b", result[1].Name.ValueString())
}

func TestComputeMatchOrder(t *testing.T) {
	mk := func(name string) ComputeNestedModel {
		m := fullComputeNestedModel()
		m.Name = types.StringValue(name)
		return m
	}
	items := []ComputeNestedModel{mk("b"), mk("a")}
	order := []ComputeNestedModel{mk("a"), mk("b")}

	result := ComputeMatchOrder(items, order)

	assert.Len(t, result, 2)
	assert.Equal(t, "a", result[0].Name.ValueString())
	assert.Equal(t, "b", result[1].Name.ValueString())
}

// TestComputeAttributesOmitsVCLOnly locks in that the Compute nested block
// schema does not expose the VCL-only attributes, which is what makes
// `logging_googlepubsub { format = ... }` inside fastly_service_compute_auto
// fail at plan time with Terraform's own "Unsupported argument" error.
func TestComputeAttributesOmitsVCLOnly(t *testing.T) {
	compute := ComputeAttributes()
	common := CommonAttributes()

	for _, name := range []string{"format", "format_version", "placement", "response_condition"} {
		assert.NotContains(t, compute, name)
		assert.Contains(t, common, name)
	}

	for _, name := range []string{"name", "project_id", "topic", "authentication", "processing_region"} {
		assert.Contains(t, compute, name)
		assert.Contains(t, common, name)
	}

	// Credentials are nested under authentication, never top-level attributes.
	for _, name := range []string{"account_name", "email", "secret_key"} {
		assert.NotContains(t, compute, name)
		assert.NotContains(t, common, name)
	}
}

// TestAuthenticationAttribute locks in the credential shape: an Optional+Computed
// `authentication` object defaulted from environment variables, containing
// account_name, email, and secret_key — matching how every other logging
// endpoint groups credentials.
func TestAuthenticationAttribute(t *testing.T) {
	auth, ok := ComputeAttributes()["authentication"].(schema.SingleNestedAttribute)
	require.True(t, ok, "authentication must be a SingleNestedAttribute")

	assert.True(t, auth.Optional)
	assert.True(t, auth.Computed)
	assert.NotNil(t, auth.Default, "authentication.account_name/email/secret_key default from Pub/Sub's env vars")

	accountName, ok := auth.Attributes["account_name"].(schema.StringAttribute)
	require.True(t, ok, "authentication.account_name must be a StringAttribute")
	assert.False(t, accountName.Sensitive)

	email, ok := auth.Attributes["email"].(schema.StringAttribute)
	require.True(t, ok, "authentication.email must be a StringAttribute")
	assert.True(t, email.Sensitive, "the Pub/Sub service account email must never be rendered in plan output")

	secretKey, ok := auth.Attributes["secret_key"].(schema.StringAttribute)
	require.True(t, ok, "authentication.secret_key must be a StringAttribute")
	assert.True(t, secretKey.Sensitive, "the Pub/Sub secret key must never be rendered in plan output")
	assert.Len(t, secretKey.Validators, 1, "secret_key must reject leading/trailing whitespace")

	assert.Len(t, auth.Attributes, 3, "authentication holds account_name, email, and secret_key for Pub/Sub")
}

// TestAuthenticationAccessors covers the object-unwrapping accessors, including
// the degenerate states the framework can hand us (null/unknown object, absent
// attribute), where an empty string is the safe answer rather than a panic.
func TestAuthenticationAccessors(t *testing.T) {
	full := fullNestedModel()
	assert.Equal(t, "service-account", full.AccountName().ValueString())
	assert.Equal(t, "fastly-pubsub-log@example-fastly-log.iam.gserviceaccount.com", full.Email().ValueString())
	assert.Equal(t, "test-secret-key", full.SecretKey().ValueString())

	tests := map[string]types.Object{
		"null object":    types.ObjectNull(authenticationAttributeTypes),
		"unknown object": types.ObjectUnknown(authenticationAttributeTypes),
		"null fields": NewAuthenticationObject(
			types.StringNull(), types.StringNull(), types.StringNull(),
		),
		"unknown fields": NewAuthenticationObject(
			types.StringUnknown(), types.StringUnknown(), types.StringUnknown(),
		),
	}
	for name, obj := range tests {
		t.Run(name, func(t *testing.T) {
			m := commonModel{Authentication: obj}
			assert.Equal(t, "", m.AccountName().ValueString())
			assert.Equal(t, "", m.Email().ValueString())
			assert.Equal(t, "", m.SecretKey().ValueString())
		})
	}
}

// TestAuthenticationEnvDefault_DefaultObject covers the schema.Default set on
// the `authentication` attribute itself (schema.go), which is what makes the
// FASTLY_GOOGLE_PUBSUB_* environment variables apply when the whole
// `authentication` block is omitted from config — a plain schema.Default on
// account_name alone is never evaluated in that case, since the framework
// marks the parent object wholly unknown instead of walking into its
// children.
func TestAuthenticationEnvDefault_DefaultObject(t *testing.T) {
	t.Run("blank when no env vars set", func(t *testing.T) {
		t.Setenv("FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME", "")
		t.Setenv("FASTLY_GCS_ACCOUNT_NAME", "")
		t.Setenv("FASTLY_GOOGLE_PUBSUB_EMAIL", "")
		t.Setenv("FASTLY_GOOGLE_PUBSUB_SECRET_KEY", "")

		var resp defaults.ObjectResponse
		authenticationEnvDefault{}.DefaultObject(context.Background(), defaults.ObjectRequest{}, &resp)

		assert.Equal(t, NewAuthenticationObject(types.StringValue(""), types.StringValue(""), types.StringValue("")), resp.PlanValue)
	})

	t.Run("account name from shared env var", func(t *testing.T) {
		t.Setenv("FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME", "test-service-account")
		t.Setenv("FASTLY_GCS_ACCOUNT_NAME", "")
		t.Setenv("FASTLY_GOOGLE_PUBSUB_EMAIL", "")
		t.Setenv("FASTLY_GOOGLE_PUBSUB_SECRET_KEY", "")

		var resp defaults.ObjectResponse
		authenticationEnvDefault{}.DefaultObject(context.Background(), defaults.ObjectRequest{}, &resp)

		assert.Equal(t, NewAuthenticationObject(
			types.StringValue("test-service-account"),
			types.StringValue(""),
			types.StringValue(""),
		), resp.PlanValue)
		assert.Empty(t, resp.Diagnostics)
	})

	t.Run("email and secret key from env", func(t *testing.T) {
		t.Setenv("FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME", "")
		t.Setenv("FASTLY_GOOGLE_PUBSUB_EMAIL", "test-pubsub@example-fastly-log.iam.gserviceaccount.com")
		t.Setenv("FASTLY_GOOGLE_PUBSUB_SECRET_KEY", "test-secret-key")

		var resp defaults.ObjectResponse
		authenticationEnvDefault{}.DefaultObject(context.Background(), defaults.ObjectRequest{}, &resp)

		assert.Equal(t, NewAuthenticationObject(
			types.StringValue(""),
			types.StringValue("test-pubsub@example-fastly-log.iam.gserviceaccount.com"),
			types.StringValue("test-secret-key"),
		), resp.PlanValue)
	})

	// Pub/Sub's legacy schema borrowed GCS's account_name env var name before
	// the shared FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME existed — see
	// deprecatedGCSAccountNameEnvVar.
	t.Run("account name falls back to deprecated env var with a warning", func(t *testing.T) {
		t.Setenv("FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME", "")
		t.Setenv("FASTLY_GCS_ACCOUNT_NAME", "legacy-service-account")
		t.Setenv("FASTLY_GOOGLE_PUBSUB_EMAIL", "")
		t.Setenv("FASTLY_GOOGLE_PUBSUB_SECRET_KEY", "")

		var resp defaults.ObjectResponse
		authenticationEnvDefault{}.DefaultObject(context.Background(), defaults.ObjectRequest{}, &resp)

		assert.Equal(t, NewAuthenticationObject(
			types.StringValue("legacy-service-account"),
			types.StringValue(""),
			types.StringValue(""),
		), resp.PlanValue)
		require.Len(t, resp.Diagnostics, 1)
		assert.Equal(t, 1, resp.Diagnostics.WarningsCount())
	})

	t.Run("account name prefers new env var over deprecated one", func(t *testing.T) {
		t.Setenv("FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME", "current-service-account")
		t.Setenv("FASTLY_GCS_ACCOUNT_NAME", "legacy-service-account")
		t.Setenv("FASTLY_GOOGLE_PUBSUB_EMAIL", "")
		t.Setenv("FASTLY_GOOGLE_PUBSUB_SECRET_KEY", "")

		var resp defaults.ObjectResponse
		authenticationEnvDefault{}.DefaultObject(context.Background(), defaults.ObjectRequest{}, &resp)

		assert.Equal(t, NewAuthenticationObject(
			types.StringValue("current-service-account"),
			types.StringValue(""),
			types.StringValue(""),
		), resp.PlanValue)
		assert.Empty(t, resp.Diagnostics)
	})
}

// TestSchemaValidators pins the accepted values for the remaining validators, so
// a change to an enum member or a bound is a test failure rather than a
// surprise at plan time: processing_region none/us/eu, format_version 1-2,
// placement "none" only, format capped at 12288.
func TestSchemaValidators(t *testing.T) {
	attrs := CommonAttributes()

	stringCases := []struct {
		name  string
		attr  string
		value string
		valid bool
	}{
		{"processing_region none", "processing_region", "none", true},
		{"processing_region us", "processing_region", "us", true},
		{"processing_region eu", "processing_region", "eu", true},
		{"processing_region rejects wrong case", "processing_region", "US", false},
		{"processing_region rejects empty", "processing_region", "", false},
		{"placement none", "placement", "none", true},
		{"placement rejects other VCL subroutines", "placement", "waf_debug", false},
		{"placement rejects empty", "placement", "", false},
		{"format at max length", "format", strings.Repeat("x", maximumFormatLength), true},
		{"format over max length", "format", strings.Repeat("x", maximumFormatLength+1), false},
	}
	for _, tt := range stringCases {
		t.Run(tt.name, func(t *testing.T) {
			a := attrs[tt.attr].(schema.StringAttribute)
			require.NotEmpty(t, a.Validators)
			resp := &validator.StringResponse{}
			for _, v := range a.Validators {
				v.ValidateString(context.Background(),
					validator.StringRequest{ConfigValue: types.StringValue(tt.value)}, resp)
			}
			assert.Equal(t, tt.valid, !resp.Diagnostics.HasError())
		})
	}

	int64Cases := []struct {
		value int64
		valid bool
	}{{0, false}, {1, true}, {2, true}, {3, false}}
	for _, tt := range int64Cases {
		t.Run(fmt.Sprintf("format_version %d", tt.value), func(t *testing.T) {
			a := attrs["format_version"].(schema.Int64Attribute)
			require.Len(t, a.Validators, 1)
			resp := &validator.Int64Response{}
			a.Validators[0].ValidateInt64(context.Background(),
				validator.Int64Request{ConfigValue: types.Int64Value(tt.value)}, resp)
			assert.Equal(t, tt.valid, !resp.Diagnostics.HasError())
		})
	}

	// name, project_id, topic, and response_condition accept any string;
	// assert that rather than leaving it implicit.
	for _, name := range []string{"name", "project_id", "topic", "response_condition"} {
		assert.Empty(t, attrs[name].(schema.StringAttribute).Validators, name)
	}
	auth := attrs["authentication"].(schema.SingleNestedAttribute)
	assert.Empty(t, auth.Attributes["account_name"].(schema.StringAttribute).Validators)
	assert.Empty(t, auth.Attributes["email"].(schema.StringAttribute).Validators)
}

// TestSecretKeyRejectsUntrimmedValues locks in that secret_key rejects leading
// or trailing whitespace, mirroring the legacy provider's
// validateStringTrimmed check on the same field.
func TestSecretKeyRejectsUntrimmedValues(t *testing.T) {
	attrs := CommonAttributes()
	auth := attrs["authentication"].(schema.SingleNestedAttribute)
	secretKey := auth.Attributes["secret_key"].(schema.StringAttribute)
	require.Len(t, secretKey.Validators, 1)

	for _, tt := range []struct {
		value string
		valid bool
	}{
		{"clean-secret-key", true},
		{" leading-space", false},
		{"trailing-space ", false},
		{"\ttab-prefixed", false},
	} {
		t.Run(tt.value, func(t *testing.T) {
			resp := &validator.StringResponse{}
			secretKey.Validators[0].ValidateString(context.Background(),
				validator.StringRequest{ConfigValue: types.StringValue(tt.value)}, resp)
			assert.Equal(t, tt.valid, !resp.Diagnostics.HasError())
		})
	}
}

func TestValidateConditionReferences(t *testing.T) {
	conditionNames := map[string]struct{}{"my-condition": {}}

	t.Run("no response_condition set", func(t *testing.T) {
		item := minimalNestedModel()
		assert.NoError(t, ValidateConditionReferences([]NestedModel{item}, nil))
	})

	t.Run("references a configured condition", func(t *testing.T) {
		item := minimalNestedModel()
		item.ResponseCondition = types.StringValue("my-condition")

		assert.NoError(t, ValidateConditionReferences([]NestedModel{item}, conditionNames))
	})

	t.Run("references a condition that isn't configured", func(t *testing.T) {
		item := minimalNestedModel()
		item.ResponseCondition = types.StringValue("missing-condition")

		err := ValidateConditionReferences([]NestedModel{item}, conditionNames)
		if assert.Error(t, err) {
			assert.Contains(t, err.Error(), `"test-pubsub"`)
			assert.Contains(t, err.Error(), `"missing-condition"`)
		}
	})

	t.Run("skips unknown condition", func(t *testing.T) {
		item := minimalNestedModel()
		item.ResponseCondition = types.StringUnknown()

		assert.NoError(t, ValidateConditionReferences([]NestedModel{item}, nil))
	})
}

// TestAuthenticationEitherOr covers the cross-field validator on the
// authentication block: either account_name, or both email and secret_key.
func TestAuthenticationEitherOr(t *testing.T) {
	t.Setenv("FASTLY_GOOGLE_SERVICE_ACCOUNT_NAME", "")
	t.Setenv("FASTLY_GCS_ACCOUNT_NAME", "")
	t.Setenv("FASTLY_GOOGLE_PUBSUB_EMAIL", "")
	t.Setenv("FASTLY_GOOGLE_PUBSUB_SECRET_KEY", "")

	tests := []struct {
		name    string
		obj     types.Object
		wantErr bool
	}{
		{
			name:    "account_name only",
			obj:     NewAuthenticationObject(types.StringValue("account"), types.StringNull(), types.StringNull()),
			wantErr: false,
		},
		{
			name:    "email and secret_key only",
			obj:     NewAuthenticationObject(types.StringNull(), types.StringValue("email"), types.StringValue("key")),
			wantErr: false,
		},
		{
			name:    "neither set",
			obj:     NewAuthenticationObject(types.StringNull(), types.StringNull(), types.StringNull()),
			wantErr: true,
		},
		{
			name:    "only email set",
			obj:     NewAuthenticationObject(types.StringNull(), types.StringValue("email"), types.StringNull()),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp := &validator.ObjectResponse{}
			authenticationEitherOr{}.ValidateObject(context.Background(), validator.ObjectRequest{ConfigValue: tt.obj}, resp)
			assert.Equal(t, tt.wantErr, resp.Diagnostics.HasError())
		})
	}
}
