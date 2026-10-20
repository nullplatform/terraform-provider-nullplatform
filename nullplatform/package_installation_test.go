package nullplatform

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// installationAPI answers every request with one status and body, and records
// the last request so a test can assert what went on the wire.
func installationAPI(t *testing.T, status int, body string) (*NullClient, *http.Request, *string) {
	t.Helper()
	seen := &http.Request{}
	sent := new(string)
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*seen = *r
		raw, _ := io.ReadAll(r.Body)
		*sent = string(raw)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return newTestClient(server), seen, sent
}

// unreachableClient points at a server that is already closed.
func unreachableClient() *NullClient {
	server := httptest.NewTLSServer(http.NotFoundHandler())
	client := newTestClient(server)
	server.Close()
	return client
}

func TestPackageInstallationClient_CreateOmitsWhatTheOperatorLeftOut(t *testing.T) {
	client, seen, sent := installationAPI(t, http.StatusCreated, `{"id":"inst-1","default_revision_id":"rev-10"}`)

	installation, err := client.CreatePackageInstallation(&PackageInstallationCreate{Nrn: "organization=1", PackageID: "pkg"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if seen.Method != http.MethodPost || seen.URL.Path != PACKAGE_INSTALLATION_PATH {
		t.Errorf("request = %s %s, want POST %s", seen.Method, seen.URL.Path, PACKAGE_INSTALLATION_PATH)
	}
	// An omitted revision must not be sent as "" — the API resolves the
	// package default only when the field is absent.
	if strings.Contains(*sent, "default_revision_id") || strings.Contains(*sent, "visible_to") {
		t.Errorf("body %s carries fields the operator did not set", *sent)
	}
	if installation.ID != "inst-1" || installation.DefaultRevisionID != "rev-10" {
		t.Errorf("decoded %+v", installation)
	}
}

func TestPackageInstallationClient_GetOfAMissingInstallationIsNotFound(t *testing.T) {
	client, _, _ := installationAPI(t, http.StatusNotFound, `{"message":"not found"}`)

	_, err := client.GetPackageInstallation("inst-1")
	if !errors.Is(err, errPackageInstallationNotFound) {
		t.Fatalf("err = %v, want errPackageInstallationNotFound", err)
	}
}

func TestPackageInstallationClient_RefusalsCarryTheAPIMessageVerbatim(t *testing.T) {
	client, _, _ := installationAPI(t, http.StatusBadRequest, `{"message":"Revision rev-x is deprecated"}`)

	_, err := client.PatchPackageInstallation("inst-1", &PackageInstallationPatch{DefaultRevisionID: "rev-x"})
	if err == nil || err.Error() != "API error updating package installation: Revision rev-x is deprecated" {
		t.Fatalf("err = %v", err)
	}
}

func TestPackageInstallationClient_AnErrorWithoutAMessageReportsStatusAndBody(t *testing.T) {
	client, _, _ := installationAPI(t, http.StatusBadGateway, `upstream down`)

	_, err := client.GetPackageInstallation("inst-1")
	if err == nil || !strings.Contains(err.Error(), "got status code: 502, body: upstream down") {
		t.Fatalf("err = %v", err)
	}
}

func TestPackageInstallationClient_AnUndecodableSuccessIsAnError(t *testing.T) {
	client, _, _ := installationAPI(t, http.StatusOK, `not json`)

	_, err := client.GetPackageInstallation("inst-1")
	if err == nil || !strings.Contains(err.Error(), "error decoding package installation") {
		t.Fatalf("err = %v", err)
	}
}

func TestPackageInstallationClient_DeleteTreatsGoneAsDone(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusNoContent, http.StatusNotFound} {
		client, seen, _ := installationAPI(t, status, ``)
		if err := client.DeletePackageInstallation("inst-1"); err != nil {
			t.Errorf("status %d: %v", status, err)
		}
		if seen.Method != http.MethodDelete || seen.URL.Path != PACKAGE_INSTALLATION_PATH+"/inst-1" {
			t.Errorf("request = %s %s", seen.Method, seen.URL.Path)
		}
	}
}

func TestPackageInstallationClient_DeleteRefusalCarriesTheAPIMessage(t *testing.T) {
	message := "Package installation inst-1 cannot be deleted: 1 service instance(s) under its visible_to were born from it"
	client, _, _ := installationAPI(t, http.StatusBadRequest, `{"message":"`+message+`"}`)

	err := client.DeletePackageInstallation("inst-1")
	if err == nil || err.Error() != "API error deleting package installation: "+message {
		t.Fatalf("err = %v", err)
	}
}

func TestPackageInstallationClient_AnUnreachableAPIFailsEveryVerb(t *testing.T) {
	client := unreachableClient()

	if _, err := client.CreatePackageInstallation(&PackageInstallationCreate{}); err == nil || !strings.Contains(err.Error(), "error making POST request") {
		t.Errorf("create: %v", err)
	}
	if _, err := client.GetPackageInstallation("inst-1"); err == nil || !strings.Contains(err.Error(), "error making GET request") {
		t.Errorf("get: %v", err)
	}
	if _, err := client.PatchPackageInstallation("inst-1", &PackageInstallationPatch{}); err == nil || !strings.Contains(err.Error(), "error making PATCH request") {
		t.Errorf("patch: %v", err)
	}
	if err := client.DeletePackageInstallation("inst-1"); err == nil || !strings.Contains(err.Error(), "error making DELETE request") {
		t.Errorf("delete: %v", err)
	}
}

// truncatedAPI promises a longer body than it sends, so reading the response
// fails mid-body, as a connection dropped by a proxy does.
func truncatedAPI(t *testing.T, status int) *NullClient {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Length", "100")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(`{"id"`))
	}))
	t.Cleanup(server.Close)
	return newTestClient(server)
}

func TestPackageInstallationClient_AResponseCutShortIsAnError(t *testing.T) {
	if _, err := truncatedAPI(t, http.StatusOK).GetPackageInstallation("inst-1"); err == nil || !strings.Contains(err.Error(), "error reading response body") {
		t.Errorf("get: %v", err)
	}
	if err := truncatedAPI(t, http.StatusBadRequest).DeletePackageInstallation("inst-1"); err == nil || !strings.Contains(err.Error(), "error reading response body") {
		t.Errorf("delete: %v", err)
	}
}

// A read that fails for any reason other than "gone" must fail the plan, not
// drop the installation from state: dropping it would offer to install again.
func TestPackageInstallationResource_AReadFailureKeepsTheInstallationInState(t *testing.T) {
	client, _, _ := installationAPI(t, http.StatusInternalServerError, `{"message":"database unavailable"}`)
	d := resourcePackageInstallation().TestResourceData()
	d.SetId("inst-1")

	diags := packageInstallationResourceRead(context.Background(), d, client)
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "database unavailable") {
		t.Fatalf("diags = %v", diags)
	}
	if d.Id() != "inst-1" {
		t.Errorf("id = %q, want the installation kept in state", d.Id())
	}
}
