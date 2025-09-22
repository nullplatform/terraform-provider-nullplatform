terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
    }
  }
}
provider "nullplatform" {
}

# Resource: Service Specification
resource "nullplatform_service_specification" "redis_service_spec" {
  name           = "Redis Service Specification"
  type           = "dependency"
  assignable_to   = "any"        # Options: "any", "dimension", "scope"

  visible_to = [
    "organization=1255165411:account=*",
  ]

  use_default_actions = true

  scopes = jsonencode({
    provider = {
      values = [
        "AWS:SERVERLESS:LAMBDA",
        "AWS:WEB_POOL:EC2INSTANCES",
        "uuid-of-a-specific-scope-specification",
      ]
    }
  })

  dimensions = jsonencode({
    environment = {
      required = true
    },
    region = {
      required = false
    }
  })

  attributes = jsonencode({
    schema = {
      type = "object"
      required = ["endpoint", "port"]
      properties = {
        endpoint = {
          type      = "string"
          export    = true
          readOnly  = true
        }
        port = {
          type      = "number"
          export    = true
          readOnly  = true
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
