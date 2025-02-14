terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {
}

resource "nullplatform_dimension" "ordered_dimension" {
  name  = "RegionTest"
  order = 2
  nrn   = "organization=1255165411:account=95118862:namespace=1991443329:application=213260358"
}

resource "nullplatform_dimension" "component_dimension" {
  name      = "DepartmentTest"
  account   = "kwik-e-mart-main"
  namespace = "services-day-dic-2024"
  order     = 3
}

output "dimension_slug" {
  description = "The generated slug for the dimension"
  value       = nullplatform_dimension.ordered_dimension.slug
}

output "dimension_status" {
  description = "The current status of the dimension"
  value       = nullplatform_dimension.component_dimension.status
}
