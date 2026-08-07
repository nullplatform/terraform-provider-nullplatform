---
page_title: "nullplatform_service Resource - nullplatform"
subcategory: ""
description: |-
  The service resource allows you to configure a Nullplatform Service
---

# nullplatform_service (Resource)

The service resource allows you to configure a Nullplatform Service

## Example Usage

```terraform
terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "null_application_id" {
  description = "Unique ID for the application"
  type        = number
}

variable "open_weather_api_key" {
  description = "API Key for consume Open Weather services"
}

variable "specification_id" {
  description = "Specification ID for the service to be imported"
  type        = string
}

data "nullplatform_application" "app" {
  id = var.null_application_id
}

resource "nullplatform_service" "redis_cache_test" {
  name             = "redis-cache"
  specification_id = "4a4f6955-5ae0-40dc-a1de-e15e5cf41abb"
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]
  dimensions       = {}
  attributes       = {}
}

data "nullplatform_service" "service" {
  id = nullplatform_service.redis_cache_test.id
}

resource "nullplatform_service" "open_weather_test" {
  name             = "open-weather"
  specification_id = var.specification_id
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]

  selectors {
    category     = "SaaS"
    imported     = true
    provider     = "OpenWeather"
    sub_category = "Weather"
  }

  attributes = {
    api_key = var.open_weather_api_key
  }
  dimensions = {}
}

output "redis" {
  value = nullplatform_service.redis_cache_test
}

output "open_weather" {
  value = nullplatform_service.open_weather_test
}

# Action-driven mode: provider triggers the spec's create+delete actions.
resource "nullplatform_service" "open_weather_provisioned" {
  name             = "open-weather-provisioned"
  specification_id = var.provisioned_specification_id
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]

  import = false

  timeouts {
    create = "10m"
    delete = "10m"
  }

  attributes = {
    api_key = var.open_weather_api_key
  }
  dimensions = {}
}

variable "provisioned_specification_id" {
  description = "Specification ID for a service whose create+delete actions should be triggered."
  type        = string
}

# Archive instead of delete: `terraform destroy` PATCHes the service to
# `archived` and waits for the transition, leaving the row, its attributes and
# its infrastructure in place. Terraform reads the flag from state at destroy
# time, so it has to be applied before the destroy runs.
resource "nullplatform_service" "open_weather_archivable" {
  name             = "open-weather-archivable"
  specification_id = var.provisioned_specification_id
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]

  import             = false
  archive_on_destroy = true

  timeouts {
    create = "10m"
    update = "10m"
    delete = "10m"
  }

  attributes = {
    api_key = var.open_weather_api_key
  }
  dimensions = {}
}

# Archiving on demand. Leave `status` out of the configuration — as every
# example above does — to let Terraform track whatever the platform reports; a
# service archived outside Terraform then stays archived instead of being
# restored by the next unrelated apply. Set `status` explicitly only when the
# apply is meant to archive (`archived`) or restore (`active`) an existing
# service; a service cannot be *created* archived. When the specification runs
# archive/unarchive as managed actions, the apply waits for the transition, so
# give the resource an `update` timeout.
#
#   resource "nullplatform_service" "open_weather_test" {
#     # ...
#     status = "archived"
#
#     timeouts {
#       update = "10m"
#     }
#   }

output "open_weather_archived_at" {
  value = nullplatform_service.open_weather_test.archived_at
}
```

## Destroy behaviour

`terraform destroy` picks one of these paths. The flags are read **from state**, so
an `apply` that sets them has to run before the destroy that relies on them.

| `force_destroy` | `archive_on_destroy` | `import` | Live status | What destroy does |
|---|---|---|---|---|
| `true` | any | any | any | Hard delete (`force=true`). The escape hatch outranks everything. |
| `false` | `true` | any | `archived` | Nothing — drops the resource from state, the row stays archived. |
| `false` | `true` | any | `archiving` | Waits for the running archive to land, then drops it from state. |
| `false` | `true` | any | anything else | `PATCH {"status":"archived"}`, waits for the transition, drops it from state. |
| `false` | `false` | `true` | any | Hard delete (`force=true`) — an imported service has no delete action to run. |
| `false` | `false` | `false` | `failed` | Hard delete (`force=true`) — the delete action cannot run on a failed service. |
| `false` | `false` | `false` | `archiving` | Refuses with an actionable error: wait for `archived`, or use `force_destroy`. |
| `false` | `false` | `false` | anything else | Triggers the specification's `delete` action and waits for it. |

