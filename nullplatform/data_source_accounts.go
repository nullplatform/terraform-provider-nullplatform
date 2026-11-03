package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceAccounts() *schema.Resource {
	return &schema.Resource{
		Description: "Lists every account of an organization. Pair it with `nullplatform_namespaces` to walk the organization's account and namespace tree, e.g. to configure providers for each namespace in bulk.",

		ReadContext: dataSourceAccountsRead,
		Schema: map[string]*schema.Schema{
			"organization_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "ID of the organization whose accounts are listed. Defaults to the organization of the provider's credentials.",
			},
			"accounts": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Every account of the organization, ordered by ID. Deleted accounts are not listed.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "A system-wide unique ID for the account.",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The name of the account.",
						},
						"organization_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The ID of the organization this account belongs to.",
						},
						"repository_prefix": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The prefix used for repositories in this account.",
						},
						"repository_provider": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The repository provider for this account.",
						},
						"slug": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The unique slug identifier for the account.",
						},
						"status": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The status of the account.",
						},
						"nrn": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The Nullplatform Resource Name (NRN) for the account.",
						},
						"settings": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Account settings as a JSON string; empty when the account has none.",
						},
					},
				},
			},
		},
	}
}

func dataSourceAccountsRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	organizationID := strconv.Itoa(d.Get("organization_id").(int))
	if d.Get("organization_id").(int) == 0 {
		fromToken, err := nullOps.GetOrganizationIDFromToken()
		if err != nil {
			return diag.FromErr(fmt.Errorf("organization_id is not set and could not be read from the provider's credentials: %v", err))
		}
		organizationID = fromToken
	}
	orgID, err := strconv.Atoi(organizationID)
	if err != nil {
		return diag.FromErr(fmt.Errorf("invalid organization ID %q: %v", organizationID, err))
	}

	accounts, err := nullOps.ListAccounts(organizationID)
	if err != nil {
		return diag.FromErr(err)
	}

	rows := make([]map[string]any, 0, len(accounts))
	for _, account := range accounts {
		settings := ""
		if account.Settings != nil {
			settingsJSON, err := json.Marshal(account.Settings)
			if err != nil {
				return diag.FromErr(fmt.Errorf("error serializing settings for account %d: %v", account.Id, err))
			}
			settings = string(settingsJSON)
		}
		rows = append(rows, map[string]any{
			"id":                  account.Id,
			"name":                account.Name,
			"organization_id":     account.OrganizationId,
			"repository_prefix":   account.RepositoryPrefix,
			"repository_provider": account.RepositoryProvider,
			"slug":                account.Slug,
			"status":              account.Status,
			"nrn":                 account.Nrn,
			"settings":            settings,
		})
	}

	if err := d.Set("organization_id", orgID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("accounts", rows); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(organizationID)

	return nil
}
