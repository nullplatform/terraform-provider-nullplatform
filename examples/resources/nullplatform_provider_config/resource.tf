terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/com/nullplatform"
      version = "0.0.15"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_provider_config" "google_cloud_config" {
  nrn           = "organization=1234567890:account=987654321:namespace=1122334455:application=9876543210"
  specification = "google-cloud-config"
  dimensions    = {}
  attributes    = jsonencode({
    project = {
      id       = "my-gcp-project"
      location = "us-central1"
    },
    networking = {
      public_balancer_ip   = "34.120.0.123",
      private_balancer_ip  = "10.0.0.10",
      public_dns_zone_name = "my-gcp-project-zone"
    },
    authentication = {
      credential_base_64 = "BASE64_ENCODED_CREDENTIAL_PLACEHOLDER"
    }
  })
}

resource "nullplatform_provider_config" "gke_config" {
  account     = "my-main-account"
  namespace   = "gcp-infrastructure"
  application = "gke-clusters"

  specification = "gke-config"
  dimensions    = {}
  attributes    = jsonencode({
    cluster = {
      id        = "primary-cluster",
      namespace = "nullplatform"
    },
    gateway = {
      public_name  = "ingress-gateway-public",
      private_name = "ingress-gateway-private"
    }
  })

  depends_on = [
    nullplatform_provider_config.google_cloud_config
  ]
}