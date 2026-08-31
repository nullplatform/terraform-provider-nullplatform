package nullplatform

// Functional: the service+link stack — destroy ordering under
// archive_on_destroy, and the restore-under-archived-parent refusal.
// Harness in functional_harness_test.go.

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

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
