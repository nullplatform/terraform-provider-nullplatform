package nullplatform

import (
	"context"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceNamespaces() *schema.Resource {
	return &schema.Resource{
		Description: "Lists every namespace of an account. The API lists namespaces one account at a time, so to cover an organization use `for_each` over the accounts of `nullplatform_accounts`.",

		ReadContext: dataSourceNamespacesRead,
		Schema: map[string]*schema.Schema{
			"account_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the account whose namespaces are listed.",
			},
			"namespaces": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Every namespace of the account, ordered by ID. Deleted namespaces are not listed.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "A system-wide unique ID for the namespace.",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The name of the namespace.",
						},
						"account_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "The ID of the account that owns this namespace.",
						},
						"slug": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The unique slug identifier for the namespace.",
						},
						"status": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The status of the namespace.",
						},
						"nrn": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "The Nullplatform Resource Name (NRN) for the namespace.",
						},
					},
				},
			},
		},
	}
}

func dataSourceNamespacesRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	accountID := strconv.Itoa(d.Get("account_id").(int))

	namespaces, err := nullOps.ListNamespaces(accountID)
	if err != nil {
		return diag.FromErr(err)
	}

	rows := make([]map[string]any, 0, len(namespaces))
	for _, namespace := range namespaces {
		rows = append(rows, map[string]any{
			"id":         namespace.Id,
			"name":       namespace.Name,
			"account_id": namespace.AccountId,
			"slug":       namespace.Slug,
			"status":     namespace.Status,
			"nrn":        namespace.Nrn,
		})
	}

	if err := d.Set("namespaces", rows); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(accountID)

	return nil
}
