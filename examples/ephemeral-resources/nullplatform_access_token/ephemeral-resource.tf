# Run an np CLI command with the provider's credentials: the API key never
# leaves the provider, and the short-lived token never reaches plan or state.
ephemeral "nullplatform_access_token" "cli" {}

resource "terraform_data" "np_cli" {
  provisioner "local-exec" {
    command = "np application list --format json"
    environment = {
      NP_TOKEN = ephemeral.nullplatform_access_token.cli.access_token
    }
  }
}
