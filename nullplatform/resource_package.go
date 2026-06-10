package nullplatform

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePackage() *schema.Resource {
	return &schema.Resource{
		Description: "The package resource publishes a nullplatform package: a versioned, immutable " +
			"bill of materials (BOM) that pins exact revisions of service specifications, action/link " +
			"specifications and artifacts. Applying a new `version` + `components` publishes a new " +
			"revision through the idempotent slug-keyed publish (PUT /packages); previously published " +
			"revisions are never mutated. The first publish sticks the package default to that " +
			"revision; later publishes only move it when `default = true`.",

		Create: PackageCreate,
		Read:   PackageRead,
		Update: PackageUpdate,
		Delete: PackageDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The owner NRN of the package. Writes (publishes, patches, delete) are gated on it.",
			},
			"slug": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "URL-safe identifier, unique per NRN. Together with nrn it is the publish key.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Human-readable display name.",
			},
			"version": {
				Type:     schema.TypeString,
				Required: true,
				Description: "Semver of the revision this configuration publishes. Bump it together with " +
					"`components` changes to publish a new revision; re-applying the same version with the " +
					"same components is an idempotent no-op.",
			},
			"components": {
				Type:        schema.TypeList,
				Required:    true,
				Description: "Bill of materials: one entry per component, each pinning an exact resource revision.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Free-form component name, unique within the revision (e.g. \"spec\", \"runtime:main\").",
						},
						"resource_type": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Routing key identifying the owning service (e.g. \"service_specification\", \"action_specification\", \"artifact\").",
						},
						"resource_id": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "UUID of the underlying resource at its owning service.",
						},
						"resource_revision_id": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "UUID of the exact snapshot/revision this component pins.",
						},
						"parent_id": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "resource_id of the owning spec/link component in this BOM (required for action/link specification components).",
						},
					},
				},
			},
			"visible_to": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Description: "NRNs allowed to consume (read/link) this package. Supports trailing-wildcard " +
					"scopes (\"organization=1:account=*\") and the global wildcard \"organization=*\" " +
					"(requires the write action org-wide). Defaults to [nrn].",
			},
			"default": {
				Type:          schema.TypeBool,
				Optional:      true,
				Default:       false,
				ConflictsWith: []string{"default_version"},
				Description: "When true, every publish from this resource also promotes the published " +
					"revision to the package default (one-shot bump-and-promote).",
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
				Type:          schema.TypeString,
				Optional:      true,
				Computed:      true,
				ConflictsWith: []string{"default"},
				Description: "Pin the package default to this published version (resolved to its " +
					"revision id and PATCHed after each apply). Mutually exclusive with `default`. " +
					"When omitted, reflects the server-side default.",
			},
			"latest_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Semver of the latest revision.",
			},
			"published_revision_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Revision UUID published for the configured `version`.",
			},
		},
	}
}

func buildPackageUpsert(d *schema.ResourceData) *PackageUpsert {
	upsert := &PackageUpsert{
		Nrn:     d.Get("nrn").(string),
		Slug:    d.Get("slug").(string),
		Name:    d.Get("name").(string),
		Version: d.Get("version").(string),
		Default: d.Get("default").(bool),
	}

	for _, raw := range d.Get("components").([]interface{}) {
		component := raw.(map[string]interface{})
		entry := PackageComponent{
			Name:               component["name"].(string),
			ResourceType:       component["resource_type"].(string),
			ResourceID:         component["resource_id"].(string),
			ResourceRevisionID: component["resource_revision_id"].(string),
		}
		if parentID, ok := component["parent_id"].(string); ok && parentID != "" {
			entry.ParentID = &parentID
		}
		upsert.Components = append(upsert.Components, entry)
	}

	if raw, ok := d.GetOk("visible_to"); ok {
		for _, entry := range raw.([]interface{}) {
			upsert.VisibleTo = append(upsert.VisibleTo, entry.(string))
		}
	}

	return upsert
}

