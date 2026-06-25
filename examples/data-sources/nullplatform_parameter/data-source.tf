terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "parameter_id" {
  description = "The system-wide unique ID of the parameter to look up."
  type        = number
}

# Look up an existing parameter by its unique ID
data "nullplatform_parameter" "example" {
  id = var.parameter_id
}

output "parameter_name" {
  value = data.nullplatform_parameter.example.name
}
