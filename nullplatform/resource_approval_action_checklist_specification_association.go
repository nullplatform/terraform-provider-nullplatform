package nullplatform

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// checklistLinkRetryInterval is how long a link refused for live policies
// waits before trying again.
var checklistLinkRetryInterval = 2 * time.Second

// approvalActionHasPolicies is the API's refusal (409) of a link while the
// action still has live policy associations: the only refusal the link waits
// out.
const approvalActionHasPolicies = "Approval action already has policies; cannot also assign a checklist specification"

func resourceApprovalActionChecklistSpecificationAssociation() *schema.Resource {
	return &schema.Resource{
		Description: "Links an approval action to a checklist specification version: while linked, the checklist decides " +
			"the action's requests instead of its policies. Removing the resource unlinks the action. The `id` is the " +
			"approval action's.\n\n" +
			"~> **Note:** One link per action, and only once the action has no live policy associations: the create waits " +
			"for them to go, up to `timeouts.create` (default `1m`). An action already linked to another version is not " +
			"taken over: import the link instead. With `-parallelism=1`, remove the policy associations in one apply and add " +
			"the link in the next.",

		CreateContext: ChecklistSpecificationLinkCreate,
		ReadContext:   ChecklistSpecificationLinkRead,
		UpdateContext: ChecklistSpecificationLinkUpdate,
		DeleteContext: ChecklistSpecificationLinkDelete,

		Importer: &schema.ResourceImporter{StateContext: importChecklistSpecificationLink},

		Timeouts: &schema.ResourceTimeout{Create: schema.DefaultTimeout(time.Minute)},

		Schema: map[string]*schema.Schema{
			"approval_action_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The id of the approval action. Changing it replaces the link.",
			},
			"checklist_specification_id": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validateChecklistSpecificationVersionID,
				Description: "The checklist specification version to link, usually " +
					"`nullplatform_checklist_specification.<name>.current_version_id`: a new version of the specification " +
					"re-links the action in place. The specification's own `id` (`<nrn>/<name>`) is refused.",
			},
		},
	}
}

// validateChecklistSpecificationVersionID refuses anything but a version id:
// at plan time when the value is known, before any request otherwise.
func validateChecklistSpecificationVersionID(value any, _ string) ([]string, []error) {
	if id := value.(string); !strings.HasPrefix(id, "spec_") {
		return nil, []error{fmt.Errorf("checklist_specification_id %q is the id of a nullplatform_checklist_specification, not a "+
			"specification version; reference nullplatform_checklist_specification.<name>.current_version_id", id)}
	}
	return nil, nil
}

// postChecklistSpecificationLink links the version, and links again while the
// action still has live policies — one apply deletes the associations as it
// links — until timeouts.create, then fails naming the policies left. Only
// that refusal is waited out, told by its message; any other fails at once.
func postChecklistSpecificationLink(ctx context.Context, d *schema.ResourceData, nullOps NullOps) error {
	actionID, specID := d.Get("approval_action_id").(string), d.Get("checklist_specification_id").(string)
	if _, errs := validateChecklistSpecificationVersionID(specID, ""); len(errs) > 0 {
		return errs[0]
	}
	ctx, cancel := context.WithTimeout(ctx, d.Timeout(schema.TimeoutCreate))
	defer cancel()
	for {
		err := nullOps.LinkChecklistSpecification(actionID, specID)
		// The message alone tells the refusal apart; the status only travels with
		// it, and is checked as well.
		var refused *ChecklistSpecificationError
		if !errors.As(err, &refused) || refused.StatusCode != http.StatusConflict || refused.Message != approvalActionHasPolicies {
			return err
		}
		select {
		case <-time.After(checklistLinkRetryInterval):
			continue
		case <-ctx.Done():
		}
		var policies []string
		if action, err := nullOps.GetApprovalAction(actionID); err == nil {
			for _, policy := range action.Policies {
				policies = append(policies, strconv.Itoa(policy.Id))
			}
		}
		return fmt.Errorf("approval action %s still has policies %s after %s: %s. Remove those policy associations (one that is "+
			"not in this configuration must be imported or deleted) and apply again", actionID, strings.Join(policies, ", "),
			d.Timeout(schema.TimeoutCreate), refused.Message)
	}
}

// ChecklistSpecificationLinkCreate links an action that has no link yet. An
// action already linked to the desired version — what a link whose answer was
// lost leaves — is adopted without a request; one linked to another version is
// imported, never taken over.
func ChecklistSpecificationLinkCreate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	actionID, specID := d.Get("approval_action_id").(string), d.Get("checklist_specification_id").(string)
	// A specification's own id never compares as a version. Under Terraform the
	// ValidateFunc already refused it in the apply's plan, once the value is
	// known; this guards a direct caller.
	if _, errs := validateChecklistSpecificationVersionID(specID, ""); len(errs) > 0 {
		return diag.FromErr(errs[0])
	}

	action, err := nullOps.GetApprovalAction(actionID)
	if err != nil {
		return diag.FromErr(err)
	}
	if action.ChecklistSpecificationId == specID {
		d.SetId(actionID)
		return ChecklistSpecificationLinkRead(ctx, d, m)
	}
	if action.ChecklistSpecificationId != "" {
		return diag.Errorf("approval action %s is already linked to checklist specification %s; import the link instead: "+
			"terraform import nullplatform_approval_action_checklist_specification_association.<name> %s",
			actionID, action.ChecklistSpecificationId, actionID)
	}
	if err := postChecklistSpecificationLink(ctx, d, nullOps); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(actionID)
	return ChecklistSpecificationLinkRead(ctx, d, m)
}

// ChecklistSpecificationLinkRead reads the link off the action: an action
// unlinked or gone takes the link out of the state, so the plan offers to
// link again; any other error keeps it.
func ChecklistSpecificationLinkRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	action, err := m.(NullOps).GetApprovalAction(d.Id())
	// The router's 404 only comes for an id that is not all digits, which no
	// link holds: that clause is for symmetry with the action's Read.
	if errors.Is(err, errApprovalActionNotFound) || (action != nil && action.Status == "deleted") ||
		(err == nil && action.ChecklistSpecificationId == "") {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	return diag.FromErr(errors.Join(d.Set("approval_action_id", d.Id()),
		d.Set("checklist_specification_id", action.ChecklistSpecificationId)))
}

// ChecklistSpecificationLinkUpdate re-links in place: the API's link replaces
// the one there.
func ChecklistSpecificationLinkUpdate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	if err := postChecklistSpecificationLink(ctx, d, m.(NullOps)); err != nil {
		return diag.FromErr(err)
	}
	return ChecklistSpecificationLinkRead(ctx, d, m)
}

// ChecklistSpecificationLinkDelete unlinks; the action keeps its
// on_checklist_fail.
func ChecklistSpecificationLinkDelete(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	return diag.FromErr(m.(NullOps).UnlinkChecklistSpecification(d.Id()))
}

// importChecklistSpecificationLink takes an action id; an action without a
// link has nothing to import.
func importChecklistSpecificationLink(_ context.Context, d *schema.ResourceData, m any) ([]*schema.ResourceData, error) {
	action, err := m.(NullOps).GetApprovalAction(d.Id())
	if err != nil {
		return nil, err
	}
	if action.ChecklistSpecificationId == "" {
		return nil, fmt.Errorf("approval action %s is not linked to a checklist specification; there is nothing to import", d.Id())
	}
	return []*schema.ResourceData{d}, nil
}
