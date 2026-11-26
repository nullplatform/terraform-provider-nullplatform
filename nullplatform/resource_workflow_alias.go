package nullplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// workflowLatestAlias is the alias the API keeps on the latest revision; it
// cannot be created, moved, activated or deleted.
const workflowLatestAlias = "latest"

func resourceWorkflowAlias() *schema.Resource {
	return &schema.Resource{
		Description: "Publishes a revision of a workflow under a name (an alias, such as `live`) and, while `active`, keeps " +
			"it activated: activation registers the revision's triggers (webhooks, schedules, and the notification " +
			"channels of the platform's triggers, such as a checklist item's). Point `revision` at " +
			"`nullplatform_workflow.<name>.revision` and every change of the definition is published in the same apply. " +
			"The resource `id` is `<workflow_id>/<name>`.\n\n" +
			"~> **Note:** Activation and deactivation act as the identity of the provider's API key: it creates and deletes " +
			"the notification channels of channel triggers, so it must be allowed to manage notification channels at " +
			"each such trigger's NRN. Moving to a new revision is not atomic: the alias moves, then activates; if that " +
			"activation fails, the API leaves the alias inactive, the apply fails, and the next apply activates it again.",

		CreateContext: WorkflowAliasCreate,
		ReadContext:   WorkflowAliasRead,
		UpdateContext: WorkflowAliasUpdate,
		DeleteContext: WorkflowAliasDelete,
		CustomizeDiff: workflowAliasWebhooksDiff,

		Importer: &schema.ResourceImporter{StateContext: importWorkflowAlias},

		Schema: map[string]*schema.Schema{
			"workflow_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				Description: "The id of the workflow, usually `nullplatform_workflow.<name>.id`: its id, not its key. " +
					"Changing it replaces the alias.",
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
				ValidateFunc: validation.All(validation.StringIsNotWhiteSpace,
					validation.StringNotInSlice([]string{workflowLatestAlias}, false)),
				Description: "The alias's name, such as `live`. `latest` is the API's own and cannot be managed. Changing " +
					"it replaces the alias: the old one is deactivated and deleted, the new one created and activated.",
			},
			"revision": {
				Type:         schema.TypeInt,
				Required:     true,
				ValidateFunc: validation.IntAtLeast(1),
				Description: "The revision the alias publishes, usually `nullplatform_workflow.<name>.revision`. While " +
					"`active`, changing it moves the alias and activates it again, which registers the triggers of the new " +
					"revision.",
			},
			"active": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
				Description: "Whether the alias is activated: its triggers registered and listening. Defaults to `true`. " +
					"With `false` the alias is published but nothing triggers it; setting it deactivates the alias. An " +
					"alias deactivated outside Terraform shows in the plan, and the next apply activates it again.",
			},
			"webhook_urls": {
				Type:      schema.TypeMap,
				Computed:  true,
				Sensitive: true,
				Elem:      &schema.Schema{Type: schema.TypeString},
				Description: "The URL of each trigger of the published revision that the engine serves over HTTP " +
					"(a `webhook` trigger, and the ones the platform's notifications reach), by trigger id, while the " +
					"alias is active; empty while it is not. Sensitive: the URL ends in a token that is the capability " +
					"to start the workflow.",
			},
		},
	}
}

// workflowAliasWebhooksDiff plans webhook_urls unknown when the apply
// activates, deactivates or moves the alias: each changes which URLs it has.
func workflowAliasWebhooksDiff(_ context.Context, d *schema.ResourceDiff, _ any) error {
	if d.Id() == "" || !(d.HasChange("active") || d.HasChange("revision") || !d.NewValueKnown("revision")) {
		return nil
	}
	return d.SetNewComputed("webhook_urls")
}

func WorkflowAliasCreate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	workflowID, name, revision := d.Get("workflow_id").(string), d.Get("name").(string), d.Get("revision").(int)

	if _, err := nullOps.CreateWorkflowAlias(workflowID, name, revision); err != nil {
		var refused *WorkflowError
		if errors.As(err, &refused) && refused.StatusCode == http.StatusConflict && refused.HasType("alias-already-exists") {
			return diag.Errorf("%s; to manage it, import it: terraform import <address> %s/%s", err, workflowID, name)
		}
		return diag.FromErr(err)
	}
	// The alias exists from here, inactive: a failed activation keeps it in the
	// state as the API reads it, and the next apply replaces it.
	d.SetId(workflowID + "/" + name)
	// Known from here, so a read that fails plans no change; the next refresh
	// reads the URLs.
	if err := d.Set("webhook_urls", map[string]string{}); err != nil {
		return diag.FromErr(err)
	}
	if d.Get("active").(bool) {
		if err := activateWorkflowAlias(nullOps, workflowID, name, revision, revision); err != nil {
			return workflowAliasFailed(ctx, d, m, err, revision, false)
		}
	}
	diags := WorkflowAliasRead(ctx, d, m)
	for i := range diags {
		if diags[i].Severity == diag.Error {
			diags[i].Severity = diag.Warning
			diags[i].Summary = fmt.Sprintf("alias %s was created, but reading it back failed: %s", d.Id(), diags[i].Summary)
		}
	}
	return diags
}

