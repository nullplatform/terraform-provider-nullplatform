package nullplatform

// Functional tests: the HashiCorp-recommended harness (terraform-plugin-testing)
// driving REAL `terraform plan / apply / import / destroy` cycles against the
// in-process provider, backed by a hermetic in-memory platform API. This is the
// layer between the unit tests (Go functions against httptest fakes) and the
// TF_ACC acceptance tests (real credentials, `make testacc`):
//
//   - the provider is the real one — schema, CRUD Contexts, waiters — with only
//     ConfigureContextFunc swapped to hand back a client that trusts the mock's
//     TLS certificate. Configure itself is not the subject under test.
//   - `resource.UnitTest` is `resource.Test` without the TF_ACC gate: it is the
//     framework's own escape hatch for mock-backed runs, so these execute on
//     every `go test` and in CI, no credentials involved.
//   - after every apply the framework refreshes and re-plans, failing on any
//     non-empty plan — the perpetual-diff class of regression is checked on
//     every step without writing a single explicit assertion for it.
//
// The first local run downloads a terraform CLI if none is installed
// (hc-install); CI pins one with hashicorp/setup-terraform.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// platformMock is a minimal, STATEFUL platform API: enough of /service for the
// full resource lifecycle, with archive semantics matching the real API — a
// direct flip by default, or an async `archiving` transition when created with
// managedArchive, so the provider's waiter is exercised through real applies.
type platformMock struct {
	mu             sync.Mutex
	services       map[string]*Service
	gets           map[string]int
	deletes        int
	managedArchive bool
	server         *httptest.Server
}

func newPlatformMock(t *testing.T, managedArchive bool) *platformMock {
	t.Helper()
	mock := &platformMock{
		services:       map[string]*Service{},
		gets:           map[string]int{},
		managedArchive: managedArchive,
	}
	mock.server = httptest.NewTLSServer(http.HandlerFunc(mock.handle))
	t.Cleanup(mock.server.Close)
	return mock
}

func (m *platformMock) handle(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := strings.TrimPrefix(r.URL.Path, "/service/")
	switch {
	case r.Method == "POST" && r.URL.Path == "/service":
		var s Service
		_ = json.NewDecoder(r.Body).Decode(&s)
		s.Id = fmt.Sprintf("svc-%d", len(m.services)+1)
		if s.Attributes == nil {
			s.Attributes = map[string]interface{}{}
		}
		m.services[s.Id] = &s
		writeJSON(w, s)

	case r.Method == "GET":
		s, ok := m.services[id]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		m.gets[id]++
		// A managed archive lands after the transition has been observed
		// in flight at least once, so the waiter genuinely waits.
		if s.Status == "archiving" && m.gets[id] > 1 {
			s.Status = "archived"
			s.ArchivedAt = "2026-08-07T12:00:00.000Z"
		}
		writeJSON(w, *s)

	case r.Method == "PATCH":
		s, ok := m.services[id]
		if !ok {
			http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
			return
		}
		var patch Service
		_ = json.NewDecoder(r.Body).Decode(&patch)
		switch {
		case patch.Status == "archived" && m.managedArchive:
			s.Status = "archiving"
			m.gets[id] = 0
		case patch.Status == "archived":
			s.Status = "archived"
			s.ArchivedAt = "2026-08-07T12:00:00.000Z"
		case patch.Status == "active" && s.Status == "archived":
			s.Status = "active"
			s.ArchivedAt = ""
		case patch.Status != "":
			s.Status = patch.Status
		}
		if patch.Attributes != nil {
			s.Attributes = patch.Attributes
		}
		writeJSON(w, *s)

	case r.Method == "DELETE":
		m.deletes++
		delete(m.services, id)
		w.WriteHeader(http.StatusNoContent)

	default:
		http.Error(w, `{"message":"unhandled route in platformMock"}`, http.StatusInternalServerError)
	}
}

func writeJSON(w http.ResponseWriter, v any) {
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(v)
}

// functionalFactories serves the REAL provider with only its configure swapped
// for a client that trusts the mock's certificate.
func functionalFactories(mock *platformMock) map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"nullplatform": func() (*schema.Provider, error) {
			provider := Provider()
			provider.ConfigureContextFunc = func(_ context.Context, _ *schema.ResourceData) (any, diag.Diagnostics) {
				return newTestClient(mock.server), nil
			}
			return provider, nil
		},
	}
}

const functionalServiceConfig = `
provider "nullplatform" {}

resource "nullplatform_service" "db" {
  name             = "functional-redis"
  specification_id = "spec-1"
  entity_nrn       = "organization=1:account=2"
  import           = true
%s
}
`

// The full archive lifecycle through real Terraform: create, archive by
// status, restore by status, import — with the framework re-planning after
// every apply and failing on any diff.
func TestFunctionalService_ArchiveLifecycle(t *testing.T) {
	shortenPolling(t)
	mock := newPlatformMock(t, false)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(mock),
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(functionalServiceConfig, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "active"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "archived_at", ""),
				),
			},
			{
				Config: fmt.Sprintf(functionalServiceConfig, `  status = "archived"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "archived"),
					resource.TestCheckResourceAttrSet("nullplatform_service.db", "archived_at"),
				),
			},
			{
				Config: fmt.Sprintf(functionalServiceConfig, `  status = "active"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "active"),
					resource.TestCheckResourceAttr("nullplatform_service.db", "archived_at", ""),
				),
			},
			{
				ResourceName:      "nullplatform_service.db",
				ImportState:       true,
				ImportStateVerify: true,
				// Local-only knobs the API cannot echo back.
				ImportStateVerifyIgnore: []string{"import", "force_destroy", "archive_on_destroy"},
			},
		},
	})
}

// A managed archive: the PATCH only starts the transition and the apply must
// wait it out — through a real terraform apply, not a direct waiter call.
func TestFunctionalService_ManagedArchiveWaits(t *testing.T) {
	shortenPolling(t)
	mock := newPlatformMock(t, true)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(mock),
		Steps: []resource.TestStep{
			{Config: fmt.Sprintf(functionalServiceConfig, "")},
			{
				Config: fmt.Sprintf(functionalServiceConfig, `  status = "archived"`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_service.db", "status", "archived"),
					resource.TestCheckResourceAttrSet("nullplatform_service.db", "archived_at"),
				),
			},
		},
	})
}

// archive_on_destroy through a real terraform destroy: the row survives as
// archived and no DELETE is ever issued.
func TestFunctionalService_ArchiveOnDestroy(t *testing.T) {
	shortenPolling(t)
	mock := newPlatformMock(t, false)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(mock),
		CheckDestroy: func(*terraform.State) error {
			mock.mu.Lock()
			defer mock.mu.Unlock()
			if mock.deletes != 0 {
				return fmt.Errorf("destroy issued %d DELETE(s); archive_on_destroy must never hard-delete", mock.deletes)
			}
			for _, s := range mock.services {
				if s.Status == "archived" {
					return nil
				}
			}
			return fmt.Errorf("no archived service survived the destroy; store: %+v", mock.services)
		},
		Steps: []resource.TestStep{
			{
				Config: fmt.Sprintf(functionalServiceConfig, `  archive_on_destroy = true`),
				Check:  resource.TestCheckResourceAttr("nullplatform_service.db", "status", "active"),
			},
		},
	})
}
