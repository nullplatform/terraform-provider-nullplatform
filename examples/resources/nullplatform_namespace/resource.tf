terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_namespace" "finance" {
  name                = "Finances"
  account_id          = "43591328"
}

resource "nullplatform_namespace" "public_sites" {
  name                = "Public Site"
  account_id          = "43591328"
}

output "finance_namespace_id" {
  description = "The ID of the Finance namespace"
  value       = nullplatform_namespace.finance.id
}

output "public_sites_namespace_id" {
  description = "The ID of the Public Sites namespace"
  value       = nullplatform_namespace.public_sites.id
}