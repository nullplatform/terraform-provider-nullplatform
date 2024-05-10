package nullplatform

import (
	"context"
	"strconv"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceApplication() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceApplicationRead,
		Schema: map[string]*schema.Schema{
			"id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"name": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"status": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"namespace_id": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"repository_url": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"slug": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"template_id": {
				Type:     schema.TypeInt,
				Computed: true,
			},
			"auto_deploy_on_creation": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"repository_app_path": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"is_mono_repo": {
				Type:     schema.TypeBool,
				Computed: true,
			},
			"nrn": {
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}
}

func dataSourceApplicationRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	app, err := nullOps.GetApplication(strconv.Itoa(d.Get("id").(int)))
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("name", app.Name)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("name", app.Name)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("status", app.Status)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("namespace_id", app.NamespaceId)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("repository_url", app.RepositoryUrl)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("slug", app.Slug)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("template_id", app.TemplateId)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("auto_deploy_on_creation", app.AutoDeployOnCreation)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("repository_app_path", app.RepositoryAppPath)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("is_mono_repo", app.IsMonoRepo)
	if err != nil {
		return diag.FromErr(err)
	}

	err = d.Set("nrn", app.Nrn)
	if err != nil {
		return diag.FromErr(err)
	}

	//fmt.Printf("ResourceData: %+v\n", d)

	// We don't have a unique ID for this data resource so we create one using a
	// timestamp format. I've seen people use a hash of the returned API data as
	// a unique key.
	//
	// NOTE:
	// That hashcode helper is no longer available! It has been moved into an
	// internal directory meaning it's not supposed to be consumed.
	//
	// Reference:
	// https://github.com/hashicorp/terraform-plugin-sdk/blob/master/internal/helper/hashcode/hashcode.go
	//
	d.SetId(strconv.FormatInt(time.Now().Unix(), 10))

	return nil
}
