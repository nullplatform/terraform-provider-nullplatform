terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}
provider "nullplatform" {}

resource "nullplatform_link_specification" "redis_link_spec" {
  name             = "Redis Link Specification"
  unique           = false
  specification_id = nullplatform_service_specification.redis_service_spec.id
  assignable_to    = "any"

  visible_to = [
    "organization=1255165411:account=*",
  ]

  # See the service specification example for the three behaviour flags. New
  # specifications should take the platform defaults (`use_default_actions` and
  # `use_default_naming` are both true) and opt into managed CRUD:
  use_managed_actions = true

  scopes = jsonencode({
    provider = {
      values = [
        "AWS:SERVERLESS:LAMBDA",
        "AWS:WEB_POOL:EC2INSTANCES",
        "uuid-of-a-specific-scope-specification",
      ]
    }
  })

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
