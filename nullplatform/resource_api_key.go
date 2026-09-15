package nullplatform

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceApiKey() *schema.Resource {
	return &schema.Resource{
		Description: "The API key resource allows you to configure an API key for the nullplatform API.",

		CreateContext: CreateApiKey,
		ReadContext:   ReadApiKey,
		UpdateContext: UpdateApiKey,
		DeleteContext: DeleteApiKey,

		// The four grant shapes are mutually exclusive, but ConflictsWith
		// cannot reach inside a set's element schema, so the rule is enforced
		// here — at plan time, rather than as an API error during apply.
		CustomizeDiff: validateApiKeyGrantsDiff,

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
				Description: "The name of the API key.",
			},
			"api_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The API key value (only available after creation).",
			},
			"masked_api_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The masked version of the API key.",
			},
			"owner_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "The ID of the user who owns the API key.",
			},
			"internal": {
				Type:     schema.TypeBool,
				Optional: true,
				Computed: true,
				ForceNew: true,
				Description: "Marks the API key as internal to nullplatform, which hides it from API key listings (`GET /api_key`) while it stays readable by ID. Meant for the keys that are platform plumbing — agents, notification channels — rather than keys a person uses. " +
					"The API accepts it only on creation and never returns it, so the value cannot be read back: changing it replaces the API key (and its secret), and a key adopted with `terraform import` comes in as unmarked regardless of its real value. Left to the API default (false) when not set",
			},
			"grants": {
				Type:     schema.TypeSet,
				Required: true,
				Description: "Where the API key may act and what it may do there. Each grant carries an " +
					"`nrn` plus exactly one shape: an existing role (`role_id` or `role_slug`), a literal " +
					"list of `actions`, or a set of roles to `inherits` and adjust. Repeat the block to " +
					"grant several roles on the same NRN.",
				// The default hash covers every attribute, computed ones
				// included, so a refresh would report a change on each of them.
				Set: apiKeyGrantHash,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"nrn": {
							Type:         schema.TypeString,
							Required:     true,
							Description:  "The NRN for the grant.",
							ValidateFunc: validation.StringIsNotEmpty,
						},
						"role_id": {
							Type:     schema.TypeInt,
							Optional: true,
							Computed: true,
							Description: "The ID of an existing role to grant at this NRN. Conflicts with " +
								"`role_slug`, `actions` and `inherits`. Do not use `role_id` in one grant and " +
								"`role_slug` in another: the API resolves every grant of a key the same way.",
						},
						"role_slug": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
							Description: "The slug of an existing role to grant at this NRN. Conflicts with " +
								"`role_id`, `actions` and `inherits`.",
						},
						"actions": {
							Type:     schema.TypeSet,
							Optional: true,
							Description: "The actions the key may call at this NRN, without naming an existing " +
								"role. Conflicts with `role_id`, `role_slug` and `inherits`. You can only grant " +
								"actions you hold at that NRN yourself.",
							Elem: &schema.Schema{Type: schema.TypeString},
						},
						"inherits": {
							Type:     schema.TypeSet,
							Optional: true,
							Description: "Slugs of existing roles, merged into a single role private to this " +
								"key and then refined by `add_actions` and `remove_actions`. Conflicts with " +
								"`role_id`, `role_slug` and `actions`. A role private to another API key " +
								"cannot be inherited.",
							Elem: &schema.Schema{Type: schema.TypeString},
						},
						"add_actions": {
							Type:        schema.TypeSet,
							Optional:    true,
							Description: "Actions to add on top of what `inherits` provides. Requires `inherits`.",
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
						"remove_actions": {
							Type:        schema.TypeSet,
							Optional:    true,
							Description: "Inherited actions to exclude. Requires `inherits`.",
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
						"effective_actions": {
							Type:     schema.TypeSet,
							Computed: true,
							Description: "The action names this grant resolves to, as reported by the API. " +
								"Set only for the `actions` and `inherits` shapes. An action that has an alias " +
								"is reported under the alias.",
							Elem: &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
			"tags": {
				Type:        schema.TypeSet,
				Optional:    true,
				Description: "List of tags of the API key.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"key": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The key of the tag.",
						},
						"value": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "The value of the tag.",
						},
					},
				},
			},
			"last_used_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp of the last usage of the API key.",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when the API key was created.",
			},
			"updated_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when the API key was last updated.",
			},
		},
	}
}

