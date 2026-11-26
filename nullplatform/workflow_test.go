package nullplatform

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"sync"
	"testing"
)

// workflowCall is one request as the server saw it.
type workflowCall struct {
	Method      string
	URI         string
	Body        string
	ContentType string
}

// newWorkflowServer answers every request with status and body, and records
// what it was asked.
func newWorkflowServer(t *testing.T, status int, body string) (*NullClient, func() []workflowCall) {
	t.Helper()
	var mu sync.Mutex
	var calls []workflowCall
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		mu.Lock()
		calls = append(calls, workflowCall{r.Method, r.RequestURI, string(raw), r.Header.Get("Content-Type")})
		mu.Unlock()
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(server.Close)
	return newTestClient(server), func() []workflowCall {
		mu.Lock()
		defer mu.Unlock()
		return append([]workflowCall(nil), calls...)
	}
}

const (
	workflowAnswer = `{"workflow":{"id":"wf_1","name":"payments-gate"},"revision":{"revision":2,"definition":{"steps":{}}}}`
	aliasAnswer    = `{"workflowId":"wf_1","name":"live","revision":2,"active":true}`
)

func TestNewWorkflowError(t *testing.T) {
	tests := []struct {
		name        string
		raw         string
		wantType    string
		wantMessage string
		wantErrors  []WorkflowValidationError
	}{
		{
			name: "problem document with validation errors",
			raw: `{"type":"https://example.test/errors/validation-failed","title":"Invalid definition","detail":"1 problem",` +
				`"errors":[{"code":"required","message":"is required","path":"steps.a.config"}]}`,
			wantType:    "https://example.test/errors/validation-failed",
			wantMessage: "Invalid definition: 1 problem",
			wantErrors:  []WorkflowValidationError{{Code: "required", Message: "is required", Path: "steps.a.config"}},
		},
		{
			name:        "problem document without errors",
			raw:         `{"type":"https://example.test/errors/alias-active","title":"Alias is active","detail":"deactivate it first"}`,
			wantType:    "https://example.test/errors/alias-active",
			wantMessage: "Alias is active: deactivate it first",
		},
		{
			name:        "problem document with only a title",
			raw:         `{"type":"https://example.test/errors/x","title":"Only title"}`,
			wantType:    "https://example.test/errors/x",
			wantMessage: "Only title",
		},
		{
			name:        "problem document with only a detail",
			raw:         `{"type":"https://example.test/errors/x","detail":"Only detail"}`,
			wantType:    "https://example.test/errors/x",
			wantMessage: "Only detail",
		},
		{
			name:        "error object with a code",
			raw:         `{"error":{"code":"E_BAD","message":"nope"}}`,
			wantMessage: "nope (E_BAD)",
		},
		{
			name:        "error object without a code",
			raw:         `{"error":{"message":"nope"}}`,
			wantMessage: "nope",
		},
		{
			name:        "error object without a message keeps the body",
			raw:         `{"error":{"code":"E_BAD"}}`,
			wantMessage: `{"error":{"code":"E_BAD"}}`,
		},
		{
			name:        "plain text is trimmed",
			raw:         "  Bad Gateway \n",
			wantMessage: "Bad Gateway",
		},
		{
			name:        "empty body",
			raw:         "",
			wantMessage: "",
		},
		{
			name:        "json that is neither shape keeps the body",
			raw:         `{"other":1}`,
			wantMessage: `{"other":1}`,
		},
		{
			name:        "json that is not an object keeps the body",
			raw:         `[1,2]`,
			wantMessage: `[1,2]`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := newWorkflowError("doing a thing", 400, []byte(tt.raw))
			if got.Op != "doing a thing" || got.StatusCode != 400 {
				t.Errorf("op and status = %q, %d", got.Op, got.StatusCode)
			}
			if got.Type != tt.wantType {
				t.Errorf("Type = %q, want %q", got.Type, tt.wantType)
			}
			if got.Message != tt.wantMessage {
				t.Errorf("Message = %q, want %q", got.Message, tt.wantMessage)
			}
			if !reflect.DeepEqual(got.Errors, tt.wantErrors) {
				t.Errorf("Errors = %+v, want %+v", got.Errors, tt.wantErrors)
			}
		})
	}
}

