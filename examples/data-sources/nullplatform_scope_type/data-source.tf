terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "scope_type_id" {
  type        = string
  description = "The ID of the scope type to look up"
  default     = "1000001"
}

# Look up an existing scope type by its ID
data "nullplatform_scope_type" "example" {
  id = var.scope_type_id
}

# Expose the provider type that implements the scope type
output "scope_type_provider_type" {
  value = data.nullplatform_scope_type.example.provider_type
}
