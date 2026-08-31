package nullplatform

// Functional: the approval machine — a parked archive waited out, a denial
// failing fast. Harness in functional_harness_test.go.

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

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
