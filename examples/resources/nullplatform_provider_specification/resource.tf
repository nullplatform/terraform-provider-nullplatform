terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {
  # Set via environment variable: export NULLPLATFORM_API_KEY="your-api-key"
  # Set via environment variable: export NULLPLATFORM_HOST="api.nullplatform.com"
}

# Example: minimal provider specification (only required fields)
resource "nullplatform_provider_specification" "minimal" {
  name = "My Custom Provider Spec"

  visible_to = [
    "organization=*",
  ]

  schema = jsonencode({
    type     = "object"
    required = ["region"]
    properties = {
      region = {
        type        = "string"
        description = "Cloud region"
      }
    }
    additionalProperties = false
  })
}

# Example: full provider specification
resource "nullplatform_provider_specification" "full" {
  name        = "AWS Configuration"
  description = "Defines settings for AWS cloud provider integration"
  icon        = "aws"
  category    = "cloud-providers"

  visible_to = [
    "organization=*",
  ]

  allow_dimensions = true

  default_dimensions = jsonencode({
    environment = "production"
  })

  schema = jsonencode({
    type     = "object"
    required = ["cluster", "region"]
    properties = {
      cluster = {
        type        = "string"
        description = "EKS cluster name"
        tag         = true
      }
      region = {
        type        = "string"
        description = "AWS region"
      }
      access_key_id = {
        type        = "string"
        description = "AWS access key ID"
        secret      = true
      }
      secret_access_key = {
        type        = "string"
        description = "AWS secret access key"
        secret      = true
      }
    }
    additionalProperties = false
    groups = [
      {
        name   = "Cluster"
        fields = ["cluster", "region"]
      },
      {
        name   = "Credentials"
        fields = ["access_key_id", "secret_access_key"]
      }
    ]
  })
}

# Output the computed attributes
output "minimal_spec_id" {
  value = nullplatform_provider_specification.minimal.id
}

output "minimal_spec_slug" {
  value = nullplatform_provider_specification.minimal.slug
}

output "full_spec_id" {
  value = nullplatform_provider_specification.full.id
}

output "full_spec_categories" {
  value = nullplatform_provider_specification.full.categories
}
