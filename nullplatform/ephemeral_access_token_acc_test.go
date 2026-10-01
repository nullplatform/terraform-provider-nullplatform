package nullplatform

import (
	"context"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-mux/tf5muxserver"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// TestAccAccessToken exchanges a real API key through the unmodified mux main.go serves.
func TestAccAccessToken(t *testing.T) {
	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_10_0)},
		ProtoV5ProviderFactories: map[string]func() (tfprotov5.ProviderServer, error){
			"nullplatform": func() (tfprotov5.ProviderServer, error) {
				mux, err := tf5muxserver.NewMuxServer(context.Background(),
					Provider().GRPCProvider,
					providerserver.NewProtocol5(NewFrameworkProvider()),
				)
				if err != nil {
					return nil, err
				}
				return mux.ProviderServer(), nil
			},
		},
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"echo": echoprovider.NewProviderServer(),
		},
		Steps: []resource.TestStep{{
			Config: `
provider "nullplatform" {}

ephemeral "nullplatform_access_token" "cli" {}

provider "echo" {
  data = ephemeral.nullplatform_access_token.cli
}

resource "echo" "token" {}
`,
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("echo.token", tfjsonpath.New("data").AtMapKey("access_token"),
					knownvalue.StringRegexp(regexp.MustCompile(`^eyJ[\w-]+\.[\w-]+\.[\w-]+$`))),
				statecheck.ExpectKnownValue("echo.token", tfjsonpath.New("data").AtMapKey("expires_at"),
					knownvalue.StringRegexp(regexp.MustCompile(`^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$`))),
			},
		}},
	})
}
