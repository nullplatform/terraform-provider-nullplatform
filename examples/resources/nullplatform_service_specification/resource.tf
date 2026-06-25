terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "null_application_id" {
  description = "Unique ID for the application the specification is scoped to"
  type        = number
}

# Resolve the application NRN to control specification visibility
data "nullplatform_application" "app" {
  id = var.null_application_id
}

resource "nullplatform_service_specification" "redis" {
  name        = "Redis Service Specification"
  description = "Managed Redis cache offered as a dependency"
  type        = "dependency"

  # NRNs this specification is visible to
  visible_to = [
    data.nullplatform_application.app.nrn,
  ]

  # Where this service can be attached: "any", "dimension" or "scope"
  assignable_to       = "any"
  use_default_actions = true

  # Scope specifications this service can run on
  scopes = jsonencode({
    provider = {
      values = [
        "AWS:SERVERLESS:LAMBDA",
        "AWS:WEB_POOL:EC2INSTANCES",
      ]
    }
  })

  dimensions = jsonencode({
    environment = {
      required = true
    }
  })

  # JSON Schema describing the service attributes and their values
  attributes = jsonencode({
    schema = {
      type     = "object"
      required = ["endpoint", "port"]
      properties = {
        endpoint = {
          type     = "string"
          export   = true
          readOnly = true
        }
        port = {
          type     = "number"
          export   = true
          readOnly = true
        }
      }
      additionalProperties = false
    }
    values = {}
  })

  selectors {
    category     = "Database Services"
    imported     = true
    provider     = "AWS"
    sub_category = "In-memory Database"
  }
}
