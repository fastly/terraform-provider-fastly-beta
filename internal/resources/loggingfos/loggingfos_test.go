package loggingfos

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	fastly "github.com/fastly/go-fastly/v19/fastly"

	"github.com/fastly/terraform-provider-fastly-beta/internal/constants"
)

func testModel() NestedModel {
	return NestedModel{
		commonModel: commonModel{
			Name:             types.StringValue("fos-logger"),
			BucketName:       types.StringValue("bucket"),
			Region:           types.StringValue("us-east"),
			Path:             types.StringValue(DefaultPath),
			Period:           types.Int64Value(DefaultPeriod),
			GzipLevel:        types.Int64Value(DefaultGzipLevel),
			CompressionCodec: types.StringValue(DefaultCompressionCodec),
			MessageType:      types.StringValue(DefaultMessageType),
			TimestampFormat:  types.StringValue(DefaultTimestampFormat),
			FileMaxBytes:     types.Int64Value(DefaultFileMaxBytes),
			PublicKey:        types.StringValue(DefaultPublicKey),
		},
		Format:            types.StringValue(constants.LoggingFOSDefaultFormat),
		FormatVersion:     types.Int64Value(DefaultFormatVersion),
		Placement:         types.StringNull(),
		ResponseCondition: types.StringValue(DefaultResponseCondition),
	}
}

func TestSchemaHasNoCredentialAttributes(t *testing.T) {
	t.Parallel()

	for name, attrs := range map[string]map[string]any{
		"common":  toAny(CommonAttributes()),
		"compute": toAny(ComputeAttributes()),
	} {
		for _, key := range []string{"authentication", "access_key", "secret_key"} {
			assert.NotContains(t, attrs, key, "%s attributes must not expose FOS credentials", name)
		}
		assert.Contains(t, attrs, "region")
		assert.Contains(t, attrs, "bucket_name")
	}

	_, hasFormat := ComputeAttributes()["format"]
	assert.False(t, hasFormat, "Compute attributes must omit VCL-only format")
	_, hasFormat = CommonAttributes()["format"]
	assert.True(t, hasFormat, "VCL attributes must include format")
}

