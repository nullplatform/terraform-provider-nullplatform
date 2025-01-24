terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

locals {
  grants = [
    {
      nrn = "organization=1:account=1"
      role_slug = "account:admin"
    },
    {
      nrn = "organization=1:account=1"
      role_slug = "account:ops"
    },
    {
      nrn = "organization=1:account=1"
      role_slug = "account:developer"
    }
  ]

  tags = [
    {
      key   = "ownership"
      value = "fintech"
    },
    {
      key   = "terraform"
      value = "true"
    }
  ]
}

resource "nullplatform_api_key" "my_api_key" {
  name = "Example API Key Name"

  dynamic "grants" {
    for_each = local.grants
    content {
      nrn       = grants.value.nrn
      role_slug = grants.value.role_slug
    }
  }

  dynamic "tags" {
    for_each = local.tags
    content {
      key   = tags.value.key
      value = tags.value.value
    }
  }
}

output "my_api_key_value" {
  value     = nullplatform_api_key.my_api_key.api_key
  sensitive = true
}

output "my_api_key_id" {
  value     = nullplatform_api_key.my_api_key.id
}
