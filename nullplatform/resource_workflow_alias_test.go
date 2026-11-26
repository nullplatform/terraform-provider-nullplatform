package nullplatform

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// workflowOpsStub answers the two reads the alias helpers make; any other
// call panics on the nil interface, which a test must never reach.
type workflowOpsStub struct {
	NullOps
	alias      *WorkflowAlias
	aliasErr   error
	definition string
	revErr     error
	askedRef   string
	askedName  string
}

func (s *workflowOpsStub) GetWorkflowAlias(ref, name string) (*WorkflowAlias, error) {
	s.askedRef, s.askedName = ref, name
	return s.alias, s.aliasErr
}

func (s *workflowOpsStub) GetWorkflowRevision(string, int) (*WorkflowWithRevision, error) {
	if s.revErr != nil {
		return nil, s.revErr
	}
	return &WorkflowWithRevision{Revision: WorkflowRevision{Definition: json.RawMessage(s.definition)}}, nil
}

func TestWorkflowActivationFailure(t *testing.T) {
	active := WorkflowAlias{Active: true, Revision: 3}
	t.Run("success is nil", func(t *testing.T) {
		if err := workflowActivationFailure(&WorkflowActivation{Alias: active}, "wf_1", "live", 3); err != nil {
			t.Errorf("err = %v, want nil", err)
		}
	})
	tests := []struct {
		name       string
		activation *WorkflowActivation
		want       string
	}{
		{"a failed trigger",
			&WorkflowActivation{Alias: active, FailedTriggers: []WorkflowTriggerResult{{Id: "hook", PluginType: "webhook", Error: "boom"}}},
			"activating alias live of workflow wf_1 on revision 3 failed: trigger hook (webhook): boom"},
		{"an activation error",
			&WorkflowActivation{Alias: active, ActivationError: "bad things"},
			"activating alias live of workflow wf_1 on revision 3 failed: bad things"},
		{"an inactive alias",
			&WorkflowActivation{Alias: WorkflowAlias{Active: false, Revision: 3}},
			"activating alias live of workflow wf_1 on revision 3 failed: the API left the alias inactive"},
		{"the alias on another revision",
			&WorkflowActivation{Alias: WorkflowAlias{Active: true, Revision: 2}},
			"activating alias live of workflow wf_1 on revision 3 failed: the alias points at revision 2"},
		{"every reason together, in order",
			&WorkflowActivation{
				Alias:           WorkflowAlias{Active: false, Revision: 2},
				FailedTriggers:  []WorkflowTriggerResult{{Id: "a", PluginType: "webhook", Error: "e1"}, {Id: "b", PluginType: "np-checklist-trigger", Error: "e2"}},
				ActivationError: "overall",
			},
			"activating alias live of workflow wf_1 on revision 3 failed: trigger a (webhook): e1; trigger b (np-checklist-trigger): e2; " +
				"overall; the API left the alias inactive; the alias points at revision 2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := workflowActivationFailure(tt.activation, "wf_1", "live", 3)
			if err == nil || err.Error() != tt.want {
				t.Errorf("err = %v, want %q", err, tt.want)
			}
		})
	}
	t.Run("an activationError already reported by a failed trigger is not repeated", func(t *testing.T) {
		err := workflowActivationFailure(&WorkflowActivation{
			Alias:           active,
			FailedTriggers:  []WorkflowTriggerResult{{Id: "hook", PluginType: "webhook", Error: "boom"}},
			ActivationError: "boom",
		}, "wf_1", "live", 3)
		if err == nil || strings.Count(err.Error(), "boom") != 1 {
			t.Errorf("err = %v, want boom once", err)
		}
	})
	t.Run("the message is redacted", func(t *testing.T) {
		const token = "supersecrettoken1234567890"
		err := workflowActivationFailure(&WorkflowActivation{Alias: active, ActivationError: "echo Authorization: Bearer " + token}, "wf_1", "live", 3)
		if err == nil || strings.Contains(err.Error(), token) {
			t.Errorf("err = %v, want the token redacted", err)
		}
	})
}

