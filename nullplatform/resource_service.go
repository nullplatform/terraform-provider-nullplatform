package nullplatform

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceService() *schema.Resource {
	return &schema.Resource{
		Description: "The service resource allows you to configure a Nullplatform Service",

		CreateContext: ServiceCreateContext,
		ReadContext:   ServiceReadContext,
		UpdateContext: ServiceUpdateContext,
		DeleteContext: ServiceDeleteContext,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the entity. Must be a non-empty string and not equal to null.",
			},
			"type": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				Default:      "dependency",
				ValidateFunc: validation.StringInSlice([]string{"dependency", "scope"}, false),
				Description:  "The type of the service. Must be one of: dependency, scope. Defaults to dependency.",
			},
			"specification_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique identifier for the entity represented as a UUID.",
			},
			"entity_nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "NRN representing a hierarchical identifier for nullplatform resourcesValue must match regular expression `^organization=[0-9]+(:account=[0-9]+)?(:namespace=[0-9]+)?(:application=[0-9]+)?(:scope=[0-9]+)?$`.",
			},
			"linkable_to": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A list of NRN representing the visibility settings for the entity. Specifies what/who can see this entity. Value must match regular expression `^organization=[0-9]+(:account=[0-9]+)?(:namespace=[0-9]+)?(:application=[0-9]+)?(:scope=[0-9]+)?$`.",
			},
			"desired_specification_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Desired unique identifier for the associated specification.",
			},
			"import": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
				ForceNew: true,
				Description: "When true (default), provisioning and decommissioning of the " +
					"underlying infrastructure are handled externally to nullplatform. " +
					"When false, the specification's create and delete actions are triggered " +
					"to handle the infrastructure lifecycle.",
			},
			"force_destroy": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				Description: "Only meaningful when `import = false`. When true, `terraform destroy` " +
					"skips the delete action and removes the service record directly via " +
					"`DELETE /service/{id}?force=true`. Use this as an escape hatch when the " +
					"service is stuck (e.g. the create action failed). Note: Terraform's " +
					"destroy reads this attribute from state, so you must run `terraform apply` " +
					"with `force_destroy = true` *before* running `terraform destroy` for it to " +
					"take effect. For tainted resources, run `terraform untaint` first so the " +
					"apply is an update rather than a replace. Has no effect when " +
					"`import = true`, where destroy already uses force.",
			},
			"messages": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
				Description: "A message and its severity level",
			},
			"attributes": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Attributes associated with the service, should be valid against the service specification attribute schema.",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Object representing dimensions with key-value pairs.",
			},
			"selectors": {
				Type:     schema.TypeList,
				Optional: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"category": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Category of the service specification",
						},
						"imported": {
							Type:        schema.TypeBool,
							Required:    true,
							Description: "Indicates whether the service is imported",
						},
						"provider": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Provider of the service (e.g., AWS, GCP)",
						},
						"sub_category": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Sub-category of the service",
						},
					},
				},
				Description: "Selectors for the service specification",
			},
			"status": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "active",
				Description: "Status of the service. Should be one of: [`pending_create`, `pending`, `creating`, `updating`, `deleting`, `active`, `deleted`, `failed`]",
			},
		},
	}
}

