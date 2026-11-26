package fakeplatform

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"
)

type wfCall struct {
	t *testing.T
	s *Server
}

func (c wfCall) do(method, path, body string) (int, Item) {
	c.t.Helper()
	req, _ := http.NewRequest(method, c.s.URL()+path, strings.NewReader(body))
	res, err := c.s.ts.Client().Do(req)
	if err != nil {
		c.t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	out := Item{}
	_ = json.Unmarshal(raw, &out)
	return res.StatusCode, out
}

const checklistWorkflow = `{"name":"Gate Check","key":"gate","steps":{"on_item":{"pluginType":"np-checklist-trigger",` +
	`"config":{"nrn":"organization=1:account=2","kind":"k1","pathPrefix":"p"}},"work":{"pluginType":"log","config":{}}}}`

func wfID(created Item) string { return Str(created["workflow"].(Item), "id") }

func TestWorkflowLifecycleCreatesAndReleasesChannels(t *testing.T) {
	s := New()
	defer s.Close()
	RegisterWorkflows(s, WorkflowEngine{})
	c := wfCall{t, s}

	status, created := c.do("POST", "/workflows/definitions", checklistWorkflow)
	if status != 201 || !strings.HasPrefix(wfID(created), "wf_") || len(wfID(created)) != 15 {
		t.Fatalf("create: %d %v", status, created)
	}
	id := wfID(created)
	def := created["revision"].(Item)["definition"].(Item)
	steps := def["steps"].(map[string]any)
	if steps["on_item"].(map[string]any)["type"] != "trigger" || steps["work"].(map[string]any)["type"] != "module" || len(def["triggers"].([]any)) != 1 {
		t.Errorf("definition not filled: %v", def)
	}
	if status, _ = c.do("GET", "/workflows/definitions/gate", ""); status != 200 {
		t.Errorf("get by key: %d", status)
	}

	base := "/workflows/definitions/" + id + "/aliases"
	c.do("POST", base, `{"name":"live","revision":1}`)
	status, act := c.do("POST", base+"/live/activate", "")
	alias := act["alias"].(Item)
	if status != 200 || alias["active"] != true || len(WorkflowChannels(s)) != 1 {
		t.Fatalf("activate: %d %v", status, act)
	}
	ch := WorkflowChannels(s)[0]
	if Str(ch, "description") != "Workflow checklist trigger: p [kind=k1]" || ch["status"] != "active" {
		t.Errorf("channel %v", ch)
	}
	q := url.Values{"nrn": {"organization=1:account=2"}, "limit": {"100"}}
	if _, list := c.do("GET", "/notification/channel?"+q.Encode(), ""); list["paging"].(map[string]any)["total"] != float64(1) {
		t.Errorf("list %v", list)
	}
	// Re-activation leaves one channel, not two.
	c.do("POST", base+"/live/activate", "")
	if len(WorkflowChannels(s)) != 1 {
		t.Errorf("duplicate channels: %d", len(WorkflowChannels(s)))
	}
	if status, out := c.do("DELETE", "/workflows/definitions/"+id, ""); status != 409 || !strings.HasSuffix(Str(out, "type"), "/alias-already-exists") {
		t.Errorf("delete with active alias: %d %v", status, out)
	}
	c.do("POST", base+"/live/deactivate", "")
	state, _ := WorkflowAlias(s, id, "live")
	if state["active"] != false || len(WorkflowChannels(s)) != 0 || state["triggerStates"] == nil {
		t.Errorf("deactivate: %v channels %d", state, len(WorkflowChannels(s)))
	}
	if status, _ = c.do("DELETE", base+"/live", ""); status != 204 {
		t.Errorf("alias delete: %d", status)
	}
	if status, _ = c.do("DELETE", "/workflows/definitions/"+id, ""); status != 204 || len(s.Items(WorkflowDefinitions)) != 0 {
		t.Errorf("workflow delete: %d", status)
	}
}

func TestWorkflowEngineOptions(t *testing.T) {
	s := New()
	defer s.Close()
	RegisterWorkflows(s, WorkflowEngine{
		RefuseActiveAliasDelete: true,
		ReportReleaseFailures:   true, UnreleasableChannelNRNs: []string{"organization=1:account=2"},
		ReportActivationError: true, DeniedChannelNRNs: []string{"organization=1:account=3"},
		UnreleasedOnDelete: true,
	})
	c := wfCall{t, s}
	_, created := c.do("POST", "/workflows/definitions", checklistWorkflow)
	id := wfID(created)
	base := "/workflows/definitions/" + id + "/aliases"
	c.do("POST", base, `{"name":"live","revision":1}`)
	c.do("POST", base+"/live/activate", "")

	if status, out := c.do("DELETE", base+"/live", ""); status != 409 || !strings.HasSuffix(Str(out, "type"), "/alias-active") ||
		Str(out, "detail") != "Alias 'live' is active; deactivate it before deleting" {
		t.Errorf("active delete: %d %v", status, out)
	}
	_, out := c.do("POST", base+"/live/deactivate", "")
	if Str(out, "deactivateError") == "" || len(WorkflowChannels(s)) != 1 {
		t.Errorf("release failure not reported: %v", out)
	}
	if status, out := c.do("DELETE", base+"/live", ""); status != 409 || !strings.HasSuffix(Str(out, "type"), "/alias-not-released") {
		t.Errorf("unreleased delete: %d %v", status, out)
	}

	denied := strings.ReplaceAll(checklistWorkflow, "account=2", "account=3")
	denied = strings.Replace(denied, `"key":"gate"`, `"key":"denied"`, 1)
	_, created = c.do("POST", "/workflows/definitions", denied)
	base = "/workflows/definitions/" + wfID(created) + "/aliases"
	c.do("POST", base, `{"name":"live","revision":1}`)
	_, out = c.do("POST", base+"/live/activate", "")
	if out["alias"].(Item)["active"] != false || len(out["failedTriggers"].([]any)) != 1 || Str(out, "activationError") == "" {
		t.Errorf("activation failure: %v", out)
	}
}

func TestWorkflowErrorsAndPatch(t *testing.T) {
	s := New()
	defer s.Close()
	RegisterWorkflows(s, WorkflowEngine{})
	c := wfCall{t, s}
	if status, out := c.do("POST", "/workflows/definitions", `{"key":"x"}`); status != 400 || out["error"].(Item)["code"] != "NAME_REQUIRED" {
		t.Errorf("name: %d %v", status, out)
	}
	if status, out := c.do("GET", "/workflows/definitions/nope", ""); status != 404 || Str(out, "detail") != "No workflow 'nope' exists" {
		t.Errorf("missing: %d %v", status, out)
	}
	_, created := c.do("POST", "/workflows/definitions", `{"name":"a b","steps":{}}`)
	id := wfID(created)
	_, patched := c.do("PATCH", "/workflows/definitions/"+id, `{"path":"//a//b/","description":""}`)
	if rec := patched["workflow"].(Item); rec["path"] != "/a/b" || rec["description"] != "" {
		t.Errorf("patch: %v", rec)
	}
	if status, _ := c.do("PUT", "/workflows/definitions/"+id, `{"definition":{"steps":{}},"mode":"new-revision"}`); status != 400 {
		t.Errorf("put without name: %d", status)
	}
	status, put := c.do("PUT", "/workflows/definitions/"+id, `{"definition":{"name":"a b","steps":{}},"mode":"new-revision"}`)
	if status != 200 || len(WorkflowRevisions(s, id)) != 2 || put["latestAlias"].(Item)["revision"] != float64(2) {
		t.Errorf("put: %d %v", status, put)
	}
}

// The engine's knobs are read live: lifting one between requests changes the
// next answer.
func TestWorkflowEngineKnobsAreReadLive(t *testing.T) {
	s := New()
	defer s.Close()
	RegisterWorkflows(s, WorkflowEngine{RefuseActiveAliasDelete: true})
	c := wfCall{t, s}
	_, created := c.do("POST", "/workflows/definitions", checklistWorkflow)
	base := "/workflows/definitions/" + wfID(created) + "/aliases"
	c.do("POST", base, `{"name":"live","revision":1}`)
	c.do("POST", base+"/live/activate", "")
	if status, _ := c.do("DELETE", base+"/live", ""); status != 409 {
		t.Fatalf("delete with the refusal: %d", status)
	}
	SetWorkflowEngine(s, func(e *WorkflowEngine) { e.RefuseActiveAliasDelete = false })
	if status, _ := c.do("DELETE", base+"/live", ""); status != 204 {
		t.Errorf("delete with the refusal lifted: %d", status)
	}
}
