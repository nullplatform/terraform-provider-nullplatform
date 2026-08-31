
terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}
provider "nullplatform" {}

resource "nullplatform_action_specification" "create_redis_action" {
  name                     = "Create Redis Instance"
  type                     = "create" # Options: "custom", "create", "update", "delete"
  service_specification_id = "your-service-spec-id"
  retryable                = false

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
          type     = "string"
          config   = "aws.vpcId"
          readOnly = true
        }
      }
      required             = ["size"]
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
        redis_arn      = { type = "string" }
        redis_endpoint = { type = "string", target = "endpoint" }
        redis_port     = { type = "number", target = "port" }
      }
      additionalProperties = false
    }
    values = {}
  })
}

resource "nullplatform_action_specification" "update_redis_action" {
  name                     = "Update Redis Instance"
  type                     = "update"
  service_specification_id = "your-service-spec-id"
  retryable                = true

  parameters = jsonencode({
    schema = {
      type = "object"
      properties = {
        size = {
          type = "string"
          enum = ["small", "medium", "large"]
        }
      }
      required             = ["size"]
      additionalProperties = false
    }
    values = {}
  })

  results = jsonencode({
    schema = {
      type = "object"
      properties = {
        redis_arn      = { type = "string" }
        redis_endpoint = { type = "string", target = "endpoint" }
        redis_port     = { type = "number", target = "port" }
      }
      additionalProperties = false
    }
    values = {}
  })
}

resource "nullplatform_action_specification" "delete_redis_action" {
  name                     = "Delete Redis Instance"
  type                     = "delete"
  service_specification_id = "your-service-spec-id"
  retryable                = true

  parameters = jsonencode({
    schema = {
      type                 = "object"
      properties           = {}
      additionalProperties = false
    }
    values = {}
  })

  results = jsonencode({
    schema = {
      type                 = "object"
      properties           = {}
      additionalProperties = false
    }
    values = {}
  })
}

# Archive opt-in: what makes `nullplatform_service.status = "archived"` run as a
# managed action instead of a direct status flip.
#
# Which of the two you declare depends on where the specification came from:
#
#   * Created with `use_default_actions` (the usual case) — the platform already
#     generated create/update/delete AND `archive`. Declaring `archive` is
#     refused ("There is already an action of type archive..."); `terraform
#     import` the generated row if you want it in state. Only `unarchive` is
#     yours to create, and creating it is what enables managed restores.
#   * Predates the archive feature, or `use_default_actions = false` — neither is
#     generated, so declare both.
#
# Their content is platform-generated from the specification's attributes
# schema, so `parameters` and `results` must be omitted — sending either is
# refused. Only `name` is yours, and it survives regeneration. Deleting these
# resources is the opt-out: archive falls back to the direct status flip.
resource "nullplatform_action_specification" "unarchive_redis_action" {
  name                     = "Restore Redis Instance"
  type                     = "unarchive"
  service_specification_id = "your-service-spec-id"
}

# Only for a specification that has no generated archive action. On a
# `use_default_actions` specification, import the generated one instead:
#   terraform import nullplatform_action_specification.archive_redis_action <id>
#
#   resource "nullplatform_action_specification" "archive_redis_action" {
#     name                     = "Archive Redis Instance"
#     type                     = "archive"
#     service_specification_id = "your-service-spec-id"
#   }
