output "provider_config_id" {
  value       = nullplatform_provider_config.kubernetes_cluster_cloud_config.id
  description = "The ID of the created provider config."
}