func ReadApiKey(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	apiKeyId, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("failed to parse API key ID: %v", err)
	}

	apiKey, err := nullOps.GetApiKey(apiKeyId)
	if err != nil {
		return diag.FromErr(err)
	}

	// Read the prior grants before overwriting them: they are what says whether
	// this key names its roles by id or by slug.
	useSlug := true
	if prior, ok := d.Get("grants").(*schema.Set); ok {
		useSlug = grantsUseSlug(prior.List())
	}

	rawContent := map[string]any{
		"name":           apiKey.Name,
		"masked_api_key": apiKey.MaskedApiKey,
		"grants":         convertFromGrants(apiKey.Grants, useSlug),
		"owner_id":       apiKey.OwnerID,
		"last_used_at":   apiKey.LastUsedAt,
		"created_at":     apiKey.CreatedAt,
		"updated_at":     apiKey.UpdatedAt,
	}

	if apiKey.Tags != nil {
		rawContent["tags"] = convertFromTags(apiKey.Tags)
	}

	for k, v := range rawContent {
		if err := d.Set(k, v); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func CreateApiKey(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	body := CreateApiKeyRequestBody{
		Name:     d.Get("name").(string),
		Grants:   convertToGrants(d),
		Internal: configuredBool(d, "internal"),
	}

	if tags := convertToTags(d); tags != nil {
		body.Tags = tags
	}

	apiKey, err := nullOps.CreateApiKey(&body)
	if err != nil {
		return diag.FromErr(err)
	}

	apiKeyId := strconv.FormatInt(apiKey.ID, 10)

	d.SetId(apiKeyId)
	d.Set("api_key", apiKey.ApiKeyValue)

	return ReadApiKey(ctx, d, m)
}

func UpdateApiKey(ctx context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	body := PatchApiKeyRequestBody{}

	if d.HasChange("name") {
		body.Name = d.Get("name").(string)
	}

	if d.HasChange("grants") {
		if grants := convertToGrants(d); grants != nil {
			body.Grants = convertToGrants(d)
		}
	}

	if d.HasChange("tags") {
		if tags := convertToTags(d); tags != nil {
			body.Tags = tags
		}
	}

	apiKeyId, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("failed to parse API key ID: %v", err)
	}

	err = nullOps.PatchApiKey(apiKeyId, &body)
	if err != nil {
		return diag.FromErr(err)
	}

	return ReadApiKey(ctx, d, m)
}

func DeleteApiKey(_ context.Context, d *schema.ResourceData, m any) diag.Diagnostics {
	nullOps := m.(NullOps)

	apiKeyId, err := strconv.ParseInt(d.Id(), 10, 64)
	if err != nil {
		return diag.Errorf("failed to parse API key ID: %v", err)
	}

	err = nullOps.DeleteApiKey(apiKeyId)
	if err != nil {
		return diag.FromErr(err)
	}

	d.SetId("")

	return nil
}

func convertFromTags(tags []Tag) []map[string]any {
	if tags == nil {
		return nil
	}

	rawTags := make([]map[string]any, len(tags))

	for i, tag := range tags {
		rawTags[i] = map[string]any{
			"key":   tag.Key,
			"value": tag.Value,
		}
	}

	return rawTags
}

func convertToTags(d *schema.ResourceData) []Tag {
	if tagsSet, ok := d.GetOk("tags"); ok {
		tagsList := tagsSet.(*schema.Set).List()
		tags := make([]Tag, len(tagsList))

		for i, t := range tagsList {
			tagMap := t.(map[string]interface{})

			tags[i] = Tag{
				Key:   tagMap["key"].(string),
				Value: tagMap["value"].(string),
			}
		}

		return tags
	}

	return nil
}

// grantStrings reads one of the element's string sets as a sorted slice. Set
// members are unordered, so sorting is what makes a request — and the hash
// built from it — reproducible.
func grantStrings(value any) []string {
	set, ok := value.(*schema.Set)
	if !ok || set.Len() == 0 {
		return nil
	}

	values := make([]string, 0, set.Len())
	for _, item := range set.List() {
		if s, ok := item.(string); ok && s != "" {
			values = append(values, s)
		}
	}
	sort.Strings(values)

	return values
}

