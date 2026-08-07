package nullplatform

// Functional tests: the HashiCorp-recommended harness (terraform-plugin-testing)
// driving REAL `terraform plan / apply / import / destroy` cycles against the
// in-process provider, backed by fakePlatform — a hermetic, STATEFUL copy of
// the platform API's archive contract. Every rule the fake enforces is a
// behavior any client can observe against the real API; the fake is the
// provider's executable copy of that contract, and `make testacc` remains the
// on-demand check that the real API still agrees.
//
//   - the provider is the real one — schema, CRUD Contexts, waiters — with only
//     ConfigureContextFunc swapped to a client that trusts the fake's TLS cert.
//   - `resource.UnitTest` is `resource.Test` without the TF_ACC gate: the
//     framework's own mode for mock-backed runs. No credentials, runs on every
//     `go test` and in CI.
//   - after every apply the framework re-plans and fails on any diff, so the
//     perpetual-diff regression class is checked on every step for free.
//
// A service's archive chain is selected by its NAME (the fake's one liberty):
//   contains "managed"  -> managed archive/unarchive actions: transitions are
//                          asynchronous, the PATCH only starts them
//   contains "manual"   -> an archive action exists but is NOT managed: the
//                          status shortcut is refused
//   contains "approval" -> managed behind an approval: the action parks in
//                          pending_create; "denied" additionally cancels it
//   otherwise           -> no actions: the direct flip
import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const fakeArchivedAt = "2026-08-07T12:00:00.000Z"

type fakePlatform struct {
	mu       sync.Mutex
	services map[string]*Service
	links    map[string]*Link
	gets     map[string]int
	deletes  int
	server   *httptest.Server
}

func newFakePlatform(t *testing.T) *fakePlatform {
	t.Helper()
	fake := &fakePlatform{
		services: map[string]*Service{},
		links:    map[string]*Link{},
		gets:     map[string]int{},
	}
	fake.server = httptest.NewTLSServer(http.HandlerFunc(fake.handle))
	t.Cleanup(fake.server.Close)
	return fake
}

func refuse(w http.ResponseWriter, format string, args ...any) {
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"message": fmt.Sprintf(format, args...)})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

func (f *fakePlatform) handle(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case strings.HasPrefix(r.URL.Path, "/service"):
		f.handleService(w, r, strings.TrimPrefix(r.URL.Path, "/service/"))
	case strings.HasPrefix(r.URL.Path, "/link"):
		f.handleLink(w, r, strings.TrimPrefix(r.URL.Path, "/link/"))
	default:
		http.Error(w, `{"message":"unhandled route in fakePlatform"}`, http.StatusInternalServerError)
	}
}

