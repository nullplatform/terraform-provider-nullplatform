package nullplatform

import (
	"context"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/function"
	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/provider/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource"
)

var (
	_ provider.Provider              = (*frameworkProvider)(nil)
	_ provider.ProviderWithFunctions = (*frameworkProvider)(nil)
)

// frameworkProvider is a terraform-plugin-framework provider that only serves
// provider-defined functions. It is muxed with the terraform-plugin-sdk/v2
// provider in main.go: resources and data sources stay on the SDKv2 provider,
// while functions (which SDKv2 cannot express) live here.
type frameworkProvider struct{}

func NewFrameworkProvider() provider.Provider {
	return &frameworkProvider{}
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

// Configure is a no-op: the functions served by this provider are pure NRN
// string manipulation and never call the API, so no client is needed.
func (p *frameworkProvider) Configure(_ context.Context, _ provider.ConfigureRequest, _ *provider.ConfigureResponse) {
}

func (p *frameworkProvider) Resources(_ context.Context) []func() resource.Resource {
	return nil
}

func (p *frameworkProvider) DataSources(_ context.Context) []func() datasource.DataSource {
	return nil
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
