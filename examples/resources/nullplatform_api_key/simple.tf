terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_api_key" "my_api_key" {
  name = "Example API Key Name"

  grants {
    nrn        = "organization=1:account=1"
    role_slug  = "account:ops"
  }

  grants {
    nrn        = "organization=1:account=1"
    role_slug  = "account:admin"
  }

  tags {
    key = "example"
    value = "true"
  }
}

output "my_api_key_value" {
  value     = nullplatform_api_key.my_api_key.api_key
  sensitive = true
}

output "my_api_key_id" {
  value     = nullplatform_api_key.my_api_key.id
}