func TestWorkflowError_Error(t *testing.T) {
	refused := &WorkflowError{
		Op: "creating workflow", StatusCode: 400, Message: "Invalid definition",
		Errors: []WorkflowValidationError{
			{Path: "steps.a", Message: "bad", Code: "invalid"},
			{Path: "name", Message: "is required"},
		},
	}
	want := "error creating workflow, got 400: Invalid definition; steps.a: bad (invalid); name: is required"
	if got := refused.Error(); got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}

	plain := &WorkflowError{Op: "reading workflow wf_1", StatusCode: 404, Message: "Not Found"}
	if got, want := plain.Error(), "error reading workflow wf_1, got 404: Not Found"; got != want {
		t.Errorf("Error() = %q, want %q", got, want)
	}
}

func TestWorkflowError_ErrorRedactsTokens(t *testing.T) {
	const token = "supersecrettoken1234567890"
	refused := &WorkflowError{
		Op: "creating workflow", StatusCode: 502,
		Message: "the gateway echoed: Authorization: Bearer " + token,
		Errors:  []WorkflowValidationError{{Path: "headers", Message: "Bearer " + token + "abc"}},
	}
	got := refused.Error()
	if strings.Contains(got, token) {
		t.Errorf("Error() leaks the token: %q", got)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Errorf("Error() = %q, want the credential replaced by REDACTED", got)
	}
}

func TestWorkflowError_HasType(t *testing.T) {
	tests := []struct {
		typ, slug string
		want      bool
	}{
		{"https://example.test/errors/alias-active", "alias-active", true},
		{"/errors/alias-active", "alias-active", true},
		{"alias-active", "alias-active", false},
		{"https://example.test/errors/not-alias-active", "alias-active", false},
		{"https://example.test/errors/alias-active", "alias-not-found", false},
		{"", "alias-active", false},
	}
	for _, tt := range tests {
		if got := (&WorkflowError{Type: tt.typ}).HasType(tt.slug); got != tt.want {
			t.Errorf("HasType(%q) on type %q = %v, want %v", tt.slug, tt.typ, got, tt.want)
		}
	}
}

func TestWorkflowError_IsAndIsWorkflowGone(t *testing.T) {
	const prefix = "https://example.test/errors/"
	tests := []struct {
		name          string
		err           error
		wantWorkflow  bool
		wantAlias     bool
		wantGoneCheck bool
	}{
		{"404 workflow-not-found", &WorkflowError{StatusCode: 404, Type: prefix + "workflow-not-found"}, true, false, true},
		{"404 alias-not-found", &WorkflowError{StatusCode: 404, Type: prefix + "alias-not-found"}, false, true, true},
		{"404 of another type", &WorkflowError{StatusCode: 404, Type: prefix + "something-else"}, false, false, false},
		{"404 without a type (a gateway page)", &WorkflowError{StatusCode: 404, Message: "Not Found"}, false, false, false},
		{"400 workflow-not-found", &WorkflowError{StatusCode: 400, Type: prefix + "workflow-not-found"}, false, false, false},
		{"410 alias-not-found", &WorkflowError{StatusCode: 410, Type: prefix + "alias-not-found"}, false, false, false},
		{"409 alias-active", &WorkflowError{StatusCode: 409, Type: prefix + "alias-active"}, false, false, false},
		{"wrapped workflow-not-found", fmt.Errorf("cannot import: %w", &WorkflowError{StatusCode: 404, Type: prefix + "workflow-not-found"}), true, false, true},
		{"an unrelated error", errors.New("boom"), false, false, false},
		{"nil", nil, false, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := errors.Is(tt.err, errWorkflowNotFound); got != tt.wantWorkflow {
				t.Errorf("errors.Is(errWorkflowNotFound) = %v, want %v", got, tt.wantWorkflow)
			}
			if got := errors.Is(tt.err, errWorkflowAliasNotFound); got != tt.wantAlias {
				t.Errorf("errors.Is(errWorkflowAliasNotFound) = %v, want %v", got, tt.wantAlias)
			}
			if got := isWorkflowGone(tt.err); got != tt.wantGoneCheck {
				t.Errorf("isWorkflowGone = %v, want %v", got, tt.wantGoneCheck)
			}
		})
	}
}

