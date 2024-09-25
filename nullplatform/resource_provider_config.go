package nullplatform

import (
	"encoding/json"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceProviderConfig() *schema.Resource {
	return &schema.Resource{
		Description: "The provider_config resource allows you to configure a nullplatform Provider",

		Create: ProviderConfigCreate,
		Read:   ProviderConfigRead,
		Update: ProviderConfigUpdate,
		Delete: ProviderConfigDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

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
			"specification": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The slug of the provider specification (e.g., 'aws-eks', 'aws-lambda_iam').",
			},
			"attributes": {
				Type:             schema.TypeString,
				Required:         true,
				Description:      "The set of attributes that this provider holds as a JSON string.",
				DiffSuppressFunc: suppressEquivalentJSON,
			},
		}),
	}
}

func suppressEquivalentJSON(k, old, new string, d *schema.ResourceData) bool {
	var oldJSON, newJSON interface{}
	if err := json.Unmarshal([]byte(old), &oldJSON); err != nil {
		return false
	}
	if err := json.Unmarshal([]byte(new), &newJSON); err != nil {
		return false
	}
	return reflect.DeepEqual(oldJSON, newJSON)
}

func ProviderConfigCreate(d *schema.ResourceData, m interface{}) error {
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

	attributesJSON := d.Get("attributes").(string)
	var attributes map[string]interface{}
	if err := json.Unmarshal([]byte(attributesJSON), &attributes); err != nil {
		return fmt.Errorf("error parsing attributes JSON: %v", err)
	}

	specificationSlug := d.Get("specification").(string)
	specificationId, err := nullOps.GetSpecificationIdFromSlug(specificationSlug, nrn)
	if err != nil {
		return fmt.Errorf("error fetching specification ID for slug %s: %v", specificationSlug, err)
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
	return ProviderConfigRead(d, m)
}

func ProviderConfigRead(d *schema.ResourceData, m interface{}) error {
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

	specificationSlug, err := nullOps.GetSpecificationSlugFromId(pc.SpecificationId)
	if err != nil {
		return fmt.Errorf("error fetching specification slug for ID %s: %v", pc.SpecificationId, err)
	}

	if err := d.Set("specification", specificationSlug); err != nil {
		return err
	}

	attributesJSON, err := json.Marshal(pc.Attributes)
	if err != nil {
		return fmt.Errorf("error serializing attributes to JSON: %v", err)
	}

	if err := d.Set("attributes", string(attributesJSON)); err != nil {
		return fmt.Errorf("error setting attributes in state: %v", err)
	}

	return nil
}

func ProviderConfigUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	providerConfigId := d.Id()

	pc := &ProviderConfig{}

	if d.HasChange("attributes") {
		attributesJSON := d.Get("attributes").(string)
		var attributes map[string]interface{}
		if err := json.Unmarshal([]byte(attributesJSON), &attributes); err != nil {
			return fmt.Errorf("error parsing attributes JSON: %v", err)
		}
		pc.Attributes = attributes
	}

	err := nullOps.PatchProviderConfig(providerConfigId, pc)
	if err != nil {
		return err
	}

	return ProviderConfigRead(d, m)
}

func ProviderConfigDelete(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)
	providerConfigId := d.Id()

	err := nullOps.DeleteProviderConfig(providerConfigId)
	if err != nil {
		return err
	}

	d.SetId("")
	return nil
}
