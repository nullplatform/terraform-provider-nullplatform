terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_id" {
  description = "ID of the application the link is scoped to."
  type        = number
}

variable "service_id" {
  description = "UUID of the service being linked."
  type        = string
}

variable "link_specification_id" {
  description = "UUID of the link specification this link implements."
  type        = string
}

data "nullplatform_application" "app" {
  id = var.application_id
}

# A link connects a service to an entity (here an application) through a
# link specification.
resource "nullplatform_link" "redis" {
  name             = "redis-cache"
  service_id       = var.service_id
  specification_id = var.link_specification_id
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]

  dimensions = {
    environment = "production"
  }
}
