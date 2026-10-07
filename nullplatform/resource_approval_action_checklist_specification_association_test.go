package nullplatform

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// actionReadFails fails every read of an approval action.
type actionReadFails struct{ NullOps }

func (actionReadFails) GetApprovalAction(string) (*ApprovalAction, error) {
	return nil, errChecklistAPIDown
}

// linkConflict refuses the first link with the refusal it is given, and takes
// the next: a retry would pass.
type linkConflict struct {
	NullOps
	status  int
	message string
	links   *int
}

func (o linkConflict) LinkChecklistSpecification(actionID, specID string) error {
	if *o.links++; *o.links > 1 {
		return nil
	}
	return &ChecklistSpecificationError{Op: "linking approval action " + actionID + " to checklist specification " + specID,
		StatusCode: o.status, Message: o.message}
}

// briefly bounds a call, so a wrong retry fails the test instead of hanging it.
func briefly(t *testing.T) context.Context {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	t.Cleanup(cancel)
	return ctx
}

func linkData(t *testing.T, specID string) *schema.ResourceData {
	d := schema.TestResourceDataRaw(t, resourceApprovalActionChecklistSpecificationAssociation().Schema,
		map[string]any{"approval_action_id": "1", "checklist_specification_id": specID})
	d.SetId("1")
	return d
}

// The arms a healthy API never reaches surface as errors, and a refusal that
// is not live policies fails at once, never waited out.
func TestChecklistSpecificationLinkResource_Failures(t *testing.T) {
	tests := []struct {
		name   string
		specID string
		call   func(ctx context.Context, ops NullOps, d *schema.ResourceData) diag.Diagnostics
		failed bool
		want   string
	}{
		{"create with a value the plan could not check", approvalNrn + "/spec",
			func(ctx context.Context, ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				return ChecklistSpecificationLinkCreate(ctx, d, ops)
			}, false,
			`checklist_specification_id "` + approvalNrn + `/spec" is the id of a nullplatform_checklist_specification, not a specification version; reference nullplatform_checklist_specification.<name>.current_version_id`},
		{"create on an action linked to another version", approvalNrn + "/spec",
			func(ctx context.Context, ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				if err := ops.LinkChecklistSpecification("1", specVersionID(1)); err != nil {
					return diag.FromErr(err)
				}
				return ChecklistSpecificationLinkCreate(ctx, d, ops)
			}, false,
			`checklist_specification_id "` + approvalNrn + `/spec" is the id of a nullplatform_checklist_specification, not a specification version; reference nullplatform_checklist_specification.<name>.current_version_id`},
		{"create reading the action", specVersionID(1),
			func(ctx context.Context, ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				return ChecklistSpecificationLinkCreate(ctx, d, ops)
			}, true,
			errChecklistAPIDown.Error()},
		{"read", specVersionID(1),
			func(ctx context.Context, ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				return ChecklistSpecificationLinkRead(ctx, d, ops)
			}, true,
			errChecklistAPIDown.Error()},
		{"update with a value the plan could not check", approvalNrn + "/spec",
			func(ctx context.Context, ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				return ChecklistSpecificationLinkUpdate(ctx, d, ops)
			}, false,
			`checklist_specification_id "` + approvalNrn + `/spec" is the id of a nullplatform_checklist_specification, not a specification version; reference nullplatform_checklist_specification.<name>.current_version_id`},
		{"update to a version that does not exist", "spec_missing",
			func(ctx context.Context, ops NullOps, d *schema.ResourceData) diag.Diagnostics {
				return ChecklistSpecificationLinkUpdate(ctx, d, ops)
			}, false,
			"error linking approval action 1 to checklist specification spec_missing, got 404: Checklist specification not found"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			shortenLinkRetries(t)
			fake, _, client := newLinkFake(t)
			newAPIAction(t, client)
			var ops NullOps = client
			if tt.failed {
				ops = actionReadFails{client}
			}
			d := linkData(t, tt.specID)

			diags := tt.call(briefly(t), ops, d)
			if !diags.HasError() || diags[0].Summary != tt.want || d.Id() != "1" {
				t.Errorf("diags %v, id %q; want %q and the link kept", diags, d.Id(), tt.want)
			}
			if posts := linkPosts(fake); posts > 1 {
				t.Errorf("sent %d links, want at most one: only live policies are waited out", posts)
			}
		})
	}
}

// Only the live-policies refusal is waited out: another 409, or the
// live-policies text with another status, fails at once.
func TestChecklistSpecificationLinkResource_OtherConflictFailsAtOnce(t *testing.T) {
	shortenLinkRetries(t)
	for _, refusal := range []struct {
		status  int
		message string
	}{
		{http.StatusConflict, "Approval action already has a checklist specification; cannot also associate policies"},
		{http.StatusBadRequest, approvalActionHasPolicies},
	} {
		_, _, client := newLinkFake(t)
		newAPIAction(t, client)
		links := 0

		diags := ChecklistSpecificationLinkCreate(briefly(t), linkData(t, specVersionID(1)), linkConflict{client, refusal.status, refusal.message, &links})
		want := fmt.Sprintf("error linking approval action 1 to checklist specification %s, got %d: %s", specVersionID(1), refusal.status, refusal.message)
		if !diags.HasError() || diags[0].Summary != want || links != 1 {
			t.Errorf("diags %v after %d links, want %q after one", diags, links, want)
		}
	}
}

// policiesNeverGo refuses every link for live policies and cannot read the
// action: the "still has policies" error at the deadline, without the ids.
type policiesNeverGo struct{ actionReadFails }

func (policiesNeverGo) LinkChecklistSpecification(actionID, specID string) error {
	return &ChecklistSpecificationError{Op: "linking approval action " + actionID + " to checklist specification " + specID,
		StatusCode: 409, Message: approvalActionHasPolicies}
}

// That error still comes when the action cannot be read at the deadline:
// without ids, never a panic.
func TestChecklistSpecificationLinkResource_TimeoutWithoutTheAction(t *testing.T) {
	shortenLinkRetries(t)
	_, _, client := newLinkFake(t)

	diags := ChecklistSpecificationLinkUpdate(briefly(t), linkData(t, specVersionID(1)), policiesNeverGo{actionReadFails{client}})
	if !diags.HasError() || !strings.HasPrefix(diags[0].Summary, "approval action 1 still has policies  after ") {
		t.Errorf("diags %v, want the timeout with no policy ids", diags)
	}
}
