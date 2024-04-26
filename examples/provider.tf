terraform {
  required_providers {
    nullplatform = {
      #version = "~> 0.0.13"
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}
