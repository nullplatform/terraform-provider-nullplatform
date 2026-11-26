package nullplatform

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"regexp"
	"strings"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// workflowKeyPattern is the API's rule for a key: a lowercase slug of at most
// 63 characters.
var workflowKeyPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

// workflowRecordKeys are the record's fields: attributes of the resource, so
// never written inside the definition. The API keeps the ones a revision was
// saved with inside it.
var workflowRecordKeys = []string{"name", "key", "description", "path", "nrn"}

// workflowDerivedKeys are what the API adds to every definition it saves.
var workflowDerivedKeys = []string{"id", "triggers", "pinnedDescriptors"}

// workflowOwnedKeys are both: never part of the definition as written.
var workflowOwnedKeys = append(append([]string{}, workflowRecordKeys...), workflowDerivedKeys...)

func resourceWorkflow() *schema.Resource {
	return &schema.Resource{
		Description: "Manages a workflow of the workflow engine: its record (name, folder, owner) and its definition. Every " +
			"change of `definition` saves a new revision; `revision` is the latest one, which a " +
			"`nullplatform_workflow_alias` publishes.\n\n" +
			"~> **Note:** The resource owns the whole workflow. A revision saved outside Terraform (a draft in the editor, " +
			"a publish from elsewhere) is drift: the plan shows it, and the next apply saves the configured definition as a " +
			"new revision. Destroying the resource deletes the workflow with every revision; it fails while an alias of it " +
			"is active.",

		CreateContext: WorkflowCreate,
		ReadContext:   WorkflowRead,
		UpdateContext: WorkflowUpdate,
		DeleteContext: WorkflowDelete,
		CustomizeDiff: workflowRevisionDiff,

		Importer: &schema.ResourceImporter{StateContext: importWorkflow},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.StringIsNotWhiteSpace,
				Description: "The workflow's name. Changing it renames the workflow in place; every revision saved after " +
					"carries the new name.",
			},
			"key": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringMatch(workflowKeyPattern, "must be a lowercase slug: letters, digits and hyphens, at most 63, starting with a letter or a digit"),
				Description: "A stable handle chosen by you, unique in the organization: a lowercase slug of at most 63 " +
					"characters. The API never changes it, so changing it, setting it or removing it replaces the workflow " +
					"(a new id, new webhook URLs). Leave it unset for a workflow that has none.",
			},
			"nrn": {
				Type:     schema.TypeString,
				Optional: true,
				Computed: true,
				Description: "The NRN that owns the workflow: who sees and edits it. When unset the API sets the " +
					"organization's. Changing it moves the workflow.",
			},
			"description": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "A description of the workflow. Changing it saves no revision.",
			},
			"path": {
				Type:         schema.TypeString,
				Optional:     true,
				Computed:     true,
				ValidateFunc: validateWorkflowPath,
				Description: "The folder the workflow is listed in, such as `/governance/releases`: a leading `/`, " +
					"segments joined by one `/`, no trailing `/`. When unset the folder is left as it is. Changing it saves " +
					"no revision.",
			},
			"definition": {
				Type:                  schema.TypeString,
				Required:              true,
				ValidateDiagFunc:      validateWorkflowDefinition,
				DiffSuppressFunc:      suppressEquivalentWorkflowDefinition,
				DiffSuppressOnRefresh: true,
				Description: "The workflow's definition as a JSON object, usually `jsonencode(yamldecode(file(...)))`: " +
					"`steps` (a map of step id to step, each with its `pluginType` and `config`), `connections`, and any " +
					"other section the engine takes (`inputs`, `variables`, `outputs`, `execution`, `metadata`, " +
					"`semanticVersion`, ...). It goes as written. The record's fields are attributes, so `name`, `key`, " +
					"`description`, `path` and `nrn` are refused inside it, and so are `id`, `triggers` and " +
					"`pinnedDescriptors`, which the API derives. Changing it saves a new revision; key order, spacing and " +
					"a step `type` the API fills in are not a change.",
			},
			"revision": {
				Type:     schema.TypeInt,
				Computed: true,
				Description: "The number of the latest revision, as the API reports it. It changes with every change of " +
					"`definition`; an alias that takes it publishes each one.",
			},
		},
	}
}

