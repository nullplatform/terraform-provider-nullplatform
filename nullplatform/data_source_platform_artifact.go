package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourcePlatformArtifact() *schema.Resource {
	return &schema.Resource{
		Description: "Looks up a platform artifact (and one of its revisions) VISIBLE at the given " +
			"nrn — owned there, shared by an ancestor, or published globally with " +
			"\"organization=*\" — by its meta fields: e.g. a git_repository by { url, reference } " +
			"or an OCI image by { registry, repository, digest }. Identity fields select the " +
			"artifact; per-revision fields (digest, reference, tag) select a revision, otherwise " +
			"the latest is used. Selecting by tag resolves the NEWEST revision carrying that tag " +
			"and computes its digest — when the tag is re-registered against a new digest, the " +
			"data source drifts to the new revision by design. Read-only: never registers anything.",
		ReadContext: dataSourcePlatformArtifactRead,
		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The NRN to resolve visibility from: matches artifacts owned there, shared by ancestors, and globals (organization=*).",
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
					"digest, tag) additionally select a specific revision. A tag resolves to the " +
					"newest revision registered with it.",
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
			"digest": {
				Type:     schema.TypeString,
				Computed: true,
				Description: "The resolved revision's content digest (sha256:…), when its meta carries " +
					"one — the pinnable value a tag-based lookup resolves for you.",
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
	// The listing is visibility-scoped, so an artifact owned at the nrn can
	// coexist with a shared/global one of the same identity. The owned one
	// wins: it's what this configuration (or its owner) registered, and it
	// keeps configs written before globals existed resolving unchanged.
	if len(matched) > 1 {
		var owned []*PlatformArtifact
		for _, artifact := range matched {
			if artifact.Nrn == nrn {
				owned = append(owned, artifact)
			}
		}
		if len(owned) == 1 {
			matched = owned
		}
	}
	if len(matched) == 0 {
		return diag.FromErr(fmt.Errorf("no %s artifact visible at %s matches meta %v", artifactType, nrn, wantedMeta))
	}
	if len(matched) > 1 {
		return diag.FromErr(fmt.Errorf("meta %v matches %d %s artifacts visible at %s; add identity fields to disambiguate", wantedMeta, len(matched), artifactType, nrn))
	}
	artifact := matched[0]

	revisions, err := nullOps.ListPlatformArtifactRevisions(artifact.ResourceID)
	if err != nil {
		return diag.FromErr(err)
	}

	// Revision match: NEWEST revision whose meta carries every requested
	// field. Sorted here rather than trusting API order — the newest-wins
	// rule is what gives tag lookups their drift semantics: re-registering a
	// tag against a new digest mints a newer revision, and the next plan
	// resolves it (and its digest) instead of the stale one. When only
	// identity fields were requested every revision matches, so this
	// resolves to the latest one.
	sort.SliceStable(revisions, func(i, j int) bool {
		return revisions[i].CreatedAt > revisions[j].CreatedAt
	})
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
	digest, _ := revision.Meta["digest"].(string)
	if err := d.Set("digest", digest); err != nil {
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