func TestWorkflowRevisionTriggers(t *testing.T) {
	tests := []struct {
		name       string
		definition string
		want       []string
	}{
		{"triggers present, in their order", `{"triggers":[{"id":"b"},{"id":"a"}],"steps":{"c":{"type":"trigger"}}}`, []string{"b", "a"}},
		{"an empty triggers list is not absent", `{"triggers":[],"steps":{"c":{"type":"trigger"}}}`, nil},
		{"triggers absent falls back to the trigger steps, sorted",
			`{"steps":{"z":{"type":"trigger"},"m":{"type":"action"},"a":{"type":"trigger"}}}`, []string{"a", "z"}},
		{"no triggers and no trigger steps", `{"steps":{"m":{"type":"action"}}}`, nil},
		{"null triggers falls back too", `{"triggers":null,"steps":{"t":{"type":"trigger"}}}`, []string{"t"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := workflowRevisionTriggers(&workflowOpsStub{definition: tt.definition}, "wf_1", 2)
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("triggers = %v, want %v", got, tt.want)
			}
		})
	}
	t.Run("a definition that is not an object", func(t *testing.T) {
		_, err := workflowRevisionTriggers(&workflowOpsStub{definition: `[1]`}, "wf_1", 2)
		if err == nil || !strings.Contains(err.Error(), "revision 2 of workflow wf_1") {
			t.Errorf("err = %v, want it to name the revision and workflow", err)
		}
	})
	t.Run("a read error is returned as is", func(t *testing.T) {
		boom := errors.New("boom")
		if _, err := workflowRevisionTriggers(&workflowOpsStub{revErr: boom}, "wf_1", 2); !errors.Is(err, boom) {
			t.Errorf("err = %v, want boom", err)
		}
	})
}

func TestImportWorkflowAlias(t *testing.T) {
	newData := func(id string) *schema.ResourceData {
		d := schema.TestResourceDataRaw(t, resourceWorkflowAlias().Schema, map[string]any{})
		d.SetId(id)
		return d
	}
	for _, id := range []string{"", "wf_1", "/live", "wf_1/", "/"} {
		stub := &workflowOpsStub{}
		_, err := importWorkflowAlias(context.Background(), newData(id), stub)
		if err == nil || !strings.Contains(err.Error(), "<workflow id or key>/<alias>") {
			t.Errorf("id %q: err = %v, want the format error", id, err)
		}
		if stub.askedRef != "" {
			t.Errorf("id %q: the API was asked", id)
		}
	}
	t.Run("by key, the state keeps the workflow id", func(t *testing.T) {
		stub := &workflowOpsStub{alias: &WorkflowAlias{WorkflowId: "wf_1", Name: "live"}}
		d := newData("my-key/live")
		out, err := importWorkflowAlias(context.Background(), d, stub)
		if err != nil || len(out) != 1 {
			t.Fatalf("out = %v, err = %v", out, err)
		}
		if stub.askedRef != "my-key" || stub.askedName != "live" {
			t.Errorf("asked %q/%q, want my-key/live", stub.askedRef, stub.askedName)
		}
		if d.Id() != "wf_1/live" || d.Get("workflow_id") != "wf_1" || d.Get("name") != "live" {
			t.Errorf("id %q, workflow_id %v, name %v", d.Id(), d.Get("workflow_id"), d.Get("name"))
		}
	})
	t.Run("the alias name is everything after the first slash", func(t *testing.T) {
		stub := &workflowOpsStub{alias: &WorkflowAlias{WorkflowId: "wf_1", Name: "a/b"}}
		if _, err := importWorkflowAlias(context.Background(), newData("wf_1/a/b"), stub); err != nil {
			t.Fatal(err)
		}
		if stub.askedName != "a/b" {
			t.Errorf("asked name %q, want a/b", stub.askedName)
		}
	})
	t.Run("a read error is wrapped and keeps its cause", func(t *testing.T) {
		gone := &WorkflowError{StatusCode: 404, Type: "https://example.test/errors/alias-not-found", Message: "no"}
		stub := &workflowOpsStub{aliasErr: gone}
		_, err := importWorkflowAlias(context.Background(), newData("wf_1/live"), stub)
		if err == nil || !errors.Is(err, errWorkflowAliasNotFound) || !strings.Contains(err.Error(), `cannot import alias "live" of workflow "wf_1"`) {
			t.Errorf("err = %v", err)
		}
	})
}
