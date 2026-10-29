package nullplatform

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourcePackage() *schema.Resource {
	return &schema.Resource{
		Description: "Looks up an existing nullplatform package by slug: either by its natural key " +
			"(`nrn`, `slug`), or among the packages visible at an NRN (`visible_at`, `slug`) — the " +
			"way to find a package published for every organization, whose owner NRN is not the " +
			"caller's. Optionally resolves a specific published version to its revision id; otherwise " +
			"`revision_id` follows the package default (falling back to latest). The revision ids " +
			"are what services bind to and what package-in-package composition will pin.",
		ReadContext: dataSourcePackageRead,
		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"nrn", "visible_at"},
				Description:  "The owner NRN of the package. Exactly one of `nrn` and `visible_at` is set.",
			},
			"visible_at": {
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"nrn", "visible_at"},
				Description: "An NRN the package must be visible at. Finds packages owned there or above it, " +
					"shared with it, or published for every organization (`visible_to` `organization=*`). " +
					"Fails when more than one visible package has the slug. For a package visible to " +
					"other organizations, the API reports `nrn` as the caller's own organization root, " +
					"never the owner's.",
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
			"components": {
				Type:     schema.TypeList,
				Computed: true,
				Description: "Bill of materials of `revision_id`: one entry per pinned component. How an " +
					"installing organization finds the resources of a package it does not own, e.g. the " +
					"service specification a scope type must point at.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Component name, unique within the revision (e.g. \"scope\", \"service\", \"create-scope\").",
						},
						"resource_type": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Kind of resource (e.g. \"service_specification\", \"action_specification\", \"artifact\").",
						},
						"resource_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "UUID of the underlying resource.",
						},
						"resource_revision_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "UUID of the snapshot or artifact revision the component pins.",
						},
						"parent_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "resource_id of the owning component, for action and link specifications.",
						},
					},
				},
			},
		},
	}
}

func flattenPackageComponents(components []PackageComponent) []map[string]interface{} {
	flattened := make([]map[string]interface{}, 0, len(components))
	for _, component := range components {
		parentID := ""
		if component.ParentID != nil {
			parentID = *component.ParentID
		}
		flattened = append(flattened, map[string]interface{}{
			"name":                 component.Name,
			"resource_type":        component.ResourceType,
			"resource_id":          component.ResourceID,
			"resource_revision_id": component.ResourceRevisionID,
			"parent_id":            parentID,
		})
	}
	return flattened
}

func dataSourcePackageRead(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	nrn := d.Get("nrn").(string)
	slug := d.Get("slug").(string)

	var pkg *Package
	var err error
	if visibleAt, ok := d.GetOk("visible_at"); ok {
		nrn = visibleAt.(string)
		pkg, err = nullOps.FindVisiblePackage(nrn, slug)
	} else {
		pkg, err = nullOps.FindPackage(nrn, slug)
	}
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

	var components []PackageComponent
	if revisionID != "" {
		revision, err := nullOps.GetPackageRevision(pkg.ID, revisionID)
		if err != nil {
			return diag.FromErr(err)
		}
		components = revision.Components
	}

	d.SetId(pkg.ID)
	if err := d.Set("components", flattenPackageComponents(components)); err != nil {
		return diag.FromErr(err)
	}
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
