terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "namespace_nrn" {
  description = "NRN of the namespace that scopes the dimension lookup."
  type        = string
}

variable "dimension_slug" {
  description = "Slug of the dimension to look up (e.g. \"environment\")."
  type        = string
}

# Look up a dimension by its slug within the given namespace NRN
data "nullplatform_dimension" "example" {
  nrn  = var.namespace_nrn
  slug = var.dimension_slug
}

output "dimension_values" {
  value = data.nullplatform_dimension.example.values
}
