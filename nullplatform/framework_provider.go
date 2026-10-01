package nullplatform

import (
	"context"
	"net/http"
	"os"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ provider.Provider                       = (*frameworkProvider)(nil)
	_ provider.ProviderWithFunctions          = (*frameworkProvider)(nil)
	_ provider.ProviderWithEphemeralResources = (*frameworkProvider)(nil)
)

// frameworkProvider is a terraform-plugin-framework provider that serves what
// SDKv2 cannot express: provider-defined functions and ephemeral resources. It
// is muxed with the terraform-plugin-sdk/v2 provider in main.go, where
// resources and data sources stay.
type frameworkProvider struct {
	httpClient *http.Client
}

type frameworkProviderModel struct {
	ApiKey    types.String `tfsdk:"api_key"`
	Host      types.String `tfsdk:"host"`
	NpApiKey  types.String `tfsdk:"np_apikey"`
	NpApiHost types.String `tfsdk:"np_api_host"`
}

func NewFrameworkProvider() provider.Provider {
	return &frameworkProvider{httpClient: newAPIHTTPClient()}
}

func (p *frameworkProvider) Metadata(_ context.Context, _ provider.MetadataRequest, resp *provider.MetadataResponse) {
	resp.TypeName = "nullplatform"
}

// Schema mirrors the SDKv2 provider schema attribute by attribute (types,
// optional/sensitive flags, descriptions, and deprecations). The mux server
// requires every muxed provider to expose an identical provider configuration
// schema; TestMuxServer_ProviderSchemasAreCompatible guards this.
func (p *frameworkProvider) Schema(_ context.Context, _ provider.SchemaRequest, resp *provider.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			API_KEY: schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Nullplatform API KEY. Can also be set with the `NULLPLATFORM_API_KEY` environment variable.",
			},
			HOST: schema.StringAttribute{
				Optional:    true,
				Description: "Nullplatform HOST. Can also be set with the `NULLPLATFORM_HOST` environment variable. If omitted, the default value is `api.nullplatform.com`",
			},
			NP_API_KEY: schema.StringAttribute{
				Optional:           true,
				Sensitive:          true,
				Description:        "Nullplatform API KEY. Can also be set with the `NP_API_KEY` environment variable.",
				DeprecationMessage: "The 'np_apikey' attribute is deprecated and will be removed in a future version. Please use 'api_key' instead.",
			},
			NP_API_HOST: schema.StringAttribute{
				Optional:           true,
				Description:        "Nullplatform API HOSTNAME. Can also be set with the `NP_API_HOST` environment variable. If omitted, the default value is `api.nullplatform.com`",
				DeprecationMessage: "The 'np_api_host' attribute is deprecated and will be removed in a future version. Please use 'host' instead.",
			},
		},
	}
}

// Configure builds the client ephemeral resources use. It reports nothing: the
// SDKv2 provider already emits every credential warning and error for the same
// configuration, and a missing API key only matters once an ephemeral opens.
func (p *frameworkProvider) Configure(ctx context.Context, req provider.ConfigureRequest, resp *provider.ConfigureResponse) {
	var config frameworkProviderModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}

	apiKey, _ := resolveAPIKey(stringOrEnv(config.ApiKey, "NULLPLATFORM_API_KEY"), config.NpApiKey.ValueString())
	host, _ := resolveAPIHost(stringOrEnv(config.Host, "NULLPLATFORM_HOST"), config.NpApiHost.ValueString())

	resp.EphemeralResourceData = &NullClient{Client: p.httpClient, ApiURL: host, ApiKey: apiKey}
}

// stringOrEnv mirrors the SDKv2 EnvDefaultFunc on api_key and host: the
// variable only fills an attribute left null, never one set to "".
func stringOrEnv(v types.String, env string) string {
	if v.IsNull() {
		return os.Getenv(env)
	}
	return v.ValueString()
}

func (p *frameworkProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil
}

func (p *frameworkProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
}

func (p *frameworkProvider) EphemeralResources(_ context.Context) []func() ephemeral.EphemeralResource {
	return []func() ephemeral.EphemeralResource{
		NewAccessTokenEphemeralResource,
	}
}

func (p *frameworkProvider) Functions(_ context.Context) []func() function.Function {
	return []func() function.Function{
		NewExtractIDFunction,
		NewExtractOrganizationIDFunction,
		NewExtractAccountIDFunction,
		NewExtractNamespaceIDFunction,
		NewExtractApplicationIDFunction,
		NewExtractScopeIDFunction,
	}
}
