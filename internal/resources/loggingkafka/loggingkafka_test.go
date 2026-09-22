package loggingkafka

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
		Format:            types.StringValue(constants.LoggingKafkaDefaultFormat),
		FormatVersion:     types.Int64Value(DefaultFormatVersion),
		Placement:         types.StringNull(),
		ResponseCondition: types.StringValue(DefaultResponseCondition),
	}
}

func defaultCommonModel() commonModel {
	return commonModel{
		Name:             types.StringValue(""),
		Brokers:          types.StringValue(""),
		Topic:            types.StringValue(""),
		CompressionCodec: types.StringValue(DefaultCompressionCodec),
		RequiredACKs:     types.StringValue(DefaultRequiredACKs),
		RequestMaxBytes:  types.Int64Value(DefaultRequestMaxBytes),
		ParseLogKeyvals:  types.BoolValue(DefaultParseLogKeyvals),
		AuthMethod:       types.StringValue(DefaultAuthMethod),
		Authentication:   NewAuthenticationObject(types.StringValue(DefaultUser), types.StringValue(DefaultPassword)),
		TLS:              NewTLSObject(types.StringValue(""), types.StringValue(""), types.StringValue(""), types.StringValue(DefaultTLSHostname)),
		UseTLS:           types.BoolValue(DefaultUseTLS),
		ProcessingRegion: types.StringValue(DefaultProcessingRegion),
	}
}

func fullNestedModel() NestedModel {
	m := defaultNestedModel()
	m.Name = types.StringValue("test-kafka")
	m.Brokers = types.StringValue("127.0.0.1:9092,127.0.0.2:9092")
	m.Topic = types.StringValue("test-topic")
	m.CompressionCodec = types.StringValue("snappy")
	m.RequiredACKs = types.StringValue("-1")
	m.RequestMaxBytes = types.Int64Value(12345)
	m.ParseLogKeyvals = types.BoolValue(true)
	m.AuthMethod = types.StringValue("scram-sha-512")
	m.Authentication = NewAuthenticationObject(types.StringValue("kafka-user"), types.StringValue("kafka-password"))
	m.TLS = NewTLSObject(
		types.StringValue("ca-cert"),
		types.StringValue("client-cert"),
		types.StringValue("client-key"),
		types.StringValue("kafka.example.com"),
	)
	m.UseTLS = types.BoolValue(true)
	m.ProcessingRegion = types.StringValue("eu")
	m.Format = types.StringValue("%h %l %u")
	m.FormatVersion = types.Int64Value(1)
	m.Placement = types.StringValue("none")
	m.ResponseCondition = types.StringValue("response-condition-1")
	return m
}

func minimalNestedModel() NestedModel {
	m := defaultNestedModel()
	m.Name = types.StringValue("test-kafka")
	m.Brokers = types.StringValue("127.0.0.1:9092")
	m.Topic = types.StringValue("test-topic")
	return m
}

func fullComputeNestedModel() ComputeNestedModel {
	return ComputeNestedModel{commonModel: fullNestedModel().commonModel}
}

// Tests for flatten.go

