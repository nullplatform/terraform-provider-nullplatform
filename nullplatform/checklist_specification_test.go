package nullplatform

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const (
	// specDefinitionAsWritten is specDefinitionV1 with its keys as a .tf would
	// write them; the API answers them in JSONB order, specDefinitionV1's.
	specDefinitionAsWritten = `{"execution_trigger":"explicit","items":[{"title":"Manual review","type":"manual","id":"manual_review"}]}`
	specDefinitionV1        = `{"items":[{"id":"manual_review","type":"manual","title":"Manual review"}],"execution_trigger":"explicit"}`
	specDefinitionV2        = `{"items":[{"id":"manual_review","type":"manual","title":"Manual review"},{"id":"security_review","type":"manual","title":"Security review"}],"execution_trigger":"explicit"}`
)

func newChecklistFake(t *testing.T) (*fakeplatform.Server, *NullClient) {
	t.Helper()
	fake := fakeplatform.New()
	fakeplatform.RegisterApproval(fake)
	fakeplatform.RegisterChecklist(fake)
	t.Cleanup(fake.Close)
	return fake, newTestClient(fake.HTTP())
}

func seedSpec(fake *fakeplatform.Server, id, nrn, name string, version int, status string) {
	fake.Seed(fakeplatform.ChecklistSpecification, id, fakeplatform.Item{
		"nrn": nrn, "name": name, "version": version, "status": status, "description": nil,
		"definition": fakeplatform.Item{"items": []any{fakeplatform.Item{"id": "manual_review"}}},
	})
}

func lastRequest(fake *fakeplatform.Server) fakeplatform.Request {
	requests := fake.Requests()
	return requests[len(requests)-1]
}

// A lineage: the create is version 1, a PATCH inserts version 2 under a new id
// and leaves version 1 as it was, the read of either lists both, and a delete
// keeps the version readable as "deleted". The definition goes out as written
// and comes back as the API orders it.
func TestChecklistSpecification_Lifecycle(t *testing.T) {
	fake, client := newChecklistFake(t)
	first := "first"

	v1, err := client.CreateChecklistSpecification(&ChecklistSpecification{
		Nrn: approvalNrn, Name: "spec", Description: &first, Definition: json.RawMessage(specDefinitionAsWritten),
	})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := lastRequest(fake).Body, `{"nrn":"`+approvalNrn+`","name":"spec","description":"first","definition":`+specDefinitionAsWritten+"}\n"; got != want {
		t.Errorf("create sent %s, want %s", got, want)
	}
	if v1.Id != "spec_0000000000000001" || v1.Version != 1 || v1.Status != "active" || string(v1.Definition) != specDefinitionV1 {
		t.Errorf("created %+v, want spec_0000000000000001, version 1, active, the definition in JSONB order", v1)
	}

	v2, err := client.PatchChecklistSpecification(v1.Id, &ChecklistSpecification{Definition: json.RawMessage(specDefinitionV2)})
	if err != nil {
		t.Fatal(err)
	}
	if got, want := lastRequest(fake).Body, `{"description":null,"definition":`+specDefinitionV2+"}\n"; got != want {
		t.Errorf("patch sent %s, want %s", got, want)
	}
	if v2.Id != "spec_0000000000000002" || v2.Version != 2 || v2.Name != "spec" || v2.Description != nil {
		t.Errorf("patched %+v, want spec_0000000000000002, version 2 of spec, description cleared", v2)
	}

	read, err := client.GetChecklistSpecification(v1.Id)
	if err != nil {
		t.Fatal(err)
	}
	if got := lastRequest(fake); got.Path != CHECKLIST_SPECIFICATION_PATH+"/"+v1.Id || got.Query != "include_versions=true" || got.Body != "" {
		t.Errorf("read asked %s?%s with %q, want the version with include_versions=true and no body", got.Path, got.Query, got.Body)
	}
	if read.Version != 1 || *read.Description != "first" || string(read.Definition) != specDefinitionV1 || len(read.Versions) != 2 ||
		read.Versions[0].Id != v1.Id || read.Versions[1].Id != v2.Id || read.Versions[1].Status != "active" {
		t.Errorf("read v1 = %+v, want version 1 untouched with versions [v1 v2]", read)
	}

	// An action on v2 is listed by v2's read and counted on v2
	// in either read's versions; v1, which no action uses, still deletes.
	fake.Seed(fakeplatform.ApprovalAction, "7", fakeplatform.Item{"nrn": approvalNrn, "entity": "deployment",
		"action": "deployment:create", "status": "active", "checklist_specification_id": v2.Id})
	inUse, err := client.GetChecklistSpecification(v2.Id)
	if err != nil || inUse.AssociatedActionsCount != 1 || len(inUse.AssociatedActions) != 1 || inUse.AssociatedActions[0].Id != 7 {
		t.Errorf("read v2 = %+v, %v; want action 7 on it", inUse, err)
	}
	if read, err = client.GetChecklistSpecification(v1.Id); err != nil || read.AssociatedActionsCount != 0 ||
		read.Versions[0].AssociatedActionsCount != 0 || read.Versions[1].AssociatedActionsCount != 1 {
		t.Errorf("read v1 = %+v, %v; want no action on v1 and one on v2", read, err)
	}

	if err := client.DeleteChecklistSpecification(v1.Id); err != nil {
		t.Fatal(err)
	}
	if body := lastRequest(fake).Body; body != "" {
		t.Errorf("delete sent %q, want no body", body)
	}
	if deleted, err := client.GetChecklistSpecification(v1.Id); err != nil || deleted.Status != "deleted" {
		t.Errorf("read after delete = %+v, %v; want status deleted, no error", deleted, err)
	}
}

