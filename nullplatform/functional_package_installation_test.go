package nullplatform

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const installationResource = "nullplatform_package_installation.lambda"

// newInstallationFake is a package with two revisions, 0.9.0 (rev-9) and the
// package default 0.10.0 (rev-10), and an installation API with the real rules.
func newInstallationFake(t *testing.T) (*fakeplatform.Server, *fakeplatform.PackageInstallationContract) {
	t.Helper()
	fake := fakeplatform.New()
	contract := &fakeplatform.PackageInstallationContract{
		Revisions:         map[string]string{"rev-9": "0.9.0", "rev-10": "0.10.0"},
		DefaultRevisionID: "rev-10",
		LiveServices:      map[string]int{},
	}
	fakeplatform.RegisterPackageInstallation(fake, contract)
	t.Cleanup(fake.Close)
	return fake, contract
}

// uninstallOutOfBand deletes an installation the way someone outside this
// Terraform state would: straight against the API.
func uninstallOutOfBand(fake *fakeplatform.Server, id string) error {
	req, err := http.NewRequest(http.MethodDelete, fake.URL()+PACKAGE_INSTALLATION_PATH+"/"+id, nil)
	if err != nil {
		return err
	}
	res, err := fake.HTTP().Client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("out-of-band uninstall answered %d", res.StatusCode)
	}
	return nil
}

func installationConfig(extra string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_package_installation" "lambda" {
  nrn        = "organization=1:account=2:namespace=3"
  package_id = "pkg-global"
%s
}
`, extra)
}

// Installing without a revision takes the package default at install time, and
// visibility defaults to the installation NRN. The optional requirements the
// organization lacks come back once, and stay in state on later plans.
func TestFunctionalPackageInstallation_InstallsOnThePackageDefault(t *testing.T) {
	fake, _ := newInstallationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: installationConfig(""),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(installationResource, "default_revision_id", "rev-10"),
				resource.TestCheckResourceAttr(installationResource, "default_version", "0.10.0"),
				resource.TestCheckResourceAttr(installationResource, "visible_to.#", "1"),
				resource.TestCheckResourceAttr(installationResource, "visible_to.0", "organization=1:account=2:namespace=3"),
				resource.TestCheckResourceAttr(installationResource, "created_by", "3"),
				resource.TestCheckResourceAttr(installationResource, "missing_optional_requirements.#", "1"),
				resource.TestCheckResourceAttr(installationResource, "missing_optional_requirements.0.category", "scope-configurations"),
			),
		}},
	})
}

// Moving the default revision is an in-place update, never a reinstall: the
// installation keeps its id and the services it has born.
func TestFunctionalPackageInstallation_MovingTheDefaultUpdatesInPlace(t *testing.T) {
	fake, _ := newInstallationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: installationConfig(`  default_revision_id = "rev-10"`),
				Check:  resource.TestCheckResourceAttr(installationResource, "id", "inst-1"),
			},
			{
				Config: installationConfig(`  default_revision_id = "rev-9"
  visible_to          = ["organization=1:account=2:namespace=3:application=4"]`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(installationResource, plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(installationResource, "id", "inst-1"),
					resource.TestCheckResourceAttr(installationResource, "default_version", "0.9.0"),
					resource.TestCheckResourceAttr(installationResource, "visible_to.0", "organization=1:account=2:namespace=3:application=4"),
				),
			},
		},
	})
}

// Changing where it is installed is another installation.
func TestFunctionalPackageInstallation_ChangingTheNrnReinstalls(t *testing.T) {
	fake, _ := newInstallationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: installationConfig("")},
			{
				Config: `
provider "nullplatform" {}