// activateWorkflowAlias publishes revision under the alias: it moves the alias
// there when it points elsewhere (from), then activates it. The activation
// sends no target revision: it registers the triggers of the revision the
// alias points at.
func activateWorkflowAlias(nullOps NullOps, workflowID, name string, revision, from int) error {
	if from != revision {
		if _, err := nullOps.MoveWorkflowAlias(workflowID, name, revision); err != nil {
			return err
		}
	}
	activation, err := nullOps.ActivateWorkflowAlias(workflowID, name)
	if err != nil {
		return err
	}
	return workflowActivationFailure(activation, workflowID, name, revision)
}

// workflowActivationFailure is the failure an activation's answer reports, or
// nil. The API answers 200 also when it failed: success is the alias active on
// the revision, with no trigger failed and no activation error.
func workflowActivationFailure(activation *WorkflowActivation, workflowID, name string, revision int) error {
	var reasons []string
	reported := map[string]bool{}
	for _, trigger := range activation.FailedTriggers {
		reasons = append(reasons, fmt.Sprintf("trigger %s (%s): %s", trigger.Id, trigger.PluginType, trigger.Error))
		reported[trigger.Error] = true
	}
	if activation.ActivationError != "" && !reported[activation.ActivationError] {
		reasons = append(reasons, activation.ActivationError)
	}
	if !activation.Alias.Active {
		reasons = append(reasons, "the API left the alias inactive")
	}
	if activation.Alias.Revision != revision {
		reasons = append(reasons, fmt.Sprintf("the alias points at revision %d", activation.Alias.Revision))
	}
	if len(reasons) == 0 {
		return nil
	}
	return errors.New(redactMessage(fmt.Sprintf("activating alias %s of workflow %s on revision %d failed: %s",
		name, workflowID, revision, strings.Join(reasons, "; "))))
}

// deactivateWorkflowAlias deactivates the alias and fails when the API
// reports a trigger it could not release: what it names is still
// registered, and the alias is the only pointer to it.
func deactivateWorkflowAlias(nullOps NullOps, workflowID, name string) error {
	deactivation, err := nullOps.DeactivateWorkflowAlias(workflowID, name)
	if err != nil {
		return err
	}
	if deactivation.DeactivateError != "" {
		return errors.New(redactMessage(fmt.Sprintf("deactivating alias %s of workflow %s: the API marked it inactive but "+
			"could not release all its triggers: %s. What it names is still registered; destroying the alias deactivates it "+
			"again, which retries the release",
			name, workflowID, deactivation.DeactivateError)))
	}
	return nil
}

// workflowAliasFailed puts in the state what the API reads after a failed
// activation or deactivation, so the next plan shows what is left to do, and
// returns the failure. When the read fails too, the state keeps the revision
// and activity the alias had before.
func workflowAliasFailed(ctx context.Context, d *schema.ResourceData, m any, failure error, revision int, active bool) diag.Diagnostics {
	diags := diag.FromErr(failure)
	diags = append(diags, diag.FromErr(d.Set("revision", revision))...)
	diags = append(diags, diag.FromErr(d.Set("active", active))...)
	for _, read := range WorkflowAliasRead(ctx, d, m) {
		if read.Severity == diag.Error {
			read.Severity = diag.Warning
			read.Summary = fmt.Sprintf("reading alias %s after the failure: %s", d.Id(), read.Summary)
		}
		diags = append(diags, read)
	}
	return diags
}

// WorkflowAliasRead reads where the alias points and whether it is active,
// and the webhook URLs of its revision's triggers while it is.
func WorkflowAliasRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	alias, err := nullOps.GetWorkflowAlias(d.Get("workflow_id").(string), d.Get("name").(string))
	if isWorkflowGone(err) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	urls := map[string]string{}
	if alias.Active {
		triggers, err := workflowRevisionTriggers(nullOps, alias.WorkflowId, alias.Revision)
		if err != nil {
			return diag.FromErr(err)
		}
		// The alias keeps what every activation registered: only the triggers of
		// the revision it publishes are its own now.
		for _, trigger := range triggers {
			if url := alias.TriggerStates[trigger].RuntimeMetadata.WebhookUrl; url != "" {
				urls[trigger] = url
			}
		}
	}
	d.SetId(alias.WorkflowId + "/" + alias.Name)
	var diags diag.Diagnostics
	for key, value := range map[string]any{
		"workflow_id": alias.WorkflowId, "name": alias.Name, "revision": alias.Revision, "active": alias.Active,
		"webhook_urls": urls,
	} {
		diags = append(diags, diag.FromErr(d.Set(key, value))...)
	}
	return diags
}

