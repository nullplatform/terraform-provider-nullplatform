package nullplatform

import (
	"context"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const NP_API_KEY = "np_apikey"
const NP_API_URL = "np_api_url"

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			NP_API_KEY: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NP_API_KEY", nil),
				Required:    true,
			},
			NP_API_URL: {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NP_API_URL", nil),
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
		apiUrl := strings.Trim(d.Get(NP_API_URL).(string), "\\")
		apiKey := d.Get(NP_API_KEY).(string)

		c := &NullClient{
			Client: &http.Client{},
			ApiKey: apiKey,
			ApiURL: apiUrl,
		}

		diag := c.GetToken()

		return c, diag
	}

	return provider
}
