terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
      version = "~> 0.0.96"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}
