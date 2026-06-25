terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# Account that owns the template; omit to create a global template
variable "account_id" {
  type    = string
  default = null
}

resource "nullplatform_technology_template" "golang" {
  name    = "Golang 1.21"
  url     = "https://github.com/nullplatform/technology-templates-golang"
  account = var.account_id

  # Provider-specific settings used when scaffolding the repository
  provider_config = {
    repository = "technology-templates-golang"
  }

  # Building blocks that make up the template
  components {
    type    = "language"
    id      = "golang"
    version = "1.21"
    metadata = jsonencode({
      version = "1.21.5"
    })
  }

  tags = [
    "golang",
    "backend",
  ]

  metadata = jsonencode({
    maintainer = "platform-team"
  })

  rules = jsonencode({})
}