// configuredDefaultVersion distinguishes a user-set default_version from the
// computed server-side value the attribute also carries (Optional+Computed).
func configuredDefaultVersion(d *schema.ResourceData) (string, bool) {
	raw := d.GetRawConfig()
	if raw.IsNull() {
		return "", false
	}
	attr := raw.GetAttr("default_version")
	if attr.IsNull() {
		return "", false
	}
	return attr.AsString(), true
}

func pinDefaultVersion(nullOps NullOps, packageID, version string) error {
	revisions, err := nullOps.ListPackageRevisions(packageID)
	if err != nil {
		return err
	}
	for _, revision := range revisions {
		if revision.Version == version {
			return nullOps.PatchPackage(packageID, &PackagePatch{DefaultRevisionID: revision.ID})
		}
	}
	return fmt.Errorf("cannot pin default_version: package %s has no published version %s", packageID, version)
}

func PackageCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	pkg, err := nullOps.UpsertPackage(buildPackageUpsert(d))
	if err != nil {
		return err
	}

	d.SetId(pkg.ID)

	if version, configured := configuredDefaultVersion(d); configured {
		if err := pinDefaultVersion(nullOps, pkg.ID, version); err != nil {
			return err
		}
	}

	return PackageRead(d, m)
}

func PackageRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	pkg, err := nullOps.GetPackage(d.Id())
	if err != nil {
		return err
	}

	if err := d.Set("nrn", pkg.Nrn); err != nil {
		return err
	}
	if err := d.Set("slug", pkg.Slug); err != nil {
		return err
	}
	if err := d.Set("name", pkg.Name); err != nil {
		return err
	}
	if err := d.Set("visible_to", pkg.VisibleTo); err != nil {
		return err
	}
	if err := d.Set("default_revision_id", pkg.DefaultRevisionID); err != nil {
		return err
	}
	if err := d.Set("latest_revision_id", pkg.LatestRevisionID); err != nil {
		return err
	}
	if err := d.Set("default_version", pkg.DefaultVersion); err != nil {
		return err
	}
	if err := d.Set("latest_version", pkg.LatestVersion); err != nil {
		return err
	}

	// Resolve the revision id of the configured version. Revisions are
	// immutable, so this only changes when `version` does.
	version := d.Get("version").(string)
	revisions, err := nullOps.ListPackageRevisions(pkg.ID)
	if err != nil {
		return err
	}
	for _, revision := range revisions {
		if revision.Version == version {
			if err := d.Set("published_revision_id", revision.ID); err != nil {
				return err
			}
			break
		}
	}

	// `components` deliberately stays as configured: it describes the BOM of
	// the revision this resource published, while the API's resolved view
	// follows the package default and would report drift that isn't ours.

	return nil
}

func PackageUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	published := false
	if d.HasChange("version") || d.HasChange("components") || d.HasChange("visible_to") || d.HasChange("default") {
		// Publishing is the natural write path and also carries the envelope
		// fields (name, visible_to) along.
		if _, err := nullOps.UpsertPackage(buildPackageUpsert(d)); err != nil {
			return err
		}
		published = true
	}

	if !published && d.HasChange("name") {
		patch := &PackagePatch{Name: d.Get("name").(string)}
		if err := nullOps.PatchPackage(d.Id(), patch); err != nil {
			return err
		}
	}

	// Re-pin after every apply that has default_version configured: the
	// publish above may have minted the version the pin points at, and the
	// PATCH is idempotent.
	if version, configured := configuredDefaultVersion(d); configured {
		if err := pinDefaultVersion(nullOps, d.Id(), version); err != nil {
			return err
		}
	}

	return PackageRead(d, m)
}

func PackageDelete(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	if err := nullOps.DeletePackage(d.Id()); err != nil {
		return fmt.Errorf("error deleting package %s: %v", d.Id(), err)
	}

	d.SetId("")
	return nil
}
