# Publish a package pinning a service specification snapshot and an artifact
# revision. Bump `version` together with component changes to publish a new
# revision; `default = true` promotes each publish to the package default.
resource "nullplatform_package" "k8s_containers" {
  nrn     = "organization=1255165411:account=95118862"
  slug    = "k8s-containers"
  name    = "Containers"
  version = "1.0.0"
  default = true

  components {
    name                 = "spec"
    resource_type        = "service_specification"
    resource_id          = nullplatform_service_specification.containers.id
    resource_revision_id = var.containers_spec_snapshot_id
  }

  components {
    name                 = "source"
    resource_type        = "artifact"
    resource_id          = nullplatform_artifact.scopes_source.artifact_id
    resource_revision_id = nullplatform_artifact.scopes_source.id
  }

  visible_to = [
    "organization=1255165411",
  ]
}

# Pin the default to an exact published version instead of promoting each
# publish (mutually exclusive with `default = true`). Handy for staged
# rollouts and rollbacks: point default_version back at a previous release
# and apply.
resource "nullplatform_package" "pinned" {
  nrn     = "organization=1255165411:account=95118862"
  slug    = "pinned-runtime"
  name    = "Pinned Runtime"
  version = "1.1.0"

  default_version = "1.0.0" # consumers stay on 1.0.0 while 1.1.0 soaks

  components {
    name                 = "spec"
    resource_type        = "service_specification"
    resource_id          = nullplatform_service_specification.containers.id
    resource_revision_id = var.containers_spec_snapshot_id
  }
}

# A package other organizations install declares what they must already have
# configured: installing it where a required provider is missing is refused,
# and a missing optional one is reported on the installation.
resource "nullplatform_package" "functions" {
  nrn        = "organization=123"
  slug       = "functions"
  name       = "Functions"
  version    = "1.0.0"
  default    = true
  visible_to = ["organization=*"]

  components {
    name                 = "scope"
    resource_type        = "service_specification"
    resource_id          = nullplatform_service_specification.functions.id
    resource_revision_id = var.functions_spec_snapshot_id
  }

  requires_provider {
    category = "cloud-providers" # any provider specification in the category
  }

  requires_provider {
    category = "metrics"
    optional = true
  }
}
