package nullplatform

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The CRUD functions' arms that Terraform cannot reach with a valid plan, or
// the fake cannot answer: driven directly, with a scripted client.

var errWorkflowStub = errors.New("refused by the stub")

// workflowCrudStub answers the calls a test scripts; any other call panics on
// the nil interface, which a test must never reach.
type workflowCrudStub struct {
	NullOps
	getWorkflow func(string) (*WorkflowWithRevision, error)
	patch       func(string, map[string]any) (*Workflow, error)
	put         func(string, map[string]json.RawMessage) (*WorkflowWithRevision, error)
	createAlias func(string, string, int) (*WorkflowAlias, error)
	getAlias    func(string, string) (*WorkflowAlias, error)
	move        func(string, string, int) (*WorkflowAlias, error)
	activate    func(string, string) (*WorkflowActivation, error)
	deactivate  func(string, string) (*WorkflowDeactivation, error)
}

func (s *workflowCrudStub) GetWorkflow(ref string) (*WorkflowWithRevision, error) {
	return s.getWorkflow(ref)
}

func (s *workflowCrudStub) PatchWorkflow(id string, patch map[string]any) (*Workflow, error) {
	return s.patch(id, patch)
}

func (s *workflowCrudStub) PutWorkflowRevision(id string, definition map[string]json.RawMessage) (*WorkflowWithRevision, error) {
	return s.put(id, definition)
}

func (s *workflowCrudStub) CreateWorkflowAlias(id, name string, revision int) (*WorkflowAlias, error) {
	return s.createAlias(id, name, revision)
}

func (s *workflowCrudStub) GetWorkflowAlias(ref, name string) (*WorkflowAlias, error) {
	return s.getAlias(ref, name)
}

func (s *workflowCrudStub) MoveWorkflowAlias(id, name string, revision int) (*WorkflowAlias, error) {
	return s.move(id, name, revision)
}

func (s *workflowCrudStub) ActivateWorkflowAlias(id, name string) (*WorkflowActivation, error) {
	return s.activate(id, name)
}

func (s *workflowCrudStub) DeactivateWorkflowAlias(id, name string) (*WorkflowDeactivation, error) {
	return s.deactivate(id, name)
}

func patchAccepted(id string, _ map[string]any) (*Workflow, error) { return &Workflow{Id: id}, nil }

func workflowData(t *testing.T, definition string) *schema.ResourceData {
	t.Helper()
	d := schema.TestResourceDataRaw(t, resourceWorkflow().Schema, map[string]any{"name": "payments-gate", "definition": definition})
	d.SetId("wf_000000000001")
	return d
}

func aliasData(t *testing.T) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, resourceWorkflowAlias().Schema,
		map[string]any{"workflow_id": "wf_000000000001", "name": "live", "revision": 2})
}

func requireDiagError(t *testing.T, diags diag.Diagnostics, want string) {
	t.Helper()
	if !diags.HasError() {
		t.Fatalf("want an error naming %q, got %v", want, diags)
	}
	for _, d := range diags {
		if d.Severity == diag.Error && strings.Contains(d.Summary, want) {
			return
		}
	}
	t.Fatalf("no error names %q: %v", want, diags)
}

func TestWorkflowCreate_DefinitionNotJSONSendsNothing(t *testing.T) {
	d := schema.TestResourceDataRaw(t, resourceWorkflow().Schema, map[string]any{"name": "payments-gate", "definition": "not json"})
	requireDiagError(t, WorkflowCreate(context.Background(), d, &workflowCrudStub{}), "definition is not a JSON object")
	if d.Id() != "" {
		t.Fatalf("id = %q, want none", d.Id())
	}
}

func TestWorkflowRead_StoredDefinitionNotAnObject(t *testing.T) {
	ops := &workflowCrudStub{getWorkflow: func(string) (*WorkflowWithRevision, error) {
		return &WorkflowWithRevision{Workflow: Workflow{Id: "wf_000000000001"},
			Revision: WorkflowRevision{Revision: 1, Definition: json.RawMessage(`"not an object"`)}}, nil
	}}
	requireDiagError(t, WorkflowRead(context.Background(), workflowData(t, `{"steps":{}}`), ops),
		"the API answered a definition that is not a JSON object")
}

func TestWorkflowUpdate_PatchRefused(t *testing.T) {
	ops := &workflowCrudStub{patch: func(string, map[string]any) (*Workflow, error) { return nil, errWorkflowStub }}
	requireDiagError(t, WorkflowUpdate(context.Background(), workflowData(t, `{"steps":{}}`), ops), errWorkflowStub.Error())
}

