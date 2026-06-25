terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "service_id" {
  description = "ID of the service the action is executed against"
  type        = string
}

variable "action_specification_id" {
  description = "ID of the action specification to execute"
  type        = string
}

# Triggers an action defined by an action specification against an existing
# service. Any change to the inputs re-triggers the action (ForceNew).
resource "nullplatform_service_action" "resize_redis" {
  service_id       = var.service_id
  specification_id = var.action_specification_id

  parameters = jsonencode({
    size = "large"
  })
}

output "action_status" {
  value = nullplatform_service_action.resize_redis.status
}

output "action_results" {
  value = nullplatform_service_action.resize_redis.results
}
