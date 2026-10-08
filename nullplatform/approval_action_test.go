package nullplatform

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"
)

// A failed GET is an error that carries the API's message, whatever shape its
// body has (the API's problem documents carry a numeric "status"); only a 404
// says the action is gone.
func TestGetApprovalAction_ErrorStatuses(t *testing.T) {
	tests := []struct {
		name     string
		status   int
		body     string
		want     string
		notFound bool
	}{
		{"not found", http.StatusNotFound, `{"message":"This is not a nullplatform.io entity or the HTTP verb is not supported."}`,
			"approval action not found: 9", true},
		{"missing or not readable", http.StatusUnauthorized, `{"type":"about:blank","title":"You're not authorized to perform this operation.","status":401,"code":"unauthorized","detail":null,"message":"You're not authorized to perform this operation."}`,
			"approval action 9 not found or not readable with this API key: You're not authorized to perform this operation.", false},
		// The API answers 403 when the key is authenticated but lacks the
		// permission; the text is this test's, not observed against the API.
		{"forbidden", http.StatusForbidden, `{"type":"about:blank","title":"Forbidden","status":403,"code":"forbidden","detail":null,"message":"Forbidden"}`,
			"approval action 9 not found or not readable with this API key: Forbidden", false},
		{"server error", http.StatusInternalServerError, `{"status":500,"message":"There was an error"}`,
			"error getting approval action resource, got 500 for 9: There was an error", false},
		// A 404 that is not the router's (a gateway's page) is not the API
		// saying the action is gone: it is an error, with the page.
		{"404 from elsewhere", http.StatusNotFound, "<html>404 Not Found</html>\n",
			"error getting approval action resource, got 404 for 9: <html>404 Not Found</html>", false},
		// A message that echoes the request's credentials is redacted, JSON or
		// not; prose after a scheme's name is not a credential.
		{"message echoing a token", http.StatusUnauthorized, `{"status":401,"message":"token Bearer eyJhbGciOiJIUzI1NiJ9.e30.c2ln rejected"}`,
			"approval action 9 not found or not readable with this API key: token Bearer REDACTED rejected", false},
		{"json without a message", http.StatusInternalServerError, `{"error":"x"}`,
			`error getting approval action resource, got 500 for 9: {"error":"x"}`, false},
		{"page echoing the header", http.StatusInternalServerError, "<html>upstream (Authorization: Bearer test-token) Digest auth required</html>",
			"error getting approval action resource, got 500 for 9: <html>upstream (Authorization: Bearer REDACTED) Digest auth required</html>", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				_, _ = w.Write([]byte(tt.body))
			}))
			defer server.Close()

			action, err := newTestClient(server).GetApprovalAction("9")
			if action != nil || err == nil || err.Error() != tt.want {
				t.Fatalf("GetApprovalAction = %v, %v; want nil, %q", action, err, tt.want)
			}
			if errors.Is(err, errApprovalActionNotFound) != tt.notFound {
				t.Errorf("errors.Is(err, errApprovalActionNotFound) = %v, want %v", !tt.notFound, tt.notFound)
			}
		})
	}
}

// The create and the policy association carry the API's refusal: an invalid
// fail path, and a policy on an action linked to a specification.
func TestApprovalActionClient_RefusalsCarryTheAPIMessage(t *testing.T) {
	_, _, client := newLinkFake(t)
	newAPIAction(t, client)

	_, err := client.CreateApprovalAction(&ApprovalAction{Nrn: approvalNrn, Entity: "deployment", Action: "deployment:create", OnChecklistFail: "foo"})
	if want := "error creating approval action resource, got status code: 400, The checklist fail path is not valid. It must be one of the following: deny, pending, manual"; err == nil || err.Error() != want {
		t.Errorf("create error = %v, want %s", err, want)
	}

	if err := client.LinkChecklistSpecification("1", specVersionID(1)); err != nil {
		t.Fatal(err)
	}
	err = client.AssociatePolicyWithAction("1", "1")
	if want := "error associating approval policy with action, got status code: 409, Approval action already has a checklist specification; cannot also associate policies"; err == nil || err.Error() != want {
		t.Errorf("associate error = %v, want %s", err, want)
	}
}