// The API's refusals reach the caller with their message and detail; only a
// 404 reads as not found.
func TestChecklistSpecification_Refusals(t *testing.T) {
	const inUse = "spec_0000000000000001"
	tests := []struct {
		name     string
		call     func(*NullClient) error
		want     string
		notFound bool
	}{
		{"invalid definition", func(c *NullClient) error {
			_, err := c.CreateChecklistSpecification(&ChecklistSpecification{Nrn: approvalNrn, Name: "bad", Definition: json.RawMessage(`{"items":[]}`)})
			return err
		}, `error creating checklist specification ` + approvalNrn + `/bad, got 422: Checklist specification validation failed: [{"rule":"items.required","path":"definition.items","message":"items must be a non-empty array"}]`, false},
		{"invalid patch", func(c *NullClient) error {
			_, err := c.PatchChecklistSpecification(inUse, &ChecklistSpecification{Definition: json.RawMessage(`{"items":[]}`)})
			return err
		}, `error updating checklist specification ` + inUse + `, got 422: Checklist specification validation failed: [{"rule":"items.required","path":"definition.items","message":"items must be a non-empty array"}]`, false},
		{"name in use", func(c *NullClient) error {
			_, err := c.CreateChecklistSpecification(&ChecklistSpecification{Nrn: approvalNrn, Name: "live", Definition: json.RawMessage(specDefinitionV1)})
			return err
		}, `error creating checklist specification ` + approvalNrn + `/live, got 400: A checklist specification with the same nrn/name/version already exists`, false},
		{"name of a deleted lineage", func(c *NullClient) error {
			_, err := c.CreateChecklistSpecification(&ChecklistSpecification{Nrn: approvalNrn, Name: "gone", Definition: json.RawMessage(specDefinitionV1)})
			return err
		}, `error creating checklist specification ` + approvalNrn + `/gone, got 400: A checklist specification with the same nrn/name/version already exists`, false},
		{"version in use", func(c *NullClient) error { return c.DeleteChecklistSpecification(inUse) },
			`error deleting checklist specification ` + inUse + `, got 409: Checklist specification is in use by one or more approval actions: Checklist specification ` + inUse + ` is referenced by 1 approval action(s)`, false},
		{"read missing", func(c *NullClient) error { _, err := c.GetChecklistSpecification("spec_missing"); return err },
			`error reading checklist specification spec_missing, got 404: Checklist specification not found`, true},
		{"patch missing", func(c *NullClient) error {
			_, err := c.PatchChecklistSpecification("spec_missing", &ChecklistSpecification{Definition: json.RawMessage(specDefinitionV1)})
			return err
		}, `error updating checklist specification spec_missing, got 404: Checklist specification not found`, true},
		{"delete missing", func(c *NullClient) error { return c.DeleteChecklistSpecification("spec_missing") },
			`error deleting checklist specification spec_missing, got 404: Checklist specification not found`, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, client := newChecklistFake(t)
			seedSpec(fake, inUse, approvalNrn, "live", 1, "active")
			seedSpec(fake, "spec_0000000000000002", approvalNrn, "gone", 1, "deleted")
			fake.Seed(fakeplatform.ApprovalAction, "7", fakeplatform.Item{"nrn": approvalNrn, "status": "active", "checklist_specification_id": inUse})
			before := len(fake.Items(fakeplatform.ChecklistSpecification))

			err := tt.call(client)
			if err == nil || err.Error() != tt.want {
				t.Fatalf("error = %v\nwant %s", err, tt.want)
			}
			if errors.Is(err, errChecklistSpecificationNotFound) != tt.notFound {
				t.Errorf("errors.Is(err, errChecklistSpecificationNotFound) = %v, want %v", !tt.notFound, tt.notFound)
			}
			if errors.Is(err, errApprovalActionNotFound) {
				t.Error("a checklist specification refusal reads as an approval action not found")
			}
			if after := len(fake.Items(fakeplatform.ChecklistSpecification)); after != before {
				t.Errorf("%d versions after the refusal, want %d", after, before)
			}
		})
	}
}

