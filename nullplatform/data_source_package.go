package nullplatform

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePackage() *schema.Resource {
	return &schema.Resource{
		Description: "Looks up an existing nullplatform package by its natural key (nrn, slug). " +
			"Optionally resolves a specific published version to its revision id; otherwise " +
			"`revision_id` follows the package default (falling back to latest). The revision ids " +
			"are what services bind to and what package-in-package composition will pin.",
		ReadContext: dataSourcePackageRead,
		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The owner NRN of the package.",
			},
			"slug": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The package slug, unique per NRN.",
			},
			"version": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Resolve this exact published semver to `revision_id`. When omitted, `revision_id` follows the package default.",
			},
			"name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Human-readable display name.",
			},
			"visible_to": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "NRNs allowed to consume this package.",
			},
			"revision_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Revision UUID for `version` when set; otherwise the default (or latest) revision.",
			},
			"default_revision_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Revision UUID services bind to by default.",
			},
			"latest_revision_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Highest-semver revision UUID.",
			},
			"default_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Semver of the default revision.",
			},
			"latest_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Semver of the latest revision.",
			},
		},
	}
}

func dataSourcePackageRead(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	nrn := d.Get("nrn").(string)
	slug := d.Get("slug").(string)

	pkg, err := nullOps.FindPackage(nrn, slug)
	if err != nil {
		return diag.FromErr(err)
	}

	revisionID := pkg.DefaultRevisionID
	if revisionID == "" {
		revisionID = pkg.LatestRevisionID
	}

	if version, ok := d.GetOk("version"); ok {
		revisions, err := nullOps.ListPackageRevisions(pkg.ID)
		if err != nil {
			return diag.FromErr(err)
		}
		revisionID = ""
		for _, revision := range revisions {
			if revision.Version == version.(string) {
				revisionID = revision.ID
				break
			}
		}
		if revisionID == "" {
			return diag.FromErr(fmt.Errorf("package %s/%s has no published version %s", nrn, slug, version))
		}
	}

	d.SetId(pkg.ID)
	if err := d.Set("name", pkg.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("visible_to", pkg.VisibleTo); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("revision_id", revisionID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("default_revision_id", pkg.DefaultRevisionID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("latest_revision_id", pkg.LatestRevisionID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("default_version", pkg.DefaultVersion); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("latest_version", pkg.LatestVersion); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
