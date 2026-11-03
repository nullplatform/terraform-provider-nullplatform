terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# The provider configurations defined exactly at an account
data "nullplatform_provider_configs" "account" {
  nrn = "organization=1234567890:account=987654321"
}

output "provider_config_ids" {
  value = [for config in data.nullplatform_provider_configs.account.provider_configs : config.id]
}
