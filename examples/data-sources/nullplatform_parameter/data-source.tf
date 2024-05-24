terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

data "nullplatform_parameter" "example" {
  id = "123"
}
