package nullplatform

import (
	// Documentation:
	// https://pkg.go.dev/github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema
	//
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// Resource:
// A 'thing' you create, and then manage (update/delete) via terraform.
//
// Data Source:
// Data you can get access to and reference within your resources.

func Provider() *schema.Provider {
	provider := &schema.Provider{
		Schema: map[string]*schema.Schema{
			"apikey": {
				Type:        schema.TypeString,
				DefaultFunc: schema.EnvDefaultFunc("NP_API_KEY", nil),
				Required:    true,
			},
		},
		ResourcesMap: map[string]*schema.Resource{
			// Naming format...
			//
			// Map key: <provider>_<thing>
			// File:    resource_<provider>_<thing>.go
			//
			// NOTE:
			// The map key is what's documented as the 'thing' a consumer of this
			// provider would add to their terraform HCL file.
			// e.g. resource "mock_example" "my_own_name_for_this" {...}
			//
			"nullplatform_scope": resourceScope(),
		},
		// DataSource is a subset of Resource.
		DataSourcesMap: map[string]*schema.Resource{
			// Naming format...
			//
			// Map key: <provider>_<thing>
			// File:    data_source_<provider>_<thing>.go
			//
			// NOTE:
			// The map key is what's documented as the 'thing' a consumer of this
			// provider would add to their terraform HCL file.
			// e.g. data_source "mock_example" "my_own_name_for_this" {...}
			//
			"nullplatform": dataSourceScope(),
		},

		// To configure the provider (i.e. create an API client)
		// then pass ConfigureFunc. The any value returned by this function
		// is stored and passed into the subsequent resources as the meta
		// parameter (this includes Data Sources as they are subsets of Resources).
		//
		// Documentation:
		// https://pkg.go.dev/github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema#ConfigureFunc
		// https://pkg.go.dev/github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema#ConfigureContextFunc
	}
	provider.ConfigureContextFunc = func(_ context.Context, d *schema.ResourceData) (any, diag.Diagnostics) {
		url := "https://api.nullplatform.com/token"
		apiKey := d.Get("apikey").(string)

		tokenReq := TokenRequest{
			Apikey: apiKey,
		}

		var buf bytes.Buffer
		err := json.NewEncoder(&buf).Encode(tokenReq)

		if err != nil {
			return nil, diag.FromErr(err)
		}

		client := &http.Client{}
		r, err := http.NewRequest("POST", url, &buf)
		if err != nil {
			return err, diag.FromErr(err)
		}

		r.Header.Add("Content-Type", "application/json")

		res, err := client.Do(r)
		if err != nil {
			return nil, diag.FromErr(err)
		}

		defer res.Body.Close()

		if res.StatusCode != http.StatusOK {
			return nil, diag.FromErr(fmt.Errorf("error creating resource, got %d, api key was %s", res.StatusCode, apiKey))
		}

		tokenResp := &TokenResponse{}
		derr := json.NewDecoder(res.Body).Decode(tokenResp)

		if derr != nil {
			return nil, diag.FromErr(derr)
		}

		if tokenResp.AccessToken == "" {
			return nil, diag.FromErr(fmt.Errorf("no access token for null platform token rsp is: %s", tokenResp))
		}

		return tokenResp.AccessToken, nil
	}

	return provider
}
