terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "application_id" {
  description = "ID of the application that owns the scope type"
  type        = number
}

variable "service_specification_id" {
  description = "ID of the service specification that implements the scope type"
  type        = string
}

# Resolve the NRN from the application instead of hardcoding it
data "nullplatform_application" "this" {
  id = var.application_id
}

resource "nullplatform_scope_type" "kubernetes_job" {
  name        = "Kubernetes Job"
  description = "Run periodic jobs on Kubernetes"
  nrn         = data.nullplatform_application.this.nrn

  # type defaults to "custom"; provider_type defaults to "service"
  provider_type = "service"
  provider_id   = var.service_specification_id
}
