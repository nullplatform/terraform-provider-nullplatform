---
# generated by https://github.com/hashicorp/terraform-plugin-docs
page_title: "nullplatform_approval_action Resource - nullplatform"
subcategory: ""
description: |-
  The approval action resource allows you to configure a nullplatform action for the approval workflow
---

# nullplatform_approval_action (Resource)

The approval action resource allows you to configure a nullplatform action for the approval workflow



<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `action` (String) The action to which this action applies. Example: `deployment:create`
- `entity` (String) The entity to which this action applies. Example: `deployment`.
- `nrn` (String) The NRN of the resource (including children resources) where the action will apply.
- `on_policy_fail` (String) The action to be taken on policy failure. Possible values: [`manual`, `deny`]
- `on_policy_success` (String) The action to be taken on policy success. Possible values: [`approve`, `manual`]

### Optional

- `dimensions` (Map of String) A key-value map with the runtime configuration dimensions that apply to this scope.

### Read-Only

- `id` (String) The ID of this resource.