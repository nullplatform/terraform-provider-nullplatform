package nullplatform

import (
	"context"
	"fmt"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/function"
)

// extractIDFromNRN returns the id of the given entity type from an NRN of the
// form "organization=1:account=2:namespace=3:application=4:scope=5".
func extractIDFromNRN(nrn, entityType string) (string, error) {
	entityType = strings.ToLower(strings.TrimSpace(entityType))
	if entityType == "" {
		return "", fmt.Errorf("entity type must not be empty")
	}

	nrn = strings.TrimSpace(nrn)
	if nrn == "" {
		return "", fmt.Errorf("NRN must not be empty")
	}

	var contained []string
	for _, segment := range strings.Split(nrn, ":") {
		key, value, found := strings.Cut(segment, "=")
		if !found || key == "" || value == "" {
			return "", fmt.Errorf("invalid NRN %q: segment %q is not in \"<entity>=<id>\" format", nrn, segment)
		}
		if key == entityType {
			return value, nil
		}
		contained = append(contained, key)
	}

	return "", fmt.Errorf("NRN %q does not contain an id for entity type %q (it contains: %s)", nrn, entityType, strings.Join(contained, ", "))
}

var _ function.Function = (*extractIDFunction)(nil)

// extractIDFunction implements provider::nullplatform::nrn_extract_id, taking
// the entity type as its first argument for cases where the type comes from a
// variable. The nrn_extract_<entity>_id variants cover the common fixed-type
// case.
type extractIDFunction struct{}

func NewExtractIDFunction() function.Function {
	return &extractIDFunction{}
}

func (f *extractIDFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = "nrn_extract_id"
}

func (f *extractIDFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     "Extract an entity id from an NRN",
		Description: "Given an entity type and an NRN (Nullplatform Resource Name), returns the id of the NRN component for that entity type. Returns an error if the NRN does not contain the requested entity type. Valid entity types include `organization`, `account`, `namespace`, `application`, and `scope`.",
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:        "entity_type",
				Description: "The entity type whose id should be extracted (e.g. `account`, `namespace`).",
			},
			function.StringParameter{
				Name:        "nrn",
				Description: "The NRN to extract the id from (e.g. `organization=1:account=2:namespace=3`).",
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *extractIDFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var entityType, nrn string

	resp.Error = function.ConcatFuncErrors(resp.Error, req.Arguments.Get(ctx, &entityType, &nrn))
	if resp.Error != nil {
		return
	}

	id, err := extractIDFromNRN(nrn, entityType)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewArgumentFuncError(1, err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, id))
}

var _ function.Function = (*extractEntityIDFunction)(nil)

// extractEntityIDFunction implements the nrn_extract_<entity>_id family of
// functions; each instance is bound to a single entity type.
type extractEntityIDFunction struct {
	entityType string
}

func NewExtractOrganizationIDFunction() function.Function {
	return &extractEntityIDFunction{entityType: "organization"}
}

func NewExtractAccountIDFunction() function.Function {
	return &extractEntityIDFunction{entityType: "account"}
}

func NewExtractNamespaceIDFunction() function.Function {
	return &extractEntityIDFunction{entityType: "namespace"}
}

func NewExtractApplicationIDFunction() function.Function {
	return &extractEntityIDFunction{entityType: "application"}
}

func NewExtractScopeIDFunction() function.Function {
	return &extractEntityIDFunction{entityType: "scope"}
}

func (f *extractEntityIDFunction) Metadata(_ context.Context, _ function.MetadataRequest, resp *function.MetadataResponse) {
	resp.Name = fmt.Sprintf("nrn_extract_%s_id", f.entityType)
}

func (f *extractEntityIDFunction) Definition(_ context.Context, _ function.DefinitionRequest, resp *function.DefinitionResponse) {
	resp.Definition = function.Definition{
		Summary:     fmt.Sprintf("Extract the %s id from an NRN", f.entityType),
		Description: fmt.Sprintf("Given an NRN (Nullplatform Resource Name), returns the id of its `%s` component. Returns an error if the NRN does not contain a `%s` component.", f.entityType, f.entityType),
		Parameters: []function.Parameter{
			function.StringParameter{
				Name:        "nrn",
				Description: "The NRN to extract the id from (e.g. `organization=1:account=2:namespace=3`).",
			},
		},
		Return: function.StringReturn{},
	}
}

func (f *extractEntityIDFunction) Run(ctx context.Context, req function.RunRequest, resp *function.RunResponse) {
	var nrn string

	resp.Error = function.ConcatFuncErrors(resp.Error, req.Arguments.Get(ctx, &nrn))
	if resp.Error != nil {
		return
	}

	id, err := extractIDFromNRN(nrn, f.entityType)
	if err != nil {
		resp.Error = function.ConcatFuncErrors(resp.Error, function.NewArgumentFuncError(0, err.Error()))
		return
	}

	resp.Error = function.ConcatFuncErrors(resp.Error, resp.Result.Set(ctx, id))
}
