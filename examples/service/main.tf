resource "nullplatform_service" "test_service_redis" {
  name             =  "fromterraform"
  specification_id = "4a4f6955-5ae0-40dc-a1de-e15e5cf41abb"
  entity_nrn       = "organization=1255165411:account=95118862:namespace=249561561:application=1460930848"
  linkable_to      = ["organization=1255165411:account=95118862:namespace=249561561:application=1460930848"]
  dimensions = {}
  selectors = {
    imported = false,
  }
  attributes = {}
}

data "nullplatform_service" "service" {
  id = nullplatform_service.test_service_redis.id
}

resource "nullplatform_service_action" "test_service_redis_provisioning" {
  name             = data.nullplatform_service.service.name
  service_id       = data.nullplatform_service.service.id
  specification_id = "bfbffa48-a3da-48bb-94da-6591ed0d4bc1"
  parameters = {
    size = "small",
  }
}