func grantString(value any) string {
	s, _ := value.(string)
	return s
}

// grantInt accepts both widths: config gives int, a value read back from the
// API arrives as int64.
func grantInt(value any) int64 {
	switch typed := value.(type) {
	case int:
		return int64(typed)
	case int64:
		return typed
	default:
		return 0
	}
}

// convertFromGrants turns the API's answer back into state. useSlug says which
// of role_id / role_slug names an existing role here: the API always returns
// both, but state must hold only the one the configuration was written with,
// or the next plan reads the grant as naming a role two ways.
func convertFromGrants(grants []ApiKeyGrantRead, useSlug bool) []map[string]any {
	rawGrants := make([]map[string]any, len(grants))

	for i, grant := range grants {
		rawGrants[i] = convertFromGrant(grant, useSlug)
	}

	return rawGrants
}

func convertFromGrant(grant ApiKeyGrantRead, useSlug bool) map[string]any {
	rawGrant := map[string]any{"nrn": grant.NRN}

	// A key-private role backs both fine-grained shapes and is recreated
	// whenever the grants are replaced, so it never reaches state: the grant
	// reads back as what it actually authorizes.
	if len(grant.Actions) > 0 {
		rawGrant["effective_actions"] = grant.Actions
	}

	switch {
	case len(grant.Inherits) > 0:
		slugs := make([]string, 0, len(grant.Inherits))
		for _, role := range grant.Inherits {
			slugs = append(slugs, role.Slug)
		}
		rawGrant["inherits"] = slugs

		if len(grant.Added) > 0 {
			rawGrant["add_actions"] = grant.Added
		}
		if len(grant.Removed) > 0 {
			rawGrant["remove_actions"] = grant.Removed
		}

	case grant.Actions != nil:
		rawGrant["actions"] = grant.Actions

	case useSlug:
		if grant.RoleSlug != nil {
			rawGrant["role_slug"] = *grant.RoleSlug
		}

	default:
		if grant.RoleID != nil {
			rawGrant["role_id"] = *grant.RoleID
		}
	}

	return rawGrant
}

// grantsUseSlug reports whether this key names its roles by slug. The API
// resolves every grant of a key by whichever of role_id / role_slug the first
// one used, so the choice belongs to the key, not to a single grant. With no
// prior state to read — a fresh import — slug wins: it is the portable one.
func grantsUseSlug(grants []any) bool {
	sawRoleID := false

	for _, g := range grants {
		grantMap, ok := g.(map[string]any)
		if !ok {
			continue
		}
		if grantString(grantMap["role_slug"]) != "" {
			return true
		}
		if grantInt(grantMap["role_id"]) != 0 {
			sawRoleID = true
		}
	}

	return !sawRoleID
}

func convertToGrants(d *schema.ResourceData) []ApiKeyGrant {
	grantsSet := d.Get("grants").(*schema.Set).List()
	grants := make([]ApiKeyGrant, len(grantsSet))

	for i, g := range grantsSet {
		grants[i] = convertToGrant(g.(map[string]interface{}))
	}

	return grants
}

func convertToGrant(grantMap map[string]interface{}) ApiKeyGrant {
	grant := ApiKeyGrant{NRN: grantString(grantMap["nrn"])}

	inherits := grantStrings(grantMap["inherits"])
	actions := grantStrings(grantMap["actions"])

	switch {
	case len(inherits) > 0:
		grant.Inherits = inherits

		// Absent rather than empty: the API treats a missing `actions` as "no
		// delta", and sending empty arrays would differ from what was written.
		delta := map[string][]string{}
		if add := grantStrings(grantMap["add_actions"]); len(add) > 0 {
			delta["add"] = add
		}
		if remove := grantStrings(grantMap["remove_actions"]); len(remove) > 0 {
			delta["remove"] = remove
		}
		if len(delta) > 0 {
			grant.Actions = delta
		}

	case len(actions) > 0:
		grant.Actions = actions

	default:
		// SDKv2 always populates both keys, zero-valued when unset, so only a
		// non-zero value means the configuration actually named a role. Sending
		// the zero value instead is how a role_id-only grant used to go out as
		// {"role_slug": ""}.
		if roleID := grantInt(grantMap["role_id"]); roleID != 0 {
			grant.RoleID = &roleID
		}
		if roleSlug := grantString(grantMap["role_slug"]); roleSlug != "" {
			grant.RoleSlug = &roleSlug
		}
	}

	return grant
}

