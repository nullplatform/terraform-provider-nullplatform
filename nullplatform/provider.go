package nullplatform

import (
	"context"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/motemen/go-loghttp"
)

const NP_API_KEY = "np_apikey"
const NP_API_HOST = "np_api_host"

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			NP_API_KEY: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NP_API_KEY", nil),
				Required:    true,
				Sensitive:   true,
				Description: "Null Platform API KEY. Can also be set with the `NP_API_KEY` environment variable.",
			},
			NP_API_HOST: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NP_API_HOST", "api.nullplatform.com"),
				Optional:    true,
				Description: "Null Platform API HOSTNAME. Can also be set with the `NP_API_HOST` environment variable. If omitted, the default value is `api.nullplatform.com`",
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"nullplatform_scope":           resourceScope(),
			"nullplatform_service":         resourceService(),
			"nullplatform_link":            resourceLink(),
			"nullplatform_parameter":       resourceParameter(),
			"nullplatform_parameter_value": resourceParameterValue(),
		},
		// DataSource is a subset of Resource.
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
		apiKey := d.Get(NP_API_KEY).(string)

		c := &NullClient{
			Client: &http.Client{
				Transport: &loghttp.Transport{},
			},
			ApiKey: apiKey,
			ApiURL: apiUrl,
		}

		return c, nil
	}

	return provider
}
