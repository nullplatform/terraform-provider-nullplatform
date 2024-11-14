package main

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceProviderConfigAzureconfiguration() *schema.Resource {
	return &schema.Resource{
		Description: "Azure",

		Create: providerConfigAzureconfigurationCreate,
		Read:   providerConfigAzureconfigurationRead,
		Update: providerConfigAzureconfigurationUpdate,
		Delete: ProviderConfigDelete,

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"dimensions": {
				Type:     schema.TypeMap,
				ForceNew: true,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A key-value map with the provider dimensions that apply to this scope.",
			},
			"networking": {
				Type:        schema.TypeList,
				Description: "Configuration for DNS zones and domain names in Azure, including both public and private DNS settings",
				MaxItems:    1,
				Required:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"domain_name": {
							Type:        schema.TypeString,
							Description: "The root domain name to be used for your Azure resources. Must be a valid domain name that you own or control",
						},
						"application_domain": {
							Type:        schema.TypeBool,
							Description: "Use account name as part of applications domains",
						},
						"public_dns_zone_name": {
							Type:        schema.TypeString,
							Description: "The name of your Azure DNS public zone. This zone will handle public-facing DNS resolution for your domain",
						},
						"private_dns_zone_name": {
							Type:        schema.TypeString,
							Description: "The name of your Azure DNS private zone. This zone will handle internal DNS resolution within your virtual network",
						},
						"public_dns_zone_resource_group_name": {
							Type:        schema.TypeString,
							Description: "The name of the Azure Resource Group containing your public DNS zone. Resource group names must be unique within your subscription",
						},
						"private_dns_zone_resource_group_name": {
							Type:        schema.TypeString,
							Description: "The name of the Azure Resource Group containing your private DNS zone. Resource group names must be unique within your subscription",
						},
					},
				},
			},
			"authentication": {
				Type:        schema.TypeList,
				Description: "Credentials and identifiers required for authenticating with Azure services",
				MaxItems:    1,
				Required:    true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"client_id": {
							Type:        schema.TypeString,
							Description: "The unique identifier (UUID) of your Azure Active Directory application registration",
						},
						"tenant_id": {
							Type:        schema.TypeString,
							Description: "The unique identifier (UUID) of your Azure Active Directory tenant where the application is registered",
						},
						"client_secret": {
							Type:        schema.TypeString,
							Description: "The secret key generated for your Azure Active Directory application. Keep this value secure and never share it",
						},
						"subscription_id": {
							Type:        schema.TypeString,
							Description: "The unique identifier (UUID) of your Azure subscription where resources will be deployed",
						},
					},
				},
			},
		}),

		CustomizeDiff: CustomizeNRNDiff,
	}
}

func providerConfigAzureconfigurationCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	var nrn string
	var err error
	if v, ok := d.GetOk("nrn"); ok {
		nrn = v.(string)
	} else {
		nrn, err = ConstructNRNFromComponents(d, nullOps)
		if err != nil {
			return fmt.Errorf("error constructing NRN: %v %s", err, nrn)
		}
	}

	dimensionsMap := d.Get("dimensions").(map[string]interface{})
	dimensions := make(map[string]string)
	for key, value := range dimensionsMap {
		dimensions[key] = value.(string)
	}

	// Build attributes from individual fields
	attributes := make(map[string]interface{})

	if v, ok := d.GetOk("authentication"); ok {
		attributes["authentication"] = v
	}

	if v, ok := d.GetOk("networking"); ok {
		attributes["networking"] = v
	}

	// Get specification ID for this provider type
	specificationId, err := nullOps.GetSpecificationIdFromSlug("azure-configuration", nrn)
	if err != nil {
		return fmt.Errorf("error fetching specification ID for azure-configuration: %v", err)
	}

	newProviderConfig := &ProviderConfig{
		Nrn:             nrn,
		Dimensions:      dimensions,
		SpecificationId: specificationId,
		Attributes:      attributes,
	}

	pc, err := nullOps.CreateProviderConfig(newProviderConfig)
	if err != nil {
		return err
	}

	d.SetId(pc.Id)
	return providerConfigAzureconfigurationRead(d, m)
}

func providerConfigAzureconfigurationRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	providerConfigId := d.Id()

	pc, err := nullOps.GetProviderConfig(providerConfigId)
	if err != nil {
		return err
	}

	if err := d.Set("nrn", pc.Nrn); err != nil {
		return err
	}

	if err := d.Set("dimensions", pc.Dimensions); err != nil {
		return err
	}

	// Verify this is the correct provider type
	specificationSlug, err := nullOps.GetSpecificationSlugFromId(pc.SpecificationId)
	if err != nil {
		return fmt.Errorf("error fetching specification slug for ID %s: %v", pc.SpecificationId, err)
	}
	if specificationSlug != "azure-configuration" {
		return fmt.Errorf("provider configuration type mismatch: expected azure-configuration, got %s", specificationSlug)
	}

	// Set individual fields from attributes
	for key, value := range pc.Attributes {
		if err := d.Set(key, value); err != nil {
			return fmt.Errorf("error setting %s: %v", key, err)
		}
	}

	return nil
}

func providerConfigAzureconfigurationUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	providerConfigId := d.Id()

	pc := &ProviderConfig{}

	// Check which fields have changed and update attributes accordingly
	attributes := make(map[string]interface{})

	if d.HasChange("authentication") {
		if v, ok := d.GetOk("authentication"); ok {
			attributes["authentication"] = v
		}
	}

	if d.HasChange("networking") {
		if v, ok := d.GetOk("networking"); ok {
			attributes["networking"] = v
		}
	}

	if len(attributes) > 0 {
		pc.Attributes = attributes
	}

	err := nullOps.PatchProviderConfig(providerConfigId, pc)
	if err != nil {
		return err
	}

	return providerConfigAzureconfigurationRead(d, m)
}
