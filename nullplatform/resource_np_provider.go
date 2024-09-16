package nullplatform

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceNpProvider() *schema.Resource {
	return &schema.Resource{
		Description: "The np_provider resource allows you to configure a nullplatform Provider",

		Create: NpProviderCreate,
		Read:   NpProviderRead,
		Update: NpProviderUpdate,
		Delete: NpProviderDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "A system-wide unique ID representing the resource.",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				ForceNew: true,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A key-value map with the provider dimensions that apply to this scope.",
			},
			"specification": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The slug of the provider specification (e.g., 'aws/eks', 'aws/lambda_iam').",
			},
			"attributes": {
				Type:     schema.TypeMap,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "The set of attributes that this provider holds.",
			},
		},
	}
}

func NpProviderCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	dimensionsMap := d.Get("dimensions").(map[string]interface{})
	dimensions := make(map[string]string)
	for key, value := range dimensionsMap {
		dimensions[key] = value.(string)
	}

	attributesMap := d.Get("attributes").(map[string]interface{})
	attributes := make(map[string]interface{})
	for key, value := range attributesMap {
		attributes[key] = value
	}

	specificationSlug := d.Get("specification").(string)
	specificationId, err := nullOps.GetSpecificationIdFromSlug(specificationSlug)
	if err != nil {
		return fmt.Errorf("error fetching specification ID for slug %s: %v", specificationSlug, err)
	}

	newNpProvider := &NpProvider{
		Nrn:             d.Get("nrn").(string),
		Dimensions:      dimensions,
		SpecificationId: specificationId,
		Attributes:      attributes,
	}

	np, err := nullOps.CreateNpProvider(newNpProvider)

	if err != nil {
		return err
	}

	d.SetId(np.Id)

	return NpProviderRead(d, m)
}

func NpProviderRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	npProviderId := d.Id()

	np, err := nullOps.GetNpProvider(npProviderId)

	if err != nil {
		return err
	}

	if err := d.Set("nrn", np.Nrn); err != nil {
		return err
	}

	if err := d.Set("dimensions", np.Dimensions); err != nil {
		return err
	}

	specificationSlug, err := nullOps.GetSpecificationSlugFromId(np.SpecificationId)
	if err != nil {
		return fmt.Errorf("error fetching specification slug for ID %s: %v", np.SpecificationId, err)
	}

	if err := d.Set("specification", specificationSlug); err != nil {
		return err
	}

	if err := d.Set("attributes", np.Attributes); err != nil {
		return err
	}

	return nil
}

func NpProviderUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	npProviderId := d.Id()

	np := &NpProvider{}

	if d.HasChange("nrn") {
		np.Nrn = d.Get("nrn").(string)
	}

	if d.HasChange("dimensions") {
		dimensionsMap := d.Get("dimensions").(map[string]interface{})
		dimensions := make(map[string]string)
		for key, value := range dimensionsMap {
			dimensions[key] = value.(string)
		}
		np.Dimensions = dimensions
	}

	if d.HasChange("specification") {
		specificationSlug := d.Get("specification").(string)
		specificationId, err := nullOps.GetSpecificationIdFromSlug(specificationSlug)
		if err != nil {
			return fmt.Errorf("error fetching specification ID for slug %s: %v", specificationSlug, err)
		}
		np.SpecificationId = specificationId
	}

	if d.HasChange("attributes") {
		attributesMap := d.Get("attributes").(map[string]interface{})
		attributes := make(map[string]interface{})
		for key, value := range attributesMap {
			attributes[key] = value
		}
		np.Attributes = attributes
	}

	err := nullOps.PatchNpProvider(npProviderId, np)
	if err != nil {
		return err
	}

	return NpProviderRead(d, m)
}

func NpProviderDelete(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	npProviderId := d.Id()

	err := nullOps.DeleteNpProvider(npProviderId)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
