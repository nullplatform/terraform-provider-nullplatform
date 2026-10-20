package nullplatform

import (
	"context"
	"errors"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePackageInstallation() *schema.Resource {
	return &schema.Resource{
		Description: "Installs a package at an NRN of the caller's organization: the organization's own " +
			"row for that package, saying where its specification is usable (`visible_to`) and which " +
			"revision new services there are born with (`default_revision_id`). Installing copies " +
			"nothing — the specification, its actions and its artifacts stay the publisher's. A " +
			"package published for every organization (`visible_to` `organization=*`) is usable " +
			"only where it is installed; look it up with the `nullplatform_package` data source's " +
			"`visible_at`. Publishing a new revision, or the publisher moving its own default, " +
			"moves no installation: change `default_revision_id` here to move new services, and " +
			"roll existing services out one by one. Destroying uninstalls; the API refuses while a " +
			"live service born from the installation remains.",

		CreateContext: packageInstallationResourceCreate,
		ReadContext:   packageInstallationResourceRead,
		UpdateContext: packageInstallationResourceUpdate,
		DeleteContext: packageInstallationResourceDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "Where the package is installed: an NRN of the caller's organization (an " +
					"organization, account, namespace or application). Changing it installs again elsewhere.",
			},
			"package_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The package to install: one visible to the organization — published for " +
					"every organization, or the organization's own. Changing it is another installation.",
			},
			"default_revision_id": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				Description: "The revision new services under `visible_to` are born with. When omitted, the " +
					"package's default at install time is resolved and stored; the installation does " +
					"not follow the package's default afterwards. Must belong to the package and not be " +
					"deprecated. Changing it moves only new services.",
			},
			"visible_to": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Description: "Where inside the installing organization the package's specification is " +
					"usable: these NRNs and everything below them. Defaults to `[nrn]`. Every entry must " +
					"stay inside the installing organization.",
			},
			"default_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Semver of `default_revision_id`.",
			},
			"created_by": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Id of the user that installed the package.",
			},
			"missing_optional_requirements": {
				Type:     schema.TypeList,
				Computed: true,
				Description: "Optional providers the package declares that the organization had not " +
					"configured at `nrn` or above when it was installed. Reported once, at install; " +
					"missing required providers refuse the install instead.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"category": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Provider category slug, when the requirement names a category.",
						},
						"specification": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Provider specification slug, when the requirement names a specification.",
						},
					},
				},
			},
		},
	}
}

func stringList(raw interface{}) []string {
	var out []string
	for _, entry := range raw.([]interface{}) {
		out = append(out, entry.(string))
	}
	return out
}

func packageInstallationResourceCreate(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	body := &PackageInstallationCreate{
		Nrn:               d.Get("nrn").(string),
		PackageID:         d.Get("package_id").(string),
		DefaultRevisionID: d.Get("default_revision_id").(string),
	}
	if raw, ok := d.GetOk("visible_to"); ok {
		body.VisibleTo = stringList(raw)
	}

	installation, err := nullOps.CreatePackageInstallation(body)
	if err != nil {
		return diag.FromErr(err)
	}
	d.SetId(installation.ID)

	missing := make([]map[string]interface{}, 0, len(installation.MissingOptionalRequirements))
	for _, requirement := range installation.MissingOptionalRequirements {
		missing = append(missing, map[string]interface{}{
			"category":      requirement.Category,
			"specification": requirement.Specification,
		})
	}
	if err := d.Set("missing_optional_requirements", missing); err != nil {
		return diag.FromErr(err)
	}

	return setPackageInstallation(d, installation)
}

func packageInstallationResourceRead(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	installation, err := nullOps.GetPackageInstallation(d.Id())
	if errors.Is(err, errPackageInstallationNotFound) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	return setPackageInstallation(d, installation)
}

func packageInstallationResourceUpdate(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	patch := &PackageInstallationPatch{}
	if d.HasChange("default_revision_id") {
		patch.DefaultRevisionID = d.Get("default_revision_id").(string)
	}
	if d.HasChange("visible_to") {
		patch.VisibleTo = stringList(d.Get("visible_to"))
	}

	installation, err := nullOps.PatchPackageInstallation(d.Id(), patch)
	if err != nil {
		return diag.FromErr(err)
	}
	return setPackageInstallation(d, installation)
}

func packageInstallationResourceDelete(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	if err := nullOps.DeletePackageInstallation(d.Id()); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}

func setPackageInstallation(d *schema.ResourceData, installation *PackageInstallation) diag.Diagnostics {
	fields := map[string]interface{}{
		"nrn":                 installation.Nrn,
		"package_id":          installation.PackageID,
		"default_revision_id": installation.DefaultRevisionID,
		"default_version":     installation.DefaultVersion,
		"visible_to":          installation.VisibleTo,
		"created_by":          installation.CreatedBy,
	}
	for key, value := range fields {
		if err := d.Set(key, value); err != nil {
			return diag.FromErr(err)
		}
	}
	return nil
}
