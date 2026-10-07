package nullplatform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

var errChecklistAPIDown = errors.New("checklist API down")

// failingChecklistOps fails the named checklist specification calls and passes
// the rest to the fake: the arms a healthy API never reaches.
type failingChecklistOps struct {
	NullOps
	failGet              map[string]bool
	failList, failDelete bool
}

func (o failingChecklistOps) GetChecklistSpecification(id string) (*ChecklistSpecification, error) {
	if o.failGet[id] {
		return nil, errChecklistAPIDown
	}
	return o.NullOps.GetChecklistSpecification(id)
}

func (o failingChecklistOps) ListChecklistSpecifications(nrn string) ([]ChecklistSpecification, error) {
	if o.failList {
		return nil, errChecklistAPIDown
	}
	return o.NullOps.ListChecklistSpecifications(nrn)
}

func (o failingChecklistOps) DeleteChecklistSpecification(id string) error {
	if o.failDelete {
		return errChecklistAPIDown
	}
	return o.NullOps.DeleteChecklistSpecification(id)
}

// specState is a checklist specification as the state holds it.
func specState(versionID string) *schema.ResourceData {
	return resourceChecklistSpecification().Data(&terraform.InstanceState{ID: approvalNrn + "/spec", Attributes: map[string]string{
		"id": approvalNrn + "/spec", "nrn": approvalNrn, "name": "spec", "definition": specJSONV2, "current_version_id": versionID,
	}})
}

// A failure on the way never passes for a missing specification: the error
// surfaces and the specification stays in the state.
func TestChecklistSpecificationResource_APIFailures(t *testing.T) {
	tests := []struct {
		name string
		ops  func(v1, v2 string) failingChecklistOps
		call func(ops NullOps, d *schema.ResourceData) diag.Diagnostics
		want string
	}{
		{"read", func(_, v2 string) failingChecklistOps { return failingChecklistOps{failGet: map[string]bool{v2: true}} },
			func(ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				return ChecklistSpecificationRead(context.Background(), d, ops)
			},
			errChecklistAPIDown.Error()},
		{"update reading the lineage", func(_, v2 string) failingChecklistOps { return failingChecklistOps{failGet: map[string]bool{v2: true}} },
			func(ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				return ChecklistSpecificationUpdate(context.Background(), d, ops)
			},
			errChecklistAPIDown.Error()},
		{"delete reading the lineage", func(_, v2 string) failingChecklistOps { return failingChecklistOps{failGet: map[string]bool{v2: true}} },
			func(ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				return ChecklistSpecificationDelete(context.Background(), d, ops)
			},
			errChecklistAPIDown.Error()},
		{"delete reading a linked version", func(v1, _ string) failingChecklistOps { return failingChecklistOps{failGet: map[string]bool{v1: true}} },
			func(ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				return ChecklistSpecificationDelete(context.Background(), d, ops)
			},
			errChecklistAPIDown.Error()},
		{"delete deleting", func(_, _ string) failingChecklistOps { return failingChecklistOps{failDelete: true} },
			func(ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				return ChecklistSpecificationDelete(context.Background(), d, ops)
			},
			errChecklistAPIDown.Error()},
		{"read without a version id", func(_, _ string) failingChecklistOps { return failingChecklistOps{} },
			func(ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				_ = d.Set("current_version_id", approvalNrn+"/spec")
				return ChecklistSpecificationRead(context.Background(), d, ops)
			},
			`checklist specification "` + approvalNrn + `/spec" has no version id in its state ("` + approvalNrn + `/spec"); import it again`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fake, client := newChecklistFake(t)
			v1, v2 := newLineage(t, client)
			if tt.name == "delete reading a linked version" {
				linkActions(fake, v1.Id, 1, 1)
			}
			ops := tt.ops(v1.Id, v2.Id)
			ops.NullOps = client
			d := specState(v2.Id)

			diags := tt.call(ops, d)
			if !diags.HasError() || diags[0].Summary != tt.want || d.Id() != approvalNrn+"/spec" {
				t.Errorf("diags %v, id %q; want %q and the specification kept", diags, d.Id(), tt.want)
			}
		})
	}
}