func TestFlattenToNestedModel(t *testing.T) {
	tests := []struct {
		name     string
		api      *fastly.Kafka
		expected NestedModel
	}{
		{
			name:     "nil returns empty model",
			api:      nil,
			expected: NestedModel{},
		},
		{
			name: "only required fields uses defaults",
			api: &fastly.Kafka{
				Name:    new("test-kafka"),
				Brokers: new("127.0.0.1:9092"),
				Topic:   new("test-topic"),
			},
			expected: minimalNestedModel(),
		},
		{
			name: "all fields populated",
			api: &fastly.Kafka{
				Name:              new("test-kafka"),
				Brokers:           new("127.0.0.1:9092,127.0.0.2:9092"),
				Topic:             new("test-topic"),
				CompressionCodec:  new("snappy"),
				RequiredACKs:      new("-1"),
				RequestMaxBytes:   new(12345),
				ParseLogKeyvals:   new(true),
				AuthMethod:        new("scram-sha-512"),
				User:              new("kafka-user"),
				Password:          new("kafka-password"),
				TLSCACert:         new("ca-cert"),
				TLSClientCert:     new("client-cert"),
				TLSClientKey:      new("client-key"),
				TLSHostname:       new("kafka.example.com"),
				UseTLS:            new(true),
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
	api := &fastly.Kafka{
		Name:             new("test-kafka"),
		Brokers:          new("127.0.0.1:9092,127.0.0.2:9092"),
		Topic:            new("test-topic"),
		CompressionCodec: new("snappy"),
		RequiredACKs:     new("-1"),
		RequestMaxBytes:  new(12345),
		ParseLogKeyvals:  new(true),
		AuthMethod:       new("scram-sha-512"),
		User:             new("kafka-user"),
		Password:         new("kafka-password"),
		TLSCACert:        new("ca-cert"),
		TLSClientCert:    new("client-cert"),
		TLSClientKey:     new("client-key"),
		TLSHostname:      new("kafka.example.com"),
		UseTLS:           new(true),
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
		api      *fastly.Kafka
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
			api: &fastly.Kafka{
				ServiceID:      new("service-123"),
				ServiceVersion: new(5),
				Name:           new("test-kafka"),
				Brokers:        new("127.0.0.1:9092"),
				Topic:          new("test-topic"),
			},
			validate: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123/5/test-kafka"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("test-kafka"), m.Name)
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
		validate  func(t *testing.T, input *fastly.CreateKafkaInput)
	}{
		{
			name:      "minimal model",
			serviceID: "service-123",
			version:   5,
			model:     minimalNestedModel(),
			validate: func(t *testing.T, input *fastly.CreateKafkaInput) {
				assert.Equal(t, "service-123", input.ServiceID)
				assert.Equal(t, 5, input.ServiceVersion)
				assert.Equal(t, "test-kafka", *input.Name)
				assert.Equal(t, "127.0.0.1:9092", *input.Brokers)
				assert.Equal(t, "test-topic", *input.Topic)
				assert.Equal(t, DefaultRequiredACKs, *input.RequiredACKs)
				assert.False(t, bool(*input.ParseLogKeyvals))
				assert.False(t, bool(*input.UseTLS))
				assert.Equal(t, "none", *input.ProcessingRegion)
				assert.Equal(t, constants.LoggingKafkaDefaultFormat, *input.Format)
				assert.Nil(t, input.CompressionCodec, "empty compression_codec must be omitted, not sent as \"\"")
				assert.Nil(t, input.AuthMethod, "empty auth_method must be omitted, not sent as \"\"")
				assert.Nil(t, input.User, "empty user must be omitted, not sent as \"\"")
				assert.Nil(t, input.Password, "empty password must be omitted, not sent as \"\"")
				assert.Nil(t, input.TLSCACert, "empty tls.ca_cert must be omitted, not sent as \"\"")
				assert.Nil(t, input.RequestMaxBytes, "zero request_max_bytes must be omitted on create, not sent as 0")
				assert.Nil(t, input.Placement, "unset placement must not be sent as \"none\" — the API treats them differently")
			},
		},
		{
			name:      "fully populated model",
			serviceID: "service-456",
			version:   10,
			model:     fullNestedModel(),
			validate: func(t *testing.T, input *fastly.CreateKafkaInput) {
				assert.Equal(t, "test-kafka", *input.Name)
				assert.Equal(t, "127.0.0.1:9092,127.0.0.2:9092", *input.Brokers)
				assert.Equal(t, "test-topic", *input.Topic)
				assert.Equal(t, "snappy", *input.CompressionCodec)
				assert.Equal(t, "-1", *input.RequiredACKs)
				assert.Equal(t, 12345, *input.RequestMaxBytes)
				assert.True(t, bool(*input.ParseLogKeyvals))
				assert.Equal(t, "scram-sha-512", *input.AuthMethod)
				assert.Equal(t, "kafka-user", *input.User)
				assert.Equal(t, "kafka-password", *input.Password)
				assert.Equal(t, "ca-cert", *input.TLSCACert)
				assert.Equal(t, "client-cert", *input.TLSClientCert)
				assert.Equal(t, "client-key", *input.TLSClientKey)
				assert.Equal(t, "kafka.example.com", *input.TLSHostname)
				assert.True(t, bool(*input.UseTLS))
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
	assert.Equal(t, "test-kafka", *input.Name)
	assert.Equal(t, "127.0.0.1:9092,127.0.0.2:9092", *input.Brokers)
	assert.Equal(t, "test-topic", *input.Topic)
	assert.Equal(t, "kafka-user", *input.User)
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
	assert.Equal(t, "test-kafka", input.Name)
	assert.Equal(t, "test-kafka", *input.NewName)
	assert.Equal(t, "127.0.0.1:9092,127.0.0.2:9092", *input.Brokers)
	assert.Equal(t, "test-topic", *input.Topic)
	assert.Equal(t, "snappy", *input.CompressionCodec)
	assert.Equal(t, "-1", *input.RequiredACKs)
	assert.Equal(t, 12345, *input.RequestMaxBytes)
	assert.True(t, bool(*input.ParseLogKeyvals))
	assert.Equal(t, "scram-sha-512", *input.AuthMethod)
	assert.Equal(t, "kafka-user", *input.User)
	assert.Equal(t, "kafka-password", *input.Password)
	assert.Equal(t, "ca-cert", *input.TLSCACert)
	assert.Equal(t, "client-key", *input.TLSClientKey)
	assert.True(t, bool(*input.UseTLS))
	assert.Equal(t, "eu", *input.ProcessingRegion)
	assert.Equal(t, "%h %l %u", *input.Format)
	assert.Equal(t, 1, *input.FormatVersion)
	assert.Equal(t, fastly.NewNullable("none"), input.Placement)
	assert.Equal(t, "response-condition-1", *input.ResponseCondition)
}

// TestBuildUpdateInputClearsClearableFields verifies that fields defaulting to
// an empty value are always sent as a concrete value on update — even when
// empty — so clearing them actually reaches the API rather than being omitted
// (which would leave a previously-set value in place). placement is cleared
// the same way, but as an explicit JSON null rather than an empty string —
// see BuildUpdateInput.
func TestBuildUpdateInputClearsClearableFields(t *testing.T) {
	input := BuildUpdateInput("service-1", 1, minimalNestedModel())

	assert.NotNil(t, input.CompressionCodec, "compression_codec must be sent even when empty")
	assert.Equal(t, "", *input.CompressionCodec)
	assert.NotNil(t, input.AuthMethod, "auth_method must be sent even when empty")
	assert.Equal(t, "", *input.AuthMethod)
	assert.NotNil(t, input.User, "user must be sent even when empty")
	assert.Equal(t, "", *input.User)
	assert.NotNil(t, input.Password, "password must be sent even when empty")
	assert.Equal(t, "", *input.Password)
	assert.NotNil(t, input.TLSCACert, "tls.ca_cert must be sent even when empty")
	assert.Equal(t, "", *input.TLSCACert)
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
	assert.Equal(t, "test-kafka", input.Name)
	assert.Equal(t, "test-kafka", *input.NewName)
	assert.Equal(t, "kafka-user", *input.User)
	assert.Equal(t, "eu", *input.ProcessingRegion)
	assert.Nil(t, input.Format)
	assert.Nil(t, input.FormatVersion)
	assert.Nil(t, input.Placement)
	assert.Nil(t, input.ResponseCondition)
}

func TestClearVCLOnlyCreateFields(t *testing.T) {
	input := &fastly.CreateKafkaInput{
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
	input := &fastly.UpdateKafkaInput{
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
// server-side values — a different default format, and placement forced to
// "none". Adopting those breaks consistency-after-apply, so they must be reset
// to exactly the values a plan produces.
func TestResetVCLOnlyToDefaults(t *testing.T) {
	// What the API actually reports back for a Compute service.
	m := FlattenToNestedModel(&fastly.Kafka{
		Name:              new("test-kafka"),
		Brokers:           new("127.0.0.1:9092"),
		Topic:             new("test-topic"),
		ProcessingRegion:  new("none"),
		Format:            new("{\n  \"time\": 1\n}\n"),
		FormatVersion:     new(1),
		Placement:         new("none"),
		ResponseCondition: new("some-condition"),
	})

	ResetVCLOnlyToDefaults(&m)

	assert.Equal(t, constants.LoggingKafkaDefaultFormat, m.Format.ValueString())
	assert.Equal(t, int64(DefaultFormatVersion), m.FormatVersion.ValueInt64())
	assert.True(t, m.Placement.IsNull(), "placement must go back to unset, not the API's forced \"none\"")
	assert.Equal(t, DefaultResponseCondition, m.ResponseCondition.ValueString())

	// Non-VCL-only fields must survive untouched.
	assert.Equal(t, "test-kafka", m.Name.ValueString())
	assert.Equal(t, "127.0.0.1:9092", m.Brokers.ValueString())
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
			name: "different brokers",
			a: func() NestedModel {
				m := minimalNestedModel()
				m.Brokers = types.StringValue("one.example.com:9092")
				return m
			}(),
			b: func() NestedModel {
				m := minimalNestedModel()
				m.Brokers = types.StringValue("two.example.com:9092")
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

	b.ProcessingRegion = types.StringValue("us")
	assert.False(t, a.ModelsEqual(b))
}

// TestComputeModelsEqualIgnoresVCLOnlyFields verifies that a Compute endpoint
// whose remote state carries VCL-only fields still compares equal to the desired
// Compute model — otherwise ComputeReconcile would issue a pointless update on
// every apply.
func TestComputeModelsEqualIgnoresVCLOnlyFields(t *testing.T) {
	desired := fullComputeNestedModel()

	remote := &fastly.Kafka{
		Name:             new("test-kafka"),
		Brokers:          new("127.0.0.1:9092,127.0.0.2:9092"),
		Topic:            new("test-topic"),
		CompressionCodec: new("snappy"),
		RequiredACKs:     new("-1"),
		RequestMaxBytes:  new(12345),
		ParseLogKeyvals:  new(true),
		AuthMethod:       new("scram-sha-512"),
		User:             new("kafka-user"),
		Password:         new("kafka-password"),
		TLSCACert:        new("ca-cert"),
		TLSClientCert:    new("client-cert"),
		TLSClientKey:     new("client-key"),
		TLSHostname:      new("kafka.example.com"),
		UseTLS:           new(true),
		ProcessingRegion: new("eu"),
		Format:           new("something-else-entirely"),
		FormatVersion:    new(1),
		Placement:        new("none"),
		ResponseCondition: new(
			"some-condition"),
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
					m.ProcessingRegion = types.StringValue("us")
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
// `logging_kafka { format = ... }` inside fastly_service_compute_auto fail at
// plan time with Terraform's own "Unsupported argument" error.
func TestComputeAttributesOmitsVCLOnly(t *testing.T) {
	compute := ComputeAttributes()
	common := CommonAttributes()

	for _, name := range []string{"format", "format_version", "placement", "response_condition"} {
		assert.NotContains(t, compute, name)
		assert.Contains(t, common, name)
	}

	for _, name := range []string{"name", "brokers", "topic", "compression_codec", "required_acks", "request_max_bytes", "parse_log_keyvals", "auth_method", "authentication", "tls", "use_tls", "processing_region"} {
		assert.Contains(t, compute, name)
		assert.Contains(t, common, name)
	}

	// user/password and the tls.* fields are nested, never top-level attributes.
	for _, name := range []string{"user", "password", "ca_cert", "client_cert", "client_key", "hostname"} {
		assert.NotContains(t, compute, name)
		assert.NotContains(t, common, name)
	}
}

// TestAuthenticationAttribute locks in the credential shape: an Optional+
// Computed `authentication` object with `user` and a Sensitive `password`
// inside, both defaulting to "" — the live (SDKv2) provider has no
// environment variable for either SASL credential.
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
	assert.False(t, user.Sensitive, "the SASL user is not credential material")

	password, ok := auth.Attributes["password"].(schema.StringAttribute)
	require.True(t, ok, "authentication.password must be a StringAttribute")
	assert.True(t, password.Optional)
	assert.True(t, password.Computed)
	assert.True(t, password.Sensitive, "the SASL password must never be rendered in plan output")

	assert.Len(t, auth.Attributes, 2, "authentication holds user and password for Kafka")
}

// TestTLSAttribute locks in the tls object shape: Optional+Computed, plainly
// defaulted to "" (Kafka has no environment variables for TLS, unlike
// Splunk/Syslog), with client_key marked Sensitive since it is credential
// material used to authenticate to the Kafka broker via mutual TLS.
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

// TestCredentialAccessors covers the object-unwrapping accessors, including
// the degenerate states the framework can hand us (null/unknown object,
// absent attribute), where an empty string is the safe answer rather than a
// panic.
func TestCredentialAccessors(t *testing.T) {
	assert.Equal(t, "kafka-user", fullNestedModel().User().ValueString())
	assert.Equal(t, "kafka-password", fullNestedModel().Password().ValueString())

	tests := map[string]types.Object{
		"null object":      types.ObjectNull(authenticationAttributeTypes),
		"unknown object":   types.ObjectUnknown(authenticationAttributeTypes),
		"null user":        NewAuthenticationObject(types.StringNull(), types.StringValue("")),
		"unknown password": NewAuthenticationObject(types.StringValue(""), types.StringUnknown()),
	}
	for name, obj := range tests {
		t.Run(name, func(t *testing.T) {
			m := commonModel{Authentication: obj}
			assert.Equal(t, "", m.User().ValueString())
			assert.Equal(t, "", m.Password().ValueString())
		})
	}
}

// TestTLSAccessors mirrors TestCredentialAccessors for the tls object's fields.
func TestTLSAccessors(t *testing.T) {
	full := fullNestedModel()
	assert.Equal(t, "ca-cert", full.TLSCACert().ValueString())
	assert.Equal(t, "client-cert", full.TLSClientCert().ValueString())
	assert.Equal(t, "client-key", full.TLSClientKey().ValueString())
	assert.Equal(t, "kafka.example.com", full.TLSHostname().ValueString())

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

// TestSchemaValidators pins the accepted values for the validators, so a
// change to an enum member or a bound is a test failure rather than a
// surprise at plan time: compression_codec gzip/snappy/lz4, required_acks
// 1/0/-1, auth_method plain/scram-sha-256/scram-sha-512, processing_region
// none/us/eu, format_version 1-2, placement "none" only, format capped at
// 12288.
func TestSchemaValidators(t *testing.T) {
	attrs := CommonAttributes()

	stringCases := []struct {
		name  string
		attr  string
		value string
		valid bool
	}{
		{"compression_codec gzip", "compression_codec", "gzip", true},
		{"compression_codec snappy", "compression_codec", "snappy", true},
		{"compression_codec lz4", "compression_codec", "lz4", true},
		{"compression_codec rejects unknown", "compression_codec", "zstd", false},
		{"required_acks 1", "required_acks", "1", true},
		{"required_acks 0", "required_acks", "0", true},
		{"required_acks -1", "required_acks", "-1", true},
		{"required_acks rejects unknown", "required_acks", "2", false},
		{"auth_method plain", "auth_method", "plain", true},
		{"auth_method scram-sha-256", "auth_method", "scram-sha-256", true},
		{"auth_method scram-sha-512", "auth_method", "scram-sha-512", true},
		{"auth_method rejects unknown", "auth_method", "scram-sha-1", false},
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

	// name, brokers, topic, request_max_bytes, and response_condition accept any
	// value; assert that rather than leaving it implicit.
	assert.Empty(t, attrs["name"].(schema.StringAttribute).Validators)
	assert.Empty(t, attrs["brokers"].(schema.StringAttribute).Validators)
	assert.Empty(t, attrs["topic"].(schema.StringAttribute).Validators)
	assert.Empty(t, attrs["request_max_bytes"].(schema.Int64Attribute).Validators)
	assert.Empty(t, attrs["response_condition"].(schema.StringAttribute).Validators)
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
			assert.Contains(t, err.Error(), `"test-kafka"`)
			assert.Contains(t, err.Error(), `"missing-condition"`)
		}
	})

	t.Run("skips unknown condition", func(t *testing.T) {
		item := minimalNestedModel()
		item.ResponseCondition = types.StringUnknown()

		assert.NoError(t, ValidateConditionReferences([]NestedModel{item}, nil))
	})
}
