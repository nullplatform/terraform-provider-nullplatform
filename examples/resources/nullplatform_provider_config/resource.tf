terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

# Use the `NP_API_KEY` environment variable
provider "nullplatform" {}

# Slugs that identify the NRN the provider config is attached to.
# The provider resolves these into the full NRN at apply time, so there
# is no need to hardcode organization/account IDs.
variable "account" {
  type        = string
  description = "Slug of the account NRN component."
}

variable "namespace" {
  type        = string
  description = "Slug of the namespace NRN component."
}

# Provider configuration for an AWS EKS cluster, scoped to an account/namespace.
resource "nullplatform_provider_config" "aws_eks" {
  account   = var.account
  namespace = var.namespace

  # Provider type slug.
  type = "aws-eks"

  # Limit this config to a specific dimension (e.g. environment).
  dimensions = {
    environment = "production"
  }

  # Provider-specific settings as a JSON string.
  attributes = jsonencode({
    cluster = {
      name   = "main-eks-cluster"
      region = "us-east-1"
    }
    networking = {
      public_balancer_subnets  = ["subnet-0a1b2c3d", "subnet-1a2b3c4d"]
      private_balancer_subnets = ["subnet-2a3b4c5d", "subnet-3a4b5c6d"]
    }
  })
}
