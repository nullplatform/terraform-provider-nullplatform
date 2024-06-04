package nullplatform

import (
	"context"
	"log"
	"reflect"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceParameter() *schema.Resource {
	return &schema.Resource{
		Description: "The parameter resource allows you to manage an application parameter.",

		Create: ParameterCreate,
		Read:   ParameterRead,
		Update: ParameterUpdate,
		Delete: ParameterDelete,

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
				Description: "Definition name of the variable.",
			},
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The NRN of the application to which the parameter belongs to.",
			},
			"type": {
				Type:        schema.TypeString,
				Default:     "environment",
				Optional:    true,
				ForceNew:    true,
				Description: "Possible values: [`environment`, `file`]",
			},
			"encoding": {
				Type:        schema.TypeString,
				Default:     "plaintext",
				Optional:    true,
				ForceNew:    true,
				Description: "Possible values: [`plaintext`, `base64`]",
			},
			"variable": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the environment variable. Required when `type = environment`.",
			},
			"destination_path": {
				Type:        schema.TypeString,
				Computed:    true,
				Optional:    true,
				Description: "The full path for file. Required when `type = file`.",
			},
			"secret": {
				Type:        schema.TypeBool,
				Default:     "false",
				Optional:    true,
				ForceNew:    true,
				Description: "`true` if the value is a secret, `false` otherwise",
			},
			"read_only": {
				Type:        schema.TypeBool,
				Default:     "false",
				Optional:    true,
				ForceNew:    true,
				Description: "`true` if the value is a secret, `false` otherwise",
			},
			"import_if_created": {
				Type:        schema.TypeBool,
				Default:     "false",
				Optional:    true,
				ForceNew:    true,
				Description: "If `true`, it avoids raising an error when the Parameter is already created. On terraform destroy the resource won't be deleted.",
			},
		},
	}
}

func ParameterCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	newParameter := &Parameter{
		Name:            d.Get("name").(string),
		Nrn:             d.Get("nrn").(string),
		Type:            d.Get("type").(string),
		Encoding:        d.Get("encoding").(string),
		Variable:        d.Get("variable").(string),
		DestinationPath: d.Get("destination_path").(string),
		Secret:          d.Get("secret").(bool),
		ReadOnly:        d.Get("read_only").(bool),
	}

	importIfCreated := d.Get("import_if_created").(bool)
	param, err := nullOps.CreateParameter(newParameter, importIfCreated)

	if err != nil {
		return err
	}

	d.Set("import_if_created", importIfCreated)

	d.SetId(strconv.Itoa(param.Id))

	return ParameterRead(d, m)
}

func ParameterRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	parameterId := d.Id()

	param, err := nullOps.GetParameter(parameterId)
	if err != nil {
		// FIXME: Validate if error == 404
		if !d.IsNewResource() {
			log.Printf("[WARN] Parameter ID %s not found, removing from state", d.Id())
			d.SetId("")
			return nil
		}
		return err
	}

	if err := d.Set("name", param.Name); err != nil {
		return err
	}

	if err := d.Set("nrn", param.Nrn); err != nil {
		return err
	}

	if err := d.Set("type", param.Type); err != nil {
		return err
	}

	if err := d.Set("encoding", param.Encoding); err != nil {
		return err
	}

	if err := d.Set("variable", param.Variable); err != nil {
		return err
	}

	if err := d.Set("destination_path", param.DestinationPath); err != nil {
		return err
	}

	if err := d.Set("secret", param.Secret); err != nil {
		return err
	}

	if err := d.Set("read_only", param.ReadOnly); err != nil {
		return err
	}

	// Value stored in the state file not returned by the Null API
	if importIfCreated, ok := d.GetOk("import_if_created"); ok {
		if err := d.Set("import_if_created", importIfCreated.(bool)); err != nil {
			return err
		}
	}

	return nil
}

func ParameterUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	parameterId := d.Id()

	param := &Parameter{}

	if d.HasChange("name") {
		param.Name = d.Get("name").(string)
	}

	if d.HasChange("nrn") {
		param.Nrn = d.Get("nrn").(string)
	}

	if d.HasChange("type") {
		param.Type = d.Get("type").(string)
	}

	if d.HasChange("encoding") {
		param.Encoding = d.Get("encoding").(string)
	}

	if d.HasChange("variable") {
		param.Variable = d.Get("variable").(string)
	}

	if d.HasChange("destination_path") {
		param.DestinationPath = d.Get("destination_path").(string)
	}

	if d.HasChange("secret") {
		param.Secret = d.Get("secret").(bool)
	}

	if d.HasChange("read_only") {
		param.ReadOnly = d.Get("read_only").(bool)
	}

	if !reflect.DeepEqual(*param, Parameter{}) {
		err := nullOps.PatchParameter(parameterId, param)
		if err != nil {
			return err
		}
	}

	return ParameterRead(d, m)
}

func ParameterDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	parameterId := d.Id()

	if !d.Get("import_if_created").(bool) {
		err := nullOps.DeleteParameter(parameterId)
		if err != nil {
			return err
		}
	}

	d.SetId("")

	return nil
}
