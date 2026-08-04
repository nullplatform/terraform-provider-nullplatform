package nullplatform

import (
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
func actionSpecsToComputedList(nullOps NullOps, actions []*ActionSpecification) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(actions))
	for _, a := range actions {
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
