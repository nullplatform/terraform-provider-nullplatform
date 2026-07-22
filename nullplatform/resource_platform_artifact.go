package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePlatformArtifact() *schema.Resource {
	return &schema.Resource{
		Description: "The artifact resource registers a platform artifact revision: an immutable, " +
			"content-addressed reference to something that lives outside nullplatform (an OCI image, " +
			"a git repository at a reference, a blob). Registration is an idempotent upsert — the " +
			"per-type stable subset of `meta` identifies the artifact, each unique `meta` blob pins " +
			"one revision. The resource ID is the revision id, ready to be used as a package BOM " +
			"component's `resource_revision_id`. Artifacts are immutable records: destroying this " +
			"resource only removes it from Terraform state.",

		Create: PlatformArtifactCreate,
		Read:   PlatformArtifactRead,
		Update: PlatformArtifactUpdate,
		Delete: PlatformArtifactDelete,

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
				Description: "The owner NRN of the artifact. Writes (new revisions, re-scoping) are gated on it.",
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.StringInSlice(
					[]string{"oci_image", "oras_artifact", "git_repository", "blob"},
					false,
				),
				Description: "Artifact type; discriminates the `meta` shape (e.g. git_repository requires { url, reference }).",
			},
			"meta": {
				Type:             schema.TypeString,
				Required:         true,
				ForceNew:         true,
				ValidateFunc:     validation.StringIsJSON,
				DiffSuppressFunc: suppressEquivalentJSON,
				Description: "JSON object with the flat meta blob for this revision. The per-type stable " +
					"subset (e.g. git_repository: url; oci_image: registry+repository) identifies the " +
					"artifact; the rest pins the revision (e.g. reference, digest). Changing it registers " +
					"a new revision (new resource).",
			},
			"visible_to": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				Description: "NRNs allowed to consume (read/link) this artifact. Supports trailing-wildcard " +
					"scopes (\"organization=1:account=*\") and the global wildcard \"organization=*\" " +
					"(requires the write action org-wide). Defaults to [nrn].",
			},
			"artifact_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The artifact (envelope) id — what BOM components use as `resource_id`.",
			},
		},
	}
}

func buildArtifactRegistration(d *schema.ResourceData) (*PlatformArtifactRegistration, error) {
	var meta map[string]interface{}
	if err := json.Unmarshal([]byte(d.Get("meta").(string)), &meta); err != nil {
		return nil, fmt.Errorf("error parsing artifact meta JSON: %v", err)
	}

	registration := &PlatformArtifactRegistration{
		Nrn:  d.Get("nrn").(string),
		Type: d.Get("type").(string),
		Meta: meta,
	}

	if raw, ok := d.GetOk("visible_to"); ok {
		for _, entry := range raw.([]interface{}) {
			registration.VisibleTo = append(registration.VisibleTo, entry.(string))
		}
	}

	return registration, nil
}

func PlatformArtifactCreate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	registration, err := buildArtifactRegistration(d)
	if err != nil {
		return err
	}

	revision, err := nullOps.RegisterPlatformArtifact(registration)
	if err != nil {
		return err
	}

	d.SetId(revision.ResourceRevisionID)

	return PlatformArtifactRead(d, m)
}

func PlatformArtifactRead(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	revision, err := nullOps.GetPlatformArtifactRevision(d.Id())
	if err != nil {
		return err
	}

	metaJSON, err := json.Marshal(revision.Meta)
	if err != nil {
		return fmt.Errorf("error serializing artifact meta to JSON: %v", err)
	}

	if err := d.Set("nrn", revision.Nrn); err != nil {
		return err
	}
	if err := d.Set("type", revision.Type); err != nil {
		return err
	}
	if err := d.Set("meta", string(metaJSON)); err != nil {
		return err
	}
	if err := d.Set("visible_to", revision.VisibleTo); err != nil {
		return err
	}
	if err := d.Set("artifact_id", revision.ResourceID); err != nil {
		return err
	}

	return nil
}

// PlatformArtifactUpdate only handles visible_to: re-registering the same
// meta is the API's write path on the existing artifact and re-scopes who
// can consume it. Everything else is ForceNew.
func PlatformArtifactUpdate(d *schema.ResourceData, m interface{}) error {
	nullOps := m.(NullOps)

	if d.HasChange("visible_to") {
		registration, err := buildArtifactRegistration(d)
		if err != nil {
			return err
		}
		if _, err := nullOps.RegisterPlatformArtifact(registration); err != nil {
			return err
		}
	}

	return PlatformArtifactRead(d, m)
}

// PlatformArtifactDelete forgets the registration: artifacts are immutable,
// content-addressed records that package revisions may pin forever, so the
// API intentionally exposes no delete.
func PlatformArtifactDelete(d *schema.ResourceData, m interface{}) error {
	d.SetId("")
	return nil
}