// workflowRevisionTriggers is the ids of a revision's triggers: the triggers
// the API derived when it saved the revision, or its steps of type trigger.
func workflowRevisionTriggers(nullOps NullOps, workflowID string, revision int) ([]string, error) {
	read, err := nullOps.GetWorkflowRevision(workflowID, revision)
	if err != nil {
		return nil, err
	}
	var definition struct {
		Triggers []struct {
			Id string `json:"id"`
		} `json:"triggers"`
		Steps map[string]struct {
			Type string `json:"type"`
		} `json:"steps"`
	}
	if err := json.Unmarshal(read.Revision.Definition, &definition); err != nil {
		return nil, fmt.Errorf("revision %d of workflow %s: the API answered a definition that is not a JSON object: %w",
			revision, workflowID, err)
	}
	var triggers []string
	for _, trigger := range definition.Triggers {
		triggers = append(triggers, trigger.Id)
	}
	if definition.Triggers == nil {
		for _, id := range sortedKeys(definition.Steps) {
			if definition.Steps[id].Type == "trigger" {
				triggers = append(triggers, id)
			}
		}
	}
	return triggers, nil
}

// WorkflowAliasUpdate moves the alias and activates it again, or deactivates
// it and moves it. On a failure the state takes what the API reads after it.
func WorkflowAliasUpdate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	workflowID, name, revision := d.Get("workflow_id").(string), d.Get("name").(string), d.Get("revision").(int)
	from, _ := d.GetChange("revision")
	wasActive, _ := d.GetChange("active")

	var err error
	if d.Get("active").(bool) {
		// Every move activates again: activation is what registers the new
		// revision's triggers.
		err = activateWorkflowAlias(nullOps, workflowID, name, revision, from.(int))
	} else {
		if wasActive.(bool) {
			err = deactivateWorkflowAlias(nullOps, workflowID, name)
		}
		if err == nil && from.(int) != revision {
			_, err = nullOps.MoveWorkflowAlias(workflowID, name, revision)
		}
	}
	if err != nil {
		return workflowAliasFailed(ctx, d, m, err, from.(int), wasActive.(bool))
	}
	return WorkflowAliasRead(ctx, d, m)
}

// WorkflowAliasDelete deactivates the alias, also when it reads inactive (a
// deactivation retries the release of what an earlier one could not), and
// deletes it. A release that fails keeps the alias.
func WorkflowAliasDelete(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	workflowID, name := d.Get("workflow_id").(string), d.Get("name").(string)

	err := deactivateWorkflowAlias(nullOps, workflowID, name)
	if err == nil {
		err = nullOps.DeleteWorkflowAlias(workflowID, name)
		var refused *WorkflowError
		if errors.As(err, &refused) && refused.StatusCode == http.StatusConflict && refused.HasType("alias-active") {
			// Activated again since the deactivation (by another apply, or in the
			// editor): once more, then the delete.
			if err = deactivateWorkflowAlias(nullOps, workflowID, name); err == nil {
				err = nullOps.DeleteWorkflowAlias(workflowID, name)
			}
		}
	}
	if err != nil && !isWorkflowGone(err) {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}

// importWorkflowAlias takes "<workflow id or key>/<alias>". The state keeps
// the workflow's id, so an import by key plans no replacement.
func importWorkflowAlias(_ context.Context, d *schema.ResourceData, m any) ([]*schema.ResourceData, error) {
	ref, name, ok := strings.Cut(d.Id(), "/")
	if !ok || ref == "" || name == "" {
		return nil, fmt.Errorf(`workflow alias import id %q is not "<workflow id or key>/<alias>"`, d.Id())
	}
	alias, err := m.(NullOps).GetWorkflowAlias(ref, name)
	if err != nil {
		return nil, fmt.Errorf("cannot import alias %q of workflow %q: %w", name, ref, err)
	}
	d.SetId(alias.WorkflowId + "/" + alias.Name)
	return []*schema.ResourceData{d}, errors.Join(d.Set("workflow_id", alias.WorkflowId), d.Set("name", alias.Name))
}
