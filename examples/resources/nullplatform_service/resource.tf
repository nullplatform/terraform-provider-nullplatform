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