func (f *fakePlatform) handleService(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case "POST":
		var s Service
		_ = json.NewDecoder(r.Body).Decode(&s)
		// The create-collision rule: a new service colliding with an ARCHIVED
		// twin is refused with the aviso naming the twin and the way out.
		for _, existing := range f.services {
			if existing.Status == "archived" && existing.Name == s.Name &&
				existing.SpecificationId == s.SpecificationId && existing.EntityNrn == s.EntityNrn {
				refuse(w, "An archived service (id %s) with the same specification, entity and dimensions "+
					"already exists - unarchive it, or request its deletion", existing.Id)
				return
			}
		}
		s.Id = fmt.Sprintf("svc-%d", len(f.services)+1)
		if s.Attributes == nil {
			s.Attributes = map[string]interface{}{}
		}
		f.services[s.Id] = &s
		writeJSON(w, s)

	case "GET":
		s, ok := f.services[id]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		f.gets[id]++
		f.progressService(s)
		writeJSON(w, *s)

	case "PATCH":
		s, ok := f.services[id]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		var patch Service
		_ = json.NewDecoder(r.Body).Decode(&patch)
		if diagMsg := f.patchService(s, &patch); diagMsg != "" {
			refuse(w, "%s", diagMsg)
			return
		}
		writeJSON(w, *s)

	case "DELETE":
		f.deletes++
		delete(f.services, id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// progressService advances asynchronous transitions one observation at a time,
// so the provider's waiter genuinely waits.
func (f *fakePlatform) progressService(s *Service) {
	switch {
	case s.Status == "archiving" && f.gets[s.Id] > 1:
		s.Status = "archived"
		s.ArchivedAt = fakeArchivedAt
	case s.Status == "updating" && f.gets[s.Id] > 1:
		s.Status = "active"
		s.ArchivedAt = ""
	case len(s.ActionsInProgress) > 0 && s.ActionsInProgress[0].Status == "pending_create":
		if f.gets[s.Id] > 2 {
			if strings.Contains(s.Name, "denied") {
				// Denial: the parked action vanishes; the status never moves.
				s.ActionsInProgress = nil
				return
			}
			// Approval: the action starts running and the transition begins.
			s.ActionsInProgress[0].Status = "in_progress"
			s.Status = "archiving"
			f.gets[s.Id] = 0
		}
	}
}

// patchService is the archive contract for services; it returns the refusal
// message, or "" when the patch is applied.
func (f *fakePlatform) patchService(s *Service, patch *Service) string {
	isArchive := patch.Status == "archived"
	isRestore := patch.Status == "active" && s.Status == "archived"

	if (isArchive || isRestore) && patch.Attributes != nil {
		return "An archive request cannot carry attributes. Apply attribute changes separately, " +
			"then PATCH the status on its own."
	}

	switch {
	case isArchive:
		if strings.Contains(s.Name, "manual") {
			return "The specification defines an 'archive' action; use the 'archive' action instead of PATCHing the status"
		}
		if s.Status != "active" && s.Status != "failed" && s.Status != "cancelled" {
			return "Only active, cancelled or failed services can be archived"
		}
		for _, l := range f.links {
			if l.ServiceId == s.Id && l.Status != "archived" {
				return "Service has non-archived links and cannot be archived; archive its links first"
			}
		}
		switch {
		case strings.Contains(s.Name, "approval"):
			s.ActionsInProgress = []ActionInProgress{{Id: "act-1", Type: "archive", Status: "pending_create"}}
			f.gets[s.Id] = 0
		case strings.Contains(s.Name, "managed"):
			s.Status = "archiving"
			f.gets[s.Id] = 0
		default:
			s.Status = "archived"
			s.ArchivedAt = fakeArchivedAt
		}
	case isRestore:
		if strings.Contains(s.Name, "managed") {
			s.Status = "updating"
			f.gets[s.Id] = 0
		} else {
			s.Status = "active"
			s.ArchivedAt = ""
		}
	case patch.Status != "":
		s.Status = patch.Status
	}
	if patch.Attributes != nil {
		s.Attributes = patch.Attributes
	}
	return ""
}

func (f *fakePlatform) handleLink(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case "POST":
		var l Link
		_ = json.NewDecoder(r.Body).Decode(&l)
		l.Id = fmt.Sprintf("lnk-%d", len(f.links)+1)
		if l.Attributes == nil {
			l.Attributes = map[string]interface{}{}
		}
		l.Status = "active"
		f.links[l.Id] = &l
		writeJSON(w, l)

	case "GET":
		l, ok := f.links[id]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		writeJSON(w, *l)

	case "PATCH":
		l, ok := f.links[id]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		var patch Link
		_ = json.NewDecoder(r.Body).Decode(&patch)
		switch {
		case patch.Status == "archived":
			l.Status = "archived"
			l.ArchivedAt = fakeArchivedAt
		case patch.Status == "active" && l.Status == "archived":
			// A link only comes back under a working parent.
			if parent, ok := f.services[l.ServiceId]; ok && parent.Status != "active" {
				refuse(w, "Service %s must be active to unarchive its links", l.ServiceId)
				return
			}
			l.Status = "active"
			l.ArchivedAt = ""
		case patch.Status != "":
			l.Status = patch.Status
		}
		if patch.Attributes != nil {
			l.Attributes = patch.Attributes
		}
		writeJSON(w, *l)

	case "DELETE":
		f.deletes++
		delete(f.links, id)
		w.WriteHeader(http.StatusNoContent)
	}
}

// functionalFactories serves the REAL provider with only its configure swapped
// for a client that trusts the fake's certificate.
func functionalFactories(fake *fakePlatform) map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"nullplatform": func() (*schema.Provider, error) {
			provider := Provider()
			provider.ConfigureContextFunc = func(_ context.Context, _ *schema.ResourceData) (any, diag.Diagnostics) {
				return newTestClient(fake.server), nil
			}
			return provider, nil
		},
	}
}

