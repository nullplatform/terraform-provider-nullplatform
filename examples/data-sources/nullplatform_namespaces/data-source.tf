terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# The namespaces of one account
data "nullplatform_namespaces" "main" {
  account_id = 1234567890
}

# Every namespace of the organization: the API lists namespaces one account
# at a time, so list the accounts and read the namespaces of each.
data "nullplatform_accounts" "all" {}

data "nullplatform_namespaces" "by_account" {
  for_each   = { for acc in data.nullplatform_accounts.all.accounts : acc.slug => acc.id }
  account_id = each.value
}

locals {
  namespaces = flatten([for ns in data.nullplatform_namespaces.by_account : ns.namespaces])

  namespaces_by_account = {
    for slug, ns in data.nullplatform_namespaces.by_account : slug => [for n in ns.namespaces : n.slug]
  }
}

# The NRN of every namespace, e.g. to configure providers in bulk with
# `for_each = toset(local.namespace_nrns)` on nullplatform_provider_config.
output "namespace_nrns" {
  value = [for ns in local.namespaces : ns.nrn]
}

output "namespaces_by_account" {
  value = local.namespaces_by_account
}