// The list is the newest live version of each lineage at exactly the nrn —
// not its ancestors', not a deleted version — read page by page.
func TestChecklistSpecification_ListLatestAtNrn(t *testing.T) {
	fake, client := newChecklistFake(t)
	pageSize := checklistSpecificationPageSize
	checklistSpecificationPageSize = 1
	t.Cleanup(func() { checklistSpecificationPageSize = pageSize })
	seedSpec(fake, "spec_0000000000000001", approvalNrn, "edited", 1, "active")
	seedSpec(fake, "spec_0000000000000002", approvalNrn, "edited", 2, "active")
	seedSpec(fake, "spec_0000000000000003", approvalNrn, "rolled-back", 1, "active")
	seedSpec(fake, "spec_0000000000000004", approvalNrn, "rolled-back", 2, "deleted")
	seedSpec(fake, "spec_0000000000000005", "organization=1:account=2:namespace=3", "inherited", 1, "active")
	seedSpec(fake, "spec_0000000000000006", approvalNrn+"0", "elsewhere", 1, "active")

	specs, err := client.ListChecklistSpecifications(approvalNrn)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, spec := range specs {
		got = append(got, fmt.Sprintf("%s v%d %s", spec.Name, spec.Version, spec.Id))
	}
	if want := "[edited v2 spec_0000000000000002 rolled-back v1 spec_0000000000000003]"; fmt.Sprint(got) != want {
		t.Errorf("listed %v, want %s", got, want)
	}
	requests := fake.Requests()
	if len(requests) != 2 || requests[0].Query != "latest_only=true&limit=1&no-merge=true&nrn=organization%3D1%3Aaccount%3D2%3Anamespace%3D3%3Aapplication%3D4&offset=0" ||
		requests[1].Query != "latest_only=true&limit=1&no-merge=true&nrn=organization%3D1%3Aaccount%3D2%3Anamespace%3D3%3Aapplication%3D4&offset=1" {
		t.Errorf("asked %v, want two pages of one", requests)
	}
	if _, err := client.ListChecklistSpecifications("organization=1:account=9"); err != nil {
		t.Errorf("an empty nrn listed with %v", err)
	}
}

