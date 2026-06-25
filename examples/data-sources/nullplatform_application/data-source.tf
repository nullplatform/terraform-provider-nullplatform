terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_id" {
  type        = number
  description = "The system-wide unique ID of the application to look up."
}

# Look up an existing application by its ID
data "nullplatform_application" "example" {
  id = var.application_id
}

output "application_nrn" {
  value = data.nullplatform_application.example.nrn
}