func TestWorkflowPaths(t *testing.T) {
	if got, want := workflowPath("wf_1"), "/workflows/definitions/wf_1"; got != want {
		t.Errorf("workflowPath = %q, want %q", got, want)
	}
	if got, want := workflowPath("my key/x?y"), "/workflows/definitions/my%20key%2Fx%3Fy"; got != want {
		t.Errorf("workflowPath escaped = %q, want %q", got, want)
	}
	if got, want := workflowAliasPath("wf_1", "live"), "/workflows/definitions/wf_1/aliases/live"; got != want {
		t.Errorf("workflowAliasPath = %q, want %q", got, want)
	}
	if got, want := workflowAliasPath("a/b", "blue green"), "/workflows/definitions/a%2Fb/aliases/blue%20green"; got != want {
		t.Errorf("workflowAliasPath escaped = %q, want %q", got, want)
	}
}

func TestWorkflowRequest_SendsTheBodyAsWritten(t *testing.T) {
	client, calls := newWorkflowServer(t, http.StatusOK, `{"ok":true}`)
	var out struct {
		Ok bool `json:"ok"`
	}
	in := map[string]any{"url": "https://example.test/x?a=1&b=<2>"}
	if err := client.workflowRequest("doing", http.MethodPost, "/some/path", in, http.StatusOK, &out); err != nil {
		t.Fatal(err)
	}
	if !out.Ok {
		t.Error("the answer was not decoded")
	}
	got := calls()
	if len(got) != 1 {
		t.Fatalf("%d requests, want 1", len(got))
	}
	if want := (workflowCall{http.MethodPost, "/some/path", `{"url":"https://example.test/x?a=1&b=<2>"}` + "\n", "application/json"}); got[0] != want {
		t.Errorf("request = %+v, want %+v", got[0], want)
	}
}

func TestWorkflowRequest_NoBodyNoOut(t *testing.T) {
	client, calls := newWorkflowServer(t, http.StatusNoContent, "")
	if err := client.workflowRequest("doing", http.MethodDelete, "/some/path", nil, http.StatusNoContent, nil); err != nil {
		t.Fatal(err)
	}
	if want := (workflowCall{http.MethodDelete, "/some/path", "", ""}); calls()[0] != want {
		t.Errorf("request = %+v, want %+v", calls()[0], want)
	}
}

func TestWorkflowRequest_UnexpectedStatusIsTheRefusal(t *testing.T) {
	client, _ := newWorkflowServer(t, http.StatusConflict,
		`{"type":"https://example.test/errors/alias-active","title":"Alias is active","detail":"deactivate it first"}`)
	err := client.workflowRequest("deleting alias live", http.MethodDelete, "/p", nil, http.StatusNoContent, nil)
	var refused *WorkflowError
	if !errors.As(err, &refused) {
		t.Fatalf("error = %v, want a *WorkflowError", err)
	}
	if refused.Op != "deleting alias live" || refused.StatusCode != 409 || !refused.HasType("alias-active") ||
		refused.Message != "Alias is active: deactivate it first" {
		t.Errorf("refusal = %+v", refused)
	}
}

func TestWorkflowRequest_ASuccessOfAnotherStatusIsARefusal(t *testing.T) {
	client, _ := newWorkflowServer(t, http.StatusOK, `{}`)
	err := client.workflowRequest("creating", http.MethodPost, "/p", map[string]any{}, http.StatusCreated, nil)
	var refused *WorkflowError
	if !errors.As(err, &refused) || refused.StatusCode != http.StatusOK {
		t.Fatalf("error = %v, want a *WorkflowError with status 200", err)
	}
}

