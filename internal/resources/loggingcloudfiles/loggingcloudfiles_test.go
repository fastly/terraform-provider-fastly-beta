package loggingcloudfiles

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	fwdefaults "github.com/hashicorp/terraform-plugin-framework/resource/schema/defaults"
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
		Format:            types.StringValue(constants.LoggingCloudfilesDefaultFormat),
		FormatVersion:     types.Int64Value(DefaultFormatVersion),
		Placement:         types.StringNull(),
		ResponseCondition: types.StringValue(DefaultResponseCondition),
	}
}

func defaultCommonModel() commonModel {
	return commonModel{
		Name:             types.StringValue(""),
		BucketName:       types.StringValue(""),
		Authentication:   NewAuthenticationObject(types.StringValue(""), types.StringValue("")),
		Path:             types.StringValue(DefaultPath),
		Period:           types.Int64Value(DefaultPeriod),
		GzipLevel:        types.Int64Value(DefaultGzipLevel),
		CompressionCodec: types.StringValue(DefaultCompressionCodec),
		MessageType:      types.StringValue(DefaultMessageType),
		TimestampFormat:  types.StringValue(DefaultTimestampFormat),
		PublicKey:        types.StringValue(DefaultPublicKey),
		Region:           types.StringValue(DefaultRegion),
		ProcessingRegion: types.StringValue(DefaultProcessingRegion),
	}
}

func fullNestedModel() NestedModel {
	m := defaultNestedModel()
	m.Name = types.StringValue("test-cloudfiles")
	m.BucketName = types.StringValue("test-bucket")
	m.Authentication = NewAuthenticationObject(
		types.StringValue("test-user"),
		types.StringValue("test-access-key"),
	)
	m.Path = types.StringValue("/logs/")
	m.Period = types.Int64Value(1800)
	m.GzipLevel = types.Int64Value(6)
	m.CompressionCodec = types.StringValue("")
	m.MessageType = types.StringValue("classic")
	m.TimestampFormat = types.StringValue("%Y")
	m.PublicKey = types.StringValue("pgp-public-key")
	m.Region = types.StringValue("LON")
	m.ProcessingRegion = types.StringValue("us")
	m.Format = types.StringValue("%h %l %u")
	m.FormatVersion = types.Int64Value(1)
	m.Placement = types.StringValue("none")
	m.ResponseCondition = types.StringValue("response-condition-1")
	return m
}

