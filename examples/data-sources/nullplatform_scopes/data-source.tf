terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# The active scopes of an application (the API also lists deleted scopes)
data "nullplatform_scopes" "active" {
  application_id = 1234567890
  status         = "active"
}

output "production_scopes" {
  value = [for scope in data.nullplatform_scopes.active.scopes : scope.slug if lookup(scope.dimensions, "environment", "") == "production"]
}
