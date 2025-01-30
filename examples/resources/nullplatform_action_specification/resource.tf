
terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
    }
  }
}
provider "nullplatform" {}

resource "nullplatform_action_specification" "create_redis_action" {
  name                   = "Create Redis Instance"
  type                   = "create" # Options: "custom", "create", "update", "delete"
  service_specification_id = "your-service-spec-id"
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

resource "nullplatform_action_specification" "update_redis_action" {
  name                   = "Update Redis Instance"
  type                   = "update"
  service_specification_id = "your-service-spec-id"
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

resource "nullplatform_action_specification" "delete_redis_action" {
  name                   = "Delete Redis Instance"
  type                   = "delete"
  service_specification_id = "your-service-spec-id"
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