func minimalNestedModel() NestedModel {
	m := defaultNestedModel()
	m.Name = types.StringValue("test-cloudfiles")
	m.BucketName = types.StringValue("test-bucket")
	m.Authentication = NewAuthenticationObject(
		types.StringValue("test-user"),
		types.StringValue("test-access-key"),
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
		api      *fastly.Cloudfiles
		expected NestedModel
	}{
		{
			name:     "nil returns empty model",
			api:      nil,
			expected: NestedModel{},
		},
		{
			name: "only required fields uses defaults",
			api: &fastly.Cloudfiles{
				Name:       new("test-cloudfiles"),
				BucketName: new("test-bucket"),
				User:       new("test-user"),
				AccessKey:  new("test-access-key"),
			},
			expected: minimalNestedModel(),
		},
		{
			name: "all fields populated",
			api: &fastly.Cloudfiles{
				Name:              new("test-cloudfiles"),
				BucketName:        new("test-bucket"),
				User:              new("test-user"),
				AccessKey:         new("test-access-key"),
				Path:              new("/logs/"),
				Period:            new(1800),
				GzipLevel:         new(6),
				CompressionCodec:  new(""),
				MessageType:       new("classic"),
				TimestampFormat:   new("%Y"),
				PublicKey:         new("pgp-public-key"),
				Region:            new("LON"),
				ProcessingRegion:  new("us"),
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
	api := &fastly.Cloudfiles{
		Name:             new("test-cloudfiles"),
		BucketName:       new("test-bucket"),
		User:             new("test-user"),
		AccessKey:        new("test-access-key"),
		Path:             new("/logs/"),
		Period:           new(1800),
		GzipLevel:        new(6),
		CompressionCodec: new(""),
		MessageType:      new("classic"),
		TimestampFormat:  new("%Y"),
		PublicKey:        new("pgp-public-key"),
		Region:           new("LON"),
		ProcessingRegion: new("us"),
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
		api      *fastly.Cloudfiles
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
			api: &fastly.Cloudfiles{
				ServiceID:      new("service-123"),
				ServiceVersion: new(5),
				Name:           new("test-cloudfiles"),
				BucketName:     new("test-bucket"),
				User:           new("test-user"),
				AccessKey:      new("test-access-key"),
			},
			validate: func(t *testing.T, m *Model) {
				assert.Equal(t, types.StringValue("service-123-5-test-cloudfiles"), m.ID)
				assert.Equal(t, types.StringValue("service-123"), m.Service)
				assert.Equal(t, types.Int64Value(5), m.Version)
				assert.Equal(t, types.StringValue("test-cloudfiles"), m.Name)
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

func TestPreserveGzipSentinel(t *testing.T) {
	tests := []struct {
		name     string
		remote   NestedModel
		desired  NestedModel
		expected types.Int64
	}{
		{
			name: "desired unset restores sentinel over API auto-managed value",
			remote: func() NestedModel {
				m := minimalNestedModel()
				m.GzipLevel = types.Int64Value(3)
				return m
			}(),
			desired:  minimalNestedModel(),
			expected: types.Int64Value(DefaultGzipLevel),
		},
		{
			name: "desired set keeps the API value",
			remote: func() NestedModel {
				m := minimalNestedModel()
				m.GzipLevel = types.Int64Value(6)
				return m
			}(),
			desired: func() NestedModel {
				m := minimalNestedModel()
				m.GzipLevel = types.Int64Value(6)
				return m
			}(),
			expected: types.Int64Value(6),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.remote
			preserveGzipSentinel(&m, tt.desired)
			assert.Equal(t, tt.expected, m.GzipLevel)
		})
	}
}

func TestInferGzipSentinelOnImport(t *testing.T) {
	tests := []struct {
		name     string
		m        commonModel
		expected types.Int64
	}{
		{
			name: "no codec, gzip_level 0 treated as unconfigured",
			m: commonModel{
				CompressionCodec: types.StringValue(""),
				GzipLevel:        types.Int64Value(0),
			},
			expected: types.Int64Value(DefaultGzipLevel),
		},
		{
			name: "codec set, gzip_level 0 is left alone",
			m: commonModel{
				CompressionCodec: types.StringValue("zstd"),
				GzipLevel:        types.Int64Value(0),
			},
			expected: types.Int64Value(0),
		},
		{
			name: "non-zero gzip_level is left alone",
			m: commonModel{
				CompressionCodec: types.StringValue(""),
				GzipLevel:        types.Int64Value(5),
			},
			expected: types.Int64Value(5),
		},
		{
			name: "already-unset sentinel is left alone",
			m: commonModel{
				CompressionCodec: types.StringValue(""),
				GzipLevel:        types.Int64Value(DefaultGzipLevel),
			},
			expected: types.Int64Value(DefaultGzipLevel),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.m
			inferGzipSentinelOnImport(&m)
			assert.Equal(t, tt.expected, m.GzipLevel)
		})
	}
}

func TestPreserveGzipSentinelList(t *testing.T) {
	read := []NestedModel{
		func() NestedModel {
			m := minimalNestedModel()
			m.Name = types.StringValue("a")
			m.GzipLevel = types.Int64Value(3)
			return m
		}(),
		func() NestedModel {
			m := minimalNestedModel()
			m.Name = types.StringValue("b")
			m.GzipLevel = types.Int64Value(6)
			return m
		}(),
		func() NestedModel {
			m := minimalNestedModel()
			m.Name = types.StringValue("c")
			m.GzipLevel = types.Int64Value(0)
			return m
		}(),
	}
	desired := []NestedModel{
		func() NestedModel {
			m := minimalNestedModel()
			m.Name = types.StringValue("a")
			return m
		}(),
		func() NestedModel {
			m := minimalNestedModel()
			m.Name = types.StringValue("b")
			m.GzipLevel = types.Int64Value(6)
			return m
		}(),
		// "c" has no entry in desired, simulating a freshly imported or
		// undiscovered-in-config endpoint.
	}

	preserveGzipSentinelList(read, desired)

	assert.Equal(t, types.Int64Value(DefaultGzipLevel), read[2].GzipLevel, "unmatched entry falls back to the import heuristic")

	assert.Equal(t, types.Int64Value(DefaultGzipLevel), read[0].GzipLevel, "unmatched-by-desired sentinel should be restored")
	assert.Equal(t, types.Int64Value(6), read[1].GzipLevel, "explicitly configured value should be preserved")
}

// Tests for expand.go

func TestBuildCreateInput(t *testing.T) {
	tests := []struct {
		name      string
		serviceID string
		version   int
		model     NestedModel
		validate  func(t *testing.T, input *fastly.CreateCloudfilesInput)
	}{
		{
			name:      "minimal model",
			serviceID: "service-123",
			version:   5,
			model:     minimalNestedModel(),
			validate: func(t *testing.T, input *fastly.CreateCloudfilesInput) {
				assert.Equal(t, "service-123", input.ServiceID)
				assert.Equal(t, 5, input.ServiceVersion)
				assert.Equal(t, "test-cloudfiles", *input.Name)
				assert.Equal(t, "test-bucket", *input.BucketName)
				assert.Equal(t, "test-user", *input.User)
				assert.Equal(t, "test-access-key", *input.AccessKey)
				assert.Equal(t, "none", *input.ProcessingRegion)
				assert.Equal(t, constants.LoggingCloudfilesDefaultFormat, *input.Format)
				assert.Nil(t, input.GzipLevel, "unset gzip_level must not be sent")
				assert.Nil(t, input.Placement, "unset placement must not be sent as \"none\" — the API treats them differently")
			},
		},
		{
			name:      "fully populated model",
			serviceID: "service-456",
			version:   10,
			model:     fullNestedModel(),
			validate: func(t *testing.T, input *fastly.CreateCloudfilesInput) {
				assert.Equal(t, "test-cloudfiles", *input.Name)
				assert.Equal(t, "test-bucket", *input.BucketName)
				assert.Equal(t, "test-user", *input.User)
				assert.Equal(t, "test-access-key", *input.AccessKey)
				assert.Equal(t, "/logs/", *input.Path)
				assert.Equal(t, 1800, *input.Period)
				assert.Equal(t, 6, *input.GzipLevel)
				assert.Equal(t, "classic", *input.MessageType)
				assert.Equal(t, "%Y", *input.TimestampFormat)
				assert.Equal(t, "pgp-public-key", *input.PublicKey)
				assert.Equal(t, "LON", *input.Region)
				assert.Equal(t, "us", *input.ProcessingRegion)
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
	assert.Equal(t, "test-cloudfiles", *input.Name)
	assert.Equal(t, "test-bucket", *input.BucketName)
	assert.Equal(t, "test-user", *input.User)
	assert.Equal(t, "test-access-key", *input.AccessKey)
	assert.Equal(t, "LON", *input.Region)
	assert.Equal(t, "us", *input.ProcessingRegion)
	assert.Nil(t, input.Format, "VCL-only fields must never be set for Compute")
	assert.Nil(t, input.FormatVersion)
	assert.Nil(t, input.Placement)
	assert.Nil(t, input.ResponseCondition)
}

func TestBuildUpdateInput(t *testing.T) {
	input := BuildUpdateInput("service-456", 10, fullNestedModel())

	assert.Equal(t, "service-456", input.ServiceID)
	assert.Equal(t, 10, input.ServiceVersion)
	assert.Equal(t, "test-cloudfiles", input.Name)
	assert.Equal(t, "test-cloudfiles", *input.NewName)
	assert.Equal(t, "test-bucket", *input.BucketName)
	assert.Equal(t, "test-user", *input.User)
	assert.Equal(t, "test-access-key", *input.AccessKey)
	assert.Equal(t, "/logs/", *input.Path)
	assert.Equal(t, 1800, *input.Period)
	assert.Equal(t, 6, *input.GzipLevel)
	assert.Equal(t, "pgp-public-key", *input.PublicKey)
	assert.Equal(t, "LON", *input.Region)
	assert.Equal(t, "us", *input.ProcessingRegion)
	assert.Equal(t, "%h %l %u", *input.Format)
	assert.Equal(t, 1, *input.FormatVersion)
	assert.Equal(t, fastly.NewNullable("none"), input.Placement)
	assert.Equal(t, "response-condition-1", *input.ResponseCondition)
}

// TestBuildUpdateInputClearsClearableFields verifies that response_condition,
// public_key, and region are always sent as concrete values on update — even
// when empty — so clearing them actually reaches the API rather than being
// omitted (which would leave a previously-set value in place). placement is
// cleared the same way, but as an explicit JSON null rather than an empty
// string — see BuildUpdateInput.
func TestBuildUpdateInputClearsClearableFields(t *testing.T) {
	input := BuildUpdateInput("service-1", 1, minimalNestedModel())

	assert.NotNil(t, input.ResponseCondition, "response_condition must be sent even when empty")
	assert.Equal(t, "", *input.ResponseCondition)
	assert.NotNil(t, input.PublicKey, "public_key must be sent even when empty")
	assert.Equal(t, "", *input.PublicKey)
	assert.NotNil(t, input.Region, "region must be sent even when empty")
	assert.Equal(t, "", *input.Region)
	assert.NotNil(t, input.Placement, "unset placement must be sent as an explicit null, not omitted (omitting leaves a previously-set \"none\" in place)")
	assert.Equal(t, fastly.NullValue[string](), input.Placement)
}

func TestBuildComputeUpdateInput(t *testing.T) {
	input := BuildComputeUpdateInput("service-456", 10, fullComputeNestedModel())

	assert.Equal(t, "service-456", input.ServiceID)
	assert.Equal(t, 10, input.ServiceVersion)
	assert.Equal(t, "test-cloudfiles", input.Name)
	assert.Equal(t, "test-cloudfiles", *input.NewName)
	assert.Equal(t, "LON", *input.Region)
	assert.Equal(t, "us", *input.ProcessingRegion)
	assert.Nil(t, input.Format)
	assert.Nil(t, input.FormatVersion)
	assert.Nil(t, input.Placement)
	assert.Nil(t, input.ResponseCondition)
}

func TestClearVCLOnlyCreateFields(t *testing.T) {
	input := &fastly.CreateCloudfilesInput{
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
	input := &fastly.UpdateCloudfilesInput{
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
	m := FlattenToNestedModel(&fastly.Cloudfiles{
		Name:              new("test-cloudfiles"),
		BucketName:        new("test-bucket"),
		User:              new("test-user"),
		AccessKey:         new("test-access-key"),
		Region:            new("LON"),
		ProcessingRegion:  new("none"),
		Format:            new("{\n  \"some\": \"format\"\n}\n"),
		FormatVersion:     new(1),
		Placement:         new("none"),
		ResponseCondition: new("some-condition"),
	})

	ResetVCLOnlyToDefaults(&m)

	assert.Equal(t, constants.LoggingCloudfilesDefaultFormat, m.Format.ValueString())
	assert.Equal(t, int64(DefaultFormatVersion), m.FormatVersion.ValueInt64())
	assert.True(t, m.Placement.IsNull(), "placement must go back to unset, not the API's forced \"none\"")
	assert.Equal(t, DefaultResponseCondition, m.ResponseCondition.ValueString())

	// Non-VCL-only fields must survive untouched.
	assert.Equal(t, "test-cloudfiles", m.Name.ValueString())
	assert.Equal(t, "test-user", m.User().ValueString())
	assert.Equal(t, "test-access-key", m.AccessKey().ValueString())
	assert.Equal(t, "LON", m.Region.ValueString())
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
	var fResp fwdefaults.StringResponse
	format.Default.DefaultString(context.Background(), fwdefaults.StringRequest{}, &fResp)
	assert.Equal(t, fResp.PlanValue, m.Format, "format must match its schema default")

	formatVersion := attrs["format_version"].(schema.Int64Attribute)
	var fvResp fwdefaults.Int64Response
	formatVersion.Default.DefaultInt64(context.Background(), fwdefaults.Int64Request{}, &fvResp)
	assert.Equal(t, fvResp.PlanValue, m.FormatVersion, "format_version must match its schema default")

	responseCondition := attrs["response_condition"].(schema.StringAttribute)
	var rcResp fwdefaults.StringResponse
	responseCondition.Default.DefaultString(context.Background(), fwdefaults.StringRequest{}, &rcResp)
	assert.Equal(t, rcResp.PlanValue, m.ResponseCondition, "response_condition must match its schema default")

	// placement is Optional-only with no Default, so an absent config value plans
	// as null — the reset has to produce null, not "".
	assert.Nil(t, attrs["placement"].(schema.StringAttribute).Default)
	assert.True(t, m.Placement.IsNull())
}

// TestUserAccessKeyAccessors covers the object-unwrapping accessors, including
// the degenerate states the framework can hand us (null/unknown object, absent
// attribute), where an empty string is the safe answer rather than a panic.
func TestUserAccessKeyAccessors(t *testing.T) {
	assert.Equal(t, "test-user", minimalNestedModel().User().ValueString())
	assert.Equal(t, "test-access-key", minimalNestedModel().AccessKey().ValueString())

	tests := map[string]types.Object{
		"null object":    types.ObjectNull(authenticationAttributeTypes),
		"unknown object": types.ObjectUnknown(authenticationAttributeTypes),
		"null values":    NewAuthenticationObject(types.StringNull(), types.StringNull()),
		"unknown values": NewAuthenticationObject(types.StringUnknown(), types.StringUnknown()),
	}
	for name, obj := range tests {
		t.Run(name, func(t *testing.T) {
			m := commonModel{Authentication: obj}
			assert.Equal(t, "", m.User().ValueString())
			assert.Equal(t, "", m.AccessKey().ValueString())
		})
	}
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
			name: "different access_key",
			a: func() NestedModel {
				m := minimalNestedModel()
				m.Authentication = NewAuthenticationObject(types.StringValue("test-user"), types.StringValue("key-1"))
				return m
			}(),
			b: func() NestedModel {
				m := minimalNestedModel()
				m.Authentication = NewAuthenticationObject(types.StringValue("test-user"), types.StringValue("key-2"))
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

	b.Region = types.StringValue("SYD")
	assert.False(t, a.ModelsEqual(b))
}

// TestComputeModelsEqualIgnoresVCLOnlyFields verifies that a Compute endpoint
// whose remote state carries VCL-only fields still compares equal to the desired
// Compute model — otherwise ComputeReconcile would issue a pointless update on
// every apply.
func TestComputeModelsEqualIgnoresVCLOnlyFields(t *testing.T) {
	desired := fullComputeNestedModel()

	remote := &fastly.Cloudfiles{
		Name:              new("test-cloudfiles"),
		BucketName:        new("test-bucket"),
		User:              new("test-user"),
		AccessKey:         new("test-access-key"),
		Path:              new("/logs/"),
		Period:            new(1800),
		GzipLevel:         new(6),
		CompressionCodec:  new(""),
		MessageType:       new("classic"),
		TimestampFormat:   new("%Y"),
		PublicKey:         new("pgp-public-key"),
		Region:            new("LON"),
		ProcessingRegion:  new("us"),
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
					m.Region = types.StringValue("LON")
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

	b[0].Region = types.StringValue("SYD")
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
// `logging_cloudfiles { format = ... }` inside fastly_service_compute_auto fail
// at plan time with Terraform's own "Unsupported argument" error.
func TestComputeAttributesOmitsVCLOnly(t *testing.T) {
	compute := ComputeAttributes()
	common := CommonAttributes()

	for _, name := range []string{"format", "format_version", "placement", "response_condition"} {
		assert.NotContains(t, compute, name)
		assert.Contains(t, common, name)
	}

	for _, name := range []string{"name", "bucket_name", "authentication", "region", "processing_region"} {
		assert.Contains(t, compute, name)
		assert.Contains(t, common, name)
	}

	// user/access_key are nested under authentication, never top-level attributes.
	assert.NotContains(t, compute, "user")
	assert.NotContains(t, compute, "access_key")
	assert.NotContains(t, common, "user")
	assert.NotContains(t, common, "access_key")
}

// TestAuthenticationAttribute locks in the credential shape: a Required
// `authentication` object with Required `user` and Required+Sensitive
// `access_key` inside, matching how every other logging endpoint groups
// credentials.
func TestAuthenticationAttribute(t *testing.T) {
	auth, ok := ComputeAttributes()["authentication"].(schema.SingleNestedAttribute)
	require.True(t, ok, "authentication must be a SingleNestedAttribute")

	assert.True(t, auth.Required, "authentication is Required: there is no FASTLY_CLOUDFILES_* env var to default it from")
	assert.False(t, auth.Computed)
	assert.Nil(t, auth.Default, "a Required object must not carry a Default")

	user, ok := auth.Attributes["user"].(schema.StringAttribute)
	require.True(t, ok, "authentication.user must be a StringAttribute")
	assert.True(t, user.Required)
	assert.False(t, user.Sensitive)

	accessKey, ok := auth.Attributes["access_key"].(schema.StringAttribute)
	require.True(t, ok, "authentication.access_key must be a StringAttribute")
	assert.True(t, accessKey.Required)
	assert.True(t, accessKey.Sensitive, "the Cloud Files access key must never be rendered in plan output")

	assert.Len(t, auth.Attributes, 2, "authentication holds user and access_key for Cloud Files")
}

// TestSchemaValidators pins the accepted values for the enum-based validators,
// so a change to an enum member or a bound is a test failure rather than a
// surprise at plan time.
func TestSchemaValidators(t *testing.T) {
	attrs := CommonAttributes()

	stringCases := []struct {
		name  string
		attr  string
		value string
		valid bool
	}{
		{"compression_codec zstd", "compression_codec", "zstd", true},
		{"compression_codec snappy", "compression_codec", "snappy", true},
		{"compression_codec gzip", "compression_codec", "gzip", true},
		{"compression_codec rejects unknown", "compression_codec", "lz4", false},
		{"message_type classic", "message_type", "classic", true},
		{"message_type loggly", "message_type", "loggly", true},
		{"message_type logplex", "message_type", "logplex", true},
		{"message_type blank", "message_type", "blank", true},
		{"message_type rejects unknown", "message_type", "verbose", false},
		{"processing_region none", "processing_region", "none", true},
		{"processing_region us", "processing_region", "us", true},
		{"processing_region eu", "processing_region", "eu", true},
		{"processing_region rejects wrong case", "processing_region", "US", false},
		{"region DFW", "region", "DFW", true},
		{"region ORD", "region", "ORD", true},
		{"region IAD", "region", "IAD", true},
		{"region LON", "region", "LON", true},
		{"region SYD", "region", "SYD", true},
		{"region HKG", "region", "HKG", true},
		{"region rejects unknown", "region", "FRA", false},
		{"region rejects wrong case", "region", "lon", false},
		{"placement none", "placement", "none", true},
		{"placement rejects other VCL subroutines", "placement", "waf_debug", false},
		{"placement rejects empty", "placement", "", false},
	}
	for _, tt := range stringCases {
		t.Run(tt.name, func(t *testing.T) {
			a := attrs[tt.attr].(schema.StringAttribute)
			require.Len(t, a.Validators, 1)
			resp := &validator.StringResponse{}
			a.Validators[0].ValidateString(context.Background(),
				validator.StringRequest{ConfigValue: types.StringValue(tt.value)}, resp)
			assert.Equal(t, tt.valid, !resp.Diagnostics.HasError())
		})
	}

	formatVersionCases := []struct {
		value int64
		valid bool
	}{{0, false}, {1, true}, {2, true}, {3, false}}
	for _, tt := range formatVersionCases {
		t.Run("format_version", func(t *testing.T) {
			a := attrs["format_version"].(schema.Int64Attribute)
			resp := &validator.Int64Response{}
			for _, v := range a.Validators {
				v.ValidateInt64(context.Background(), validator.Int64Request{ConfigValue: types.Int64Value(tt.value)}, resp)
			}
			assert.Equal(t, tt.valid, !resp.Diagnostics.HasError())
		})
	}

	gzipLevelCases := []struct {
		value int64
		valid bool
	}{{-1, false}, {0, true}, {9, true}, {10, false}}
	for _, tt := range gzipLevelCases {
		t.Run("gzip_level range", func(t *testing.T) {
			a := attrs["gzip_level"].(schema.Int64Attribute)
			resp := &validator.Int64Response{}
			for _, v := range a.Validators {
				// gzipLevelCodecConflict is covered separately in validators_test.go;
				// it looks up a sibling attribute this bare request doesn't provide.
				if _, ok := v.(gzipLevelCodecConflict); ok {
					continue
				}
				v.ValidateInt64(context.Background(), validator.Int64Request{ConfigValue: types.Int64Value(tt.value)}, resp)
			}
			assert.Equal(t, tt.valid, !resp.Diagnostics.HasError())
		})
	}

	// name and response_condition accept any string; assert that rather than
	// leaving it implicit.
	assert.Empty(t, attrs["name"].(schema.StringAttribute).Validators)
	assert.Empty(t, attrs["response_condition"].(schema.StringAttribute).Validators)
	assert.Empty(t, attrs["bucket_name"].(schema.StringAttribute).Validators)
	auth := attrs["authentication"].(schema.SingleNestedAttribute)
	assert.Empty(t, auth.Attributes["user"].(schema.StringAttribute).Validators)
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
			assert.Contains(t, err.Error(), `"test-cloudfiles"`)
			assert.Contains(t, err.Error(), `"missing-condition"`)
		}
	})

	t.Run("skips unknown condition", func(t *testing.T) {
		item := minimalNestedModel()
		item.ResponseCondition = types.StringUnknown()

		assert.NoError(t, ValidateConditionReferences([]NestedModel{item}, nil))
	})
}