func TestWorkflowRequest_UndecodableAnswer(t *testing.T) {
	client, _ := newWorkflowServer(t, http.StatusOK, "not json")
	var out map[string]any
	err := client.workflowRequest("reading", http.MethodPost, "/p", nil, http.StatusOK, &out)
	if err == nil {
		t.Fatal("want an error for an answer that is not JSON")
	}
	var refused *WorkflowError
	if errors.As(err, &refused) {
		t.Errorf("a decode failure is not a refusal: %v", err)
	}
}

func TestWorkflowRequest_TransportError(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	client := newTestClient(server)
	server.Close()
	// POST: a GET would retry with a backoff.
	err := client.workflowRequest("doing", http.MethodPost, "/p", nil, http.StatusOK, nil)
	if err == nil {
		t.Fatal("want a transport error")
	}
	var refused *WorkflowError
	if errors.As(err, &refused) {
		t.Errorf("a transport failure is not a refusal: %v", err)
	}
}

func TestWorkflowRequest_UnencodableBodySendsNothing(t *testing.T) {
	client, calls := newWorkflowServer(t, http.StatusOK, `{}`)
	err := client.workflowRequest("doing", http.MethodPost, "/p", make(chan int), http.StatusOK, nil)
	if err == nil {
		t.Fatal("want an encoding error")
	}
	if n := len(calls()); n != 0 {
		t.Errorf("%d requests sent, want none", n)
	}
}

