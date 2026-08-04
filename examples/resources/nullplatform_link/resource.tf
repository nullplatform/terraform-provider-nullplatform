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

data "nullplatform_service" "redis" {
  id = nullplatform_service.redis_cache_test.id
}

resource "nullplatform_link" "link_redis" {
  name             = "link_from_terraform_2"
  service_id       = data.nullplatform_service.redis.id
  specification_id = "66919464-05e6-4d78-bb8c-902c57881ddd"
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]
  dimensions = {
    environment = "development",
    country     = "argentina",
  }
  attributes = {}
}

output "link" {
  value = nullplatform_link.link_redis
}

# Archive instead of delete. Set this on the links of a service that also uses
# `archive_on_destroy`: the API refuses to archive a service while any of its
# links is not archived, and Terraform destroys links before the service they
# belong to — so without this the destroy hard-deletes the links and leaves a
# restorable service with nothing attached.
#
#   resource "nullplatform_link" "link_redis" {
#     # ...
#     archive_on_destroy = true
#
#     timeouts {
#       update = "10m"
#       delete = "10m"
#     }
#   }

output "link_archived_at" {
  value = nullplatform_link.link_redis.archived_at
}
