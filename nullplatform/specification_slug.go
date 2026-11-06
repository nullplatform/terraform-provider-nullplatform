package nullplatform

import (
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// The slug a specification is created with. The API derives it from the name
// unless the request gives one, and a given one must be exactly what the
// derivation can produce, so nothing downstream tells the two apart. It never
// changes afterwards: the API ignores it on update, so a changed slug here is
// a new specification (ForceNew), never an update the platform would drop.
//
// A publisher gives one to keep a namespaced identifier under a plain name —
// "AWS Lambda" published as `np-aws-lambda` — so a specification published for
// every organization can live next to the copy an organization already runs
// under the derived `aws-lambda`: channels filter by the specification slug
// and the agent resolves the package worker by slug, so equal slugs double-route.
var specificationSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func specificationSlugSchema(what string) *schema.Schema {
	return &schema.Schema{
		Type:     schema.TypeString,
		Optional: true,
		Computed: true,
		ForceNew: true,
		ValidateFunc: validation.StringMatch(specificationSlugPattern,
			"a slug is lowercase letters and digits joined by single dashes, exactly what the platform derives from a name"),
		Description: "Slug of the " + what + ". Omitted, the platform derives it from `name`; given, it is stored " +
			"as sent and must be lowercase letters and digits joined by single dashes (what the derivation " +
			"produces). It never changes afterwards, so changing it replaces the " + what + ". Give one to keep " +
			"a namespaced identifier under a plain name, e.g. \"AWS Lambda\" as `np-aws-lambda`.",
	}
}
