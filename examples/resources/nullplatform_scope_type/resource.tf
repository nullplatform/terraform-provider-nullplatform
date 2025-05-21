terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

variable "null_application_id" {
  description = "Unique ID for the application"
  type        = number
}

variable "service_specification_id" {
  description = "The service specification ID that implements the scope type"
  type        = string
}

data "nullplatform_application" "app" {
  id = var.null_application_id
}

resource "nullplatform_scope_type" "kubernetes_job" {
  name         = "Kubernetes Job"
  type         = "custom"
  description  = "Run periodic jobs in Kubernetes"
  nrn          = data.nullplatform_application.app.nrn
  provider_type = "service"
  provider_id   = var.service_specification_id
}

resource "nullplatform_scope_type" "database_backup" {
  name         = "Database Backup"
  type         = "custom"
  description  = "Automated database backup mechanism"
  nrn          = data.nullplatform_application.app.nrn
  provider_type = "service"
  provider_id   = var.service_specification_id
}

output "k8s_job_scope_type" {
  value = nullplatform_scope_type.kubernetes_job
}

output "db_backup_scope_type" {
  value = nullplatform_scope_type.database_backup
}