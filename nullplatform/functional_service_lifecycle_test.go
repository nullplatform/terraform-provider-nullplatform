package nullplatform

// Functional: the direct-flip and managed archive lifecycles through real
// terraform applies. Harness in functional_harness_test.go.

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

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