// A read that fails right after the create warns: the specification exists,
// its state comes from the create's answer, and the next refresh completes it.
func TestChecklistSpecificationResource_CreateReadFailsWarns(t *testing.T) {
	_, client := newChecklistFake(t)
	d := resourceChecklistSpecification().TestResourceData()
	for key, value := range map[string]string{"nrn": approvalNrn, "name": "spec", "definition": specJSONV1} {
		_ = d.Set(key, value)
	}

	diags := ChecklistSpecificationCreate(context.Background(), d, failingChecklistOps{NullOps: client,
		failGet: map[string]bool{specVersionID(1): true}})
	if diags.HasError() || len(diags) != 1 || diags[0].Severity != diag.Warning || !strings.Contains(diags[0].Summary, errChecklistAPIDown.Error()) {
		t.Errorf("diags %+v; want one warning carrying the read's error", diags)
	}
	if d.Id() != approvalNrn+"/spec" {
		t.Errorf("id %q, want %q", d.Id(), approvalNrn+"/spec")
	}
	for key, want := range map[string]any{"current_version_id": specVersionID(1), "version": 1, "definition": specJSONV1, "description": ""} {
		if got := d.Get(key); got != want {
			t.Errorf("%s = %v, want %v", key, got, want)
		}
	}
}

// An import by "<nrn>/<name>" whose list fails says so.
func TestChecklistSpecificationResource_ImportListFails(t *testing.T) {
	_, client := newChecklistFake(t)
	d := resourceChecklistSpecification().Data(&terraform.InstanceState{ID: approvalNrn + "/spec"})

	_, err := importChecklistSpecification(context.Background(), d, failingChecklistOps{NullOps: client, failList: true})
	if !errors.Is(err, errChecklistAPIDown) {
		t.Errorf("import error = %v, want %v", err, errChecklistAPIDown)
	}
	if strings.Contains(d.Get("current_version_id").(string), "spec_") {
		t.Error("a failed import set a version")
	}
}

// withoutVersions answers every read without its lineage, as a read that did
// not ask for it.
type withoutVersions struct{ NullOps }

func (o withoutVersions) GetChecklistSpecification(id string) (*ChecklistSpecification, error) {
	spec, err := o.NullOps.GetChecklistSpecification(id)
	if spec != nil {
		spec.Versions = nil
	}
	return spec, err
}

// A read without its lineage still destroys the version it read, or refuses
// while an action is on it: never a destroy that deletes nothing.
func TestChecklistSpecificationResource_DeleteWithoutVersions(t *testing.T) {
	for _, linked := range []bool{false, true} {
		t.Run(fmt.Sprintf("linked %v", linked), func(t *testing.T) {
			fake, client := newChecklistFake(t)
			_, v2 := newLineage(t, client)
			if linked {
				linkActions(fake, v2.Id, 7, 1)
			}
			d := specState(v2.Id)

			diags := ChecklistSpecificationDelete(context.Background(), d, withoutVersions{client})
			switch {
			case linked && (!diags.HasError() || !strings.Contains(diags[0].Summary, "(approval actions 7)")):
				t.Errorf("diags %v, want the refusal naming action 7", diags)
			case !linked && (diags.HasError() || specStatuses(fake)[v2.Id] != "deleted" || d.Id() != ""):
				t.Errorf("diags %v, %s %s, id %q; want version 2 deleted and the state empty", diags, v2.Id, specStatuses(fake)[v2.Id], d.Id())
			}
		})
	}
}

// A destroy with a broken version id in its state refuses instead of
// asking the API about a version that is not one.
func TestChecklistSpecificationResource_DeleteWithoutVersionID(t *testing.T) {
	_, client := newChecklistFake(t)
	d := specState(approvalNrn + "/spec")

	diags := ChecklistSpecificationDelete(context.Background(), d, client)
	if want := `checklist specification "` + approvalNrn + `/spec" has no version id in its state ("` + approvalNrn + `/spec"); import it again`; !diags.HasError() || diags[0].Summary != want {
		t.Errorf("diags %v, want %s", diags, want)
	}
}

// An edit of a lineage deleted whole outside Terraform (reached with
// -refresh=false) is refused: a PATCH would bring the name back.
func TestChecklistSpecificationResource_UpdateOfADeletedLineage(t *testing.T) {
	fake, client := newChecklistFake(t)
	v1, v2 := newLineage(t, client)
	for _, id := range []string{v2.Id, v1.Id} {
		if err := client.DeleteChecklistSpecification(id); err != nil {
			t.Fatal(err)
		}
	}
	d := specState(v2.Id)

	diags := ChecklistSpecificationUpdate(context.Background(), d, client)
	want := `every version of checklist specification "` + approvalNrn + `/spec" was deleted outside Terraform; remove it from the state (terraform state rm) or import another name`
	if !diags.HasError() || diags[0].Summary != want || len(requestsOf(fake, http.MethodPatch)) != 1 {
		t.Errorf("diags %v, PATCHes %d; want %s and only newLineage's PATCH", diags, len(requestsOf(fake, http.MethodPatch)), want)
	}
}
