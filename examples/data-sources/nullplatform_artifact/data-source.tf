# Resolve a git repository artifact revision by url + reference — the ids are
# ready to pin in a package BOM.
data "nullplatform_artifact" "scopes_at_1_10" {
  nrn  = "organization=1255165411:account=95118862"
  type = "git_repository"
  meta = jsonencode({
    url       = "https://github.com/nullplatform/scopes.git"
    reference = "1.10.0"
  })
}

# Identity-only lookup: resolves the artifact and its latest revision.
data "nullplatform_artifact" "scopes_latest" {
  nrn  = "organization=1255165411:account=95118862"
  type = "git_repository"
  meta = jsonencode({
    url = "https://github.com/nullplatform/scopes.git"
  })
}

output "scopes_revision_id" {
  value = data.nullplatform_artifact.scopes_at_1_10.revision_id
}

# Tag-based lookup: resolve an OCI image by its human-readable tag instead of
# a digest — the data source finds the NEWEST revision registered with that
# tag and computes its digest for you. When the tag is re-registered against a
# new image, the next plan drifts to the new revision (digest changes), by design.
data "nullplatform_artifact" "worker_v1" {
  nrn  = "organization=1255165411:account=95118862"
  type = "oci_image"
  meta = jsonencode({
    registry   = "public.ecr.aws"
    repository = "nullplatform/scopes/lambda"
    tag        = "v1.2.0"
  })
}

output "worker_pinned_reference" {
  value = "public.ecr.aws/nullplatform/scopes/lambda@${data.nullplatform_artifact.worker_v1.digest}"
}
