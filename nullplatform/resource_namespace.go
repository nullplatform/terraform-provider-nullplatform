package nullplatform

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceNamespace() *schema.Resource {
	return &schema.Resource{
		Description: "The namespace resource allows you to configure a nullplatform namespace",

		Create: NamespaceCreate,
		Read:   NamespaceRead,
		Update: NamespaceUpdate,
		Delete: NamespaceDelete,

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
				ForceNew:    true,
				Description: "The name of the namespace. Maximum length is 60 characters.",
			},
			"slug": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "An account-wide unique slug for the namespace. Maximum length is 60 characters.",
			},
			"status": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "The status of the namespace. Possible values: [active, inactive].",
			},
			"account_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "The ID of the account that owns this namespace",
			},
		},
	}
}

func NamespaceCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	client := nullOps.(*NullClient)

	newNamespace := &Namespace{
		Name:      d.Get("name").(string),
		Slug:      d.Get("slug").(string),
		Status:    "active",
		AccountId: d.Get("account_id").(int),
	}

	namespace, err := client.CreateNamespace(newNamespace)
	if err != nil {
		return fmt.Errorf("error creating namespace: %w", err)
	}

	d.SetId(strconv.Itoa(namespace.Id))
	return NamespaceRead(d, m)
}

func NamespaceRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	namespaceId := d.Id()

	namespace, err := nullOps.GetNamespace(namespaceId)
	if err != nil {
		if namespace != nil && namespace.Status == "inactive" {
			d.SetId("")
			return fmt.Errorf("namespace with ID '%s' is inactive and has been removed from state", namespaceId)
		}
		return fmt.Errorf("failed to fetch namespace with ID '%s': %w", namespaceId, err)
	}

	if err := d.Set("name", namespace.Name); err != nil {
		return fmt.Errorf("failed to set 'name' for namespace with ID '%s': %w", namespaceId, err)
	}
	if err := d.Set("slug", namespace.Slug); err != nil {
		return fmt.Errorf("failed to set 'slug' for namespace with ID '%s': %w", namespaceId, err)
	}
	if err := d.Set("status", namespace.Status); err != nil {
		return fmt.Errorf("failed to set 'status' for namespace with ID '%s': %w", namespaceId, err)
	}
	if err := d.Set("account_id", namespace.AccountId); err != nil {
		return fmt.Errorf("failed to set 'account_id' for namespace with ID '%s': %w", namespaceId, err)
	}

	return nil
}

func NamespaceUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	namespaceId := d.Id()

	namespace := &Namespace{}

	if d.HasChange("name") {
		namespace.Name = d.Get("name").(string)
	}
	if d.HasChange("slug") {
		namespace.Slug = d.Get("slug").(string)
	}
	if d.HasChange("status") {
		namespace.Status = d.Get("status").(string)
	}

	err := nullOps.PatchNamespace(namespaceId, namespace)
	if err != nil {
		return fmt.Errorf("failed to update namespace with ID '%s': %w", namespaceId, err)
	}

	if err := NamespaceRead(d, m); err != nil {
		return fmt.Errorf("error reading namespace after update for ID '%s': %w", namespaceId, err)
	}

	return nil
}

func NamespaceDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	namespaceId := d.Id()

	err := nullOps.DeleteNamespace(namespaceId)
	if err != nil {
		return fmt.Errorf("failed to delete namespace with ID '%s': %w", namespaceId, err)
	}

	d.SetId("")
	return nil
}