func serviceConfig(name, extra string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_service" "db" {
  name             = %q
  specification_id = "spec-1"
  entity_nrn       = "organization=1:account=2"
  import           = true
%s
}
`, name, extra)
}

// --- The direct-flip chain -------------------------------------------------

// The full archive lifecycle through real Terraform: create, archive by
// status, restore by status, import — the framework re-planning after every
// apply and failing on any diff.
func TestFunctionalService_ArchiveLifecycle(t *testing.T) {
	shortenPolling(t)
	fake := newFakePlatform(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: serviceConfig("functional-redis", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "active"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "archived_at", ""),
				),
			},
			{
				Config: serviceConfig("functional-redis", `  status = "archived"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "archived"),
					resource.TestCheckResourceAttrSet("nullplatform_service.db", "archived_at"),
				),
			},
			{
				Config: serviceConfig("functional-redis", `  status = "active"`),
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
	fake := newFakePlatform(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: serviceConfig("managed-redis", "")},
			{
				Config: serviceConfig("managed-redis", `  status = "archived"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "archived"),
					resource.TestCheckResourceAttrSet("nullplatform_service.db", "archived_at"),
				),
			},
			{
				Config: serviceConfig("managed-redis", `  status = "active"`),
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
	fake := newFakePlatform(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: serviceConfig("manual-redis", "")},
			{
				Config:      serviceConfig("manual-redis", `  status = "archived"`),
				ExpectError: regexp.MustCompile(`use the 'archive' action`),
			},
			// The refusal left the service untouched and the diff re-offers.
			{
				Config: serviceConfig("manual-redis", ""),
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
	fake := newFakePlatform(t)

	withAttrs := func(extra string) string {
		return serviceConfig("functional-redis", "  attributes = { size = \"small\" }\n"+extra)
	}
	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: withAttrs("")},
			{
				Config: serviceConfig("functional-redis", "  attributes = { size = \"large\" }\n  status = \"archived\""),
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
	fake := newFakePlatform(t)

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
			{Config: serviceConfig("functional-redis", `  status = "archived"`)},
			{
				Config:      serviceConfig("functional-redis", `  status = "archived"`) + twin,
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
	fake := newFakePlatform(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: serviceConfig("approval-redis", "")},
			{
				Config: serviceConfig("approval-redis", `  status = "archived"`),
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
	fake := newFakePlatform(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: serviceConfig("approval-denied-redis", "")},
			{
				Config:      serviceConfig("approval-denied-redis", `  status = "archived"`),
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
	fake := newFakePlatform(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		CheckDestroy: func(*terraform.State) error {
			fake.mu.Lock()
			defer fake.mu.Unlock()
			if fake.deletes != 0 {
				return fmt.Errorf("destroy issued %d DELETE(s); archive_on_destroy must never hard-delete", fake.deletes)
			}
			for _, s := range fake.services {
				if s.Status != "archived" {
					return fmt.Errorf("service %s ended %q, want archived", s.Id, s.Status)
				}
			}
			for _, l := range fake.links {
				if l.Status != "archived" {
					return fmt.Errorf("link %s ended %q, want archived", l.Id, l.Status)
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
	fake := newFakePlatform(t)

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
