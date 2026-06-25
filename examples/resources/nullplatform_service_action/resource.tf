terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

variable "service_id" {
  description = "ID of the service the action is executed against"
  type        = string
}

variable "action_specification_id" {
  description = "ID of the action specification to execute"
  type        = string
}

# Resolve the target service and the action specification by ID
data "nullplatform_service" "target" {
  id = var.service_id
}

data "nullplatform_service_specification" "resize" {
  id = var.action_specification_id
}

# Trigger the action against the service. Any change to the inputs
# re-triggers the action, since all attributes are ForceNew.
resource "nullplatform_service_action" "resize_redis" {
  service_id       = data.nullplatform_service.target.id
  specification_id = data.nullplatform_service_specification.resize.id

  parameters = jsonencode({
    size = "large"
  })
}

output "action_status" {
  value = nullplatform_service_action.resize_redis.status
}
