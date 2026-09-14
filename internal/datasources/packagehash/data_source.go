package packagehash

import (
	"context"
	"encoding/base64"
	"fmt"
	"os"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct{}

type DataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Content  types.String `tfsdk:"content"`
	Filename types.String `tfsdk:"filename"`
	Hash     types.String `tfsdk:"hash"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_package_hash"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to generate a SHA512 hash of all files within a Wasm deployment package, for use with `fastly_service_compute` or `fastly_service_compute_auto`.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Terraform data source identifier. Mirrors hash.",
			},
			"content": schema.StringAttribute{
				Optional:    true,
				Description: "The contents of the Wasm deployment package as a base64 encoded string (e.g. could be provided using an input variable or via external data source output variable). Conflicts with `filename`. Exactly one of these two arguments must be specified",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("filename")),
				},
			},
			"filename": schema.StringAttribute{
				Optional:    true,
				Description: "The path to the Wasm deployment package within your local filesystem. Conflicts with `content`. Exactly one of these two arguments must be specified",
				Validators: []validator.String{
					stringvalidator.ExactlyOneOf(path.MatchRoot("content")),
				},
			},
			"hash": schema.StringAttribute{
				Computed:    true,
				Description: "A SHA512 hash of all files (in sorted order) within the package.",
			},
		},
	}
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var state DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	tflog.Debug(ctx, "Generating Fastly package hash")

	pkgPath := state.Filename.ValueString()
	if pkgPath == "" {
		tmp, err := writeTempPackage(state.Content.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Error writing package content to disk", err.Error())
			return
		}
		defer os.Remove(tmp)
		pkgPath = tmp
	}

	hash, err := hashPackage(pkgPath)
	if err != nil {
		resp.Diagnostics.AddError("Error generating package hash", err.Error())
		return
	}

	state.ID = types.StringValue(hash)
	state.Hash = types.StringValue(hash)

	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// writeTempPackage decodes base64-encoded content to a temporary file and returns its path.
func writeTempPackage(content string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(content)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64 content: %w", err)
	}

	f, err := os.CreateTemp("", "fastly-package-*.tar.gz")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary package file: %w", err)
	}
	defer f.Close()

	if _, err := f.Write(data); err != nil {
		return "", fmt.Errorf("failed to write package content to disk: %w", err)
	}

	return f.Name(), nil
}
