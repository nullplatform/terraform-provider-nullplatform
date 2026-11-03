terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
}

provider "nullplatform" {}

# The services visible at an application: its own and those of its ancestors
data "nullplatform_services" "app" {
  nrn = "organization=1234567890:account=987654321:namespace=1122334455:application=9876543210"
}

output "service_names" {
  value = [for service in data.nullplatform_services.app.services : service.name]
}
