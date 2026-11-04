terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# Every account of the provider credentials' organization
data "nullplatform_accounts" "all" {}

# Every account of an explicit organization
data "nullplatform_accounts" "other_org" {
  organization_id = 1234567890
}

output "account_slugs" {
  value = [for acc in data.nullplatform_accounts.all.accounts : acc.slug]
}
