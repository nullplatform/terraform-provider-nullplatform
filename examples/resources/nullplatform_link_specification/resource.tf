terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_id" {
  description = "ID of the application that can see this link specification."
  type        = number
}

variable "service_specification_id" {
  description = "UUID of the service specification this link specification is associated with."
  type        = string
}

data "nullplatform_application" "app" {
  id = var.application_id
}

# A link specification defines how a service can be linked to an entity.
resource "nullplatform_link_specification" "redis" {
  name             = "Redis Link Specification"
  unique           = false
  specification_id = var.service_specification_id
  assignable_to    = "any"

  visible_to = [
    data.nullplatform_application.app.nrn,
  ]

  use_default_actions = true

  # Scope specifications this link can run on
  scopes = jsonencode({
    provider = {
      values = ["AWS:SERVERLESS:LAMBDA", "AWS:WEB_POOL:EC2INSTANCES"]
    }
  })

  # JSON Schema for the link attributes
  attributes = jsonencode({
    schema = {
      type       = "object"
      properties = {}
    }
    values = {}
  })

  selectors {
    category     = "Integration Services"
    imported     = true
    provider     = "AWS"
    sub_category = "In-memory Database"
  }
}
