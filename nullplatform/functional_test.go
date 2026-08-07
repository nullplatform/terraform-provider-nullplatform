package nullplatform

// Functional tests: the HashiCorp-recommended harness (terraform-plugin-testing)
// driving REAL `terraform plan / apply / import / destroy` cycles against the
// in-process provider, backed by internal/fakeplatform — a generic stateful
// REST engine plus the archive behavior module. Only ConfigureContextFunc is
// swapped, for a client that trusts the fake's TLS certificate.
//
//   - `resource.UnitTest` is `resource.Test` without the TF_ACC gate: the
//     framework's own mode for mock-backed runs. No credentials, every
//     `go test`, CI included.
//   - after every apply the framework re-plans and fails on any diff, so the
//     perpetual-diff regression class is checked on every step for free.
//   - a specification's ARCHIVE CHAIN is registered per test and keyed on
//     specification_id — the same document the real API resolves it from.
import (
	"context"
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

func newFakePlatform(t *testing.T, chains map[string]fakeplatform.Chain) *fakeplatform.Server {
	t.Helper()
	fake := fakeplatform.New()
	fakeplatform.RegisterArchive(fake, chains)
	t.Cleanup(fake.Close)
	return fake
}

// functionalFactories serves the REAL provider with only its configure swapped
// for a client that trusts the fake's certificate.
func functionalFactories(fake *fakeplatform.Server) map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"nullplatform": func() (*schema.Provider, error) {
			provider := Provider()
			provider.ConfigureContextFunc = func(_ context.Context, _ *schema.ResourceData) (any, diag.Diagnostics) {
				return newTestClient(fake.HTTP()), nil
			}
			return provider, nil
		},
	}
}

func serviceConfig(spec, extra string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_service" "db" {
  name             = "functional-redis"
  specification_id = %q
  entity_nrn       = "organization=1:account=2"
  import           = true
%s
}
`, spec, extra)
}

// --- The direct-flip chain -------------------------------------------------

// The full archive lifecycle through real Terraform: create, archive by
// status, restore by status, import — the framework re-planning after every
// apply and failing on any diff.
func TestFunctionalService_ArchiveLifecycle(t *testing.T) {
	shortenPolling(t)
	fake := newFakePlatform(t, nil)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: serviceConfig("spec-1", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "active"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "archived_at", ""),
				),
			},
			{
				Config: serviceConfig("spec-1", `  status = "archived"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "archived"),
					resource.TestCheckResourceAttrSet("nullplatform_service.db", "archived_at"),
				),
			},
			{
				Config: serviceConfig("spec-1", `  status = "active"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "active"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "archived_at", ""),
				),
			},
			{
				ResourceName:            "nullplatform_service.db",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"import", "force_destroy", "archive_on_destroy"},
			},
		},
	})
}

// --- The managed chain -----------------------------------------------------

// A managed archive and restore: each PATCH only starts the transition and the
// apply must wait it out — through real applies, not direct waiter calls.
func TestFunctionalService_ManagedArchiveAndRestoreWait(t *testing.T) {
	shortenPolling(t)
	fake := newFakePlatform(t, map[string]fakeplatform.Chain{"spec-managed": fakeplatform.ChainManaged})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: serviceConfig("spec-managed", "")},
			{
				Config: serviceConfig("spec-managed", `  status = "archived"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "archived"),
					resource.TestCheckResourceAttrSet("nullplatform_service.db", "archived_at"),
				),
			},
			{
				Config: serviceConfig("spec-managed", `  status = "active"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "active"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "archived_at", ""),
				),
			},
		},
	})
}

// --- The refusals, surfaced through real applies ---------------------------

// An archive action that is not managed refuses the status shortcut; the apply
// fails with the API's message and the resource stays put.
func TestFunctionalService_UnmanagedActionRefusesTheShortcut(t *testing.T) {
	shortenPolling(t)
	fake := newFakePlatform(t, map[string]fakeplatform.Chain{"spec-manual": fakeplatform.ChainManual})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: serviceConfig("spec-manual", "")},
			{
				Config:      serviceConfig("spec-manual", `  status = "archived"`),
				ExpectError: regexp.MustCompile(`use the 'archive' action`),
			},
			// The refusal left the service untouched and the diff re-offers.
			{
				Config: serviceConfig("spec-manual", ""),
				Check:  resource.TestCheckResourceAttr("nullplatform_service.db", "status", "active"),
			},
		},
	})
}

// Changing attributes and archiving in ONE apply: the API refuses the combined
// PATCH, so the provider must split them — attributes first, then the bare
// status. The fake enforces the refusal, so this passing proves the split.
func TestFunctionalService_AttributeChangeAndArchiveInOneApply(t *testing.T) {
	shortenPolling(t)
	fake := newFakePlatform(t, nil)

	withAttrs := func(extra string) string {
		return serviceConfig("spec-1", "  attributes = { size = \"small\" }\n"+extra)
	}
	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: withAttrs("")},
			{
				Config: serviceConfig("spec-1", "  attributes = { size = \"large\" }\n  status = \"archived\""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "archived"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "attributes.size", "large"),
				),
			},
		},
	})
}

// Creating a service that collides with an archived twin is refused with the
// aviso naming the twin and how to resolve it.
func TestFunctionalService_ArchivedTwinCreateIsRefused(t *testing.T) {
	shortenPolling(t)
	fake := newFakePlatform(t, nil)

	twin := `
resource "nullplatform_service" "twin" {
  name             = "functional-redis"
  specification_id = "spec-1"
  entity_nrn       = "organization=1:account=2"
  import           = true
}
`
	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: serviceConfig("spec-1", `  status = "archived"`)},
			{
				Config:      serviceConfig("spec-1", `  status = "archived"`) + twin,
				ExpectError: regexp.MustCompile(`unarchive it, or request its deletion`),
			},
		},
	})
}

// --- The approval machine --------------------------------------------------

// An archive behind an approval policy parks; the apply waits it out and the
// transition lands once the approval arrives.
func TestFunctionalService_ApprovalParkedArchiveWaits(t *testing.T) {
	shortenPolling(t)
	fake := newFakePlatform(t, map[string]fakeplatform.Chain{"spec-approval": fakeplatform.ChainApproval})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: serviceConfig("spec-approval", "")},
			{
				Config: serviceConfig("spec-approval", `  status = "archived"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "archived"),
					resource.TestCheckResourceAttrSet("nullplatform_service.db", "archived_at"),
				),
			},
		},
	})
}

