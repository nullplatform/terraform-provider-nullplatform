terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# The account that will own the namespace
variable "account_id" {
  type        = number
  description = "ID of the nullplatform account that owns this namespace"
}

resource "nullplatform_namespace" "finance" {
  name       = "Finance"
  account_id = var.account_id

  # Optional account-wide unique slug (defaults to a value derived from the name)
  slug = "finance"
}

output "namespace_id" {
  description = "The ID of the namespace"
  value       = nullplatform_namespace.finance.id
}

output "namespace_nrn" {
  description = "The NRN of the namespace"
  value       = nullplatform_namespace.finance.nrn
}
