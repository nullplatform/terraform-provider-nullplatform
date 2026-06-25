terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

variable "scope_id" {
  description = "ID of the scope to look up."
  type        = string
}

# Look up an existing scope by its ID.
data "nullplatform_scope" "example" {
  id = var.scope_id
}

output "scope_nrn" {
  value = data.nullplatform_scope.example.nrn
}