func toAny[T any](in map[string]T) map[string]any {
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func TestBuildCreateInput(t *testing.T) {
	t.Parallel()

	m := testModel()
	m.FileMaxBytes = types.Int64Value(2097152)
	m.CompressionCodec = types.StringValue("zstd")

	input := BuildCreateInput("svc", 3, m)

	assert.Equal(t, "svc", input.ServiceID)
	assert.Equal(t, 3, input.ServiceVersion)
	assert.Equal(t, "fos-logger", *input.Name)
	assert.Equal(t, "bucket", *input.BucketName)
	assert.Equal(t, "us-east", *input.Region)
	assert.Equal(t, "zstd", *input.CompressionCodec)
	require.NotNil(t, input.FileMaxBytes)
	assert.Equal(t, 2097152, *input.FileMaxBytes)
	assert.Nil(t, input.GzipLevel, "an unset gzip_level must not be sent")
	assert.Nil(t, input.Placement)
	assert.Nil(t, input.ResponseCondition, "an empty response_condition must not be sent")
}

func TestBuildCreateInputOmitsUnsetFileMaxBytes(t *testing.T) {
	t.Parallel()

	input := BuildCreateInput("svc", 1, testModel())
	assert.Nil(t, input.FileMaxBytes)
}

func TestBuildCreateInputSendsExplicitGzipLevelZero(t *testing.T) {
	t.Parallel()

	m := testModel()
	m.GzipLevel = types.Int64Value(0)

	input := BuildCreateInput("svc", 1, m)
	require.NotNil(t, input.GzipLevel)
	assert.Equal(t, 0, *input.GzipLevel)
}

func TestBuildComputeCreateInputOmitsVCLOnlyFields(t *testing.T) {
	t.Parallel()

	input := BuildComputeCreateInput("svc", 1, ComputeNestedModel{commonModel: testModel().commonModel})

	assert.Nil(t, input.Format)
	assert.Nil(t, input.FormatVersion)
	assert.Nil(t, input.Placement)
	assert.Nil(t, input.ResponseCondition)
	assert.Equal(t, "us-east", *input.Region)
}

func TestBuildUpdateInput(t *testing.T) {
	t.Parallel()

	m := testModel()
	m.Region = types.StringValue("eu-central")

	input := BuildUpdateInput("svc", 2, m)

	assert.Equal(t, "fos-logger", input.Name)
	assert.Equal(t, "eu-central", *input.Region)
	require.NotNil(t, input.FileMaxBytes)
	assert.Equal(t, 0, *input.FileMaxBytes, "update must send file_max_bytes = 0 so a limit can be removed")
	require.NotNil(t, input.Placement, "an unset placement must be sent as an explicit null")
	require.NotNil(t, input.GzipLevel, "unset level with no codec resets compression to 0")
	assert.Equal(t, 0, *input.GzipLevel)
}

func TestBuildUpdateInputGzipLevelWithCodec(t *testing.T) {
	t.Parallel()

	m := testModel()
	m.CompressionCodec = types.StringValue("zstd")
	m.GzipLevel = types.Int64Value(DefaultGzipLevel)

	input := BuildUpdateInput("svc", 2, m)

	assert.Nil(t, input.GzipLevel, "the API rejects gzip_level alongside a codec")
	assert.Equal(t, "zstd", *input.CompressionCodec)
}

func TestPreserveGzipSentinelSurfacesOutOfBandLevel(t *testing.T) {
	t.Parallel()

	unset := commonModel{GzipLevel: types.Int64Value(DefaultGzipLevel)}

	for name, tt := range map[string]struct {
		remote commonModel
		want   int64
	}{
		"level 0 stays unset":      {commonModel{GzipLevel: types.Int64Value(0), CompressionCodec: types.StringValue("")}, DefaultGzipLevel},
		"codec in effect":          {commonModel{GzipLevel: types.Int64Value(0), CompressionCodec: types.StringValue("zstd")}, DefaultGzipLevel},
		"out-of-band level drifts": {commonModel{GzipLevel: types.Int64Value(5), CompressionCodec: types.StringValue("")}, 5},
	} {
		t.Run(name, func(t *testing.T) {
			m := tt.remote
			preserveGzipSentinelCommon(&m, unset)
			assert.Equal(t, tt.want, m.GzipLevel.ValueInt64())
		})
	}
}

func TestFlattenToNestedModel(t *testing.T) {
	t.Parallel()

	api := &fastly.FOS{
		Name:              new("fos-logger"),
		BucketName:        new("bucket"),
		Region:            new("us-east"),
		Path:              new("logs/"),
		Period:            new(600),
		GzipLevel:         new(0),
		CompressionCodec:  new("zstd"),
		MessageType:       new("classic"),
		TimestampFormat:   new("%Y"),
		FileMaxBytes:      new(1048576),
		PublicKey:         new("key"),
		Format:            new("fmt"),
		FormatVersion:     new(1),
		Placement:         new("none"),
		ResponseCondition: new("cond"),
	}

	m := FlattenToNestedModel(api)

	assert.Equal(t, "fos-logger", m.Name.ValueString())
	assert.Equal(t, "us-east", m.Region.ValueString())
	assert.Equal(t, "logs/", m.Path.ValueString())
	assert.Equal(t, int64(600), m.Period.ValueInt64())
	assert.Equal(t, "zstd", m.CompressionCodec.ValueString())
	assert.Equal(t, int64(1048576), m.FileMaxBytes.ValueInt64())
	assert.Equal(t, "none", m.Placement.ValueString())
	assert.Equal(t, "cond", m.ResponseCondition.ValueString())
}

func TestFlattenToNestedModelAppliesDefaults(t *testing.T) {
	t.Parallel()

	m := FlattenToNestedModel(&fastly.FOS{Name: new("n"), BucketName: new("b"), Region: new("r")})

	assert.Equal(t, int64(DefaultPeriod), m.Period.ValueInt64())
	assert.Equal(t, DefaultMessageType, m.MessageType.ValueString())
	assert.Equal(t, constants.LoggingFOSDefaultFormat, m.Format.ValueString())
	assert.True(t, m.Placement.IsNull())
}

func TestModelsEqualDetectsRegionChange(t *testing.T) {
	t.Parallel()

	a := testModel()
	b := testModel()
	assert.True(t, a.ModelsEqual(b))

	b.Region = types.StringValue("other")
	assert.False(t, a.ModelsEqual(b))
}

func TestPreserveGzipSentinel(t *testing.T) {
	t.Parallel()

	desired := testModel()
	remote := FlattenToNestedModel(&fastly.FOS{Name: new("fos-logger"), GzipLevel: new(0)})
	preserveGzipSentinel(&remote, desired)

	assert.Equal(t, int64(DefaultGzipLevel), remote.GzipLevel.ValueInt64(),
		"an unset gzip_level must keep the sentinel instead of the API's auto-managed 0")
}
