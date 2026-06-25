terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

variable "application_id" {
  type        = number
  description = "ID of the application whose NRN scopes the metadata specification"
}

# Resolve the NRN from the application instead of hardcoding it
data "nullplatform_application" "this" {
  id = var.application_id
}

resource "nullplatform_metadata_specification" "cost_center" {
  name        = "Cost Center"
  description = "Cost center attribution for the application"
  nrn         = data.nullplatform_application.this.nrn

  # Entity this metadata is attached to and the metadata key
  entity   = "application"
  metadata = "cost_center"

  # JSON Schema describing the accepted metadata values
  schema = jsonencode({
    type = "object"
    properties = {
      code = {
        type        = "string"
        description = "Internal cost center code"
      }
      owner = {
        type = "string"
      }
    }
    required             = ["code"]
    additionalProperties = false
  })
}
