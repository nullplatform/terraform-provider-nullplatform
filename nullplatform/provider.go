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
			},
			NP_API_HOST: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NP_API_HOST", nil),
				Required:    true,
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			"nullplatform_scope": resourceScope(),
		},
		// DataSource is a subset of Resource.
		DataSourcesMap: map[string]*schema.Resource{
			"nullplatform_scope": dataSourceScope(),
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

		diag := c.GetToken()

		return c, diag
	}

	return provider
}
