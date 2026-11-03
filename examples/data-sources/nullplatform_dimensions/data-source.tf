terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# The dimensions visible at an account, with their values
data "nullplatform_dimensions" "account" {
  nrn = "organization=1234567890:account=987654321"
}

output "environments" {
  value = flatten([
    for dimension in data.nullplatform_dimensions.account.dimensions :
    [for value in dimension.values : value.slug] if dimension.slug == "environment"
  ])
}
