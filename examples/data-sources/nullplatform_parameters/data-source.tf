terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# The parameters visible at an application, without their values
data "nullplatform_parameters" "app" {
  nrn = "organization=1234567890:account=987654321:namespace=1122334455:application=9876543210"
}

output "environment_variables" {
  value = [for parameter in data.nullplatform_parameters.app.parameters : parameter.variable if parameter.type == "environment"]
}
