terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_metadata_specification" "cost_center" {
  name        = "Cost Center"
  description = "Cost center attribution for the application"
  nrn         = "organization=1:account=2:namespace=3:application=123"

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
