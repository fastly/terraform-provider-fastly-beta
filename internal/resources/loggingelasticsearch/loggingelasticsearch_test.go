package loggingelasticsearch

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
		Format:            types.StringValue(constants.LoggingElasticsearchDefaultFormat),
		FormatVersion:     types.Int64Value(DefaultFormatVersion),
		Placement:         types.StringNull(),
		ResponseCondition: types.StringValue(DefaultResponseCondition),
	}
}

func defaultCommonModel() commonModel {
	return commonModel{
		Name:              types.StringValue(""),
		Index:             types.StringValue(""),
		URL:               types.StringValue(""),
		Authentication:    NewAuthenticationObject(types.StringValue(DefaultUser), types.StringValue(DefaultPassword)),
		TLS:               NewTLSObject(types.StringValue(""), types.StringValue(""), types.StringValue(""), types.StringValue(DefaultTLSHostname)),
		Pipeline:          types.StringValue(DefaultPipeline),
		ProcessingRegion:  types.StringValue(DefaultProcessingRegion),
		RequestMaxBytes:   types.Int64Value(DefaultRequestMaxBytes),
		RequestMaxEntries: types.Int64Value(DefaultRequestMaxEntries),
	}
}

func fullNestedModel() NestedModel {
	m := defaultNestedModel()
	m.Name = types.StringValue("test-elasticsearch")
	m.Index = types.StringValue("logs-index")
	m.URL = types.StringValue("https://elasticsearch.example.com")
	m.Authentication = NewAuthenticationObject(types.StringValue("es-user"), types.StringValue("es-password"))
	m.TLS = NewTLSObject(
		types.StringValue("ca-cert"),
		types.StringValue("client-cert"),
		types.StringValue("client-key"),
		types.StringValue("elasticsearch.example.com"),
	)
	m.Pipeline = types.StringValue("my-pipeline")
	m.ProcessingRegion = types.StringValue("eu")
	m.RequestMaxBytes = types.Int64Value(1000)
	m.RequestMaxEntries = types.Int64Value(100)
	m.Format = types.StringValue("%h %l %u")
	m.FormatVersion = types.Int64Value(1)
	m.Placement = types.StringValue("none")
	m.ResponseCondition = types.StringValue("response-condition-1")
	return m
}

func minimalNestedModel() NestedModel {
	m := defaultNestedModel()
	m.Name = types.StringValue("test-elasticsearch")
	m.Index = types.StringValue("logs-index")
	m.URL = types.StringValue("https://elasticsearch.example.com")
	return m
}

func fullComputeNestedModel() ComputeNestedModel {
	return ComputeNestedModel{commonModel: fullNestedModel().commonModel}
}

// Tests for flatten.go

