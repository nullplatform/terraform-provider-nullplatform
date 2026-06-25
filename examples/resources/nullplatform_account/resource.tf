terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

resource "nullplatform_account" "main" {
  name = "My Account"
  # Unique, URL-safe identifier for the account
  slug = "my-account"

  # Repository configuration used when scaffolding application repos
  repository_prefix   = "my-org"
  repository_provider = "github"

  # Account settings as a JSON string
  settings = jsonencode({
    notification_channels = ["email"]
  })
}

output "account_id" {
  description = "The ID of the account"
  value       = nullplatform_account.main.id
}

output "account_nrn" {
  description = "The Nullplatform Resource Name (NRN) of the account"
  value       = nullplatform_account.main.nrn
}
