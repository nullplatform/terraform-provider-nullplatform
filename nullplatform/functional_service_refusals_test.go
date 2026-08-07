package nullplatform

// Functional: the API's refusals surfaced through real applies — the
// unmanaged shortcut, the attributes/archive split, the archived-twin
// collision. Harness in functional_harness_test.go.

import (
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

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
