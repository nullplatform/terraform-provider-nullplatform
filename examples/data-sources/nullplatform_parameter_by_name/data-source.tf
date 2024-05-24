terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

data "nullplatform_parameter_by_name" "example" {
  nrn  = "organization=1:account=2:namespace=3:application=4"
  name = "LOG_LEVEL"
}