resource "nullplatform_package_installation" "lambda" {
  nrn        = "organization=1:account=7"
  package_id = "pkg-global"
}
`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(installationResource, plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr(installationResource, "id", "inst-2"),
			},
		},
	})
}

// The API's refusal reaches the operator verbatim: a revision of another package.
func TestFunctionalPackageInstallation_RefusedRevisionSurfacesTheAPIMessage(t *testing.T) {
	fake, _ := newInstallationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config:      installationConfig(`  default_revision_id = "rev-of-another-package"`),
			ExpectError: regexp.MustCompile(`Revision rev-of-another-package does not belong to package pkg-global`),
		}},
	})
}

// An explicit visible_to is installed as given instead of the [nrn] default.
func TestFunctionalPackageInstallation_ExplicitVisibilityIsInstalledAsGiven(t *testing.T) {
	fake, _ := newInstallationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: installationConfig(`  visible_to = ["organization=1:account=2:namespace=3:application=4", "organization=1:account=2:namespace=5"]`),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(installationResource, "visible_to.#", "2"),
				resource.TestCheckResourceAttr(installationResource, "visible_to.0", "organization=1:account=2:namespace=3:application=4"),
				resource.TestCheckResourceAttr(installationResource, "visible_to.1", "organization=1:account=2:namespace=5"),
			),
		}},
	})
}

// A refused move of the default revision surfaces the API's message, and the
// installation stays on the revision it had: the next refresh reads it back.
func TestFunctionalPackageInstallation_RefusedMoveKeepsTheInstalledRevision(t *testing.T) {
	fake, _ := newInstallationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: installationConfig(`  default_revision_id = "rev-9"`)},
			{
				Config:      installationConfig(`  default_revision_id = "rev-of-another-package"`),
				ExpectError: regexp.MustCompile(`Revision rev-of-another-package does not belong to package pkg-global`),
			},
			{
				Config: installationConfig(`  default_revision_id = "rev-9"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(installationResource, "id", "inst-1"),
					resource.TestCheckResourceAttr(installationResource, "default_version", "0.9.0"),
				),
			},
		},
	})
}

// Uninstalling while a service born from the installation is alive is refused
// with the API's message; once the services are gone, destroy goes through.
func TestFunctionalPackageInstallation_UninstallRefusedWhileServicesLive(t *testing.T) {
	fake, contract := newInstallationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: installationConfig(""),
				Check: func(*terraform.State) error {
					contract.LiveServices["inst-1"] = 2
					return nil
				},
			},
			{
				Config:      installationConfig(""),
				Destroy:     true,
				ExpectError: regexp.MustCompile(`Package installation inst-1 cannot be deleted: 2 service instance\(s\) under its visible_to were born from it`),
			},
			{
				Config: installationConfig(""),
				Check: func(*terraform.State) error {
					contract.LiveServices["inst-1"] = 0
					return nil
				},
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if items := fake.Items("package_installations"); len(items) != 0 {
				return fmt.Errorf("installation still present after destroy: %v", items)
			}
			return nil
		},
	})
}

// An installation removed outside Terraform drops from state, and the next
// plan offers to install again instead of failing.
func TestFunctionalPackageInstallation_RemovedOutOfBandIsReinstalled(t *testing.T) {
	fake, _ := newInstallationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: installationConfig(""),
				Check: func(*terraform.State) error {
					return uninstallOutOfBand(fake, "inst-1")
				},
				ExpectNonEmptyPlan: true,
			},
			{
				Config: installationConfig(""),
				Check:  resource.TestCheckResourceAttr(installationResource, "id", "inst-2"),
			},
		},
	})
}

// Import by id reads the installation back with no diff.
func TestFunctionalPackageInstallation_Import(t *testing.T) {
	fake, _ := newInstallationFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: installationConfig("")},
			{
				ResourceName:            installationResource,
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"missing_optional_requirements"},
			},
		},
	})
}

// The global-package path end to end: look the package up by visibility, then
// install the revision the data source resolved.
func TestFunctionalPackageInstallation_FromTheVisibilityLookup(t *testing.T) {
	fake, _ := newInstallationFake(t)
	fake.Register("packages", "pkg", fakeplatform.Hooks{
		OnList: func(s *fakeplatform.Server) (any, *fakeplatform.Refusal) {
			if s.Query().Get("visible_to") == "" || s.Query().Get("slug") != "scopes-lambda-global" {
				return map[string]any{"results": []any{}}, nil
			}
			return map[string]any{"results": []any{map[string]any{
				"id": "pkg-global", "slug": "scopes-lambda-global", "nrn": "organization=1",
				"visible_to": []any{"organization=*"}, "default_revision_id": "rev-10", "default_version": "0.10.0",
				"latest_revision_id": "rev-10", "latest_version": "0.10.0",
			}}}, nil
		},
	})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: `
provider "nullplatform" {}

data "nullplatform_package" "lambda" {
  visible_at = "organization=1:account=2"
  slug       = "scopes-lambda-global"
}

resource "nullplatform_package_installation" "lambda" {
  nrn                 = "organization=1:account=2"
  package_id          = data.nullplatform_package.lambda.id
  default_revision_id = data.nullplatform_package.lambda.revision_id
}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(installationResource, "package_id", "pkg-global"),
				resource.TestCheckResourceAttr(installationResource, "default_version", "0.10.0"),
			),
		}},
	})
}