// What never reaches the API, or comes back from something that is not the
// API, still surfaces: a definition that is not JSON, a connection that fails,
// a gateway's page or message (their echoed credentials redacted), a body
// without a message, a 404 that is not the API's and a server error.
func TestChecklistSpecification_TransportErrors(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"html", "<html>502 Bad Gateway</html>\n", "<html>502 Bad Gateway</html>"},
		{"html echoing the request", "<html>upstream DELETE (headers: Authorization: Bearer test-token) {\"access_token\":\"t0k\"}</html>",
			`<html>upstream DELETE (headers: Authorization: Bearer REDACTED) {"access_token":"REDACTED"}</html>`},
		{"json without a message", `{"error":"x"}`, `{"error":"x"}`},
		{"problem echoing a token", `{"message":"token Bearer eyJhbGciOiJIUzI1NiJ9.e30.c2ln rejected","detail":"Authorization: Bearer test-token"}`,
			"token Bearer REDACTED rejected: Authorization: Bearer REDACTED"},
		{"prose after a scheme", `{"message":"Digest auth required"}`, "Digest auth required"},
	} {
		gateway := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(tc.body))
		}))
		if err := newTestClient(gateway).DeleteChecklistSpecification("spec_1"); err == nil ||
			err.Error() != "error deleting checklist specification spec_1, got 502: "+tc.want {
			t.Errorf("%s: error = %v", tc.name, err)
		}
		gateway.Close()
	}

	elsewhere := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("<html>404 Not Found</html>"))
	}))
	defer elsewhere.Close()
	if _, err := newTestClient(elsewhere).GetChecklistSpecification("spec_1"); err == nil || errors.Is(err, errChecklistSpecificationNotFound) {
		t.Errorf("a 404 that is not the API's read as %v, want an error that is not \"not found\"", err)
	}

	failing := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"type":"about:blank","title":"There was an error","status":500,"detail":null,"message":"There was an error","id":0}`))
	}))
	defer failing.Close()
	if _, err := newTestClient(failing).ListChecklistSpecifications(approvalNrn); err == nil ||
		err.Error() != "error listing checklist specifications of "+approvalNrn+", got 500: There was an error" {
		t.Errorf("list error = %v", err)
	}

	closed := httptest.NewTLSServer(http.NotFoundHandler())
	client := newTestClient(closed)
	closed.Close()
	if err := client.DeleteChecklistSpecification("spec_1"); err == nil {
		t.Error("a delete against a closed server answered no error")
	}

	if _, err := client.CreateChecklistSpecification(&ChecklistSpecification{Definition: json.RawMessage(`{`)}); err == nil {
		t.Error("a definition that is not JSON was sent")
	}
}

// A list whose total is more than the pages hold ends when a page comes back
// empty, and a lineage that comes twice across pages is kept once.
func TestChecklistSpecification_ListEndsAndDeduplicates(t *testing.T) {
	pages := []string{
		`{"results":[{"id":"spec_a"},{"id":"spec_b"},{"id":"spec_c"}],"total":10}`,
		`{"results":[{"id":"spec_c"}],"total":10}`,
		`{"results":[],"total":10}`,
	}
	calls := 0
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		calls++
		if calls > len(pages) {
			// A client that keeps asking past the empty page fails here
			// instead of hanging the suite.
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		_, _ = w.Write([]byte(pages[calls-1]))
	}))
	defer server.Close()

	specs, err := newTestClient(server).ListChecklistSpecifications(approvalNrn)
	var ids []string
	for _, spec := range specs {
		ids = append(ids, spec.Id)
	}
	if err != nil || fmt.Sprint(ids) != "[spec_a spec_b spec_c]" || calls != 3 {
		t.Errorf("listed %v, %v in %d calls; want [spec_a spec_b spec_c] in 3", ids, err, calls)
	}
}

// An id is a path segment, never part of the query: one with a "?" still asks
// include_versions=true.
func TestChecklistSpecification_IDIsEscaped(t *testing.T) {
	fake, client := newChecklistFake(t)
	_, err := client.GetChecklistSpecification("spec_a?b")
	if got := lastRequest(fake); !errors.Is(err, errChecklistSpecificationNotFound) ||
		got.Path != CHECKLIST_SPECIFICATION_PATH+"/spec_a?b" || got.Query != "include_versions=true" {
		t.Errorf("asked %s?%s (%v), want the id whole and include_versions=true", got.Path, got.Query, err)
	}
}

// The rules the resource stands on: lineages are (nrn, name) — two in one nrn,
// one name in two nrns —, a read lists only its own lineage, and a PATCH of
// the description alone makes a version that keeps the definition.
func TestChecklistSpecification_LineagesAreNrnAndName(t *testing.T) {
	_, client := newChecklistFake(t)
	other := "organization=1:account=2:namespace=3:application=5"
	for _, spec := range []ChecklistSpecification{{Nrn: approvalNrn, Name: "first"}, {Nrn: approvalNrn, Name: "second"}, {Nrn: other, Name: "first"}} {
		spec.Definition = json.RawMessage(specDefinitionV1)
		if _, err := client.CreateChecklistSpecification(&spec); err != nil {
			t.Errorf("create %s/%s: %v", spec.Nrn, spec.Name, err)
		}
	}

	described := "described"
	next, err := client.PatchChecklistSpecification("spec_0000000000000001", &ChecklistSpecification{Description: &described})
	if err != nil || next.Version != 2 || *next.Description != described || string(next.Definition) != specDefinitionV1 {
		t.Fatalf("patched %+v, %v; want version 2, described, the definition kept", next, err)
	}
	read, err := client.GetChecklistSpecification("spec_0000000000000001")
	if err != nil || len(read.Versions) != 2 || read.Versions[0].Name != "first" || read.Versions[1].Id != next.Id ||
		read.Versions[0].Nrn != approvalNrn || read.Versions[1].Nrn != approvalNrn {
		t.Errorf("read %+v, %v; want only first's two versions at %s", read.Versions, err, approvalNrn)
	}
}

// The definition comes back as the API answers it, whatever it was stored as:
// the API's key order (integer keys first), no HTML
// escaping.
func TestChecklistSpecification_DefinitionAsTheAPIAnswers(t *testing.T) {
	fake, client := newChecklistFake(t)
	for version := 1; version <= 2; version++ {
		fake.Seed(fakeplatform.ChecklistSpecification, specVersionID(version), fakeplatform.Item{
			"nrn": approvalNrn, "name": "seeded", "version": version, "status": "active",
			"definition": fakeplatform.Item{"execution_trigger": "explicit", "10": "ten", "2": "two", "a": "x",
				"items": []any{fakeplatform.Item{"title": "R&D <review>", "type": "manual", "id": "rd"}}},
		})
	}

	// Version 2 is never read on its own: its bytes come from the lineage.
	read, err := client.GetChecklistSpecification(specVersionID(1))
	want := `{"2":"two","10":"ten","a":"x","items":[{"id":"rd","type":"manual","title":"R&D <review>"}],"execution_trigger":"explicit"}`
	if err != nil || string(read.Definition) != want || len(read.Versions) != 2 || string(read.Versions[1].Definition) != want {
		t.Errorf("read %s (versions %+v), %v\nwant %s", read.Definition, read.Versions, err, want)
	}
}
