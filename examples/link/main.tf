resource "nullplatform_link" "link_redis" {
  name             = "link_from_terraform_2"
  status           = "active"
  service_id       = "36432d16-ec69-48e4-b5f5-18a63e7034c1"
  specification_id = "66919464-05e6-4d78-bb8c-902c57881ddd"
  entity_nrn       = "organization=1255165411:account=95118862:namespace=249561561:application=1460930848"
  linkable_to      = ["organization=1255165411:account=95118862:namespace=249561561:application=1460930848"]
  selectors = {
    imported = false,
  }
  dimensions = {
    environment = "development",
    country     = "argentina",
  }
  attributes = {}
}