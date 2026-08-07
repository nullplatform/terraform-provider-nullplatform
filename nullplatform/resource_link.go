package nullplatform

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceLink() *schema.Resource {
	return &schema.Resource{
		Description: "The link resource allows you to configure a Nullplatform Link",

		// Context handlers rather than the legacy pair: archive and restore are
		// asynchronous when the specification runs them as managed actions, so
		// Update and Delete need a context and a timeout to wait on. Create and
		// Read keep their plain signatures underneath.
		CreateContext: LinkCreateContext,
		ReadContext:   LinkReadContext,
		UpdateContext: LinkUpdateContext,
		DeleteContext: LinkDeleteContext,

		Timeouts: &schema.ResourceTimeout{
			// Only the archive/restore wait uses these; every other update is a
			// single PATCH that never reaches the waiter.
			Update: schema.DefaultTimeout(10 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the entity. Must be a non-empty string and not equal to null.",
			},
			"slug": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Slug of the entity. Automatically generated from `name`.",
			},
			"service_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique identifier for the entity represented as a UUID.",
			},
			"specification_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique identifier for the entity represented as a UUID.",
			},
			"entity_nrn": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "NRN representing a hierarchical identifier for nullplatform resources. Value must match regular expression `^organization=[0-9]+(:account=[0-9]+)?(:namespace=[0-9]+)?(:application=[0-9]+)?(:scope=[0-9]+)?$`.",
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
				Computed:    true,
				Description: "Desired unique identifier for the associated specification.",
			},
			"attributes": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Attributes associated with the link, should be valid against the link specification attribute schema.",
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
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
				Description: "Key-value object representing instance selectors.",
			},
			"status": {
				Type:     schema.TypeString,
				Optional: true,
				// Optional+Computed: an omitted `status` tracks whatever the platform
				// reports. Without Computed, an unset configuration read back as "" and
				// diffed against the `active` the API assigns, so every plan showed a
				// phantom `active` -> `""` change that PATCHed nothing; and once archive
				// exists, that same diff would be a silent restore of a link archived
				// out of band.
				Computed: true,
				Description: "Status of the link. Should be one of: [`pending_create`, `pending`, " +
					"`creating`, `updating`, `deleting`, `archiving`, `active`, `archived`, `deleted`, " +
					"`failed`, `cancelled`]. Leave it unset to track the platform's value: Terraform " +
					"then never plans a status change on its own. Setting `archived` archives the " +
					"link (which removes its parameters) and setting `active` on an archived link " +
					"restores them.",
			},
			"archived_at": {
				Type:     schema.TypeString,
				Computed: true,
				Description: "Timestamp of the last time the link was archived. Empty while the link " +
					"is not archived.",
			},
			"archive_on_destroy": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				Description: "When true, `terraform destroy` archives the link " +
					"(`PATCH {\"status\": \"archived\"}`) and waits for the transition to finish " +
					"instead of deleting it. Set it on the links of a service that also uses " +
					"`archive_on_destroy`: a service cannot be archived while it still has " +
					"non-archived links, and a destroy that deletes the links while archiving the " +
					"service leaves a restorable service whose links are gone. Terraform's destroy " +
					"reads this attribute from state, so you must run `terraform apply` with " +
					"`archive_on_destroy = true` *before* running `terraform destroy` for it to " +
					"take effect.",
			},
		},
	}
}

func LinkCreateContext(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	if err := LinkCreate(d, m); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func LinkReadContext(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	if err := LinkRead(d, m); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func LinkCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	name := d.Get("name").(string)
	specificationId := d.Get("specification_id").(string)
	serviceId := d.Get("service_id").(string)
	entityNrn := d.Get("entity_nrn").(string)
	linkableTo := d.Get("linkable_to").([]interface{})
	status := d.Get("status").(string)
	attributes := d.Get("attributes").(map[string]interface{})
	dimensions := d.Get("dimensions").(map[string]interface{})
	selectors := d.Get("selectors").(map[string]interface{})

	newLink := &Link{
		Name:            name,
		ServiceId:       serviceId,
		SpecificationId: specificationId,
		EntityNrn:       entityNrn,
		LinkableTo:      linkableTo,
		Status:          status,
		Selectors:       selectors,
		Attributes:      attributes,
		Dimensions:      dimensions,
	}

	l, err := nullOps.CreateLink(newLink)

	if err != nil {
		return err
	}

	d.SetId(l.Id)

	// `status` and `archived_at` are Computed: unknown in the plan when the
	// configuration omits them, so Create has to land a concrete value or state
	// keeps the empty string until the next refresh.
	if err := d.Set("status", l.Status); err != nil {
		return err
	}
	if err := d.Set("archived_at", l.ArchivedAt); err != nil {
		return err
	}

	return nil
}

func LinkRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	linkId := d.Id()

	l, err := nullOps.GetLink(linkId)

	if err != nil {
		d.SetId("")
		return err
	}

	if err := d.Set("name", l.Name); err != nil {
		return err
	}

	if err := d.Set("slug", l.Slug); err != nil {
		return err
	}

	if err := d.Set("service_id", l.ServiceId); err != nil {
		return err
	}

	if err := d.Set("specification_id", l.SpecificationId); err != nil {
		return err
	}

	if err := d.Set("desired_specification_id", l.DesiredSpecificationId); err != nil {
		return err
	}

	if err := d.Set("entity_nrn", l.EntityNrn); err != nil {
		return err
	}

	if err := d.Set("linkable_to", l.LinkableTo); err != nil {
		return err
	}

	if err := d.Set("status", l.Status); err != nil {
		return err
	}

	if err := d.Set("archived_at", l.ArchivedAt); err != nil {
		return err
	}

	if err := d.Set("dimensions", l.Dimensions); err != nil {
		return err
	}

	if err := d.Set("selectors", l.Selectors); err != nil {
		return err
	}

	if err := d.Set("attributes", l.Attributes); err != nil {
		return err
	}

	return nil
}