func ServiceCreateContext(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	name := d.Get("name").(string)
	specificationId := d.Get("specification_id").(string)
	entityNrn := d.Get("entity_nrn").(string)
	linkableTo := d.Get("linkable_to").([]interface{})
	desiredSpecificationId := d.Get("desired_specification_id").(string)
	status := d.Get("status").(string)
	if !importMode(d) {
		// Action-driven mode: the create action requires the service to be
		// in 'pending' on POST /service so the action can transition it to
		// active. The schema's 'active' default is the right default for
		// import=true (declarative), but wrong here.
		status = "pending"
	}
	messages := d.Get("messages").([]interface{})
	attributes := d.Get("attributes").(map[string]interface{})
	dimensions := d.Get("dimensions").(map[string]interface{})
	selectorsList := d.Get("selectors").([]interface{})
	var selectors Selectors
	if len(selectorsList) > 0 {
		selectorsMap := selectorsList[0].(map[string]interface{})
		selectors = Selectors{
			Category:    selectorsMap["category"].(string),
			Imported:    selectorsMap["imported"].(bool),
			Provider:    selectorsMap["provider"].(string),
			SubCategory: selectorsMap["sub_category"].(string),
		}
	}

	newService := &Service{
		Name:                   name,
		Type:                   d.Get("type").(string),
		SpecificationId:        specificationId,
		DesiredSpecificationId: desiredSpecificationId,
		EntityNrn:              entityNrn,
		LinkableTo:             linkableTo,
		Status:                 status,
		Messages:               messages,
		Selectors:              &selectors,
		Attributes:             attributes,
		Dimensions:             dimensions,
	}

	s, err := nullOps.CreateService(newService)

	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(s.Id)

	if !importMode(d) {
		attrs, _ := d.Get("attributes").(map[string]interface{})
		if err := triggerServiceAction(ctx, nullOps, s.Id, s.SpecificationId, "create", attrs, d.Timeout(schema.TimeoutCreate)); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func ServiceReadContext(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	serviceID := d.Id()

	s, err := nullOps.GetService(serviceID)

	if err != nil {
		d.SetId("")
		return diag.FromErr(err)
	}

	if err := d.Set("name", s.Name); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("type", s.Type); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("specification_id", s.SpecificationId); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("desired_specification_id", s.DesiredSpecificationId); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("entity_nrn", s.EntityNrn); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("linkable_to", s.LinkableTo); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("status", s.Status); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("dimensions", s.Dimensions); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("messages", s.Messages); err != nil {
		return diag.FromErr(err)
	}

	selectors := []map[string]interface{}{
		{
			"category":     s.Selectors.Category,
			"imported":     s.Selectors.Imported,
			"provider":     s.Selectors.Provider,
			"sub_category": s.Selectors.SubCategory,
		},
	}
	if err := d.Set("selectors", selectors); err != nil {
		return diag.FromErr(err)
	}

	attributeMap := mapOfInterfacesToMapOfStrings(s.Attributes)
	if err := d.Set("attributes", attributeMap); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func ServiceUpdateContext(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	serviceID := d.Id()

	log.Println("serviceID:", serviceID)

	ps := &Service{}

	if d.HasChange("name") {
		ps.Name = d.Get("name").(string)
	}

	if d.HasChange("status") {
		ps.Status = d.Get("status").(string)
	}

	if d.HasChange("specification_id") {
		ps.SpecificationId = d.Get("specification_id").(string)
	}

	if d.HasChange("entity_nrn") {
		ps.EntityNrn = d.Get("entity_nrn").(string)
	}

	if d.HasChange("linkable_to") {
		ps.LinkableTo = d.Get("linkable_to").([]interface{})
	}

	if d.HasChange("dimensions") {
		dimensions := d.Get("dimensions").(map[string]interface{})

		ps.Dimensions = dimensions
	}

	if d.HasChange("attributes") {
		attributes := d.Get("attributes").(map[string]interface{})

		ps.Attributes = attributes
	}

	if d.HasChange("selectors") {
		selectorsList := d.Get("selectors").([]interface{})
		if len(selectorsList) > 0 {
			selectorsMap := selectorsList[0].(map[string]interface{})
			ps.Selectors = &Selectors{
				Category:    selectorsMap["category"].(string),
				Imported:    selectorsMap["imported"].(bool),
				Provider:    selectorsMap["provider"].(string),
				SubCategory: selectorsMap["sub_category"].(string),
			}
		}
	}

	if !reflect.DeepEqual(*ps, Service{}) {
		err := nullOps.PatchService(serviceID, ps)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func ServiceDeleteContext(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	serviceID := d.Id()

	if importMode(d) || d.Get("force_destroy").(bool) {
		if err := nullOps.DeleteService(serviceID, true); err != nil {
			return diag.FromErr(err)
		}
		d.SetId("")
		return nil
	}

	// Stuck-service recovery: a service whose create action failed sits in
	// status="failed" and cannot be cleanly torn down via its delete action
	// (the workflow runtime can't drive a broken state machine). Detect this
	// case by reading the live status and force-delete instead. Without this,
	// users would have to either (a) untaint+apply force_destroy=true into
	// state then destroy, or (b) manually clean up via the API.
	current, err := nullOps.GetService(serviceID)
	if err == nil && current != nil && current.Status == "failed" {
		log.Printf("[INFO] service %s is in status=failed; force-deleting instead of triggering delete action", serviceID)
		if err := nullOps.DeleteService(serviceID, true); err != nil {
			return diag.FromErr(err)
		}
		d.SetId("")
		return nil
	}

	specificationID := d.Get("specification_id").(string)
	attrs, _ := d.Get("attributes").(map[string]interface{})
	if err := triggerServiceAction(ctx, nullOps, serviceID, specificationID, "delete", attrs, d.Timeout(schema.TimeoutDelete)); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

const actionPollInterval = 15 * time.Second

func waitForActionTerminal(ctx context.Context, nullOps NullOps, serviceID, actionID string, timeout time.Duration) (*ActionInstance, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{"pending_create", "pending", "in_progress"},
		Target:  []string{"success"},
		Refresh: func() (interface{}, string, error) {
			a, err := nullOps.GetServiceAction(serviceID, actionID)
			if err != nil {
				return nil, "", err
			}
			if a.Status == "failed" || a.Status == "cancelled" {
				return a, a.Status, fmt.Errorf("action %s ended in status %q: %s",
					actionID, a.Status, summarizeMessages(a.Messages))
			}
			return a, a.Status, nil
		},
		Timeout:    timeout,
		Delay:      actionPollInterval,
		MinTimeout: actionPollInterval,
	}
	raw, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		return nil, err
	}
	if a, ok := raw.(*ActionInstance); ok {
		return a, nil
	}
	return nil, nil
}

func triggerServiceAction(ctx context.Context, nullOps NullOps, serviceID, specificationID, actionType string, attributes map[string]interface{}, timeout time.Duration) error {
	specs, err := nullOps.ListActionSpecifications(specificationID)
	if err != nil {
		return fmt.Errorf("listing action specifications: %w", err)
	}
	actionSpec, err := findActionSpecByType(specs, actionType)
	if err != nil {
		return fmt.Errorf("specification %s: %w", specificationID, err)
	}

	parameters, err := projectAttributesToParameters(attributes, actionSpec.Parameters)
	if err != nil {
		return fmt.Errorf("projecting attributes onto %s action parameter schema: %w", actionType, err)
	}

	action, err := nullOps.CreateServiceAction(serviceID, &ActionInstance{
		SpecificationId: actionSpec.Id,
		Parameters:      parameters,
	})
	if err != nil {
		return fmt.Errorf("creating %s action: %w", actionType, err)
	}

	if _, err := waitForActionTerminal(ctx, nullOps, serviceID, action.Id, timeout); err != nil {
		return err
	}
	return nil
}

// importMode reads the `import` attribute defensively. The schema's `Default: true`
// only applies during plan-time evaluation of new resources; for legacy state
// written before this attribute existed, the field is absent and `d.Get` returns
// the zero value (false). This helper restores the desired default by treating
// missing-from-state as `true`.
func importMode(d *schema.ResourceData) bool {
	if v, exists := d.GetOkExists("import"); exists {
		return v.(bool)
	}
	return true
}
