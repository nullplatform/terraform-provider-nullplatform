package nullplatform

import (
	"context"
	"fmt"
	"strconv"

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
				Type:        schema.TypeString,
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
		},
	}
}

func AccountCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	client := nullOps.(*NullClient)

	organizationID, err := client.GetOrganizationIDFromToken()
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

	account, err := nullOps.CreateAccount(newAccount)
	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(account.Id))

	// Set the computed organization_id in the state
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
