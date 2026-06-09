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
