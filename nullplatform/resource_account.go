package nullplatform

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceAccount() *schema.Resource {
	return &schema.Resource{
		Description: "The account resource allows you to configure a nullplatform account",

		Create: AccountCreate,
		Read:   AccountRead,
		Update: AccountUpdate,
		Delete: AccountDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the account",
			},
			"organization_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				ForceNew:    true,
				Description: "The ID of the organization this account belongs to (computed from authentication token)",
			},
			"repository_prefix": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The prefix used for repositories in this account",
			},
			"repository_provider": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The repository provider for this account",
			},
			"slug": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The unique slug identifier for the account",
			},
			"nrn": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The Nullplatform Resource Name (NRN) for the account",
			},
			"settings": {
				Type:        schema.TypeList,
				Optional:    true,
				MaxItems:    1,
				Description: "Configuration settings for the account",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"url_overrides": {
							Type:        schema.TypeList,
							Optional:    true,
							MaxItems:    1,
							Description: "URL override settings",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"home_url": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Override URL for home page",
									},
									"documentation_url": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Override URL for documentation",
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

func AccountCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	client := nullOps.(*NullClient)

	organizationIDStr, err := client.GetOrganizationIDFromToken()
	if err != nil {
		return fmt.Errorf("error getting organization ID from token: %w", err)
	}

	organizationID, err := strconv.Atoi(organizationIDStr)

	if err != nil {
		return fmt.Errorf("error getting organization ID from token: %w", err)
	}

	newAccount := &Account{
		Name:               d.Get("name").(string),
		OrganizationId:     organizationID,
		RepositoryPrefix:   d.Get("repository_prefix").(string),
		RepositoryProvider: d.Get("repository_provider").(string),
		Slug:               d.Get("slug").(string),
	}

	// Handle settings with nested structure
	if settings, ok := d.GetOk("settings"); ok {
		settingsList := settings.([]interface{})
		if len(settingsList) > 0 && settingsList[0] != nil {
			settingsMap := settingsList[0].(map[string]interface{})
			flatSettings := make(map[string]interface{})
			
			// Handle url_overrides nested object
			if urlOverrides, exists := settingsMap["url_overrides"]; exists && urlOverrides != nil {
				urlOverridesList := urlOverrides.([]interface{})
				if len(urlOverridesList) > 0 && urlOverridesList[0] != nil {
					urlOverridesMap := urlOverridesList[0].(map[string]interface{})
					for key, value := range urlOverridesMap {
						if value != nil {
							flatSettings["url_overrides."+key] = value
						}
					}
				}
			}
			
			newAccount.Settings = flatSettings
		}
	}

	account, err := nullOps.CreateAccount(newAccount)
	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(account.Id))

	if err := d.Set("organization_id", account.OrganizationId); err != nil {
		return fmt.Errorf("error setting organization_id: %w", err)
	}

	return AccountRead(d, m)
}

func AccountRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	accountId := d.Id()

	account, err := nullOps.GetAccount(accountId)
	if err != nil {
		if account.Status == "inactive" {
			d.SetId("")
			return nil
		}
		return err
	}

	if err := d.Set("name", account.Name); err != nil {
		return err
	}
	if err := d.Set("organization_id", account.OrganizationId); err != nil {
		return err
	}
	if err := d.Set("repository_prefix", account.RepositoryPrefix); err != nil {
		return err
	}
	if err := d.Set("repository_provider", account.RepositoryProvider); err != nil {
		return err
	}
	if err := d.Set("slug", account.Slug); err != nil {
		return err
	}
	if err := d.Set("nrn", account.Nrn); err != nil {
		return err
	}
	// Convert flat settings back to nested structure
	if account.Settings != nil && len(account.Settings) > 0 {
		urlOverridesMap := make(map[string]interface{})
		
		for key, value := range account.Settings {
			if strings.HasPrefix(key, "url_overrides.") {
				// Extract nested key
				nestedKey := strings.TrimPrefix(key, "url_overrides.")
				urlOverridesMap[nestedKey] = value
			}
		}
		
		// Build settings structure
		if len(urlOverridesMap) > 0 {
			settingsMap := map[string]interface{}{
				"url_overrides": []interface{}{urlOverridesMap},
			}
			if err := d.Set("settings", []interface{}{settingsMap}); err != nil {
				return err
			}
		}
	}

	return nil
}

func AccountUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	accountId := d.Id()

	account := &Account{}

	if d.HasChange("name") {
		account.Name = d.Get("name").(string)
	}
	if d.HasChange("repository_prefix") {
		account.RepositoryPrefix = d.Get("repository_prefix").(string)
	}
	if d.HasChange("repository_provider") {
		account.RepositoryProvider = d.Get("repository_provider").(string)
	}
	if d.HasChange("slug") {
		account.Slug = d.Get("slug").(string)
	}
	if d.HasChange("settings") {
		if settings, ok := d.GetOk("settings"); ok {
			settingsList := settings.([]interface{})
			if len(settingsList) > 0 && settingsList[0] != nil {
				settingsMap := settingsList[0].(map[string]interface{})
				flatSettings := make(map[string]interface{})
				
				// Handle url_overrides nested object
				if urlOverrides, exists := settingsMap["url_overrides"]; exists && urlOverrides != nil {
					urlOverridesList := urlOverrides.([]interface{})
					if len(urlOverridesList) > 0 && urlOverridesList[0] != nil {
						urlOverridesMap := urlOverridesList[0].(map[string]interface{})
						for key, value := range urlOverridesMap {
							if value != nil {
								flatSettings["url_overrides."+key] = value
							}
						}
					}
				}
				
				account.Settings = flatSettings
			} else {
				account.Settings = make(map[string]interface{})
			}
		} else {
			account.Settings = nil
		}
	}

	err := nullOps.PatchAccount(accountId, account)
	if err != nil {
		return err
	}

	return AccountRead(d, m)
}

func AccountDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	accountId := d.Id()

	err := nullOps.DeleteAccount(accountId)
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}
