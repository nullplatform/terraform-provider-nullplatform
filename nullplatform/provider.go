package nullplatform

import (
	"context"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/motemen/go-loghttp"
)

const API_KEY = "api_key"
const NP_API_KEY = "np_apikey"
const NP_API_HOST = "np_api_host"

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			API_KEY: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("API_KEY", nil),
				Optional:    true,
				Sensitive:   true,
				Description: "Null Platform API KEY. Can also be set with the `NP_API_KEY` environment variable.",
			},
			NP_API_KEY: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NP_API_KEY", nil),
				Optional:    true,
				Sensitive:   true,
				Description: "Null Platform API KEY. Can also be set with the `NP_API_KEY` environment variable.",
				Deprecated:  "Use 'api_key' instead. This field will be removed in a future version.",
			},
			NP_API_HOST: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NP_API_HOST", "api.nullplatform.com"),
				Optional:    true,
				Description: "Null Platform API HOSTNAME. Can also be set with the `NP_API_HOST` environment variable. If omitted, the default value is `api.nullplatform.com`",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"nullplatform_scope":                 resourceScope(),
			"nullplatform_service":               resourceService(),
			"nullplatform_link":                  resourceLink(),
			"nullplatform_parameter":             resourceParameter(),
			"nullplatform_parameter_value":       resourceParameterValue(),
			"nullplatform_approval_action":       resourceApprovalAction(),
			"nullplatform_approval_policy":       resourceApprovalPolicy(),
			"nullplatform_notification_channel":  resourceNotificationChannel(),
			"nullplatform_runtime_configuration": resourceRuntimeConfiguration(),
			"nullplatform_provider_config":       resourceProviderConfig(),
		},
		DataSourcesMap: map[string]*schema.Resource{
			"nullplatform_scope":             dataSourceScope(),
			"nullplatform_application":       dataSourceApplication(),
			"nullplatform_service":           dataSourceService(),
			"nullplatform_parameter":         dataSourceParameter(),
			"nullplatform_parameter_by_name": dataSourceParameterByName(),
		},
	}

	provider.ConfigureContextFunc = func(_ context.Context, d *schema.ResourceData) (any, diag.Diagnostics) {
		apiUrl := strings.Trim(d.Get(NP_API_HOST).(string), "\\")
		var apiKey string
		var diags diag.Diagnostics

		if v, ok := d.GetOk(API_KEY); ok {
			apiKey = v.(string)
		} else if v, ok := d.GetOk(NP_API_KEY); ok {
			apiKey = v.(string)
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Warning,
				Summary:  "Deprecated API Key Usage",
				Detail:   "You are using the deprecated 'np_apikey'. Please update your configuration to use 'api_key' instead.",
			})
		} else {
			diags = append(diags, diag.Diagnostic{
				Severity: diag.Error,
				Summary:  "Missing API Key",
				Detail:   "Either 'api_key' or 'np_apikey' must be set. Please provide an API key for authentication.",
			})
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
