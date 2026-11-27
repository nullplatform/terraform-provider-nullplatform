terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_type_id" {
  description = "ID of the application type"
  type        = string
}

# Look up an existing application type by its ID
data "nullplatform_application_type" "agent" {
  id = var.application_type_id
}

# The snapshot new applications of the type are created on
output "application_type_snapshot" {
  value = data.nullplatform_application_type.agent.last_snapshot_id
}
