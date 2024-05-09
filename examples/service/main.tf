resource "nullplatform_service" "redis_cache_test" {
  name             =  "redis-cache"
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
  id = nullplatform_service.redis_cache_test.id
}

resource "nullplatform_service" "open_weather_test" {
  name              = "open weather"
  specification_id  = var.specification_id
  entity_nrn        = "organization=123456:account=12345"
  linkable_to       = ["organization=123456:account=12345"]
  status            = "creating"
  selectors = {
    "category"      = "SaaS"
    "imported"      = true
    "provider"      = "OpenWeather"
    "sub_category"  = "Weather"
  }
  attributes = {
    api_key = var.api_key
  }
  dimensions = {}
}