// validateWorkflowPath takes a folder only as the API stores it, so a plan
// never differs from the read over spelling.
func validateWorkflowPath(v any, key string) ([]string, []error) {
	path := v.(string)
	if path == "" {
		return nil, []error{fmt.Errorf("%s must not be empty: omit it to leave the folder as it is", key)}
	}
	var segments []string
	for _, segment := range strings.Split(path, "/") {
		if segment != "" {
			segments = append(segments, segment)
		}
	}
	if normalized := "/" + strings.Join(segments, "/"); path != normalized {
		return nil, []error{fmt.Errorf("%s must be written as the API stores it: %q", key, normalized)}
	}
	return nil, nil
}

// validateWorkflowDefinition takes a JSON object with steps and without the
// keys the resource or the API own.
func validateWorkflowDefinition(v any, path cty.Path) diag.Diagnostics {
	var definition map[string]json.RawMessage
	if err := json.Unmarshal([]byte(v.(string)), &definition); err != nil || definition == nil {
		return diag.Errorf("definition must be a JSON object (jsonencode of the definition): %v", err)
	}
	var steps map[string]json.RawMessage
	if err := json.Unmarshal(definition["steps"], &steps); err != nil || steps == nil {
		return diag.Errorf("definition must have steps: an object of step id to step")
	}
	var diags diag.Diagnostics
	for _, key := range workflowRecordKeys {
		if _, ok := definition[key]; ok {
			diags = append(diags, diag.Diagnostic{Severity: diag.Error, AttributePath: path,
				Summary: fmt.Sprintf("definition must not have %q: set the resource's %s attribute instead", key, key)})
		}
	}
	for _, key := range workflowDerivedKeys {
		if _, ok := definition[key]; ok {
			diags = append(diags, diag.Diagnostic{Severity: diag.Error, AttributePath: path,
				Summary: fmt.Sprintf("definition must not have %q: the API derives it", key)})
		}
	}
	return diags
}

// workflowDefinitionsEquivalent reports whether a stored definition is the
// written one: the same JSON once the keys the API adds or keeps beside it are
// left out, and a step's type where the same step on the other side has none
// (the API fills it in from the plugin, so writing it down or leaving it out
// saves the same definition).
func workflowDefinitionsEquivalent(written, stored string) bool {
	var w, s map[string]any
	if json.Unmarshal([]byte(written), &w) != nil || json.Unmarshal([]byte(stored), &s) != nil {
		return false
	}
	return reflect.DeepEqual(workflowDefinitionAsWritten(w, s), workflowDefinitionAsWritten(s, w))
}

// workflowDefinitionAsWritten is definition without the record's and the
// API's keys, and without the type of each step whose step in other has none.
// It copies what it changes.
func workflowDefinitionAsWritten(definition, other map[string]any) map[string]any {
	out := map[string]any{}
	for key, value := range definition {
		out[key] = value
	}
	for _, key := range workflowOwnedKeys {
		delete(out, key)
	}
	steps, ok := out["steps"].(map[string]any)
	if !ok {
		return out
	}
	otherSteps, _ := other["steps"].(map[string]any)
	stepsOut := map[string]any{}
	for id, step := range steps {
		stepOut, isObject := step.(map[string]any)
		otherStep, _ := otherSteps[id].(map[string]any)
		if _, typed := otherStep["type"]; !isObject || otherStep == nil || typed {
			stepsOut[id] = step
			continue
		}
		copied := map[string]any{}
		for key, value := range stepOut {
			copied[key] = value
		}
		delete(copied, "type")
		stepsOut[id] = copied
	}
	out["steps"] = stepsOut
	return out
}

func suppressEquivalentWorkflowDefinition(_, old, new string, _ *schema.ResourceData) bool {
	return workflowDefinitionsEquivalent(new, old)
}

// workflowRevisionDiff plans the revision unknown when the apply saves a new
// one: only a definition that is not the same one written otherwise.
func workflowRevisionDiff(_ context.Context, d *schema.ResourceDiff, _ any) error {
	if d.Id() == "" {
		return nil
	}
	old, new := d.GetChange("definition")
	if workflowDefinitionsEquivalent(new.(string), old.(string)) {
		return nil
	}
	return d.SetNewComputed("revision")
}

// workflowBody is the definition's keys, as written, and name: what every
// saved revision carries (the API refuses a revision without a name).
func workflowBody(d *schema.ResourceData) (map[string]json.RawMessage, error) {
	body := map[string]json.RawMessage{}
	if err := json.Unmarshal([]byte(d.Get("definition").(string)), &body); err != nil {
		return nil, fmt.Errorf("definition is not a JSON object: %w", err)
	}
	name, _ := json.Marshal(d.Get("name").(string))
	body["name"] = name
	return body, nil
}

