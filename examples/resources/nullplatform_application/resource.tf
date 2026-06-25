terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# ID of the namespace that will own the application
variable "namespace_id" {
  type = number
}

resource "nullplatform_application" "this" {
  name           = "my-api"
  namespace_id   = var.namespace_id
  repository_url = "https://github.com/my-org/my-api"

  # Deploy the application as soon as it is created
  auto_deploy_on_creation = true

  # Application located in a subfolder of a monorepo
  is_mono_repo        = true
  repository_app_path = "services/my-api"

  # Free-form JSON for tags and application settings
  tags = jsonencode({
    team        = "platform"
    environment = "production"
  })

  settings = jsonencode({
    runtime = "nodejs"
  })
}