// apiKeyGrantHash identifies a grant by its NRN and the shape the configuration
// wrote, and by nothing the API adds on read. The default set hash covers every
// attribute, so the key-private role id — which changes each time the grants are
// replaced — would make every refresh look like a change.
func apiKeyGrantHash(value any) int {
	grantMap, ok := value.(map[string]any)
	if !ok {
		return 0
	}

	var identity strings.Builder
	identity.WriteString(grantString(grantMap["nrn"]))

	switch {
	case len(grantStrings(grantMap["inherits"])) > 0:
		fmt.Fprintf(&identity, "|inherits:%v|add:%v|remove:%v",
			grantStrings(grantMap["inherits"]),
			grantStrings(grantMap["add_actions"]),
			grantStrings(grantMap["remove_actions"]))

	case len(grantStrings(grantMap["actions"])) > 0:
		fmt.Fprintf(&identity, "|actions:%v", grantStrings(grantMap["actions"]))

	case grantString(grantMap["role_slug"]) != "":
		fmt.Fprintf(&identity, "|role_slug:%s", grantString(grantMap["role_slug"]))

	default:
		fmt.Fprintf(&identity, "|role_id:%d", grantInt(grantMap["role_id"]))
	}

	return schema.HashString(identity.String())
}

func validateApiKeyGrantsDiff(_ context.Context, d *schema.ResourceDiff, _ any) error {
	grantsSet, ok := d.Get("grants").(*schema.Set)
	if !ok {
		return nil
	}

	grants := make([]map[string]any, 0, grantsSet.Len())
	for _, g := range grantsSet.List() {
		if grantMap, ok := g.(map[string]any); ok {
			grants = append(grants, grantMap)
		}
	}

	return validateGrantShapes(grants)
}

// validateGrantShapes enforces what the API's oneOf enforces, plus the one rule
// the oneOf cannot see: a key names its roles either by id or by slug, never
// both, because verifyExistanceOfRoles resolves every grant of a key by
// whichever key the first one used.
func validateGrantShapes(grants []map[string]any) error {
	usesRoleID, usesRoleSlug := false, false

	for _, grant := range grants {
		nrn := grantString(grant["nrn"])

		hasRole := grantInt(grant["role_id"]) != 0 || grantString(grant["role_slug"]) != ""
		hasActions := len(grantStrings(grant["actions"])) > 0
		hasInherits := len(grantStrings(grant["inherits"])) > 0

		shapes := 0
		for _, present := range []bool{hasRole, hasActions, hasInherits} {
			if present {
				shapes++
			}
		}
		if shapes != 1 {
			return fmt.Errorf(
				"grant %q must set exactly one of an existing role (role_id or role_slug), "+
					"actions, or inherits; it sets %d", nrn, shapes)
		}

		if !hasInherits {
			for _, field := range []string{"add_actions", "remove_actions"} {
				if len(grantStrings(grant[field])) > 0 {
					return fmt.Errorf("grant %q sets %s, which requires inherits", nrn, field)
				}
			}
		}

		if hasRole {
			usesRoleID = usesRoleID || grantInt(grant["role_id"]) != 0
			usesRoleSlug = usesRoleSlug || grantString(grant["role_slug"]) != ""
		}
	}

	// Both on one grant is what the API returns on read, and harmless; both
	// across different grants is the case it cannot resolve.
	if usesRoleID && usesRoleSlug && !everyRoleGrantCarriesBoth(grants) {
		return fmt.Errorf(
			"this API key names roles by role_id in one grant and by role_slug in another; " +
				"use the same one throughout, because the API resolves every grant of a key the same way")
	}

	return nil
}

func everyRoleGrantCarriesBoth(grants []map[string]any) bool {
	for _, grant := range grants {
		hasID := grantInt(grant["role_id"]) != 0
		hasSlug := grantString(grant["role_slug"]) != ""
		if (hasID || hasSlug) && hasID != hasSlug {
			return false
		}
	}
	return true
}