func TestFlattenToNestedModel(t *testing.T) {
	tests := []struct {
		name     string
		api      *fastly.Elasticsearch
		expected NestedModel
	}{
		{
			name:     "nil returns empty model",
			api:      nil,
			expected: NestedModel{},
		},
		{
			name: "only required fields uses defaults",
			api: &fastly.Elasticsearch{
				Name:  new("test-elasticsearch"),
				Index: new("logs-index"),
				URL:   new("https://elasticsearch.example.com"),
			},
			expected: minimalNestedModel(),
		},
		{
			name: "all fields populated",
			api: &fastly.Elasticsearch{
				Name:              new("test-elasticsearch"),
				Index:             new("logs-index"),
				URL:               new("https://elasticsearch.example.com"),
				User:              new("es-user"),
				Password:          new("es-password"),
				TLSCACert:         new("ca-cert"),
				TLSClientCert:     new("client-cert"),
				TLSClientKey:      new("client-key"),
				TLSHostname:       new("elasticsearch.example.com"),
				Pipeline:          new("my-pipeline"),
				ProcessingRegion:  new("eu"),
				RequestMaxBytes:   new(1000),
				RequestMaxEntries: new(100),
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
	api := &fastly.Elasticsearch{
		Name:              new("test-elasticsearch"),
		Index:             new("logs-index"),
		URL:               new("https://elasticsearch.example.com"),
		User:              new("es-user"),
		Password:          new("es-password"),
		TLSCACert:         new("ca-cert"),
		TLSClientCert:     new("client-cert"),
		TLSClientKey:      new("client-key"),
		TLSHostname:       new("elasticsearch.example.com"),
		Pipeline:          new("my-pipeline"),
		ProcessingRegion:  new("eu"),
		RequestMaxBytes:   new(1000),
		RequestMaxEntries: new(100),
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
		api      *fastly.Elasticsearch
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
			api: &fastly.Elasticsearch{
				ServiceID:      new("service-123"),
				ServiceVersion: new(5),
				Name:           new("test-elasticsearch"),
				Index:          new("logs-index"),
				URL:            new("https://elasticsearch.example.com"),
			},
			validate: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-test-elasticsearch"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("test-elasticsearch"), m.Name)
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
		validate  func(t *testing.T, input *fastly.CreateElasticsearchInput)
	}{
		{
			name:      "minimal model",
			serviceID: "service-123",
			version:   5,
			model:     minimalNestedModel(),
			validate: func(t *testing.T, input *fastly.CreateElasticsearchInput) {
				assert.Equal(t, "service-123", input.ServiceID)
				assert.Equal(t, 5, input.ServiceVersion)
				assert.Equal(t, "test-elasticsearch", *input.Name)
				assert.Equal(t, "logs-index", *input.Index)
				assert.Equal(t, "https://elasticsearch.example.com", *input.URL)
				assert.Equal(t, "none", *input.ProcessingRegion)
				assert.Equal(t, constants.LoggingElasticsearchDefaultFormat, *input.Format)
				assert.Nil(t, input.User, "empty user must be omitted, not sent as \"\"")
				assert.Nil(t, input.Password, "empty password must be omitted, not sent as \"\"")
				assert.Nil(t, input.TLSCACert, "empty tls.ca_cert must be omitted, not sent as \"\"")
				assert.Nil(t, input.Pipeline, "empty pipeline must be omitted, not sent as \"\"")
				assert.Nil(t, input.RequestMaxBytes, "zero request_max_bytes must be omitted, not sent as 0")
				assert.Nil(t, input.RequestMaxEntries, "zero request_max_entries must be omitted, not sent as 0")
				assert.Nil(t, input.Placement, "unset placement must not be sent as \"none\" — the API treats them differently")
			},
		},
		{
			name:      "fully populated model",
			serviceID: "service-456",
			version:   10,
			model:     fullNestedModel(),
			validate: func(t *testing.T, input *fastly.CreateElasticsearchInput) {
				assert.Equal(t, "test-elasticsearch", *input.Name)
				assert.Equal(t, "logs-index", *input.Index)
				assert.Equal(t, "https://elasticsearch.example.com", *input.URL)
				assert.Equal(t, "es-user", *input.User)
				assert.Equal(t, "es-password", *input.Password)
				assert.Equal(t, "ca-cert", *input.TLSCACert)
				assert.Equal(t, "client-cert", *input.TLSClientCert)
				assert.Equal(t, "client-key", *input.TLSClientKey)
				assert.Equal(t, "elasticsearch.example.com", *input.TLSHostname)
				assert.Equal(t, "my-pipeline", *input.Pipeline)
				assert.Equal(t, "eu", *input.ProcessingRegion)
				assert.Equal(t, 1000, *input.RequestMaxBytes)
				assert.Equal(t, 100, *input.RequestMaxEntries)
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
	assert.Equal(t, "test-elasticsearch", *input.Name)
	assert.Equal(t, "logs-index", *input.Index)
	assert.Equal(t, "https://elasticsearch.example.com", *input.URL)
	assert.Equal(t, "es-user", *input.User)
	assert.Equal(t, "my-pipeline", *input.Pipeline)
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
	assert.Equal(t, "test-elasticsearch", input.Name)
	assert.Equal(t, "test-elasticsearch", *input.NewName)
	assert.Equal(t, "logs-index", *input.Index)
	assert.Equal(t, "https://elasticsearch.example.com", *input.URL)
	assert.Equal(t, "es-user", *input.User)
	assert.Equal(t, "es-password", *input.Password)
	assert.Equal(t, "ca-cert", *input.TLSCACert)
	assert.Equal(t, "client-key", *input.TLSClientKey)
	assert.Equal(t, "my-pipeline", *input.Pipeline)
	assert.Equal(t, "eu", *input.ProcessingRegion)
	assert.Equal(t, 1000, *input.RequestMaxBytes)
	assert.Equal(t, 100, *input.RequestMaxEntries)
	assert.Equal(t, "%h %l %u", *input.Format)
	assert.Equal(t, 1, *input.FormatVersion)
	assert.Equal(t, fastly.NewNullable("none"), input.Placement)
	assert.Equal(t, "response-condition-1", *input.ResponseCondition)
}

// TestBuildUpdateInputClearsClearableFields verifies empty-default fields are
// sent as concrete values on update, not omitted (which would leave a
// previously-set value in place).
func TestBuildUpdateInputClearsClearableFields(t *testing.T) {
	input := BuildUpdateInput("service-1", 1, minimalNestedModel())

	assert.NotNil(t, input.User, "user must be sent even when empty")
	assert.Equal(t, "", *input.User)
	assert.NotNil(t, input.Password, "password must be sent even when empty")
	assert.Equal(t, "", *input.Password)
	assert.NotNil(t, input.TLSCACert, "tls.ca_cert must be sent even when empty")
	assert.Equal(t, "", *input.TLSCACert)
	assert.NotNil(t, input.Pipeline, "pipeline must be sent even when empty")
	assert.Equal(t, "", *input.Pipeline)
	assert.NotNil(t, input.RequestMaxBytes, "request_max_bytes must be sent even when zero")
	assert.Equal(t, 0, *input.RequestMaxBytes)
	assert.NotNil(t, input.ResponseCondition, "response_condition must be sent even when empty")
	assert.Equal(t, "", *input.ResponseCondition)
	assert.NotNil(t, input.Placement, "unset placement must be sent as an explicit null, not omitted (omitting leaves a previously-set \"none\" in place)")
	assert.Equal(t, fastly.NullValue[string](), input.Placement)
}

func TestBuildComputeUpdateInput(t *testing.T) {
	input := BuildComputeUpdateInput("service-456", 10, fullComputeNestedModel())

	assert.Equal(t, "service-456", input.ServiceID)
	assert.Equal(t, 10, input.ServiceVersion)
	assert.Equal(t, "test-elasticsearch", input.Name)
	assert.Equal(t, "test-elasticsearch", *input.NewName)
	assert.Equal(t, "es-user", *input.User)
	assert.Equal(t, "my-pipeline", *input.Pipeline)
	assert.Equal(t, "eu", *input.ProcessingRegion)
	assert.Nil(t, input.Format)
	assert.Nil(t, input.FormatVersion)
	assert.Nil(t, input.Placement)
	assert.Nil(t, input.ResponseCondition)
}

func TestClearVCLOnlyCreateFields(t *testing.T) {
	input := &fastly.CreateElasticsearchInput{
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
	input := &fastly.UpdateElasticsearchInput{
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

// TestResetVCLOnlyToDefaults covers the Compute read-back path: the API
// reports its own VCL-only values, which must be discarded to avoid a
// consistency-after-apply failure.
func TestResetVCLOnlyToDefaults(t *testing.T) {
	// What the API actually reports back for a Compute service.
	m := FlattenToNestedModel(&fastly.Elasticsearch{
		Name:              new("test-elasticsearch"),
		Index:             new("logs-index"),
		URL:               new("https://elasticsearch.example.com"),
		ProcessingRegion:  new("none"),
		Format:            new("{\n  \"time\": 1\n}\n"),
		FormatVersion:     new(1),
		Placement:         new("none"),
		ResponseCondition: new("some-condition"),
	})

	ResetVCLOnlyToDefaults(&m)

	assert.Equal(t, constants.LoggingElasticsearchDefaultFormat, m.Format.ValueString())
	assert.Equal(t, int64(DefaultFormatVersion), m.FormatVersion.ValueInt64())
	assert.True(t, m.Placement.IsNull(), "placement must go back to unset, not the API's forced \"none\"")
	assert.Equal(t, DefaultResponseCondition, m.ResponseCondition.ValueString())

	// Non-VCL-only fields must survive untouched.
	assert.Equal(t, "test-elasticsearch", m.Name.ValueString())
	assert.Equal(t, "logs-index", m.Index.ValueString())
	assert.Equal(t, "https://elasticsearch.example.com", m.URL.ValueString())
	assert.Equal(t, "none", m.ProcessingRegion.ValueString())
}

// TestResetVCLOnlyToDefaultsMatchesPlannedDefaults ties the reset values to
// the schema's own declared defaults.
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
			name: "different url",
			a: func() NestedModel {
				m := minimalNestedModel()
				m.URL = types.StringValue("https://one.example.com")
				return m
			}(),
			b: func() NestedModel {
				m := minimalNestedModel()
				m.URL = types.StringValue("https://two.example.com")
				return m
			}(),
			expected: false,
		},
		{
			name: "different password",
			a: func() NestedModel {
				m := minimalNestedModel()
				m.Authentication = NewAuthenticationObject(types.StringValue("user"), types.StringValue("password-1"))
				return m
			}(),
			b: func() NestedModel {
				m := minimalNestedModel()
				m.Authentication = NewAuthenticationObject(types.StringValue("user"), types.StringValue("password-2"))
				return m
			}(),
			expected: false,
		},
		{
			name: "different tls client_key",
			a: func() NestedModel {
				m := minimalNestedModel()
				m.TLS = NewTLSObject(types.StringValue(""), types.StringValue(""), types.StringValue("key-1"), types.StringValue(""))
				return m
			}(),
			b: func() NestedModel {
				m := minimalNestedModel()
				m.TLS = NewTLSObject(types.StringValue(""), types.StringValue(""), types.StringValue("key-2"), types.StringValue(""))
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

	b.Pipeline = types.StringValue("other-pipeline")
	assert.False(t, a.ModelsEqual(b))
}

// TestComputeModelsEqualIgnoresVCLOnlyFields verifies remote VCL-only values
// don't cause a spurious update on Compute.
func TestComputeModelsEqualIgnoresVCLOnlyFields(t *testing.T) {
	desired := fullComputeNestedModel()

	remote := &fastly.Elasticsearch{
		Name:              new("test-elasticsearch"),
		Index:             new("logs-index"),
		URL:               new("https://elasticsearch.example.com"),
		User:              new("es-user"),
		Password:          new("es-password"),
		TLSCACert:         new("ca-cert"),
		TLSClientCert:     new("client-cert"),
		TLSClientKey:      new("client-key"),
		TLSHostname:       new("elasticsearch.example.com"),
		Pipeline:          new("my-pipeline"),
		ProcessingRegion:  new("eu"),
		RequestMaxBytes:   new(1000),
		RequestMaxEntries: new(100),
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
					m.Pipeline = types.StringValue("other-pipeline")
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

	b[0].Pipeline = types.StringValue("other-pipeline")
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

// TestComputeAttributesOmitsVCLOnly locks in that Compute's schema omits the
// VCL-only attributes entirely.
func TestComputeAttributesOmitsVCLOnly(t *testing.T) {
	compute := ComputeAttributes()
	common := CommonAttributes()

	for _, name := range []string{"format", "format_version", "placement", "response_condition"} {
		assert.NotContains(t, compute, name)
		assert.Contains(t, common, name)
	}

	for _, name := range []string{"name", "index", "url", "authentication", "tls", "pipeline", "processing_region", "request_max_bytes", "request_max_entries"} {
		assert.Contains(t, compute, name)
		assert.Contains(t, common, name)
	}

	// user/password and the tls.* fields are nested, never top-level attributes.
	for _, name := range []string{"user", "password", "ca_cert", "client_cert", "client_key", "hostname"} {
		assert.NotContains(t, compute, name)
		assert.NotContains(t, common, name)
	}
}

// TestAuthenticationAttribute locks in the credential shape: no env var
// fallback, password marked Sensitive.
func TestAuthenticationAttribute(t *testing.T) {
	auth, ok := ComputeAttributes()["authentication"].(schema.SingleNestedAttribute)
	require.True(t, ok, "authentication must be a SingleNestedAttribute")

	assert.True(t, auth.Optional)
	assert.True(t, auth.Computed)
	assert.NotNil(t, auth.Default)

	user, ok := auth.Attributes["user"].(schema.StringAttribute)
	require.True(t, ok, "authentication.user must be a StringAttribute")
	assert.True(t, user.Optional)
	assert.True(t, user.Computed)
	assert.False(t, user.Sensitive)

	password, ok := auth.Attributes["password"].(schema.StringAttribute)
	require.True(t, ok, "authentication.password must be a StringAttribute")
	assert.True(t, password.Optional)
	assert.True(t, password.Computed)
	assert.True(t, password.Sensitive, "the Elasticsearch password must never be rendered in plan output")

	assert.Len(t, auth.Attributes, 2)
}

// TestTLSAttribute locks in the tls object shape: no env var fallback (unlike
// other logging endpoints), client_key marked Sensitive.
func TestTLSAttribute(t *testing.T) {
	tls, ok := ComputeAttributes()["tls"].(schema.SingleNestedAttribute)
	require.True(t, ok, "tls must be a SingleNestedAttribute")

	assert.True(t, tls.Optional)
	assert.True(t, tls.Computed)
	assert.NotNil(t, tls.Default)

	clientKey, ok := tls.Attributes["client_key"].(schema.StringAttribute)
	require.True(t, ok)
	assert.True(t, clientKey.Sensitive, "the TLS client private key must never be rendered in plan output")

	for _, name := range []string{"ca_cert", "client_cert", "hostname"} {
		attr, ok := tls.Attributes[name].(schema.StringAttribute)
		require.True(t, ok, "tls.%s must be a StringAttribute", name)
		assert.False(t, attr.Sensitive, "tls.%s is not credential material", name)
	}

	assert.Len(t, tls.Attributes, 4)
}

// TestAuthenticationAccessors covers the null/unknown object states, which
// must return "" rather than panic.
func TestAuthenticationAccessors(t *testing.T) {
	full := fullNestedModel()
	assert.Equal(t, "es-user", full.User().ValueString())
	assert.Equal(t, "es-password", full.Password().ValueString())

	tests := map[string]types.Object{
		"null object":    types.ObjectNull(authenticationAttributeTypes),
		"unknown object": types.ObjectUnknown(authenticationAttributeTypes),
		"null fields":    NewAuthenticationObject(types.StringNull(), types.StringNull()),
		"unknown fields": NewAuthenticationObject(types.StringUnknown(), types.StringUnknown()),
	}
	for name, obj := range tests {
		t.Run(name, func(t *testing.T) {
			m := commonModel{Authentication: obj}
			assert.Equal(t, "", m.User().ValueString())
			assert.Equal(t, "", m.Password().ValueString())
		})
	}
}

// TestTLSAccessors mirrors TestAuthenticationAccessors for the tls object's fields.
func TestTLSAccessors(t *testing.T) {
	full := fullNestedModel()
	assert.Equal(t, "ca-cert", full.TLSCACert().ValueString())
	assert.Equal(t, "client-cert", full.TLSClientCert().ValueString())
	assert.Equal(t, "client-key", full.TLSClientKey().ValueString())
	assert.Equal(t, "elasticsearch.example.com", full.TLSHostname().ValueString())

	tests := map[string]types.Object{
		"null object":    types.ObjectNull(tlsAttributeTypes),
		"unknown object": types.ObjectUnknown(tlsAttributeTypes),
	}
	for name, obj := range tests {
		t.Run(name, func(t *testing.T) {
			m := commonModel{TLS: obj}
			assert.Equal(t, "", m.TLSCACert().ValueString())
			assert.Equal(t, "", m.TLSClientCert().ValueString())
			assert.Equal(t, "", m.TLSClientKey().ValueString())
			assert.Equal(t, "", m.TLSHostname().ValueString())
		})
	}
}

// TestSchemaValidators pins the accepted values for processing_region,
// format_version, placement, format length, and the tls.* whitespace-trimmed
// check (unique to Elasticsearch among the logging endpoints).
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

	tlsAttrs := attrs["tls"].(schema.SingleNestedAttribute).Attributes
	trimmedCases := []struct {
		name  string
		attr  string
		value string
		valid bool
	}{
		{"ca_cert accepts trimmed value", "ca_cert", "-----BEGIN CERTIFICATE-----", true},
		{"ca_cert rejects leading whitespace", "ca_cert", " -----BEGIN CERTIFICATE-----", false},
		{"client_cert rejects trailing whitespace", "client_cert", "-----BEGIN CERTIFICATE----- \n", false},
		{"client_key rejects trailing whitespace", "client_key", "-----BEGIN PRIVATE KEY----- \n", false},
	}
	for _, tt := range trimmedCases {
		t.Run(tt.name, func(t *testing.T) {
			a := tlsAttrs[tt.attr].(schema.StringAttribute)
			require.NotEmpty(t, a.Validators)
			resp := &validator.StringResponse{}
			for _, v := range a.Validators {
				v.ValidateString(context.Background(),
					validator.StringRequest{ConfigValue: types.StringValue(tt.value)}, resp)
			}
			assert.Equal(t, tt.valid, !resp.Diagnostics.HasError())
		})
	}
	// hostname has no trimmed validator.
	assert.Empty(t, tlsAttrs["hostname"].(schema.StringAttribute).Validators)

	// name, index, pipeline, and response_condition accept any value; assert
	// that rather than leaving it implicit.
	assert.Empty(t, attrs["name"].(schema.StringAttribute).Validators)
	assert.Empty(t, attrs["index"].(schema.StringAttribute).Validators)
	assert.Empty(t, attrs["pipeline"].(schema.StringAttribute).Validators)
	assert.Empty(t, attrs["response_condition"].(schema.StringAttribute).Validators)
	assert.Empty(t, attrs["request_max_bytes"].(schema.Int64Attribute).Validators)
	assert.Empty(t, attrs["request_max_entries"].(schema.Int64Attribute).Validators)
	auth := attrs["authentication"].(schema.SingleNestedAttribute)
	assert.Empty(t, auth.Attributes["user"].(schema.StringAttribute).Validators)
	assert.Empty(t, auth.Attributes["password"].(schema.StringAttribute).Validators)
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
			assert.Contains(t, err.Error(), `"test-elasticsearch"`)
			assert.Contains(t, err.Error(), `"missing-condition"`)
		}
	})

	t.Run("skips unknown condition", func(t *testing.T) {
		item := minimalNestedModel()
		item.ResponseCondition = types.StringUnknown()

		assert.NoError(t, ValidateConditionReferences([]NestedModel{item}, nil))
	})
}
