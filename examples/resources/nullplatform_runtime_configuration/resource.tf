terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# Runtime configuration applied to the development environment dimension.
# Changing dimensions forces a new resource.
resource "nullplatform_runtime_configuration" "dev" {
  nrn = "organization=1:account=2:namespace=3:application=123"

  dimensions = {
    environment = "dev"
  }

  # Settings that make up this runtime configuration
  values = {
    cpu    = "0.5"
    memory = "512"
  }
}
