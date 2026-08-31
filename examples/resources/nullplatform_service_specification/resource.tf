terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}
provider "nullplatform" {
}

# Resource: Service Specification
resource "nullplatform_service_specification" "redis_service_spec" {
  name          = "Redis Service Specification"
  type          = "dependency"
  assignable_to = "any" # Options: "any", "dimension", "scope"

  visible_to = [
    "organization=1255165411:account=*",
  ]

  # The behaviour flags. All three are optional: leave one out and the
  # specification tracks the platform's default rather than a provider guess.
  #
  #   use_default_actions  default true   platform generates and owns the CRUD actions
  #   use_default_naming   default true   instance names come from the schema
  #   use_managed_actions  default false  CRUD verbs run *through* those actions
  #
  # Recommended for a new specification: take the two defaults and opt into
  # managed actions, so a PATCH mints `update`, a DELETE mints `delete`, and
  # `status = "archived"` mints `archive` instead of writing the row directly.
  use_managed_actions = true

  # Only set these to depart from the platform default — for example
  # `use_default_actions = false` to author every action yourself with
  # nullplatform_action_specification (which also rules out managed actions).

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
