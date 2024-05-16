terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
      version = "~> 0.0.14"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}
