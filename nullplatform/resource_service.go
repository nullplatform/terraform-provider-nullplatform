package nullplatform

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/retry"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceService() *schema.Resource {
	return &schema.Resource{
		Description: "The service resource allows you to configure a Nullplatform Service",

		CreateContext: ServiceCreateContext,
		ReadContext:   ServiceReadContext,
		UpdateContext: ServiceUpdateContext,
		DeleteContext: ServiceDeleteContext,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(10 * time.Minute),
			// Update covers the archive/restore wait only; every other update is
			// a single PATCH and never reaches the waiter.
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the entity. Must be a non-empty string and not equal to null.",
			},
			"specification_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique identifier for the entity represented as a UUID.",
			},
			"entity_nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "NRN representing a hierarchical identifier for nullplatform resourcesValue must match regular expression `^organization=[0-9]+(:account=[0-9]+)?(:namespace=[0-9]+)?(:application=[0-9]+)?(:scope=[0-9]+)?$`.",
			},
			"linkable_to": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "A list of NRN representing the visibility settings for the entity. Specifies what/who can see this entity. Value must match regular expression `^organization=[0-9]+(:account=[0-9]+)?(:namespace=[0-9]+)?(:application=[0-9]+)?(:scope=[0-9]+)?$`.",
			},
			"desired_specification_id": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Desired unique identifier for the associated specification.",
			},
			"import": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
				ForceNew: true,
				Description: "When true (default), provisioning and decommissioning of the " +
					"underlying infrastructure are handled externally to nullplatform. " +
					"When false, the specification's create and delete actions are triggered " +
					"to handle the infrastructure lifecycle.",
			},
			"force_destroy": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				Description: "Only meaningful when `import = false`. When true, `terraform destroy` " +
					"skips the delete action and removes the service record directly via " +
					"`DELETE /service/{id}?force=true`. Use this as an escape hatch when the " +
					"service is stuck (e.g. the create action failed). Note: Terraform's " +
					"destroy reads this attribute from state, so you must run `terraform apply` " +
					"with `force_destroy = true` *before* running `terraform destroy` for it to " +
					"take effect. For tainted resources, run `terraform untaint` first so the " +
					"apply is an update rather than a replace. Has no effect when " +
					"`import = true`, where destroy already uses force.",
			},
			"archive_on_destroy": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				Description: "When true, `terraform destroy` archives the service " +
					"(`PATCH {\"status\": \"archived\"}`) and waits for the transition to finish " +
					"instead of deleting it. The service row, its attributes and its " +
					"infrastructure survive and can be restored later by setting " +
					"`status = \"active\"` on a re-imported resource. Note: Terraform's destroy " +
					"reads this attribute from state, so you must run `terraform apply` with " +
					"`archive_on_destroy = true` *before* running `terraform destroy` for it to " +
					"take effect. For tainted resources, run `terraform untaint` first so the " +
					"apply is an update rather than a replace. `force_destroy = true` wins over " +
					"this flag: an escape hatch is asked for when the record must be gone.",
			},
			"messages": {
				Type:     schema.TypeList,
				Computed: true,
				Elem: &schema.Schema{
					Type: schema.TypeMap,
					Elem: &schema.Schema{
						Type: schema.TypeString,
					},
				},
				Description: "A message and its severity level",
			},
			"attributes": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Attributes associated with the service, should be valid against the service specification attribute schema.",
			},
			"dimensions": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Object representing dimensions with key-value pairs.",
			},
			"selectors": {
				Type:     schema.TypeList,
				Optional: true,
				// Computed too: the API answers a selectors object even when the
				// configuration declares none, and Read records it. Without
				// Computed, a config that omits the block planned its removal on
				// every run — a perpetual diff the functional harness caught.
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"category": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Category of the service specification",
						},
						"imported": {
							Type:        schema.TypeBool,
							Required:    true,
							Description: "Indicates whether the service is imported",
						},
						"provider": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Provider of the service (e.g., AWS, GCP)",
						},
						"sub_category": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Sub-category of the service",
						},
					},
				},
				Description: "Selectors for the service specification",
			},
			"status": {
				Type:     schema.TypeString,
				Optional: true,
				// Optional+Computed, *not* `Default: "active"`. With a schema default,
				// a service archived out of band planned as `archived` -> `active` on
				// the next unrelated apply and was silently restored. Computed means an
				// omitted `status` keeps whatever the platform reports, so only an
				// explicit `status = "active"` restores an archived service — a visible,
				// deliberate diff. The create-time `active` default now lives in
				// ServiceCreateContext.
				Computed: true,
				Description: "Status of the service. Should be one of: [`pending_create`, `pending`, " +
					"`creating`, `updating`, `deleting`, `archiving`, `active`, `archived`, `deleted`, " +
					"`failed`, `cancelled`]. Defaults to `active` when the configuration omits it on " +
					"create (`pending` when `import = false`, so the specification's create action " +
					"drives the transition). Leave it unset to track the platform's value: Terraform " +
					"then never plans a status change on its own. Setting `archived` archives the " +
					"service and setting `active` on an archived service restores it; when the " +
					"specification runs archive/unarchive as managed actions, the apply waits for " +
					"the transition to land.",
			},
			"archived_at": {
				Type:     schema.TypeString,
				Computed: true,
				Description: "Timestamp of the last time the service was archived. Empty while the " +
					"service is not archived.",
			},
		},
	}
}

