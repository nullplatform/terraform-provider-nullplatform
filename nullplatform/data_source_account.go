package nullplatform

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceAccount() *schema.Resource {
	return &schema.Resource{
		Description: "Provides information about an Account",

		ReadContext: dataSourceAccountRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "A system-wide unique ID for the Account",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The name of the account",
			},
			"organization_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the organization this account belongs to",
			},
			"repository_prefix": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The prefix used for repositories in this account",
			},
			"repository_provider": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The repository provider for this account",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The unique slug identifier for the account",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The status of the account",
			},
			"nrn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The Nullplatform Resource Name (NRN) for the account",
			},
			"settings": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Account settings as a JSON string",
			},
		},
	}
}

func dataSourceAccountRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	account, err := nullOps.GetAccount(strconv.Itoa(d.Get("id").(int)))
	if err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", account.Name); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("organization_id", account.OrganizationId); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("repository_prefix", account.RepositoryPrefix); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("repository_provider", account.RepositoryProvider); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("slug", account.Slug); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("status", account.Status); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("nrn", account.Nrn); err != nil {
		return diag.FromErr(err)
	}

	if account.Settings != nil {
		settingsJSON, jerr := json.Marshal(account.Settings)
		if jerr != nil {
			return diag.FromErr(jerr)
		}
		if err := d.Set("settings", string(settingsJSON)); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(strconv.Itoa(account.Id))

	return nil
}
