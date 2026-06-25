terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "parameter_name" {
  type        = string
  description = "Definition name of the parameter to look up."
  default     = "LOG_LEVEL"
}

variable "nrn" {
  type        = string
  description = "The NRN of the application to which the parameter belongs."
}

# Look up a parameter by its name and NRN
data "nullplatform_parameter_by_name" "example" {
  name = var.parameter_name
  nrn  = var.nrn
}

output "parameter_type" {
  value = data.nullplatform_parameter_by_name.example.type
}