func TestWorkflowUpdate_DefinitionNotJSONSavesNothing(t *testing.T) {
	ops := &workflowCrudStub{patch: patchAccepted}
	requireDiagError(t, WorkflowUpdate(context.Background(), workflowData(t, "not json"), ops), "definition is not a JSON object")
}

func TestWorkflowUpdate_RevisionRefused(t *testing.T) {
	ops := &workflowCrudStub{
		patch: patchAccepted,
		put:   func(string, map[string]json.RawMessage) (*WorkflowWithRevision, error) { return nil, errWorkflowStub },
	}
	requireDiagError(t, WorkflowUpdate(context.Background(), workflowData(t, `{"steps":{}}`), ops), errWorkflowStub.Error())
}

func TestWorkflowDefinitionsEquivalent_StepsNotAnObject(t *testing.T) {
	if !workflowDefinitionsEquivalent(`{"steps":"x"}`, `{"steps":"x","id":"wf_1"}`) {
		t.Fatal("the same non-object steps must be equivalent")
	}
	if workflowDefinitionsEquivalent(`{"steps":"x"}`, `{"steps":"y"}`) {
		t.Fatal("different non-object steps must not be equivalent")
	}
}

func TestWorkflowError_IsNoOtherTarget(t *testing.T) {
	err := &WorkflowError{StatusCode: 404, Type: "https://workflow-system.dev/errors/workflow-not-found"}
	if errors.Is(err, io.EOF) {
		t.Fatal("a workflow error is not io.EOF")
	}
}

func TestWorkflowAliasCreate_RefusedLeavesNoAlias(t *testing.T) {
	d := aliasData(t)
	ops := &workflowCrudStub{createAlias: func(string, string, int) (*WorkflowAlias, error) { return nil, errWorkflowStub }}
	requireDiagError(t, WorkflowAliasCreate(context.Background(), d, ops), errWorkflowStub.Error())
	if d.Id() != "" {
		t.Fatalf("id = %q, want none", d.Id())
	}
}

func TestActivateWorkflowAlias_MoveRefused(t *testing.T) {
	ops := &workflowCrudStub{move: func(string, string, int) (*WorkflowAlias, error) { return nil, errWorkflowStub }}
	if err := activateWorkflowAlias(ops, "wf_000000000001", "live", 2, 1); !errors.Is(err, errWorkflowStub) {
		t.Fatalf("err = %v, want the move's refusal", err)
	}
}

func TestActivateWorkflowAlias_ActivateRefused(t *testing.T) {
	ops := &workflowCrudStub{activate: func(string, string) (*WorkflowActivation, error) { return nil, errWorkflowStub }}
	if err := activateWorkflowAlias(ops, "wf_000000000001", "live", 2, 2); !errors.Is(err, errWorkflowStub) {
		t.Fatalf("err = %v, want the activation's refusal", err)
	}
}

func TestDeactivateWorkflowAlias_Refused(t *testing.T) {
	ops := &workflowCrudStub{deactivate: func(string, string) (*WorkflowDeactivation, error) { return nil, errWorkflowStub }}
	if err := deactivateWorkflowAlias(ops, "wf_000000000001", "live"); !errors.Is(err, errWorkflowStub) {
		t.Fatalf("err = %v, want the deactivation's refusal", err)
	}
}

// A failure whose read-back fails too keeps what the alias had before, and
// the read's error is a warning beside the failure.
func TestWorkflowAliasFailed_ReadBackFailsToo(t *testing.T) {
	d := aliasData(t)
	d.SetId("wf_000000000001/live")
	ops := &workflowCrudStub{getAlias: func(string, string) (*WorkflowAlias, error) { return nil, errors.New("read refused") }}
	diags := workflowAliasFailed(context.Background(), d, ops, errWorkflowStub, 1, true)
	requireDiagError(t, diags, errWorkflowStub.Error())
	warned := false
	for _, w := range diags {
		warned = warned || (w.Severity == diag.Warning && strings.Contains(w.Summary, "reading alias wf_000000000001/live after the failure"))
	}
	if !warned || d.Get("revision").(int) != 1 || !d.Get("active").(bool) {
		t.Fatalf("diags %v, revision %v, active %v: want the read's warning and revision 1, active", diags, d.Get("revision"), d.Get("active"))
	}
}

func TestWorkflowAliasRead_RevisionReadFails(t *testing.T) {
	d := aliasData(t)
	d.SetId("wf_000000000001/live")
	ops := &workflowOpsStub{
		alias:  &WorkflowAlias{WorkflowId: "wf_000000000001", Name: "live", Revision: 2, Active: true},
		revErr: errWorkflowStub,
	}
	requireDiagError(t, WorkflowAliasRead(context.Background(), d, ops), errWorkflowStub.Error())
}
