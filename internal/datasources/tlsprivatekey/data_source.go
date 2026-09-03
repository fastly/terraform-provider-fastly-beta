package tlsprivatekey

import (
	"context"
	"fmt"
	"time"

	fastlyclient "github.com/fastly/terraform-provider-fastly-beta/internal/client"
	"github.com/fastly/terraform-provider-fastly-beta/internal/service"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-log/tflog"

	"github.com/fastly/go-fastly/v17/fastly"
)

var _ datasource.DataSource = &DataSource{}

type DataSource struct {
	client *fastly.Client
}

type DataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	Name          types.String `tfsdk:"name"`
	KeyLength     types.Int64  `tfsdk:"key_length"`
	KeyType       types.String `tfsdk:"key_type"`
	PublicKeySHA1 types.String `tfsdk:"public_key_sha1"`
	CreatedAt     types.String `tfsdk:"created_at"`
	Replace       types.Bool   `tfsdk:"replace"`
}

func NewDataSource() datasource.DataSource {
	return &DataSource{}
}

func (d *DataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_tls_private_key"
}

func (d *DataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Use this data source to get the ID of a TLS private key, or to look up its metadata by name, key type, key length, or public key SHA1. The private key material itself is never returned once set.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Fastly private key ID. Conflicts with all other filters.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(
						path.MatchRoot("name"),
						path.MatchRoot("created_at"),
						path.MatchRoot("key_length"),
						path.MatchRoot("key_type"),
						path.MatchRoot("public_key_sha1"),
					),
				},
			},
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The name of the private key.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"key_length": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "The key length used to generate the private key.",
				Validators: []validator.Int64{
					int64validator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"key_type": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The algorithm used to generate the private key. Currently, the only allowed value is `RSA`.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"public_key_sha1": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "The SHA1 digest of the private key's public key. Useful for safely identifying the key.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"created_at": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Timestamp (GMT) when the private key was created.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("id")),
				},
			},
			"replace": schema.BoolAttribute{
				Computed:    true,
				Description: "A recommendation from Fastly to replace this private key and all associated certificates.",
			},
		},
	}
}

func (d *DataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	data, diags := fastlyclient.FromProviderData(req.ProviderData)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() || data == nil {
		return
	}
	d.client = data.Client
}

func (d *DataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var config DataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var key *fastly.PrivateKey

	if id := service.StringValue(config.ID); id != "" {
		tflog.Debug(ctx, "Reading Fastly TLS private key", map[string]any{"id": id})

		found, err := d.client.GetPrivateKey(ctx, &fastly.GetPrivateKeyInput{ID: id})
		if err != nil {
			resp.Diagnostics.AddError("Error reading TLS private key", err.Error())
			return
		}
		key = found
	} else {
		tflog.Debug(ctx, "Listing Fastly TLS private keys to find a match")

		keys, err := listPrivateKeys(ctx, d.client, "")
		if err != nil {
			resp.Diagnostics.AddError("Error listing TLS private keys", err.Error())
			return
		}

		matches := filterPrivateKeys(keys, config)
		switch len(matches) {
		case 0:
			resp.Diagnostics.AddError("No matching TLS private key found", "your query returned no results. Please change your search criteria and try again")
			return
		case 1:
			key = matches[0]
		default:
			resp.Diagnostics.AddError("Multiple matching TLS private keys found", "your query returned more than one result. Please change to a more specific search criteria")
			return
		}
	}

	if key.Replace {
		resp.Diagnostics.AddWarning(
			"Fastly recommends replacing this private key",
			fmt.Sprintf("Fastly recommends that private key %q (id: %s) and all associated certificates be replaced.", key.Name, key.ID),
		)
	}

	state := flattenToDataSourceModel(key)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func listPrivateKeys(ctx context.Context, client *fastly.Client, filterInUse string) ([]*fastly.PrivateKey, error) {
	var keys []*fastly.PrivateKey
	pageNumber := 1
	for {
		list, err := client.ListPrivateKeys(ctx, &fastly.ListPrivateKeysInput{
			FilterInUse: filterInUse,
			PageNumber:  pageNumber,
			PageSize:    10,
		})
		if err != nil {
			return nil, err
		}
		if len(list) == 0 {
			break
		}
		pageNumber++
		keys = append(keys, list...)
	}
	return keys, nil
}

func filterPrivateKeys(keys []*fastly.PrivateKey, config DataSourceModel) []*fastly.PrivateKey {
	name := service.StringValue(config.Name)
	keyType := service.StringValue(config.KeyType)
	publicKeySHA1 := service.StringValue(config.PublicKeySHA1)
	keyLength := service.Int64Value(config.KeyLength)
	createdAt := service.StringValue(config.CreatedAt)

	var matches []*fastly.PrivateKey
	for _, k := range keys {
		if name != "" && k.Name != name {
			continue
		}
		if keyType != "" && k.KeyType != keyType {
			continue
		}
		if publicKeySHA1 != "" && k.PublicKeySHA1 != publicKeySHA1 {
			continue
		}
		if keyLength != 0 && int64(k.KeyLength) != keyLength {
			continue
		}
		if createdAt != "" && (k.CreatedAt == nil || k.CreatedAt.Format(time.RFC3339) != createdAt) {
			continue
		}
		matches = append(matches, k)
	}
	return matches
}

func flattenToDataSourceModel(k *fastly.PrivateKey) DataSourceModel {
	m := DataSourceModel{
		ID:            types.StringValue(k.ID),
		Name:          types.StringValue(k.Name),
		KeyLength:     types.Int64Value(int64(k.KeyLength)),
		KeyType:       types.StringValue(k.KeyType),
		PublicKeySHA1: types.StringValue(k.PublicKeySHA1),
		CreatedAt:     types.StringNull(),
		Replace:       types.BoolValue(k.Replace),
	}

	if k.CreatedAt != nil {
		m.CreatedAt = types.StringValue(k.CreatedAt.Format(time.RFC3339))
	}

	return m
}
