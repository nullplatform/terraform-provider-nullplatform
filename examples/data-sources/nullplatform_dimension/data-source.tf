terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {
}

data "nullplatform_dimension" "by_id" {
  nrn = "organization=1205600439:account=1016594569:namespace=1933968243"
  id = "1008402567"
}

data "nullplatform_dimension" "by_slug" {
  nrn = "organization=1205600439:account=1016594569:namespace=1933968243"
  slug = "environment"
}

output "by_id" {
  value = data.nullplatform_dimension.by_id
}

output "by_slug" {
  value = data.nullplatform_dimension.by_slug
}
