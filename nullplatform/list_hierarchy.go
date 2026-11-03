package nullplatform

import "strconv"

// The listings below walk the entity hierarchy below an account. Each
// endpoint filters and orders differently; what each one does was checked
// against the API (2026-11-03), and the request each listing sends follows
// from it:
//
//	endpoint                          filter (required)   order sent        why
//	/application                      namespace_id        sort=id:asc       only this filter returns each row's nrn
//	/scope                            application_id      sort=id:asc       unsorted pages repeat rows and drop others
//	/service                          nrn                 sort=created_at   sort=id is refused (400); created_at is the stablest key accepted
//	/runtime_configuration/dimension  nrn                 sort=id:asc
//	/provider                         nrn                 none              the API ignores sort
//	/parameter                        nrn                 none              the API ignores sort
//
// Offset paging is only whole when the order is stable, so where the API
// honours sort=id:asc the listing asks for it (see ListNamespaces).

// ListApplications lists every application of a namespace.
func (c *NullClient) ListApplications(namespaceID string) ([]*Application, error) {
	return listAllPages(c, APPLICATION_PATH,
		map[string]string{"namespace_id": namespaceID, "sort": "id:asc"}, "applications",
		func(a *Application) string { return strconv.Itoa(a.Id) })
}

// ScopeListItem is a scope as the scope listing reads it: only the fields
// the listing exposes. Scope rows carry capabilities whose shape varies
// between scopes (capabilities.visibility is a string on some, an object on
// others), which the full Scope type cannot decode; reading only these
// fields keeps one scope's capabilities from failing the whole listing.
type ScopeListItem struct {
	Id            int               `json:"id"`
	Name          string            `json:"name"`
	Slug          string            `json:"slug"`
	Status        string            `json:"status"`
	Type          string            `json:"type"`
	ApplicationId int               `json:"application_id"`
	Nrn           string            `json:"nrn"`
	Domain        string            `json:"domain"`
	Dimensions    map[string]string `json:"dimensions"`
}

// ListScopes lists every scope of an application. The API lists deleted
// scopes too; status, when set, is passed to the API as its filter.
func (c *NullClient) ListScopes(applicationID, status string) ([]*ScopeListItem, error) {
	params := map[string]string{"application_id": applicationID, "sort": "id:asc"}
	if status != "" {
		params["status"] = status
	}
	return listAllPages(c, SCOPE_PATH, params, "scopes",
		func(s *ScopeListItem) string { return strconv.Itoa(s.Id) })
}

// ListServices lists the services visible at an NRN: those attached to it
// and to its ancestors, never to its descendants. GET /service refuses
// sort=id (400 "querystring/sort must match pattern
// ^(created_at|updated_at|name|status|type):(asc|desc)$"), so the walk is
// ordered by creation time, the stablest key it accepts.
func (c *NullClient) ListServices(nrn string) ([]*Service, error) {
	return listAllPages(c, SERVICE_PATH,
		map[string]string{"nrn": nrn, "sort": "created_at:asc"}, "services",
		func(s *Service) string { return s.Id })
}

// ListDimensions lists the dimensions visible at an NRN: those defined at it
// and at its ancestors.
func (c *NullClient) ListDimensions(nrn string) ([]*Dimension, error) {
	return listAllPages(c, DIMENSION_PATH,
		map[string]string{"nrn": nrn, "sort": "id:asc"}, "dimensions",
		func(d *Dimension) string { return strconv.Itoa(d.ID) })
}

// ListProviderConfigs lists the provider configurations defined exactly at
// an NRN — not its ancestors' nor its descendants'. The API ignores sort, so
// none is sent.
func (c *NullClient) ListProviderConfigs(nrn string) ([]*ProviderConfig, error) {
	return listAllPages(c, PROVIDER_CONFIG_PATH,
		map[string]string{"nrn": nrn}, "provider configurations",
		func(p *ProviderConfig) string { return p.Id })
}

// ListParameters lists the parameters visible at an NRN: those defined at it
// and at its ancestors. It always asks the API to hide values
// (hide_values=true): the listing describes parameters, and secret values
// never leave the platform for it. The API ignores sort, so none is sent.
func (c *NullClient) ListParameters(nrn string) ([]*Parameter, error) {
	return listAllPages(c, PARAMETER_PATH,
		map[string]string{"nrn": nrn, "hide_values": "true"}, "parameters",
		func(p *Parameter) string { return strconv.Itoa(p.Id) })
}