// Every method of the client: what it sends and what it expects.
func TestWorkflowMethods_RequestShape(t *testing.T) {
	raw := func(s string) json.RawMessage { return json.RawMessage(s) }
	definition := map[string]json.RawMessage{"steps": raw(`{}`)}
	tests := []struct {
		name       string
		status     int
		answer     string
		call       func(c *NullClient) error
		wantMethod string
		wantURI    string
		wantBody   string
	}{
		{
			name: "CreateWorkflow", status: 201, answer: workflowAnswer,
			call: func(c *NullClient) error {
				_, err := c.CreateWorkflow(map[string]json.RawMessage{"name": raw(`"payments-gate"`), "steps": raw(`{}`)})
				return err
			},
			wantMethod: "POST", wantURI: "/workflows/definitions", wantBody: `{"name":"payments-gate","steps":{}}` + "\n",
		},
		{
			name: "GetWorkflow", status: 200, answer: workflowAnswer,
			call:       func(c *NullClient) error { _, err := c.GetWorkflow("wf_1"); return err },
			wantMethod: "GET", wantURI: "/workflows/definitions/wf_1", wantBody: "",
		},
		{
			name: "GetWorkflow by key with escaping", status: 200, answer: workflowAnswer,
			call:       func(c *NullClient) error { _, err := c.GetWorkflow("a/b"); return err },
			wantMethod: "GET", wantURI: "/workflows/definitions/a%2Fb", wantBody: "",
		},
		{
			name: "GetWorkflowRevision", status: 200, answer: workflowAnswer,
			call:       func(c *NullClient) error { _, err := c.GetWorkflowRevision("wf_1", 3); return err },
			wantMethod: "GET", wantURI: "/workflows/definitions/wf_1?revision=3", wantBody: "",
		},
		{
			name: "PutWorkflowRevision", status: 200, answer: workflowAnswer,
			call:       func(c *NullClient) error { _, err := c.PutWorkflowRevision("wf_1", definition); return err },
			wantMethod: "PUT", wantURI: "/workflows/definitions/wf_1", wantBody: `{"definition":{"steps":{}},"mode":"new-revision"}` + "\n",
		},
		{
			name: "PatchWorkflow", status: 200, answer: `{"workflow":{"id":"wf_1","name":"renamed"}}`,
			call: func(c *NullClient) error {
				_, err := c.PatchWorkflow("wf_1", map[string]any{"name": "renamed", "description": ""})
				return err
			},
			wantMethod: "PATCH", wantURI: "/workflows/definitions/wf_1", wantBody: `{"description":"","name":"renamed"}` + "\n",
		},
		{
			name: "DeleteWorkflow", status: 204, answer: "",
			call:       func(c *NullClient) error { return c.DeleteWorkflow("wf_1") },
			wantMethod: "DELETE", wantURI: "/workflows/definitions/wf_1", wantBody: "",
		},
		{
			name: "CreateWorkflowAlias", status: 201, answer: aliasAnswer,
			call:       func(c *NullClient) error { _, err := c.CreateWorkflowAlias("wf_1", "live", 2); return err },
			wantMethod: "POST", wantURI: "/workflows/definitions/wf_1/aliases", wantBody: `{"name":"live","revision":2}` + "\n",
		},
		{
			name: "GetWorkflowAlias", status: 200, answer: aliasAnswer,
			call:       func(c *NullClient) error { _, err := c.GetWorkflowAlias("wf_1", "live"); return err },
			wantMethod: "GET", wantURI: "/workflows/definitions/wf_1/aliases/live", wantBody: "",
		},
		{
			name: "MoveWorkflowAlias", status: 200, answer: aliasAnswer,
			call:       func(c *NullClient) error { _, err := c.MoveWorkflowAlias("wf_1", "live", 4); return err },
			wantMethod: "PUT", wantURI: "/workflows/definitions/wf_1/aliases/live", wantBody: `{"revision":4}` + "\n",
		},
		{
			name: "ActivateWorkflowAlias", status: 200, answer: `{"alias":` + aliasAnswer + `,"failedTriggers":[]}`,
			call:       func(c *NullClient) error { _, err := c.ActivateWorkflowAlias("wf_1", "live"); return err },
			wantMethod: "POST", wantURI: "/workflows/definitions/wf_1/aliases/live/activate", wantBody: "",
		},
		{
			name: "DeactivateWorkflowAlias", status: 200, answer: `{"alias":` + aliasAnswer + `}`,
			call:       func(c *NullClient) error { _, err := c.DeactivateWorkflowAlias("wf_1", "live"); return err },
			wantMethod: "POST", wantURI: "/workflows/definitions/wf_1/aliases/live/deactivate", wantBody: "",
		},
		{
			name: "DeleteWorkflowAlias", status: 204, answer: "",
			call:       func(c *NullClient) error { return c.DeleteWorkflowAlias("wf_1", "live") },
			wantMethod: "DELETE", wantURI: "/workflows/definitions/wf_1/aliases/live", wantBody: "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, calls := newWorkflowServer(t, tt.status, tt.answer)
			if err := tt.call(client); err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			got := calls()
			if len(got) != 1 {
				t.Fatalf("%d requests, want 1", len(got))
			}
			if got[0].Method != tt.wantMethod || got[0].URI != tt.wantURI || got[0].Body != tt.wantBody {
				t.Errorf("sent %s %s %q, want %s %s %q", got[0].Method, got[0].URI, got[0].Body, tt.wantMethod, tt.wantURI, tt.wantBody)
			}
			if tt.wantBody == "" && got[0].ContentType != "" {
				t.Errorf("a bodyless request carried Content-Type %q", got[0].ContentType)
			}
			if tt.wantBody != "" && got[0].ContentType != "application/json" {
				t.Errorf("Content-Type = %q, want application/json", got[0].ContentType)
			}
		})
	}
}

