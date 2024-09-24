terraform {
  required_providers {
    nullplatform = {
      source  = "nullplatform/com/nullplatform"
      version = "0.0.15"
    }
  }
}

provider "nullplatform" {}

data "nullplatform_provider_config" "existing_google_cloud_config" {
  id = "12345"
}

data "nullplatform_provider_config" "existing_gke_config" {
  id = "67890"
}

output "google_cloud_config_details" {
  value = {
    nrn           = data.nullplatform_provider_config.existing_google_cloud_config.nrn
    specification = data.nullplatform_provider_config.existing_google_cloud_config.specification
    project_id    = jsondecode(data.nullplatform_provider_config.existing_google_cloud_config.attributes).project.id
    location      = jsondecode(data.nullplatform_provider_config.existing_google_cloud_config.attributes).project.location
  }
}

output "gke_config_details" {
  value = {
    account      = data.nullplatform_provider_config.existing_gke_config.account
    namespace    = data.nullplatform_provider_config.existing_gke_config.namespace
    application  = data.nullplatform_provider_config.existing_gke_config.application
    cluster_id   = jsondecode(data.nullplatform_provider_config.existing_gke_config.attributes).cluster.id
    cluster_ns   = jsondecode(data.nullplatform_provider_config.existing_gke_config.attributes).cluster.namespace
  }
}