terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# Look up an existing scope type by its ID
data "nullplatform_scope_type" "example" {
  id = "1000001"
}

# Expose the provider type that implements the scope type
output "scope_type_provider_type" {
  value = data.nullplatform_scope_type.example.provider_type
}
