terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
      version = "~> 0.0.98"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}