func ServiceCreateContext(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	name := d.Get("name").(string)
	specificationId := d.Get("specification_id").(string)
	entityNrn := d.Get("entity_nrn").(string)
	linkableTo := d.Get("linkable_to").([]interface{})
	desiredSpecificationId := d.Get("desired_specification_id").(string)
	status := d.Get("status").(string)
	if status == "" {
		// `status` is Optional+Computed with no schema default, so an omitted
		// configuration value reads back as "". 'active' is the declarative
		// default this resource has always POSTed; it lives here now instead of
		// in the schema so that an out-of-band archive is not planned away
		// (see the `status` schema comment).
		status = "active"
	}
	if !importMode(d) {
		// Action-driven mode: the create action requires the service to be
		// in 'pending' on POST /service so the action can transition it to
		// active. The 'active' default is the right default for
		// import=true (declarative), but wrong here.
		status = "pending"
	}
	messages := d.Get("messages").([]interface{})
	attributes := d.Get("attributes").(map[string]interface{})
	dimensions := d.Get("dimensions").(map[string]interface{})
	selectorsList := d.Get("selectors").([]interface{})
	var selectors Selectors
	if len(selectorsList) > 0 {
		selectorsMap := selectorsList[0].(map[string]interface{})
		selectors = Selectors{
			Category:    selectorsMap["category"].(string),
			Imported:    selectorsMap["imported"].(bool),
			Provider:    selectorsMap["provider"].(string),
			SubCategory: selectorsMap["sub_category"].(string),
		}
	}

	newService := &Service{
		Name:                   name,
		SpecificationId:        specificationId,
		DesiredSpecificationId: desiredSpecificationId,
		EntityNrn:              entityNrn,
		LinkableTo:             linkableTo,
		Status:                 status,
		Messages:               messages,
		Selectors:              &selectors,
		Attributes:             attributes,
		Dimensions:             dimensions,
	}

	s, err := nullOps.CreateService(newService)

	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId(s.Id)

	// `status` and `archived_at` are Computed: unknown in the plan when the
	// configuration omits them, so Create has to land a concrete value or state
	// keeps the empty string until the next refresh.
	createdStatus := s.Status
	if createdStatus == "" {
		createdStatus = status
	}
	if err := d.Set("status", createdStatus); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("archived_at", s.ArchivedAt); err != nil {
		return diag.FromErr(err)
	}

	if !importMode(d) {
		attrs, _ := d.Get("attributes").(map[string]interface{})
		if err := triggerServiceAction(ctx, nullOps, s.Id, s.SpecificationId, "create", attrs, d.Timeout(schema.TimeoutCreate)); err != nil {
			return diag.FromErr(err)
		}
	}

	// Create ends in Read — the SDK's recommended shape — so EVERY computed
	// attribute lands in state and the post-apply plan is empty without a
	// refresh (`messages` alone stayed unknown forever before this). Best
	// effort: the service exists (and any create action has succeeded), so a
	// failed read must not fail the apply — the next refresh corrects it.
	if diags := ServiceReadContext(ctx, d, m); diags.HasError() {
		log.Printf("[WARN] could not read service %s back after create: %v", s.Id, diags)
	}
	return nil
}