// Two actions alike — nrn, entity, action, dimensions, absent dimensions
// being {} — cannot both be live: the API refuses the second, so
// a provider that lost one from its state can never quietly make a twin. One
// that differs in entity or dimensions is not a twin.
func TestApprovalActionClient_TwinRefused(t *testing.T) {
	production := map[string]string{"environment": "production"}
	tests := []struct {
		name          string
		first, second *ApprovalAction
		twin          bool
	}{
		{"same dimensions", &ApprovalAction{Entity: "deployment", Dimensions: production}, &ApprovalAction{Entity: "deployment", Dimensions: production}, true},
		{"no dimensions", &ApprovalAction{Entity: "deployment"}, &ApprovalAction{Entity: "deployment"}, true},
		{"another entity", &ApprovalAction{Entity: "deployment"}, &ApprovalAction{Entity: "scope"}, false},
		{"other dimensions", &ApprovalAction{Entity: "deployment", Dimensions: production}, &ApprovalAction{Entity: "deployment", Dimensions: map[string]string{"environment": "staging"}}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, client := newLinkFake(t)
			for _, action := range []*ApprovalAction{tt.first, tt.second} {
				action.Nrn, action.Action = approvalNrn, "deployment:create"
			}
			if _, err := client.CreateApprovalAction(tt.first); err != nil {
				t.Fatal(err)
			}

			dimensions := tt.first.Dimensions
			if dimensions == nil {
				dimensions = map[string]string{}
			}
			first, err := client.GetApprovalAction("1")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(first.Dimensions, dimensions) {
				t.Fatalf("first action's dimensions %#v, want %#v: {} when it had none", first.Dimensions, dimensions)
			}

			_, err = client.CreateApprovalAction(tt.second)
			want := "error creating approval action resource, got status code: 400, There is already an action for the given parameters"
			if tt.twin && (err == nil || err.Error() != want) || !tt.twin && err != nil {
				t.Fatalf("second create: %v, twin %v", err, tt.twin)
			}
			if !tt.twin {
				return
			}
			if err := client.DeleteApprovalAction("1"); err != nil {
				t.Fatal(err)
			}
			if _, err := client.CreateApprovalAction(tt.second); err != nil {
				t.Errorf("create after the first was deleted: %v, want it created", err)
			}
		})
	}
}

// A policy associated twice to an action is refused while the first
// association is live; once it is deleted, or archived by a migration and the
// action unlinked, the same policy associates again.
func TestApprovalActionClient_DuplicateAssociationRefused(t *testing.T) {
	_, approvals, client := newLinkFake(t)
	newAPIAction(t, client)
	withLivePolicies(t, client, 1)

	err := client.AssociatePolicyWithAction("1", "1")
	if want := "error associating approval policy with action, got status code: 400, Policy is already associated with approval action"; err == nil || err.Error() != want {
		t.Errorf("second association: %v, want %s", err, want)
	}
	if err := client.DisassociatePolicyFromAction("1", "1"); err != nil {
		t.Fatal(err)
	}
	if err := client.AssociatePolicyWithAction("1", "1"); err != nil {
		t.Errorf("association after the delete: %v, want it created", err)
	}

	approvals.Migrate("1", specVersionID(1))
	if err := client.UnlinkChecklistSpecification("1"); err != nil {
		t.Fatal(err)
	}
	if err := client.AssociatePolicyWithAction("1", "1"); err != nil {
		t.Errorf("association over an archived one: %v, want it created", err)
	}
	if action, err := client.GetApprovalAction("1"); err != nil || len(action.Policies) != 1 {
		t.Errorf("action %+v, %v; want the policy listed once, live and archived", action, err)
	}
}

// Deleting an association the migration archived answers 204 and changes
// nothing: the GET keeps listing it, linked and unlinked (observed against the
// API).
func TestApprovalActionClient_ArchivedAssociationDeleteChangesNothing(t *testing.T) {
	_, approvals, client := newLinkFake(t)
	newAPIAction(t, client)
	withLivePolicies(t, client, 1)
	approvals.Migrate("1", specVersionID(1))

	if err := client.DisassociatePolicyFromAction("1", "1"); err != nil {
		t.Fatalf("delete of the archived association: %v, want 204", err)
	}
	for _, step := range []string{"linked", "unlinked"} {
		if step == "unlinked" {
			if err := client.UnlinkChecklistSpecification("1"); err != nil {
				t.Fatal(err)
			}
		}
		if action, err := client.GetApprovalAction("1"); err != nil || len(action.Policies) != 1 {
			t.Errorf("%s: action %+v, %v; want the archived policy still listed", step, action, err)
		}
	}
}

// A link seeds the fail path from on_policy_fail: deny stays deny.
func TestApprovalActionClient_LinkSeedsDeny(t *testing.T) {
	_, _, client := newLinkFake(t)
	if _, err := client.CreateChecklistSpecification(&ChecklistSpecification{Nrn: approvalNrn, Name: "spec", Definition: json.RawMessage(specJSONV1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CreateApprovalAction(&ApprovalAction{Nrn: approvalNrn, Entity: "deployment", Action: "deployment:create", OnPolicyFail: "deny"}); err != nil {
		t.Fatal(err)
	}
	if err := client.LinkChecklistSpecification("1", specVersionID(1)); err != nil {
		t.Fatal(err)
	}
	if action, err := client.GetApprovalAction("1"); err != nil || action.OnChecklistFail != "deny" {
		t.Errorf("action %+v, %v; want on_checklist_fail deny", action, err)
	}
}
