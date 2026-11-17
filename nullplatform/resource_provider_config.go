package nullplatform

import (
	"context"
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
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"dimensions": {
				Type:     schema.TypeMap,
				ForceNew: true,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A key-value map with the provider dimensions that apply to this scope. Optional: when omitted, the provider config applies without dimension scoping.",
			},
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The slug of the provider type (e.g., 'aws-eks', 'aws-lambda_iam').",
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
	if reflect.DeepEqual(oldJSON, newJSON) {
		return true
	}
	// The API populates its own defaults into attributes it stores (e.g. naming
	// patterns, traffic_manager.port), so the state (old) holds keys the config
	// (new) never set. Treat the config as satisfied when every key it DOES set
	// matches the state, ignoring the extra server-populated keys. This avoids a
	// permanent drift that would otherwise try to remove those defaults.
	// Known limitation: removing a key that was set to a non-default value to
	// revert it to the default is also suppressed; set it explicitly to change it.
	return isJSONSubset(newJSON, oldJSON)
}

// isJSONSubset reports whether sub is contained in sup: every object key in sub
// exists in sup with a value that is itself a subset (recursively for objects,
// deep-equal otherwise). Non-object values must be deep-equal.
func isJSONSubset(sub, sup interface{}) bool {
	subMap, subIsMap := sub.(map[string]interface{})
	supMap, supIsMap := sup.(map[string]interface{})
	if subIsMap && supIsMap {
		for key, subVal := range subMap {
			supVal, ok := supMap[key]
			if !ok || !isJSONSubset(subVal, supVal) {
				return false
			}
		}
		return true
	}
	return reflect.DeepEqual(sub, sup)
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

	var dimensions map[string]string
	if v, ok := d.GetOk("dimensions"); ok {
		dimensionsMap := v.(map[string]interface{})
		dimensions = make(map[string]string, len(dimensionsMap))
		for key, value := range dimensionsMap {
			dimensions[key] = value.(string)
		}
	}

	attributesJSON := d.Get("attributes").(string)
	var attributes map[string]interface{}
	if err := json.Unmarshal([]byte(attributesJSON), &attributes); err != nil {
		return fmt.Errorf("error parsing attributes JSON: %v", err)
	}

	typeSlug := d.Get("type").(string)
	specificationId, err := nullOps.GetSpecificationIdFromSlug(typeSlug, nrn)
	if err != nil {
		return fmt.Errorf("error fetching specification ID for slug %s: %v", typeSlug, err)
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

	if err := d.Set("type", specificationSlug); err != nil {
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