func ServiceReadContext(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	serviceID := d.Id()

	s, err := nullOps.GetService(serviceID)

	if err != nil {
		d.SetId("")
		return diag.FromErr(err)
	}

	if err := d.Set("name", s.Name); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("specification_id", s.SpecificationId); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("desired_specification_id", s.DesiredSpecificationId); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("entity_nrn", s.EntityNrn); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("linkable_to", s.LinkableTo); err != nil {
		return diag.FromErr(err)
	}

	// Guarded like the create fallback: an older API that answers no status
	// must not blank a value state already holds.
	if s.Status != "" {
		if err := d.Set("status", s.Status); err != nil {
			return diag.FromErr(err)
		}
	}

	if err := d.Set("archived_at", s.ArchivedAt); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("dimensions", s.Dimensions); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("messages", s.Messages); err != nil {
		return diag.FromErr(err)
	}

	// A response without `selectors` must not crash the plugin — the same
	// guard both specification resources carry.
	if s.Selectors != nil {
		selectors := []map[string]interface{}{
			{
				"category":     s.Selectors.Category,
				"imported":     s.Selectors.Imported,
				"provider":     s.Selectors.Provider,
				"sub_category": s.Selectors.SubCategory,
			},
		}
		if err := d.Set("selectors", selectors); err != nil {
			return diag.FromErr(err)
		}
	}

	attributeMap := mapOfInterfacesToMapOfStrings(s.Attributes)
	if err := d.Set("attributes", attributeMap); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func ServiceUpdateContext(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	serviceID := d.Id()

	log.Println("serviceID:", serviceID)

	// Classify the status change before the PATCH: archive and restore are their
	// own verbs on the API, and a specification whose archive/unarchive action is
	// managed runs them asynchronously, so the PATCH only *starts* the
	// transition. Everything else stays a single PATCH with no polling.
	transition, fromStatus := statusTransition(d)

	ps := &Service{}

	if d.HasChange("name") {
		ps.Name = d.Get("name").(string)
	}

	if d.HasChange("status") {
		ps.Status = d.Get("status").(string)
	}

	if d.HasChange("specification_id") {
		ps.SpecificationId = d.Get("specification_id").(string)
	}

	if d.HasChange("entity_nrn") {
		ps.EntityNrn = d.Get("entity_nrn").(string)
	}

	if d.HasChange("linkable_to") {
		ps.LinkableTo = d.Get("linkable_to").([]interface{})
	}

	if d.HasChange("dimensions") {
		dimensions := d.Get("dimensions").(map[string]interface{})

		ps.Dimensions = dimensions
	}

	if d.HasChange("attributes") {
		attributes := d.Get("attributes").(map[string]interface{})

		ps.Attributes = attributes
	}

	if d.HasChange("selectors") {
		selectorsList := d.Get("selectors").([]interface{})
		if len(selectorsList) > 0 {
			selectorsMap := selectorsList[0].(map[string]interface{})
			ps.Selectors = &Selectors{
				Category:    selectorsMap["category"].(string),
				Imported:    selectorsMap["imported"].(bool),
				Provider:    selectorsMap["provider"].(string),
				SubCategory: selectorsMap["sub_category"].(string),
			}
		}
	}

	// An archive/restore request cannot carry attributes: the minted action's
	// parameters are `{}` and the direct flip writes none, so the API refuses the
	// combination rather than dropping them silently. Terraform users routinely
	// change both in one apply, so send the attributes first on their own and let
	// the status transition follow — the same two applies they would otherwise be
	// told to run by hand.
	if transition != "" && ps.Attributes != nil {
		attributesOnly := &Service{Attributes: ps.Attributes}
		ps.Attributes = nil
		if err := nullOps.PatchService(serviceID, attributesOnly); err != nil {
			return diag.FromErr(err)
		}
	}

	if !reflect.DeepEqual(*ps, Service{}) {
		err := nullOps.PatchService(serviceID, ps)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if transition != "" {
		s, err := waitForServiceStatusTerminal(ctx, nullOps, serviceID, transition, fromStatus, d.Timeout(schema.TimeoutUpdate))
		if err != nil {
			return diag.FromErr(err)
		}
		if s != nil {
			if err := d.Set("status", s.Status); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("archived_at", s.ArchivedAt); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	return nil
}

func ServiceDeleteContext(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)
	serviceID := d.Id()

	forceDestroy := d.Get("force_destroy").(bool)

	// force_destroy is the escape hatch and outranks archive_on_destroy: someone
	// reaching for it wants the record gone, not preserved.
	if d.Get("archive_on_destroy").(bool) && !forceDestroy {
		return archiveServiceOnDestroy(ctx, d, nullOps, serviceID)
	}

	if importMode(d) || forceDestroy {
		if err := nullOps.DeleteService(serviceID, true); err != nil {
			return diag.FromErr(err)
		}
		d.SetId("")
		return nil
	}

	// Stuck-service recovery: a service whose create action failed sits in
	// status="failed" and cannot be cleanly torn down via its delete action
	// (the workflow runtime can't drive a broken state machine). Detect this
	// case by reading the live status and force-delete instead. Without this,
	// users would have to either (a) untaint+apply force_destroy=true into
	// state then destroy, or (b) manually clean up via the API.
	//
	// An `archived` service needs nothing special here: the delete action's
	// status guard admits active/failed/cancelled/archived, so it falls through
	// to the ordinary delete-action path below. `archiving` does not, and its
	// refusal is an opaque 400 from the mint — name it instead.
	current, err := nullOps.GetService(serviceID)
	if err == nil && current != nil {
		if current.Status == "failed" {
			log.Printf("[INFO] service %s is in status=failed; force-deleting instead of triggering delete action", serviceID)
			if err := nullOps.DeleteService(serviceID, true); err != nil {
				return diag.FromErr(err)
			}
			d.SetId("")
			return nil
		}
		if current.Status == "archiving" {
			return diag.Errorf("service %s is being archived and cannot be deleted yet: the delete "+
				"action only runs on an active, failed, cancelled or archived service. Wait for the "+
				"archive to reach `archived` and destroy again, or apply `force_destroy = true` to "+
				"remove the record directly.", serviceID)
		}
	}

	specificationID := d.Get("specification_id").(string)
	attrs, _ := d.Get("attributes").(map[string]interface{})
	if err := triggerServiceAction(ctx, nullOps, serviceID, specificationID, "delete", attrs, d.Timeout(schema.TimeoutDelete)); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

// archiveServiceOnDestroy implements `archive_on_destroy`: destroy archives the
// service instead of deleting it, then drops it from state. The row, its
// attributes and its infrastructure survive and stay restorable.
func archiveServiceOnDestroy(ctx context.Context, d *schema.ResourceData, nullOps NullOps, serviceID string) diag.Diagnostics {
	current, err := nullOps.GetService(serviceID)
	if err != nil {
		return diag.FromErr(fmt.Errorf("reading service %s before archiving it on destroy: %w", serviceID, err))
	}
	if current == nil {
		d.SetId("")
		return nil
	}

	switch current.Status {
	case "archived":
		// Already archived out of band, or by a destroy that died after the
		// transition landed. Re-PATCHing `archived` is refused (only an active,
		// failed or cancelled service archives), so there is nothing left to do.
		log.Printf("[INFO] service %s is already archived; dropping it from state without a status change", serviceID)
		d.SetId("")
		return nil
	case "archiving":
		// An archive is already running — join it rather than starting a second
		// one, which the mint would refuse.
		log.Printf("[INFO] service %s is already archiving; waiting for it to land instead of re-issuing the archive", serviceID)
	default:
		if err := nullOps.PatchService(serviceID, &Service{Status: "archived"}); err != nil {
			return diag.FromErr(err)
		}
	}

	if _, err := waitForServiceStatusTerminal(ctx, nullOps, serviceID, "archive", current.Status, d.Timeout(schema.TimeoutDelete)); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}

// statusTransition names the lifecycle verb a `status` change requests and the
// status the instance is transitioning from, or ("", "") for an ordinary
// metadata update. Services and links classify identically, so they share this.
//
// Mirrors the API's own classification: `archived` always requests an archive,
// while `active` is a restore only when the instance is currently archived —
// `active` on anything else stays the legacy direct write it always was.
func statusTransition(d *schema.ResourceData) (transition, fromStatus string) {
	if !d.HasChange("status") {
		return "", ""
	}
	previous, desired := d.GetChange("status")
	from, _ := previous.(string)
	to, _ := desired.(string)

	if to == "archived" {
		return "archive", from
	}
	if to == "active" && from == "archived" {
		return "unarchive", from
	}
	return "", ""
}

// A var rather than a const so tests can shrink it.
var actionPollInterval = 15 * time.Second

// statusPollInterval is shorter than actionPollInterval: a status transition can
// be over before the first poll (an unmanaged specification flips it inside the
// PATCH), so the wait should not be dominated by the interval. A var rather than
// a const so tests can shrink it.
var statusPollInterval = 5 * time.Second

// waitForServiceStatusTerminal polls the service until an archive or a restore
// lands.
//
// Sibling of waitForActionTerminal, watching the instance instead of an action,
// because there is not always an action to watch: a specification with a managed
// archive/unarchive action mints one and the PATCH returns while the transition
// is still running, but a specification with no such action flips the status
// synchronously and mints nothing. Polling the instance covers both, and the
// synchronous case costs exactly one GET.
//
// The transient statuses come from the API's action-outcome table: an archive
// sits in `archiving`, and a restore has no `unarchiving` status of its own — it
// transits `updating` and lands on the ordinary `active` success path.
func waitForServiceStatusTerminal(ctx context.Context, nullOps NullOps, serviceID, transition, fromStatus string, timeout time.Duration) (*Service, error) {
	raw, err := waitForInstanceStatusTerminal(ctx, instanceStatusWait{
		entity:     "service",
		id:         serviceID,
		transition: transition,
		fromStatus: fromStatus,
		timeout:    timeout,
		read: func() (any, string, []interface{}, []ActionInProgress, error) {
			s, err := nullOps.GetService(serviceID)
			if err != nil {
				return nil, "", nil, nil, err
			}
			return s, s.Status, s.Messages, s.ActionsInProgress, nil
		},
	})
	if err != nil {
		return nil, err
	}
	if s, ok := raw.(*Service); ok {
		return s, nil
	}
	return nil, nil
}

// ActionInProgress is the summary GET /service/:id and GET /link/:id attach as
// `actions_in_progress`: every action still running against the instance, with
// its type joined from the action specification. `pending_create` is the parked
// state — an action minted behind an approval policy sits there, untouchable by
// its agent, until someone approves it.
type ActionInProgress struct {
	Id     string `json:"id,omitempty"`
	Type   string `json:"type,omitempty"`
	Status string `json:"status,omitempty"`
}

// instanceStatusWait is the input to waitForInstanceStatusTerminal. `read`
// returns the entity itself (handed back to the caller), its status, any
// messages to quote when the transition fails — links carry none — and the
// instance's in-progress actions, which is where a parked approval shows up.
type instanceStatusWait struct {
	entity     string
	id         string
	transition string
	fromStatus string
	timeout    time.Duration
	read       func() (any, string, []interface{}, []ActionInProgress, error)
}

// waitForInstanceStatusTerminal polls a service or a link until an archive or a
// restore lands. Both entities have the same status machine, so they share the
// waiter rather than keeping twin copies that drift.
func waitForInstanceStatusTerminal(ctx context.Context, w instanceStatusWait) (any, error) {
	var pending, target []string
	// An instance may be archived from `failed`, so `failed` cannot double as the
	// failure signal when that is where the transition started.
	failedIsFailure := w.fromStatus != "failed"

	switch w.transition {
	case "archive":
		// Only an active, failed or cancelled instance archives.
		pending = []string{"active", "cancelled", "archiving"}
		if !failedIsFailure {
			pending = append(pending, "failed")
		}
		target = []string{"archived"}
	case "unarchive":
		// Only an archived instance restores.
		pending = []string{"archived", "updating"}
		target = []string{"active"}
	default:
		return nil, fmt.Errorf("unknown %s status transition %q", w.entity, w.transition)
	}

	// A managed transition behind an approval policy parks its action in
	// `pending_create` and the instance NEVER leaves its from-status until a
	// human (or an approval webhook) decides — which a status poller cannot
	// tell apart from "slow". Track the parked action so the wait can explain
	// itself: a WARN while it waits (auto-approvals land in seconds, so the
	// wait itself is correct), the reason in the timeout error instead of a
	// generic "timed out", and a prompt failure when the parked action
	// disappears while the status never moved — that is a denial (or a manual
	// cancel), and nothing will ever move again.
	var parked *ActionInProgress
	parkedWarned := false

	stateConf := &retry.StateChangeConf{
		Pending: pending,
		Target:  target,
		Refresh: func() (interface{}, string, error) {
			entity, status, messages, actions, err := w.read()
			if err != nil {
				return nil, "", err
			}
			if status == "failed" && failedIsFailure {
				return entity, status, fmt.Errorf("%s %s ended in status %q while running %s: %s",
					w.entity, w.id, status, w.transition, summarizeMessages(messages))
			}

			switch nowParked := parkedTransitionAction(actions, w.transition); {
			case nowParked != nil:
				parked = nowParked
				if !parkedWarned {
					parkedWarned = true
					log.Printf("[WARN] %s %s: the %s action %s is waiting for approval; "+
						"the apply waits for it (timeout %s)", w.entity, w.id, w.transition, parked.Id, w.timeout)
				}
			case parked != nil && actionInProgress(actions, parked.Id):
				// Approved: the action left `pending_create` for a running status.
				// It is not parked anymore — the ordinary wait takes over (and a
				// later agent failure reports through the `failed` branch above).
				parked = nil
			case parked != nil && status == w.fromStatus:
				// The parked action is GONE and the status never moved: the
				// approval was denied (or the action cancelled by hand), and
				// nothing will ever move again. Say so now, not at the timeout.
				return entity, status, fmt.Errorf("the %s action %s on %s %s was cancelled while waiting "+
					"for approval (denied, or cancelled by hand) and the %s stayed %q. Re-run apply to "+
					"request the %s again",
					w.transition, parked.Id, w.entity, w.id, w.entity, status, w.transition)
			}
			return entity, status, nil
		},
		Timeout: w.timeout,
		// No initial delay: an unmanaged specification has already flipped the
		// status by the time the PATCH returns, and that case must not pay a
		// poll interval it does not need.
		Delay:      0,
		MinTimeout: statusPollInterval,
	}
	raw, err := stateConf.WaitForStateContext(ctx)
	if err != nil && parked != nil {
		// The generic timeout would read as the platform being stuck; it is not.
		return raw, fmt.Errorf("%s %s did not finish its %s within %s because action %s is still "+
			"waiting for approval. Approve it (or raise the update timeout) and re-run apply: %w",
			w.entity, w.id, w.transition, w.timeout, parked.Id, err)
	}
	return raw, err
}

// parkedTransitionAction finds the in-progress action that carries this
// transition and is parked awaiting approval. Matching on type keeps an
// unrelated parked action (a custom verb waiting for its own approval) from
// being blamed for this wait.
func parkedTransitionAction(actions []ActionInProgress, transition string) *ActionInProgress {
	for i := range actions {
		if actions[i].Type == transition && actions[i].Status == "pending_create" {
			return &actions[i]
		}
	}
	return nil
}

// actionInProgress reports whether the action is still among the instance's
// in-progress actions, whatever its status. The distinction matters after an
// approval: the action leaves `pending_create` but stays in-progress, which is
// the opposite of a denial, where it disappears entirely.
func actionInProgress(actions []ActionInProgress, id string) bool {
	for i := range actions {
		if actions[i].Id == id {
			return true
		}
	}
	return false
}

func waitForActionTerminal(ctx context.Context, nullOps NullOps, serviceID, actionID string, timeout time.Duration) (*ActionInstance, error) {
	stateConf := &retry.StateChangeConf{
		Pending: []string{"pending_create", "pending", "in_progress"},
		Target:  []string{"success"},
		Refresh: func() (interface{}, string, error) {
			a, err := nullOps.GetServiceAction(serviceID, actionID)
			if err != nil {
				return nil, "", err
			}
			if a.Status == "failed" || a.Status == "cancelled" {
				return a, a.Status, fmt.Errorf("action %s ended in status %q: %s",
					actionID, a.Status, summarizeMessages(a.Messages))
			}
			return a, a.Status, nil
		},
		Timeout:    timeout,
		Delay:      actionPollInterval,
		MinTimeout: actionPollInterval,
	}
	raw, err := stateConf.WaitForStateContext(ctx)
	if err != nil {
		return nil, err
	}
	if a, ok := raw.(*ActionInstance); ok {
		return a, nil
	}
	return nil, nil
}

func triggerServiceAction(ctx context.Context, nullOps NullOps, serviceID, specificationID, actionType string, attributes map[string]interface{}, timeout time.Duration) error {
	specs, err := nullOps.ListActionSpecifications(specificationID)
	if err != nil {
		return fmt.Errorf("listing action specifications: %w", err)
	}
	actionSpec, err := findActionSpecByType(specs, actionType)
	if err != nil {
		return fmt.Errorf("specification %s: %w", specificationID, err)
	}

	parameters, err := projectAttributesToParameters(attributes, actionSpec.Parameters)
	if err != nil {
		return fmt.Errorf("projecting attributes onto %s action parameter schema: %w", actionType, err)
	}

	action, err := nullOps.CreateServiceAction(serviceID, &ActionInstance{
		SpecificationId: actionSpec.Id,
		Parameters:      parameters,
	})
	if err != nil {
		return fmt.Errorf("creating %s action: %w", actionType, err)
	}

	if _, err := waitForActionTerminal(ctx, nullOps, serviceID, action.Id, timeout); err != nil {
		return err
	}
	return nil
}

// importMode reads the `import` attribute defensively. The schema's `Default: true`
// only applies during plan-time evaluation of new resources; for legacy state
// written before this attribute existed, the field is absent and `d.Get` returns
// the zero value (false). This helper restores the desired default by treating
// missing-from-state as `true`.
func importMode(d *schema.ResourceData) bool {
	if v, exists := d.GetOkExists("import"); exists {
		return v.(bool)
	}
	return true
}
