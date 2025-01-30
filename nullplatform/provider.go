package nullplatform

import (
	"context"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/motemen/go-loghttp"
)

const API_KEY = "api_key"
const HOST = "host"
const NP_API_KEY = "np_apikey"
const NP_API_HOST = "np_api_host"

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			API_KEY: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NULLPLATFORM_API_KEY", nil),
				Optional:    true,
				Sensitive:   true,
				Description: "Nullplatform API KEY. Can also be set with the `NULLPLATFORM_API_KEY` environment variable.",
			},
			HOST: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NULLPLATFORM_HOST", "api.nullplatform.com"),
				Optional:    true,
				Description: "Nullplatform HOST. Can also be set with the `NULLPLATFORM_HOST` environment variable. If omitted, the default value is `api.nullplatform.com`",
			},
			NP_API_KEY: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NP_API_KEY", nil),
				Optional:    true,
				Sensitive:   true,
				Description: "Nullplatform API KEY. Can also be set with the `NP_API_KEY` environment variable.",
				Deprecated:  "The 'np_apikey' attribute is deprecated and will be removed in a future version. Please use 'api_key' instead.",
			},
			NP_API_HOST: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NP_API_HOST", nil),
				Optional:    true,
				Description: "Nullplatform API HOSTNAME. Can also be set with the `NP_API_HOST` environment variable. If omitted, the default value is `api.nullplatform.com`",
				Deprecated:  "The 'np_api_host' attribute is deprecated and will be removed in a future version. Please use 'host' instead.",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"nullplatform_account":                resourceAccount(),
			"nullplatform_api_key":                resourceApiKey(),
			"nullplatform_approval_action":        resourceApprovalAction(),
			"nullplatform_approval_policy":        resourceApprovalPolicy(),
			"nullplatform_dimension":              resourceDimension(),
			"nullplatform_dimension_value":        resourceDimensionValue(),
			"nullplatform_link":                   resourceLink(),
			"nullplatform_metadata_specification": resourceMetadataSpecification(),
			"nullplatform_namespace":              resourceNamespace(),
			"nullplatform_notification_channel":   resourceNotificationChannel(),
			"nullplatform_parameter":              resourceParameter(),
			"nullplatform_parameter_value":        resourceParameterValue(),
			"nullplatform_provider_config":        resourceProviderConfig(),
			"nullplatform_runtime_configuration":  resourceRuntimeConfiguration(),
			"nullplatform_scope":                  resourceScope(),
			"nullplatform_service":                resourceService(),
			"nullplatform_service_specification":  resourceServiceSpecification(),
			"nullplatform_action_specification":   resourceActionSpecification(),
			"nullplatform_link_specification":     resourceLinkSpecification(),
			"nullplatform_authz_grant":            resourceAuthzGrant(),
			"nullplatform_user":                   resourceUser(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"nullplatform_dimension":         dataSourceDimension(),
			"nullplatform_scope":             dataSourceScope(),
			"nullplatform_application":       dataSourceApplication(),
			"nullplatform_service":           dataSourceService(),
			"nullplatform_parameter":         dataSourceParameter(),
			"nullplatform_parameter_by_name": dataSourceParameterByName(),
		},
	}

	provider.ConfigureContextFunc = func(_ context.Context, d *schema.ResourceData) (any, diag.Diagnostics) {
		apiKey, apiKeyDiags := getAPIKey(d)
		apiUrl, apiUrlDiags := getAPIHost(d)

		diags := append(apiKeyDiags, apiUrlDiags...)
		if len(diags) > 0 && hasErrors(diags) {
			return nil, diags
		}

		c := &NullClient{
			Client: &http.Client{
				Transport: &loghttp.Transport{},
			},
			ApiKey: apiKey,
			ApiURL: apiUrl,
		}

		return c, diags
	}

	return provider
}

func getAPIKey(d *schema.ResourceData) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v, ok := d.GetOk(API_KEY); ok {
		return v.(string), diags
	}
	if v, ok := d.GetOk(NP_API_KEY); ok {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  "Deprecated API Key Usage",
			Detail:   "You are using the deprecated 'np_apikey'. Please update your configuration to use 'api_key' instead.",
		})
		return v.(string), diags
	}
	diags = append(diags, diag.Diagnostic{
		Severity: diag.Error,
		Summary:  "Missing API Key",
		Detail:   "Either 'api_key' or 'np_apikey' must be set. Please provide an API key for authentication.",
	})
	return "", diags
}

func getAPIHost(d *schema.ResourceData) (string, diag.Diagnostics) {
	var diags diag.Diagnostics
	if v, ok := d.GetOk(HOST); ok {
		return v.(string), diags
	}
	if v, ok := d.GetOk(NP_API_HOST); ok {
		diags = append(diags, diag.Diagnostic{
			Severity: diag.Warning,
			Summary:  "Deprecated Host Usage",
			Detail:   "You are using the deprecated 'np_api_host'. Please update your configuration to use 'host' instead.",
		})
		return v.(string), diags
	}
	diags = append(diags, diag.Diagnostic{
		Severity: diag.Error,
		Summary:  "Missing API Host",
		Detail:   "Either 'host' or 'np_api_host' must be set. Please provide a host for authentication.",
	})
	return "", diags
}

func hasErrors(diags diag.Diagnostics) bool {
	for _, d := range diags {
		if d.Severity == diag.Error {
			return true
		}
	}
	return false
}
