terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

resource "nullplatform_provider_config" "google_cloud_config" {
  nrn           = "organization=1234567890:account=987654321:namespace=1122334455:application=9876543210"
  type = "google-cloud-config"
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

  type = "gke-config"
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

  /*
    - Why is this necessary?

    When creating provider's, there are resources that depend on other resources.
    In this case, the `gke_config` resource depends on the `google_cloud_config` resource.
    Since without the `google_cloud_config` resource, the `gke_config` resource cannot be created.
    To avoid conflicts, and concurrency issues, we need to specify the dependency between the resources.

  */
  depends_on = [
    nullplatform_provider_config.google_cloud_config
  ]
}