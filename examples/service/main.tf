resource "nullplatform_service" "test_service" {
  name              = "open weather"
  specification_id  = var.specification_id
  entity_nrn        = "organization=1255165411:account=95118862"
  linkable_to       = ["organization=1255165411:account=95118862"]
  status            = "creating"
  selectors = {
    "category"      = "SaaS"
    "imported"      = true
    "provider"      = "OpenWeather"
    "sub_category"  = "Weather"
  }
  attributes = {
    api_key         = var.api_key
  }
  dimensions        = {}
}