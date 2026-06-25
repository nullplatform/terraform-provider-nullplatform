terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

data "nullplatform_parameter" "example" {
  id = "123"
}
