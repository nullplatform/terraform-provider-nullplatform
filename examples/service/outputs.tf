output "service" {
  value = nullplatform_service.test_service_redis
}

output "service_provisioned" {
  value = nullplatform_service_action.test_service_redis_provisioning
}