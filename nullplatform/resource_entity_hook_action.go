package nullplatform

import (
	"context"
	"fmt"
	"reflect"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEntityHookAction() *schema.Resource {
	return &schema.Resource{
		Description: "The entity hook action resource allows you to configure a nullplatform action for the entity hook workflow",

		Create: EntityHookActionCreate,
		Read:   EntityHookActionRead,
		Update: EntityHookActionUpdate,
		Delete: EntityHookActionDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"entity": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The type of entity affected by this entity hook action. Possible values: [`application`, `scope`, `deployment`]. Example: `scope`.",
			},
			"action": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The specific action you want to be notified about. Possible values: [`deployment:create`, `deployment:write`, `deployment:delete`, `scope:create`, `scope:write`, `scope:delete`, `application:create`, `application:write`, `application:delete`]. Example: `scope:create`.",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				ForceNew: true,
				Optional: true,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Key-value pairs defining the scope of the action. Defaults to empty map. Example: `{\"environment\":\"production\",\"country\":\"us\"}`.",
			},
			"on_policy_success": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "manual",
				Description: "The action to be taken on the entity hook success. Defaults to \"manual\". Note: Currently, only \"manual\" is supported. Possible values: [`manual`].",
			},
			"on_policy_fail": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "manual",
				Description: "The action to be taken on entity hook failure. Defaults to \"manual\". Note: Currently, only \"manual\" is supported. Possible values: [`manual`].",
			},
			"when": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Specifies whether the hook occurs before or after nullplatform's internal logic. Possible values: [`before`, `after`]. Example: `before`.",
			},
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The type of entity hook action. Note: Currently, only \"hook\" is supported. Possible values: [`hook`]. Example: `hook`.",
			},
			"on": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Specifies the type of action that triggers the hook. Possible values: [`create`, `update`, `delete`]. Example: `create`.",
			},
		}),
	}
}

func EntityHookActionCreate(d *schema.ResourceData, m any) error {
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
	entity := d.Get("entity").(string)
	action := d.Get("action").(string)
	onPolicySuccess := d.Get("on_policy_success").(string)
	onPolicyFail := d.Get("on_policy_fail").(string)
	when := d.Get("when").(string)
	hookType := d.Get("type").(string)
	on := d.Get("on").(string)

	// Get dimensions or use empty map as default
	dimensions := make(map[string]string)
	if v, ok := d.GetOk("dimensions"); ok {
		dimensionsMap := v.(map[string]any)
		for key, value := range dimensionsMap {
			dimensions[key] = value.(string)
		}
	}

	newEntityHookAction := &EntityHookAction{
		Nrn:             nrn,
		Entity:          entity,
		Action:          action,
		Dimensions:      dimensions,
		OnPolicySuccess: onPolicySuccess,
		OnPolicyFail:    onPolicyFail,
		When:            when,
		Type:            hookType,
		On:              on,
	}

	entityHookAction, err := nullOps.CreateEntityHookAction(newEntityHookAction)
	if err != nil {
		return err
	}

	d.SetId(entityHookAction.Id)

	return EntityHookActionRead(d, m)
}

func EntityHookActionRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	entityHookActionId := d.Id()

	entityHookAction, err := nullOps.GetEntityHookAction(entityHookActionId)
	if err != nil {
		if entityHookAction != nil && entityHookAction.Status == "deleted" {
			d.SetId("")
			return nil
		}
		return err
	}

	if err := d.Set("nrn", entityHookAction.Nrn); err != nil {
		return err
	}

	if err := d.Set("entity", entityHookAction.Entity); err != nil {
		return err
	}

	if err := d.Set("action", entityHookAction.Action); err != nil {
		return err
	}

	if err := d.Set("dimensions", entityHookAction.Dimensions); err != nil {
		return err
	}

	if err := d.Set("on_policy_success", entityHookAction.OnPolicySuccess); err != nil {
		return err
	}

	if err := d.Set("on_policy_fail", entityHookAction.OnPolicyFail); err != nil {
		return err
	}

	if err := d.Set("when", entityHookAction.When); err != nil {
		return err
	}

	if err := d.Set("type", entityHookAction.Type); err != nil {
		return err
	}

	if err := d.Set("on", entityHookAction.On); err != nil {
		return err
	}

	return nil
}

func EntityHookActionUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	entityHookActionId := d.Id()

	entityHookAction := &EntityHookAction{}
	hasChanges := false

	if d.HasChange("nrn") {
		entityHookAction.Nrn = d.Get("nrn").(string)
		hasChanges = true
	}

	if d.HasChange("entity") {
		entityHookAction.Entity = d.Get("entity").(string)
		hasChanges = true
	}

	if d.HasChange("action") {
		entityHookAction.Action = d.Get("action").(string)
		hasChanges = true
	}

	if d.HasChange("dimensions") {
		dimensionsMap := d.Get("dimensions").(map[string]interface{})

		dimensions := make(map[string]string)
		for key, value := range dimensionsMap {
			dimensions[key] = value.(string)
		}

		entityHookAction.Dimensions = dimensions
		hasChanges = true
	}

	if d.HasChange("on_policy_success") {
		entityHookAction.OnPolicySuccess = d.Get("on_policy_success").(string)
		hasChanges = true
	}

	if d.HasChange("on_policy_fail") {
		entityHookAction.OnPolicyFail = d.Get("on_policy_fail").(string)
		hasChanges = true
	}

	if d.HasChange("when") {
		entityHookAction.When = d.Get("when").(string)
		hasChanges = true
	}

	if d.HasChange("type") {
		entityHookAction.Type = d.Get("type").(string)
		hasChanges = true
	}

	if d.HasChange("on") {
		entityHookAction.On = d.Get("on").(string)
		hasChanges = true
	}

	if hasChanges && !reflect.DeepEqual(*entityHookAction, EntityHookAction{}) {
		err := nullOps.PatchEntityHookAction(entityHookActionId, entityHookAction)
		if err != nil {
			return err
		}
	}

	return EntityHookActionRead(d, m)
}

func EntityHookActionDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	entityHookActionId := d.Id()

	err := nullOps.DeleteEntityHookAction(entityHookActionId)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