~> **Archiving a service archives its links first.** The API refuses to archive a
service that still has non-archived links. Terraform destroys links before the
service they belong to, so set `archive_on_destroy = true` on the
`nullplatform_link` resources too — otherwise the destroy hard-deletes the links
and leaves a restorable service with nothing attached to it.

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `entity_nrn` (String) NRN representing a hierarchical identifier for nullplatform resourcesValue must match regular expression `^organization=[0-9]+(:account=[0-9]+)?(:namespace=[0-9]+)?(:application=[0-9]+)?(:scope=[0-9]+)?$`.
- `name` (String) Name of the entity. Must be a non-empty string and not equal to null.
- `specification_id` (String) Unique identifier for the entity represented as a UUID.

### Optional

- `archive_on_destroy` (Boolean) When true, `terraform destroy` archives the service (`PATCH {"status": "archived"}`) and waits for the transition to finish instead of deleting it. The service row, its attributes and its infrastructure survive and can be restored later by setting `status = "active"` on a re-imported resource. Note: Terraform's destroy reads this attribute from state, so you must run `terraform apply` with `archive_on_destroy = true` *before* running `terraform destroy` for it to take effect. For tainted resources, run `terraform untaint` first so the apply is an update rather than a replace. `force_destroy = true` wins over this flag: an escape hatch is asked for when the record must be gone.
- `attributes` (Map of String) Attributes associated with the service, should be valid against the service specification attribute schema.
- `desired_specification_id` (String) Desired unique identifier for the associated specification.
- `dimensions` (Map of String) Object representing dimensions with key-value pairs.
- `force_destroy` (Boolean) Only meaningful when `import = false`. When true, `terraform destroy` skips the delete action and removes the service record directly via `DELETE /service/{id}?force=true`. Use this as an escape hatch when the service is stuck (e.g. the create action failed). Note: Terraform's destroy reads this attribute from state, so you must run `terraform apply` with `force_destroy = true` *before* running `terraform destroy` for it to take effect. For tainted resources, run `terraform untaint` first so the apply is an update rather than a replace. Has no effect when `import = true`, where destroy already uses force.
- `import` (Boolean) When true (default), provisioning and decommissioning of the underlying infrastructure are handled externally to nullplatform. When false, the specification's create and delete actions are triggered to handle the infrastructure lifecycle.
- `linkable_to` (List of String) A list of NRN representing the visibility settings for the entity. Specifies what/who can see this entity. Value must match regular expression `^organization=[0-9]+(:account=[0-9]+)?(:namespace=[0-9]+)?(:application=[0-9]+)?(:scope=[0-9]+)?$`.
- `selectors` (Block List, Max: 1) Selectors for the service specification (see [below for nested schema](#nestedblock--selectors))
- `status` (String) Status of the service. Should be one of: [`pending_create`, `pending`, `creating`, `updating`, `deleting`, `archiving`, `active`, `archived`, `deleted`, `failed`, `cancelled`]. Defaults to `active` when the configuration omits it on create (`pending` when `import = false`, so the specification's create action drives the transition). Leave it unset to track the platform's value: Terraform then never plans a status change on its own. Setting `archived` archives the service and setting `active` on an archived service restores it; when the specification runs archive/unarchive as managed actions, the apply waits for the transition to land.
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))

### Read-Only

- `archived_at` (String) Timestamp of the last time the service was archived. Empty while the service is not archived.
- `id` (String) The ID of this resource.
- `messages` (List of Map of String) A message and its severity level

<a id="nestedblock--selectors"></a>
### Nested Schema for `selectors`

Required:

- `category` (String) Category of the service specification
- `imported` (Boolean) Indicates whether the service is imported
- `provider` (String) Provider of the service (e.g., AWS, GCP)
- `sub_category` (String) Sub-category of the service


<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `create` (String)
- `delete` (String)
- `update` (String)


