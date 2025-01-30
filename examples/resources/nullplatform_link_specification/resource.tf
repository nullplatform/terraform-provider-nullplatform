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

# Resource: Action Specification
resource "nullplatform_action_specification" "create_redis_action" {
  name                   = "Create Redis Instance"
  type                   = "create" # Options: "custom", "create", "update", "delete"
  service_specification_id = nullplatform_service_specification.redis_service_spec.id
  retryable              = false

  parameters = jsonencode({
    schema = {
      type = "object"
      properties = {
        size = {
          type    = "string"
          enum    = ["small", "medium", "large"]
          default = "small"
        }
        vpc_id = {
          type      = "string"
          config    = "aws.vpcId"
          readOnly  = true
        }
      }
      required = ["size"]
      additionalProperties = false
    }
    values = {
      size = "medium"
    }
  })

  results = jsonencode({
    schema = {
      type = "object"
      properties = {
        redis_arn       = { type = "string" }
        redis_endpoint  = { type = "string", target = "endpoint" }
        redis_port      = { type = "number", target = "port" }
      }
      additionalProperties = false
    }
    values = {}
  })
}

# Resource: Action Specification for Updating Redis
resource "nullplatform_action_specification" "update_redis_action" {
  name                   = "Update Redis Instance"
  type                   = "update"
  service_specification_id = nullplatform_service_specification.redis_service_spec.id
  retryable              = true

  parameters = jsonencode({
    schema = {
      type = "object"
      properties = {
        size = {
          type    = "string"
          enum    = ["small", "medium", "large"]
        }
      }
      required = ["size"]
      additionalProperties = false
    }
    values = {}
  })

  results = jsonencode({
    schema = {
      type = "object"
      properties = {
        redis_arn       = { type = "string" }
        redis_endpoint  = { type = "string", target = "endpoint" }
        redis_port      = { type = "number", target = "port" }
      }
      additionalProperties = false
    }
    values = {}
  })
}

# Resource: Action Specification for Deleting Redis
resource "nullplatform_action_specification" "delete_redis_action" {
  name                   = "Delete Redis Instance"
  type                   = "delete"
  service_specification_id = nullplatform_service_specification.redis_service_spec.id
  retryable              = true

  parameters = jsonencode({
    schema = {
      type = "object"
      properties = {}
      additionalProperties = false
    }
    values = {}
  })

  results = jsonencode({
    schema = {
      type = "object"
      properties = {}
      additionalProperties = false
    }
    values = {}
  })
}

# Resource: Link between Redis Service and Application
resource "nullplatform_link" "redis_link" {
  name             = "Redis Application Link"
  service_id       = nullplatform_service_specification.redis_service_spec.id
  specification_id = nullplatform_link_specification.redis_link_spec.id
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]

  dimensions = jsonencode({
    environment = "development"
    country     = "argentina"
  })

  attributes = jsonencode({
    schema = {}
    values = {}
  })
}

# Output the Redis Service Specification details
output "redis_service_spec" {
  description = "Details of the Redis Service Specification"
  value       = nullplatform_service_specification.redis_service_spec
}

# Output the Redis Link Specification details
output "redis_link_spec" {
  description = "Details of the Redis Link Specification"
  value       = nullplatform_link_specification.redis_link_spec
}

# Output the Create Redis Action Specification details
output "create_redis_action_spec" {
  description = "Details of the Create Redis Action Specification"
  value       = nullplatform_action_specification.create_redis_action
}

# Output the Redis Link details
output "redis_link" {
  description = "Details of the Redis Link"
  value       = nullplatform_link.redis_link
}