func LinkUpdateContext(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	linkId := d.Id()

	// Same classification the service resource uses: archive and restore are
	// their own verbs, and a managed archive/unarchive action runs
	// asynchronously, so the PATCH only starts the transition.
	transition, fromStatus := statusTransition(d)

	l := &Link{}

	if d.HasChange("name") {
		l.Name = d.Get("name").(string)
	}

	if d.HasChange("slug") {
		l.Slug = d.Get("slug").(string)
	}

	if d.HasChange("service_id") {
		l.ServiceId = d.Get("service_id").(string)
	}

	if d.HasChange("status") {
		l.Status = d.Get("status").(string)
	}

	if d.HasChange("specification_id") {
		l.SpecificationId = d.Get("specification_id").(string)
	}

	if d.HasChange("desired_specification_id") {
		l.DesiredSpecificationId = d.Get("desired_specification_id").(string)
	}

	if d.HasChange("entity_nrn") {
		l.EntityNrn = d.Get("entity_nrn").(string)
	}

	if d.HasChange("linkable_to") {
		l.LinkableTo = d.Get("linkable_to").([]interface{})
	}

	if d.HasChange("dimensions") {
		dimensions := d.Get("dimensions").(map[string]interface{})

		l.Dimensions = dimensions
	}

	if d.HasChange("attributes") {
		attributes := d.Get("attributes").(map[string]interface{})

		l.Attributes = attributes
	}

	if d.HasChange("selectors") {
		selectors := d.Get("selectors").(map[string]interface{})

		l.Selectors = selectors
	}

	// See ServiceUpdateContext: an archive/restore request cannot carry
	// attributes, so they travel in their own PATCH first.
	if transition != "" && l.Attributes != nil {
		attributesOnly := &Link{Attributes: l.Attributes}
		l.Attributes = nil
		if err := nullOps.PatchLink(linkId, attributesOnly); err != nil {
			return diag.FromErr(err)
		}
	}

	if !reflect.DeepEqual(*l, Link{}) {
		err := nullOps.PatchLink(linkId, l)
		if err != nil {
			return diag.FromErr(err)
		}
	}

	if transition != "" {
		link, err := waitForLinkStatusTerminal(ctx, nullOps, linkId, transition, fromStatus, d.Timeout(schema.TimeoutUpdate))
		if err != nil {
			return diag.FromErr(err)
		}
		if link != nil {
			if err := d.Set("status", link.Status); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("archived_at", link.ArchivedAt); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	return nil
}

// waitForLinkStatusTerminal is the link half of waitForServiceStatusTerminal:
// same status machine, same waiter, a different read. A link carries no
// `messages`, so a failed transition reports only the status it landed on.
func waitForLinkStatusTerminal(ctx context.Context, nullOps NullOps, linkID, transition, fromStatus string, timeout time.Duration) (*Link, error) {
	raw, err := waitForInstanceStatusTerminal(ctx, instanceStatusWait{
		entity:     "link",
		id:         linkID,
		transition: transition,
		fromStatus: fromStatus,
		timeout:    timeout,
		read: func() (any, string, []interface{}, []ActionInProgress, error) {
			l, err := nullOps.GetLink(linkID)
			if err != nil {
				return nil, "", nil, nil, err
			}
			return l, l.Status, nil, l.ActionsInProgress, nil
		},
	})
	if err != nil {
		return nil, err
	}
	if l, ok := raw.(*Link); ok {
		return l, nil
	}
	return nil, nil
}

func LinkDeleteContext(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	linkId := d.Id()

	if d.Get("archive_on_destroy").(bool) {
		return archiveLinkOnDestroy(ctx, d, nullOps, linkId)
	}

	err := nullOps.DeleteLink(linkId)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return nil
}

// archiveLinkOnDestroy implements `archive_on_destroy`: destroy archives the
// link instead of deleting it, then drops it from state. Twin of
// archiveServiceOnDestroy, and the reason it exists — a service cannot be
// archived while any of its links is not archived, and hard-deleting the links
// of a service being archived leaves a restorable service with nothing attached.
func archiveLinkOnDestroy(ctx context.Context, d *schema.ResourceData, nullOps NullOps, linkID string) diag.Diagnostics {
	current, err := nullOps.GetLink(linkID)
	if err != nil {
		return diag.FromErr(fmt.Errorf("reading link %s before archiving it on destroy: %w", linkID, err))
	}
	if current == nil {
		d.SetId("")
		return nil
	}

	switch current.Status {
	case "archived":
		// Already archived out of band, or by a destroy that died after the
		// transition landed. Re-PATCHing `archived` is refused, so there is
		// nothing left to do.
		log.Printf("[INFO] link %s is already archived; dropping it from state without a status change", linkID)
		d.SetId("")
		return nil
	case "archiving":
		// An archive is already running — join it rather than starting a second
		// one, which the mint would refuse.
		log.Printf("[INFO] link %s is already archiving; waiting for it to land instead of re-issuing the archive", linkID)
	default:
		if err := nullOps.PatchLink(linkID, &Link{Status: "archived"}); err != nil {
			return diag.FromErr(err)
		}
	}

	if _, err := waitForLinkStatusTerminal(ctx, nullOps, linkID, "archive", current.Status, d.Timeout(schema.TimeoutDelete)); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")
	return nil
}
