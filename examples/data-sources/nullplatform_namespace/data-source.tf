terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

data "nullplatform_namespace" "example" {
  id = 456
}

output "namespace_slug" {
  value = data.nullplatform_namespace.example.slug
}

output "namespace_nrn" {
  value = data.nullplatform_namespace.example.nrn
}
