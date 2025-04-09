terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
    }
  }
}
provider "nullplatform" {}

resource "nullplatform_link_specification" "redis_link_spec" {
  name               = "Redis Link Specification"
  unique             = false
  specification_id   = nullplatform_service_specification.redis_service_spec.id
  assignable_to      = "any"

  visible_to = [
    "organization=1255165411:account=*",
  ]

  use_default_actions = true

  dimensions = jsonencode({}) # No specific dimensions

  attributes = jsonencode({
    schema = {}
    values = {}
  })

  selectors {
    category     = "Integration Services"
    imported     = true
    provider     = "GCP"
    sub_category = "In-memory Database Integration"
  }
}
