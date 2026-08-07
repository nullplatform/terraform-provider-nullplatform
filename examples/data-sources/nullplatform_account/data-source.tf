terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

data "nullplatform_account" "example" {
  id = 123
}

output "account_slug" {
  value = data.nullplatform_account.example.slug
}

output "account_nrn" {
  value = data.nullplatform_account.example.nrn
}
