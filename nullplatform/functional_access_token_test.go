package nullplatform

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/hashicorp/terraform-plugin-go/tfprotov6"
	"github.com/hashicorp/terraform-plugin-testing/echoprovider"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const functionalApiKey = "functional-api-key"

var functionalTokenExpiry = time.Date(2030, 1, 2, 3, 4, 5, 0, time.UTC)

func signedTestToken(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	token, err := jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte("functional"))
	if err != nil {
		t.Fatalf("signing test token: %v", err)
	}
	return token
}

func newAccessTokenFake(t *testing.T, accessToken string) *fakeplatform.Server {
	t.Helper()
	fake := fakeplatform.New()
	fakeplatform.RegisterToken(fake, functionalApiKey, accessToken)
	t.Cleanup(fake.Close)
	return fake
}

// accessTokenConfig echoes the ephemeral result into state, the only way to observe it.
func accessTokenConfig(fake *fakeplatform.Server, apiKeyLine string) string {
	return fmt.Sprintf(`
provider "nullplatform" {
  host = %q
  %s
}

ephemeral "nullplatform_access_token" "cli" {}

provider "echo" {
  data = ephemeral.nullplatform_access_token.cli
}

resource "echo" "token" {}
`, strings.TrimPrefix(fake.URL(), "https://"), apiKeyLine)
}

func accessTokenTestCase(fake *fakeplatform.Server, steps ...resource.TestStep) resource.TestCase {
	return resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_10_0)},
		ProtoV5ProviderFactories: functionalMuxFactories(fake),
		ProtoV6ProviderFactories: map[string]func() (tfprotov6.ProviderServer, error){
			"echo": echoprovider.NewProviderServer(),
		},
		Steps: steps,
	}
}

func TestFunctionalAccessToken_ExchangesProviderApiKey(t *testing.T) {
	token := signedTestToken(t, jwt.MapClaims{"exp": functionalTokenExpiry.Unix()})
	fake := newAccessTokenFake(t, token)

	resource.UnitTest(t, accessTokenTestCase(fake, resource.TestStep{
		Config: accessTokenConfig(fake, fmt.Sprintf("api_key = %q", functionalApiKey)),
		ConfigStateChecks: []statecheck.StateCheck{
			statecheck.ExpectKnownValue("echo.token", tfjsonpath.New("data").AtMapKey("access_token"), knownvalue.StringExact(token)),
			statecheck.ExpectKnownValue("echo.token", tfjsonpath.New("data").AtMapKey("expires_at"), knownvalue.StringExact("2030-01-02T03:04:05Z")),
		},
	}))
}

// The pattern the resource exists for (#156): hand the token to the np CLI in a provisioner.
func TestFunctionalAccessToken_FeedsLocalExecEnvironment(t *testing.T) {
	token := signedTestToken(t, jwt.MapClaims{"exp": functionalTokenExpiry.Unix()})
	fake := newAccessTokenFake(t, token)
	out := filepath.Join(t.TempDir(), "np_token")

	resource.UnitTest(t, resource.TestCase{
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_10_0)},
		ProtoV5ProviderFactories: functionalMuxFactories(fake),
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`
provider "nullplatform" {
  host    = %q
  api_key = %q
}

ephemeral "nullplatform_access_token" "cli" {}

resource "terraform_data" "np_cli" {
  provisioner "local-exec" {
    command     = "printf %%s \"$NP_TOKEN\" > %s"
    environment = { NP_TOKEN = ephemeral.nullplatform_access_token.cli.access_token }
  }
}
`, strings.TrimPrefix(fake.URL(), "https://"), functionalApiKey, out),
			Check: func(state *terraform.State) error {
				for name, rs := range state.RootModule().Resources {
					for key, value := range rs.Primary.Attributes {
						if strings.Contains(value, token) {
							return fmt.Errorf("the access token leaked into state at %s.%s", name, key)
						}
					}
				}
				got, err := os.ReadFile(out)
				if err != nil {
					return err
				}
				if string(got) != token {
					return fmt.Errorf("NP_TOKEN seen by local-exec = %q, want the issued token", got)
				}
				return nil
			},
		}},
	})
}

func TestFunctionalAccessToken_WithoutExpiryLeavesExpiresAtNull(t *testing.T) {
	fake := newAccessTokenFake(t, signedTestToken(t, jwt.MapClaims{"sub": "agent"}))

	resource.UnitTest(t, accessTokenTestCase(fake, resource.TestStep{
		Config: accessTokenConfig(fake, fmt.Sprintf("api_key = %q", functionalApiKey)),
		ConfigStateChecks: []statecheck.StateCheck{
			statecheck.ExpectKnownValue("echo.token", tfjsonpath.New("data").AtMapKey("expires_at"), knownvalue.Null()),
		},
	}))
}

func TestFunctionalAccessToken_EnvApiKeyFillsOmittedAttribute(t *testing.T) {
	t.Setenv("NULLPLATFORM_API_KEY", functionalApiKey)
	token := signedTestToken(t, jwt.MapClaims{"sub": "agent"})
	fake := newAccessTokenFake(t, token)

	resource.UnitTest(t, accessTokenTestCase(fake, resource.TestStep{
		Config: accessTokenConfig(fake, ""),
		ConfigStateChecks: []statecheck.StateCheck{
			statecheck.ExpectKnownValue("echo.token", tfjsonpath.New("data").AtMapKey("access_token"), knownvalue.StringExact(token)),
		},
	}))
}

func TestFunctionalAccessToken_Refusals(t *testing.T) {
	tests := []struct {
		name, apiKeyLine, envApiKey, want string
	}{
		{"rejected key surfaces the 401", `api_key = "wrong-key"`, "", `failed to get access token, got 401`},
		{"no key at all", "", "", `Missing API Key`},
		// Like the SDKv2 EnvDefaultFunc, the variable never overrides an api_key set to "".
		{"empty key ignores the env", `api_key = ""`, functionalApiKey, `Missing API Key`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("NULLPLATFORM_API_KEY", tt.envApiKey)
			t.Setenv(NP_API_KEY_ENV, "")
			fake := newAccessTokenFake(t, "unused")

			resource.UnitTest(t, accessTokenTestCase(fake, resource.TestStep{
				Config:      accessTokenConfig(fake, tt.apiKeyLine),
				ExpectError: regexp.MustCompile(tt.want),
			}))
		})
	}
}

func TestTokenExpiry_UnparsableTokenHasNoExpiry(t *testing.T) {
	if exp := tokenExpiry("not-a-jwt"); !exp.IsZero() {
		t.Fatalf("tokenExpiry(not-a-jwt) = %v, want zero", exp)
	}
}
