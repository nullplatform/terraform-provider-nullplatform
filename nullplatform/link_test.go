package nullplatform

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// The link twin of TestPatchService_SurfacesTheRefusalMessage. Every link-side
// archive refusal — "cannot carry attributes", "use the 'archive' action
// instead", "must be active to unarchive its links" — arrives as a 400 whose
// body is the only thing naming the guard, and it used to go to the provider's
// stdout while the operator saw a bare "got 400".
func TestPatchLink_SurfacesTheRefusalMessage(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"Service svc-1 must be active to unarchive its links"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	err := c.PatchLink("lnk-123", &Link{Status: "active"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error %q should carry the status code", err.Error())
	}
	if !strings.Contains(err.Error(), "must be active to unarchive its links") {
		t.Errorf("error %q should carry the API's message", err.Error())
	}
}

// Creating a link that collides with an ARCHIVED twin is refused with a message
// naming the twin and how to resolve it (unarchive it, or request its
// deletion). That guidance is only useful if the error carries it — the
// previous error shape decoded the body into NullErrors and printed its `Id`
// as a "status code", so the message never reached the operator.
func TestCreateLink_SurfacesTheArchivedTwinMessage(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"An archived link (id lnk-9) with the same specification and service already exists - unarchive it, or request its deletion"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	_, err := c.CreateLink(&Link{Name: "twin-link"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error %q should carry the status code", err.Error())
	}
	if !strings.Contains(err.Error(), "unarchive it, or request its deletion") {
		t.Errorf("error %q should carry the API's resolution guidance", err.Error())
	}
}

// The service half of the same collision (the matrix's create-collision
// "aviso"): CreateService already carried the body in its error, so this only
// pins that the aviso keeps reaching the operator.
func TestCreateService_SurfacesTheArchivedTwinMessage(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"message":"An archived service (id svc-9) with the same specification, entity and dimensions already exists - unarchive it, or request its deletion"}`))
	}))
	defer server.Close()

	c := newTestClient(server)
	_, err := c.CreateService(&Service{Name: "twin-service"})
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !strings.Contains(err.Error(), "unarchive it, or request its deletion") {
		t.Errorf("error %q should carry the API's resolution guidance", err.Error())
	}
}

// The API answers a link delete with 204 No Content; the client treated
// anything but 200 as failure, so every hard link delete errored with
// "got 204" AFTER the row was already gone. Found by the functional fake
// enforcing the API's real status codes on the framework's post-test destroy.
func TestDeleteLink_AcceptsNoContent(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := newTestClient(server).DeleteLink("lnk-1"); err != nil {
		t.Fatalf("a 204 delete is success, got: %v", err)
	}
}
