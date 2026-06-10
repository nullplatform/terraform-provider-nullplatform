# Register a git repository at a specific reference (commit sha, tag or
# branch). The resource id is the revision id — ready to pin in a package BOM.
resource "nullplatform_artifact" "scopes_source" {
  nrn  = "organization=1255165411:account=95118862"
  type = "git_repository"
  meta = jsonencode({
    url       = "https://github.com/nullplatform/scopes.git"
    reference = "1.10.0"
  })
}

# Register an OCI image by digest, shared with another organization scope.
resource "nullplatform_artifact" "runtime_image" {
  nrn  = "organization=1255165411:account=95118862"
  type = "oci_image"
  meta = jsonencode({
    registry   = "ghcr.io"
    repository = "nullplatform/runtime"
    digest     = "sha256:4f53cda18c2baa0c0354bb5f9a3ecbe5ed12ab4d8e11ba873c2f11161202b945"
  })

  visible_to = [
    "organization=1255165411:account=95118862",
    "organization=1255165411:account=12345",
  ]
}