// A DENIED approval fails fast — the parked action vanished with the status
// unmoved, and the error says so instead of running out the timeout.
func TestFunctionalService_DeniedApprovalFailsFast(t *testing.T) {
	shortenPolling(t)
	fake := newFakePlatform(t, map[string]fakeplatform.Chain{"spec-denied": fakeplatform.ChainApprovalDenied})

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: serviceConfig("spec-denied", "")},
			{
				Config:      serviceConfig("spec-denied", `  status = "archived"`),
				ExpectError: regexp.MustCompile(`cancelled while waiting`),
			},
		},
	})
}

// --- The service+link stack ------------------------------------------------

const stackConfig = `
provider "nullplatform" {}

resource "nullplatform_service" "db" {
  name               = "stack-redis"
  specification_id   = "spec-1"
  entity_nrn         = "organization=1:account=2"
  import             = true
  archive_on_destroy = true
}

resource "nullplatform_link" "app" {
  name               = "stack-link"
  service_id         = nullplatform_service.db.id
  specification_id   = "lspec-1"
  entity_nrn         = "organization=1:account=2"
  archive_on_destroy = true
}
`

// Destroying a stack with archive_on_destroy on both: Terraform's dependency
// graph destroys the link first, which is exactly the order the API demands —
// a service with non-archived links refuses to archive, and the fake enforces
// it, so this passing proves the ordering end to end. Nothing is hard-deleted.
func TestFunctionalStack_ArchiveOnDestroyArchivesLinkFirst(t *testing.T) {
	shortenPolling(t)
	fake := newFakePlatform(t, nil)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		CheckDestroy: func(*terraform.State) error {
			if fake.Deletes != 0 {
				return fmt.Errorf("destroy issued %d DELETE(s); archive_on_destroy must never hard-delete", fake.Deletes)
			}
			for _, collection := range []string{"service", "link"} {
				for _, item := range fake.Items(collection) {
					if status := fakeplatform.Str(item, "status"); status != "archived" {
						return fmt.Errorf("%s %v ended %q, want archived", collection, item["id"], status)
					}
				}
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: stackConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "active"),
					resource.TestCheckResourceAttr("nullplatform_link.app", "status", "active"),
				),
			},
		},
	})
}

// A link may not be restored under a parent that is not active — the refusal
// arrives through a real apply with the API's message.
func TestFunctionalLink_RestoreUnderArchivedParentIsRefused(t *testing.T) {
	shortenPolling(t)
	fake := newFakePlatform(t, nil)

	stack := func(linkStatus, serviceStatus string) string {
		return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_service" "db" {
  name             = "stack-redis"
  specification_id = "spec-1"
  entity_nrn       = "organization=1:account=2"
  import           = true
%s
}

resource "nullplatform_link" "app" {
  name             = "stack-link"
  service_id       = nullplatform_service.db.id
  specification_id = "lspec-1"
  entity_nrn       = "organization=1:account=2"
%s
}
`, serviceStatus, linkStatus)
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: stack("", "")},
			// Archive link first (the order the API demands), then the parent.
			{Config: stack(`  status = "archived"`, "")},
			{Config: stack(`  status = "archived"`, `  status = "archived"`)},
			{
				Config:      stack(`  status = "active"`, `  status = "archived"`),
				ExpectError: regexp.MustCompile(`must be active to unarchive its links`),
			},
		},
	})
}
