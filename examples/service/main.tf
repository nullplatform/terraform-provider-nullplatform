data "nullplatform_application" "app" {
  id = var.null_application_id
}


resource "nullplatform_service" "redis_cache_test" {
  name             =  "redis-cache"
  specification_id = "4a4f6955-5ae0-40dc-a1de-e15e5cf41abb"
  entity_nrn       = data.nullplatform_application.app.nrn
  linkable_to      = [data.nullplatform_application.app.nrn]
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
  name              = "open-weather"
  specification_id  = var.specification_id
  entity_nrn        = data.nullplatform_application.app.nrn
  linkable_to       = [data.nullplatform_application.app.nrn]
  status            = "active"
  selectors = {
    category      = "SaaS",
    imported      = true,
    provider      = "OpenWeather",
    sub_category  = "Weather",
  }
  attributes = {
    api_key = var.api_key
  }
  dimensions = {}
}
