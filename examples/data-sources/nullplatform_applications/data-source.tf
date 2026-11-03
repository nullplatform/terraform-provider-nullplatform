terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# The applications of one namespace
data "nullplatform_applications" "main" {
  namespace_id = 1234567890
}

# Every application of an account: one listing per namespace
data "nullplatform_namespaces" "all" {
  account_id = 1234567890
}

data "nullplatform_applications" "by_namespace" {
  for_each     = { for ns in data.nullplatform_namespaces.all.namespaces : ns.slug => ns.id }
  namespace_id = each.value
}

output "application_nrns" {
  value = flatten([for apps in data.nullplatform_applications.by_namespace : [for app in apps.applications : app.nrn]])
}
