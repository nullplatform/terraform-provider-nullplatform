# Look up a package by (nrn, slug); revision_id follows the package default.
data "nullplatform_package" "k8s_containers" {
  nrn  = "organization=1255165411:account=95118862"
  slug = "k8s-containers"
}

# Pin an exact published version instead.
data "nullplatform_package" "k8s_containers_v1" {
  nrn     = "organization=1255165411:account=95118862"
  slug    = "k8s-containers"
  version = "1.0.0"
}

# A package published for every organization is found by where it is visible,
# not by its owner: visible_at is an NRN of your own organization.
data "nullplatform_package" "global_lambda" {
  visible_at = "organization=123:account=456"
  slug       = "scopes-lambda"
}

output "default_revision_id" {
  value = data.nullplatform_package.k8s_containers.revision_id
}

# components is the resolved revision's bill of materials: the way to reach the
# resources of a package you do not own, e.g. the service specification a scope
# type must point at.
output "global_lambda_specification_id" {
  value = one([
    for c in data.nullplatform_package.global_lambda.components : c.resource_id
    if c.resource_type == "service_specification"
  ])
}
