package nullplatform

import (
	"context"
	"fmt"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// optionalBoolFromRaw is configuredBool (utils.go) for callers that hold a raw
// cty configuration value instead of *schema.ResourceData — CustomizeDiff is
// one. Same contract: the configured value, or nil when the configuration says
// nothing, because a null is the only way to tell "absent" from "false".
func optionalBoolFromRaw(raw cty.Value, key string) *bool {
	if raw.IsNull() || !raw.IsKnown() {
		return nil
	}
	if !raw.Type().IsObjectType() || !raw.Type().HasAttribute(key) {
		return nil
	}
	value := raw.GetAttr(key)
	if value.IsNull() || !value.IsKnown() {
		return nil
	}
	configured := value.True()
	return &configured
}

// validateManagedRequiresDefaultActions mirrors the API's own guard
// (`use_managed_actions` requires `use_default_actions`) so the rejection
// arrives at plan time instead of as a 400 mid-apply. Like the API's version it
// fires only when BOTH flags are explicitly configured: an absent value means
// the platform default applies, and the two defaults (use_managed_actions
// false, use_default_actions true) cannot combine into a violation.
func validateManagedRequiresDefaultActions(_ context.Context, diff *schema.ResourceDiff, _ any) error {
	raw := diff.GetRawConfig()
	return managedRequiresDefaultActions(
		optionalBoolFromRaw(raw, "use_managed_actions"),
		optionalBoolFromRaw(raw, "use_default_actions"),
	)
}

// managedRequiresDefaultActions holds the rule itself, separately from where the
// values come from. nil means "not configured".
func managedRequiresDefaultActions(managed, defaults *bool) error {
	if managed == nil || defaults == nil {
		return nil
	}
	if *managed && !*defaults {
		return fmt.Errorf("use_managed_actions requires use_default_actions: managed CRUD runs " +
			"through the generated actions, and a specification that authors its own actions has " +
			"none to run. Set use_default_actions to true, or leave use_managed_actions off")
	}
	return nil
}
