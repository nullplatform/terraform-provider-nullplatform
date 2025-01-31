terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# Example Technology Template - Golang 1.17.9
resource "nullplatform_technology_template" "golang_1_17" {
  name = "Golang 1.17.9"
  url  = "https://github.com/nullplatform/technology-templates-golang"

  provider_config = {
    repository = "technology-templates-golang"
  }

  components {
    type    = "language"
    id      = "google"
    version = "1.17"
    metadata = jsonencode({
      "version": "1.17.9"
    })
  }

  tags = [
    "golang",
    "backend"
  ]

  metadata = jsonencode({})
  rules    = jsonencode({})
}