func WorkflowCreate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	body, err := workflowBody(d)
	if err != nil {
		return diag.FromErr(err)
	}
	for _, key := range []string{"key", "description", "path", "nrn"} {
		if value := d.Get(key).(string); value != "" {
			body[key], _ = json.Marshal(value)
		}
	}
	created, err := m.(NullOps).CreateWorkflow(body)
	if err != nil {
		return diag.FromErr(err)
	}
	// The workflow exists from here: a read that fails warns, and the next
	// refresh completes the state.
	d.SetId(created.Workflow.Id)
	diags := diag.FromErr(d.Set("revision", created.Revision.Revision))
	return append(diags, warnInstead(WorkflowRead(ctx, d, m),
		fmt.Sprintf("workflow %s was created, but reading it back failed: ", d.Id()))...)
}

// WorkflowRead reads the record and the latest revision, the one the API
// reports as latest (resolvedVia latest).
func WorkflowRead(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	read, err := m.(NullOps).GetWorkflow(d.Id())
	if isWorkflowGone(err) {
		d.SetId("")
		return nil
	}
	if err != nil {
		return diag.FromErr(err)
	}
	definition, err := workflowStateDefinition(d.Get("definition").(string), read.Revision.Definition)
	if err != nil {
		return diag.FromErr(err)
	}
	workflow := read.Workflow
	d.SetId(workflow.Id)
	var diags diag.Diagnostics
	for key, value := range map[string]any{
		"name": workflow.Name, "key": workflow.Key, "nrn": workflow.Nrn, "description": workflow.Description,
		"path": workflow.Path, "definition": definition, "revision": read.Revision.Revision,
	} {
		diags = append(diags, diag.FromErr(d.Set(key, value))...)
	}
	return diags
}

// workflowStateDefinition is the definition to keep in the state: the one
// written while the stored one is equivalent to it, else the stored one
// without the keys the resource or the API own, as compact JSON with sorted
// keys.
func workflowStateDefinition(written string, stored json.RawMessage) (string, error) {
	if workflowDefinitionsEquivalent(written, string(stored)) {
		return written, nil
	}
	var definition map[string]any
	if err := json.Unmarshal(stored, &definition); err != nil {
		return "", fmt.Errorf("the API answered a definition that is not a JSON object: %w", err)
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(workflowDefinitionAsWritten(definition, definition)); err != nil {
		return "", err
	}
	return strings.TrimSuffix(out.String(), "\n"), nil
}

// WorkflowUpdate renames, describes, moves or refiles the workflow with one
// PATCH of what changed, and saves a new revision only when the definition
// changed. On an error the state keeps what the API had before.
func WorkflowUpdate(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	d.Partial(true)

	patch := map[string]any{}
	for _, key := range []string{"name", "description", "path", "nrn"} {
		if d.HasChange(key) {
			// A removed description goes as "": the API takes no null.
			patch[key] = d.Get(key).(string)
		}
	}
	if len(patch) > 0 {
		if _, err := nullOps.PatchWorkflow(d.Id(), patch); err != nil {
			return diag.FromErr(err)
		}
	}

	old, new := d.GetChange("definition")
	if !workflowDefinitionsEquivalent(new.(string), old.(string)) {
		body, err := workflowBody(d)
		if err != nil {
			return diag.FromErr(err)
		}
		saved, err := nullOps.PutWorkflowRevision(d.Id(), body)
		if err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("revision", saved.Revision.Revision); err != nil {
			return diag.FromErr(err)
		}
	}

	d.Partial(false)
	return WorkflowRead(ctx, d, m)
}

// WorkflowDelete deletes the workflow with every revision. The API refuses it
// while an alias is active; its message says which.
func WorkflowDelete(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	if err := m.(NullOps).DeleteWorkflow(d.Id()); err != nil && !isWorkflowGone(err) {
		return diag.FromErr(err)
	}
	d.SetId("")
	return nil
}

// importWorkflow takes the workflow's id (wf_..., or an older id without the
// prefix) or its key: the API resolves either, and the state keeps the id.
func importWorkflow(_ context.Context, d *schema.ResourceData, m any) ([]*schema.ResourceData, error) {
	read, err := m.(NullOps).GetWorkflow(d.Id())
	if err != nil {
		return nil, fmt.Errorf("cannot import workflow %q: %w", d.Id(), err)
	}
	d.SetId(read.Workflow.Id)
	return []*schema.ResourceData{d}, nil
}
