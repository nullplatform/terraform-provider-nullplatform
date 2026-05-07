terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

locals {
  dimensions = var.dimensions
}

resource "nullplatform_provider_config" "kubernetes_cluster_cloud_config" {
  nrn  = var.nrn
  type = "eks-configuration"
  attributes = jsonencode({
    cluster = {
      id        = var.cluster_name
      namespace = var.cluster_namespace
    }
    security = {
      image_pull_secrets = [
        "regcred"
      ]
    }
    balancer = {
      private_name             = var.private_lb_name
      public_name              = var.public_lb_name
      additional_public_names  = var.additional_public_names
      additional_private_names = var.additional_private_names
      alb_capacity_threshold   = var.alb_capacity_threshold
    }
  })
  dimensions = jsondecode(local.dimensions)
}
