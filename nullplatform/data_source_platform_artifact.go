package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourcePlatformArtifact() *schema.Resource {
	return &schema.Resource{
		Description: "Looks up an existing platform artifact (and one of its revisions) by its " +
			"meta fields — e.g. a git_repository by { url, reference } or an OCI image by " +
			"{ registry, repository, digest }. Identity fields select the artifact; when " +
			"per-revision fields are included the matching revision is resolved, otherwise the " +
			"latest revision is used. Read-only: never registers anything.",
		ReadContext: dataSourcePlatformArtifactRead,
		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The owner NRN the artifact is registered under.",
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice(
					[]string{"oci_image", "oras_artifact", "git_repository", "blob"},
					false,
				),
				Description: "The artifact type.",
			},
			"meta": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsJSON,
				Description: "JSON object with the meta fields to match. Identity fields (e.g. url, " +
					"registry+repository) select the artifact; per-revision fields (e.g. reference, " +
					"digest) additionally select a specific revision.",
			},
			"artifact_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The artifact (envelope) id — use as a BOM component's `resource_id`.",
			},
			"revision_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The resolved revision id — use as a BOM component's `resource_revision_id`.",
			},
			"revision_meta": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "JSON object with the full meta blob of the resolved revision.",
			},
			"visible_to": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "NRNs allowed to consume this artifact.",
			},
			"latest_revision_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The artifact's most recent revision id.",
			},
		},
	}
}

// metaMatches reports whether every key in `wanted` is present in `actual`
// with an equal (JSON) value.
func metaMatches(wanted, actual map[string]interface{}) bool {
	for key, wantedValue := range wanted {
		actualValue, present := actual[key]
		if !present {
			return false
		}
		wantedJSON, err := json.Marshal(wantedValue)
		if err != nil {
			return false
		}
		actualJSON, err := json.Marshal(actualValue)
		if err != nil {
			return false
		}
		if string(wantedJSON) != string(actualJSON) {
			return false
		}
	}
	return true
}

func dataSourcePlatformArtifactRead(_ context.Context, d *schema.ResourceData, m interface{}) diag.Diagnostics {
	nullOps := m.(NullOps)

	nrn := d.Get("nrn").(string)
	artifactType := d.Get("type").(string)

	var wantedMeta map[string]interface{}
	if err := json.Unmarshal([]byte(d.Get("meta").(string)), &wantedMeta); err != nil {
		return diag.FromErr(fmt.Errorf("error parsing meta JSON: %v", err))
	}

	artifacts, err := nullOps.ListPlatformArtifacts(nrn, artifactType)
	if err != nil {
		return diag.FromErr(err)
	}

	// Identity match: the artifact whose identity_meta is a subset of the
	// requested meta (e.g. matching url for git_repository).
	var matched []*PlatformArtifact
	for _, artifact := range artifacts {
		if metaMatches(artifact.IdentityMeta, wantedMeta) {
			matched = append(matched, artifact)
		}
	}
	if len(matched) == 0 {
		return diag.FromErr(fmt.Errorf("no %s artifact under %s matches meta %v", artifactType, nrn, wantedMeta))
	}
	if len(matched) > 1 {
		return diag.FromErr(fmt.Errorf("meta %v matches %d %s artifacts under %s; add identity fields to disambiguate", wantedMeta, len(matched), artifactType, nrn))
	}
	artifact := matched[0]

	revisions, err := nullOps.ListPlatformArtifactRevisions(artifact.ResourceID)
	if err != nil {
		return diag.FromErr(err)
	}

	// Revision match: newest revision whose meta carries every requested
	// field. When only identity fields were requested every revision
	// matches, so this resolves to the latest one.
	var revision *PlatformArtifactRevision
	for _, candidate := range revisions {
		if metaMatches(wantedMeta, candidate.Meta) {
			revision = candidate
			break
		}
	}
	if revision == nil {
		return diag.FromErr(fmt.Errorf("artifact %s has no revision matching meta %v", artifact.ResourceID, wantedMeta))
	}

	revisionMetaJSON, err := json.Marshal(revision.Meta)
	if err != nil {
		return diag.FromErr(fmt.Errorf("error serializing revision meta to JSON: %v", err))
	}

	d.SetId(revision.ResourceRevisionID)
	if err := d.Set("artifact_id", artifact.ResourceID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("revision_id", revision.ResourceRevisionID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("revision_meta", string(revisionMetaJSON)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("visible_to", artifact.VisibleTo); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("latest_revision_id", artifact.LatestRevisionID); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