// A method refuses any status but the one it expects: the API's 200 on a
// create, say, is not taken as a success.
func TestWorkflowMethods_RefuseAnotherStatus(t *testing.T) {
	client, _ := newWorkflowServer(t, http.StatusOK, `{}`)
	if _, err := client.CreateWorkflow(map[string]json.RawMessage{}); err == nil {
		t.Error("CreateWorkflow accepted a 200")
	}
	if err := client.DeleteWorkflow("wf_1"); err == nil {
		t.Error("DeleteWorkflow accepted a 200")
	}
	if _, err := client.CreateWorkflowAlias("wf_1", "live", 1); err == nil {
		t.Error("CreateWorkflowAlias accepted a 200")
	}
	if err := client.DeleteWorkflowAlias("wf_1", "live"); err == nil {
		t.Error("DeleteWorkflowAlias accepted a 200")
	}
}

func TestWorkflowMethods_DecodeTheAnswer(t *testing.T) {
	client, _ := newWorkflowServer(t, http.StatusOK, workflowAnswer)
	read, err := client.GetWorkflow("wf_1")
	if err != nil {
		t.Fatal(err)
	}
	if read.Workflow.Id != "wf_1" || read.Workflow.Name != "payments-gate" || read.Revision.Revision != 2 ||
		string(read.Revision.Definition) != `{"steps":{}}` {
		t.Errorf("decoded %+v", read)
	}

	client, _ = newWorkflowServer(t, http.StatusOK, `{"alias":{"workflowId":"wf_1","name":"live","revision":2,"active":true,`+
		`"triggerStates":{"hook":{"runtimeMetadata":{"webhookUrl":"https://example.test/hook/abc"}}}},`+
		`"failedTriggers":[{"id":"t","pluginType":"webhook","status":"failed","error":"boom"}],"activationError":"bad"}`)
	activation, err := client.ActivateWorkflowAlias("wf_1", "live")
	if err != nil {
		t.Fatal(err)
	}
	if !activation.Alias.Active || activation.ActivationError != "bad" || len(activation.FailedTriggers) != 1 ||
		activation.FailedTriggers[0].Error != "boom" ||
		activation.Alias.TriggerStates["hook"].RuntimeMetadata.WebhookUrl != "https://example.test/hook/abc" {
		t.Errorf("decoded %+v", activation)
	}

	client, _ = newWorkflowServer(t, http.StatusOK, `{"alias":`+aliasAnswer+`,"deactivateError":"stuck"}`)
	deactivation, err := client.DeactivateWorkflowAlias("wf_1", "live")
	if err != nil || deactivation.DeactivateError != "stuck" {
		t.Errorf("decoded %+v, %v", deactivation, err)
	}

	client, _ = newWorkflowServer(t, http.StatusOK, `{"workflow":{"id":"wf_1","name":"renamed","path":"/a"}}`)
	patched, err := client.PatchWorkflow("wf_1", map[string]any{"name": "renamed"})
	if err != nil || patched.Name != "renamed" || patched.Path != "/a" {
		t.Errorf("decoded %+v, %v", patched, err)
	}
}

func TestWorkflowMethods_NotFoundIsGone(t *testing.T) {
	problem := func(slug string) string {
		return `{"type":"https://example.test/errors/` + slug + `","title":"Not found","detail":"nothing here"}`
	}
	client, _ := newWorkflowServer(t, http.StatusNotFound, problem("workflow-not-found"))
	_, err := client.GetWorkflow("wf_1")
	if !errors.Is(err, errWorkflowNotFound) || !isWorkflowGone(err) {
		t.Errorf("a workflow-not-found 404 read as %v, want errWorkflowNotFound", err)
	}

	client, _ = newWorkflowServer(t, http.StatusNotFound, problem("alias-not-found"))
	_, err = client.GetWorkflowAlias("wf_1", "live")
	if !errors.Is(err, errWorkflowAliasNotFound) || !isWorkflowGone(err) {
		t.Errorf("an alias-not-found 404 read as %v, want errWorkflowAliasNotFound", err)
	}

	client, _ = newWorkflowServer(t, http.StatusNotFound, "<html>Not Found</html>")
	_, err = client.GetWorkflow("wf_1")
	if err == nil || isWorkflowGone(err) {
		t.Errorf("a gateway's 404 read as %v, want an error that is not gone", err)
	}
}
