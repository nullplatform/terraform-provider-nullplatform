package nullplatform

import (
	"context"
	"sort"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// actionSpecificationsComputedSchema is the computed `action_specifications`
// attribute shared by the service_specification and link_specification
// resources. When a spec uses default actions, the platform creates them
// server-side (they are not Terraform resources), so this exposes them for
// pinning into a package bill-of-materials without a separate data source.
//
// For each entry, a package component pins:
//
//	resource_id          = id
//	resource_revision_id = last_snapshot_id
//	parent_id            = the owning spec's id
func actionSpecificationsComputedSchema() *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeList,
		Computed: true,
		Description: "Default-created action specifications for this spec (populated when use_default_actions is true). " +
			"Pin each into a package BOM component: resource_id = id, resource_revision_id = last_snapshot_id, parent_id = this spec's id.",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"id":               {Type: schema.TypeString, Computed: true},
				"name":             {Type: schema.TypeString, Computed: true},
				"slug":             {Type: schema.TypeString, Computed: true},
				"last_snapshot_id": {Type: schema.TypeString, Computed: true},
			},
		},
	}
}

// actionSpecsToComputedList shapes a spec's action specifications into the
// `action_specifications` computed list, resolving each action's newest
// snapshot id (best-effort — a missing snapshot yields an empty string rather
// than failing the read).
//
// The list is sorted by slug so its order is STABLE across reads and applies.
// A slug (create-/update-/delete-<spec>) is a stable identity even when the
// underlying action is re-created with a new id on a spec update, so a package
// BOM that pins these components as an ordered list keeps a consistent order
// and Terraform's positional tracking never sees a component "move".
func actionSpecsToComputedList(nullOps NullOps, actions []*ActionSpecification) []map[string]interface{} {
	sorted := make([]*ActionSpecification, len(actions))
	copy(sorted, actions)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Slug < sorted[j].Slug })

	out := make([]map[string]interface{}, 0, len(sorted))
	for _, a := range sorted {
		snapshotID, _ := nullOps.GetLatestSnapshotID("action_specification", a.Id)
		out = append(out, map[string]interface{}{
			"id":               a.Id,
			"name":             a.Name,
			"slug":             a.Slug,
			"last_snapshot_id": snapshotID,
		})
	}
	return out
}

// specBOMCustomizeDiff forces the computed BOM attributes (last_snapshot_id and
// action_specifications) to recompute whenever any of the given content fields
// change on an existing spec.
//
// A spec update mints a NEW snapshot and re-creates its default actions with new
// ids. Without this, the plan keeps the OLD (known) snapshot/action values while
// the apply produces the new ones, so any package that pins them into its
// components list fails at apply with "Provider produced inconsistent final
// plan". Marking these unknown at plan lets apply resolve the new values
// consistently. On create (empty Id) everything is already unknown, so this is a
// no-op there and on plans that don't touch the spec's content.
func specBOMCustomizeDiff(contentFields ...string) schema.CustomizeDiffFunc {
	return func(ctx context.Context, d *schema.ResourceDiff, m interface{}) error {
		if d.Id() == "" {
			return nil
		}
		changed := false
		for _, f := range contentFields {
			if d.HasChange(f) {
				changed = true
				break
			}
		}
		if !changed {
			return nil
		}
		if err := d.SetNewComputed("last_snapshot_id"); err != nil {
			return err
		}
		return d.SetNewComputed("action_specifications")
	}
}
