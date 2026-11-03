package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const (
	testAppNRN     = "organization=1:account=2:namespace=3:application=4"
	testAccountNRN = "organization=1:account=2"
)

// stableQueryServer pages a listing in a stable order whatever sort it is
// sent, like /provider and /parameter (which ignore sort), and records every
// request's query. guard, when set, may answer a request itself first.
func stableQueryServer(t *testing.T, path string, items []any, requests *[]url.Values, guard func(w http.ResponseWriter, q url.Values) bool) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		*requests = append(*requests, q)
		if guard != nil && guard(w, q) {
			return
		}
		limit, _ := strconv.Atoi(q.Get("limit"))
		offset, _ := strconv.Atoi(q.Get("offset"))
		page := []any{}
		for i := offset; i < offset+limit && i < len(items); i++ {
			page = append(page, items[i])
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{"paging": map[string]any{"limit": limit, "offset": offset}, "results": page})
	}))
}

// refuseSortByID is GET /service's real answer to any sort on id.
func refuseSortByID(w http.ResponseWriter, q url.Values) bool {
	if !strings.HasPrefix(q.Get("sort"), "id:") {
		return false
	}
	w.WriteHeader(http.StatusBadRequest)
	_, _ = w.Write([]byte(`{"statusCode":400,"code":"FST_ERR_VALIDATION","error":"Bad Request","message":"querystring/sort must match pattern \"^(created_at|updated_at|name|status|type):(asc|desc)$\""}`))
	return true
}

func assertEveryQuery(t *testing.T, requests []url.Values, want map[string]string, absent ...string) {
	t.Helper()
	if len(requests) < 2 {
		t.Fatalf("got %d requests, want the listing walked across pages", len(requests))
	}
	for i, q := range requests {
		for key, value := range want {
			if q.Get(key) != value {
				t.Errorf("request %d: %s = %q, want %q (query %v)", i, key, q.Get(key), value, q)
			}
		}
		for _, key := range absent {
			if q.Has(key) {
				t.Errorf("request %d sends %s=%q, want it absent", i, key, q.Get(key))
			}
		}
		if q.Get("limit") == "" || q.Get("offset") == "" {
			t.Errorf("request %d carries no limit/offset: %v", i, q)
		}
	}
}

func idsOf[T any](items []*T, id func(*T) string) string {
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = id(item)
	}
	return strings.Join(parts, ",")
}

// --- client walks -----------------------------------------------------------

func TestListApplications_WalksEveryPageInIDOrder(t *testing.T) {
	setPlatformArtifactPageSize(t, 2)
	items := []any{}
	for i := 1; i <= 5; i++ {
		items = append(items, Application{Id: i, NamespaceId: 3})
	}
	var requests []url.Values
	server := orderSensitiveListServer(t, "/application", items, &requests)
	defer server.Close()

	apps, err := newTestClient(server).ListApplications("3")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := idsOf(apps, func(a *Application) string { return strconv.Itoa(a.Id) }); got != "1,2,3,4,5" {
		t.Errorf("got applications %s, want 1..5 each once in id order", got)
	}
	assertEveryQuery(t, requests, map[string]string{"namespace_id": "3", "sort": "id:asc"})
}

// The API lists deleted scopes, and unsorted its pages overlap: on a real
// 37-scope application an unsorted walk returned 35 distinct scopes.
func TestListScopes_WalksEveryPageInIDOrder(t *testing.T) {
	setPlatformArtifactPageSize(t, 2)
	items := []any{}
	for i := 1; i <= 5; i++ {
		items = append(items, ScopeListItem{Id: i, ApplicationId: 4})
	}
	var requests []url.Values
	server := orderSensitiveListServer(t, "/scope", items, &requests)
	defer server.Close()

	scopes, err := newTestClient(server).ListScopes("4", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := idsOf(scopes, func(s *ScopeListItem) string { return strconv.Itoa(s.Id) }); got != "1,2,3,4,5" {
		t.Errorf("got scopes %s, want 1..5 each once in id order", got)
	}
	assertEveryQuery(t, requests, map[string]string{"application_id": "4", "sort": "id:asc"}, "status")
}

func TestListScopes_PassesStatusToTheAPI(t *testing.T) {
	setPlatformArtifactPageSize(t, 1)
	var requests []url.Values
	server := orderSensitiveListServer(t, "/scope", []any{ScopeListItem{Id: 1}, ScopeListItem{Id: 2}}, &requests)
	defer server.Close()

	if _, err := newTestClient(server).ListScopes("4", "active"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertEveryQuery(t, requests, map[string]string{"application_id": "4", "status": "active", "sort": "id:asc"})
}

// GET /service refuses sort=id; the walk orders by creation time instead.
func TestListServices_OrdersByCreationTimeTheAPIAccepts(t *testing.T) {
	setPlatformArtifactPageSize(t, 2)
	items := []any{}
	for i := 1; i <= 5; i++ {
		items = append(items, Service{Id: fmt.Sprintf("svc-%d", i), EntityNrn: testAppNRN})
	}
	var requests []url.Values
	server := stableQueryServer(t, "/service", items, &requests, refuseSortByID)
	defer server.Close()

	services, err := newTestClient(server).ListServices(testAppNRN)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := idsOf(services, func(s *Service) string { return s.Id }); got != "svc-1,svc-2,svc-3,svc-4,svc-5" {
		t.Errorf("got services %s, want svc-1..svc-5", got)
	}
	assertEveryQuery(t, requests, map[string]string{"nrn": testAppNRN, "sort": "created_at:asc"})
}

func TestListDimensions_WalksEveryPageInIDOrder(t *testing.T) {
	setPlatformArtifactPageSize(t, 2)
	items := []any{}
	for i := 1; i <= 5; i++ {
		items = append(items, Dimension{ID: i, NRN: testAccountNRN})
	}
	var requests []url.Values
	server := orderSensitiveListServer(t, "/runtime_configuration/dimension", items, &requests)
	defer server.Close()

	dimensions, err := newTestClient(server).ListDimensions(testAppNRN)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := idsOf(dimensions, func(d *Dimension) string { return strconv.Itoa(d.ID) }); got != "1,2,3,4,5" {
		t.Errorf("got dimensions %s, want 1..5 each once in id order", got)
	}
	assertEveryQuery(t, requests, map[string]string{"nrn": testAppNRN, "sort": "id:asc"})
}

// /provider ignores sort, so the listing sends none.
func TestListProviderConfigs_WalksEveryPageWithoutSort(t *testing.T) {
	setPlatformArtifactPageSize(t, 2)
	items := []any{}
	for i := 1; i <= 5; i++ {
		items = append(items, ProviderConfig{Id: fmt.Sprintf("pc-%d", i), Nrn: testAccountNRN})
	}
	var requests []url.Values
	server := stableQueryServer(t, "/provider", items, &requests, nil)
	defer server.Close()

	configs, err := newTestClient(server).ListProviderConfigs(testAccountNRN)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := idsOf(configs, func(p *ProviderConfig) string { return p.Id }); got != "pc-1,pc-2,pc-3,pc-4,pc-5" {
		t.Errorf("got provider configs %s, want pc-1..pc-5", got)
	}
	assertEveryQuery(t, requests, map[string]string{"nrn": testAccountNRN}, "sort")
}

// The parameter listing never reads values: every page asks the API to hide
// them.
func TestListParameters_AlwaysHidesValuesAndSendsNoSort(t *testing.T) {
	setPlatformArtifactPageSize(t, 2)
	items := []any{}
	for i := 1; i <= 5; i++ {
		items = append(items, Parameter{Id: i, Nrn: testAppNRN})
	}
	var requests []url.Values
	server := stableQueryServer(t, "/parameter", items, &requests, nil)
	defer server.Close()

	parameters, err := newTestClient(server).ListParameters(testAppNRN)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := idsOf(parameters, func(p *Parameter) string { return strconv.Itoa(p.Id) }); got != "1,2,3,4,5" {
		t.Errorf("got parameters %s, want 1..5", got)
	}
	assertEveryQuery(t, requests, map[string]string{"nrn": testAppNRN, "hide_values": "true"}, "sort")
}

// --- data sources -----------------------------------------------------------

// readListDataSource runs a list data source's read against one page of
// items and checks the attributes it sets.
func readListDataSource(t *testing.T, res *schema.Resource, read func(context.Context, *schema.ResourceData, any) diagLike, path string, items []any, config map[string]any, wantID string, want map[string]any) []url.Values {
	t.Helper()
	var requests []url.Values
	server := stableQueryServer(t, path, items, &requests, nil)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, res.Schema, config)
	if diags := read(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if d.Id() != wantID {
		t.Errorf("id = %q, want %q", d.Id(), wantID)
	}
	for key, value := range want {
		if got := d.Get(key); fmt.Sprint(got) != fmt.Sprint(value) {
			t.Errorf("%s = %v, want %v", key, got, value)
		}
	}
	return requests
}

type diagLike interface{ HasError() bool }

func adapt[D diagLike](f func(context.Context, *schema.ResourceData, any) D) func(context.Context, *schema.ResourceData, any) diagLike {
	return func(ctx context.Context, d *schema.ResourceData, m any) diagLike { return f(ctx, d, m) }
}

func TestDataSourceApplicationsRead(t *testing.T) {
	reqs := readListDataSource(t, dataSourceApplications(), adapt(dataSourceApplicationsRead), "/application", []any{
		Application{Id: 10, Name: "API", Slug: "api", Status: "active", NamespaceId: 3, Nrn: testAppNRN, RepositoryUrl: "https://github.com/acme/api",
			RepositoryAppPath: "svc/api", IsMonoRepo: true, TemplateId: 7, AutoDeployOnCreation: true, Tags: map[string]any{"team": "core"}},
		Application{Id: 11, Slug: "bare"},
	}, map[string]any{"namespace_id": 3}, "3", map[string]any{
		"applications.#": 2, "applications.0.id": 10, "applications.0.name": "API", "applications.0.slug": "api",
		"applications.0.status": "active", "applications.0.namespace_id": 3, "applications.0.nrn": testAppNRN,
		"applications.0.repository_url": "https://github.com/acme/api", "applications.0.repository_app_path": "svc/api",
		"applications.0.is_mono_repo": true, "applications.0.template_id": 7, "applications.0.auto_deploy_on_creation": true,
		"applications.0.tags": `{"team":"core"}`, "applications.1.slug": "bare", "applications.1.tags": "",
	})
	if reqs[0].Get("namespace_id") != "3" {
		t.Errorf("listed namespace %q, want 3", reqs[0].Get("namespace_id"))
	}
}

func TestDataSourceScopesRead(t *testing.T) {
	reqs := readListDataSource(t, dataSourceScopes(), adapt(dataSourceScopesRead), "/scope", []any{
		ScopeListItem{Id: 20, Name: "Prod", Slug: "prod", Status: "active", Type: "web_pool_k8s", ApplicationId: 4, Nrn: testAppNRN + ":scope=20",
			Domain: "prod.example.com", Dimensions: map[string]string{"environment": "production"}},
	}, map[string]any{"application_id": 4, "status": "active"}, "4/active", map[string]any{
		"scopes.#": 1, "scopes.0.id": 20, "scopes.0.name": "Prod", "scopes.0.slug": "prod", "scopes.0.status": "active",
		"scopes.0.type": "web_pool_k8s", "scopes.0.application_id": 4, "scopes.0.nrn": testAppNRN + ":scope=20",
		"scopes.0.domain": "prod.example.com", "scopes.0.dimensions.environment": "production",
	})
	if reqs[0].Get("application_id") != "4" || reqs[0].Get("status") != "active" {
		t.Errorf("listed %v, want application_id=4 and status=active", reqs[0])
	}
}

func TestDataSourceScopesRead_WithoutStatusListsEveryStatus(t *testing.T) {
	reqs := readListDataSource(t, dataSourceScopes(), adapt(dataSourceScopesRead), "/scope", []any{
		ScopeListItem{Id: 20, Status: "active"}, ScopeListItem{Id: 21, Status: "deleted"},
	}, map[string]any{"application_id": 4}, "4", map[string]any{"scopes.#": 2, "scopes.1.status": "deleted"})
	if reqs[0].Has("status") {
		t.Errorf("sent status=%q with none configured", reqs[0].Get("status"))
	}
}

func TestDataSourceServicesRead(t *testing.T) {
	readListDataSource(t, dataSourceServices(), adapt(dataSourceServicesRead), "/service", []any{
		Service{Id: "svc-1", Name: "Cache", Slug: "cache", Status: "active", SpecificationId: "spec-1", EntityNrn: testAppNRN,
			Dimensions: map[string]any{"environment": "production"}},
	}, map[string]any{"nrn": testAppNRN}, testAppNRN, map[string]any{
		"services.#": 1, "services.0.id": "svc-1", "services.0.name": "Cache", "services.0.slug": "cache", "services.0.status": "active",
		"services.0.specification_id": "spec-1", "services.0.entity_nrn": testAppNRN, "services.0.dimensions.environment": "production",
	})
}

func TestDataSourceDimensionsRead(t *testing.T) {
	readListDataSource(t, dataSourceDimensions(), adapt(dataSourceDimensionsRead), "/runtime_configuration/dimension", []any{
		Dimension{ID: 30, Name: "Environment", Slug: "environment", Status: "active", NRN: testAccountNRN, Order: 1,
			Values: []DimensionValue{{ID: 31, Name: "Production", Slug: "production", Status: "active", NRN: testAccountNRN}}},
	}, map[string]any{"nrn": testAppNRN}, testAppNRN, map[string]any{
		"dimensions.#": 1, "dimensions.0.id": 30, "dimensions.0.name": "Environment", "dimensions.0.slug": "environment",
		"dimensions.0.status": "active", "dimensions.0.nrn": testAccountNRN, "dimensions.0.order": 1,
		"dimensions.0.values.#": 1, "dimensions.0.values.0.id": 31, "dimensions.0.values.0.name": "Production",
		"dimensions.0.values.0.slug": "production", "dimensions.0.values.0.status": "active", "dimensions.0.values.0.nrn": testAccountNRN,
	})
}

func TestDataSourceProviderConfigsRead(t *testing.T) {
	readListDataSource(t, dataSourceProviderConfigs(), adapt(dataSourceProviderConfigsRead), "/provider", []any{
		ProviderConfig{Id: "pc-1", Nrn: testAccountNRN, SpecificationId: "spec-9", Dimensions: map[string]string{"environment": "production"}},
	}, map[string]any{"nrn": testAccountNRN}, testAccountNRN, map[string]any{
		"provider_configs.#": 1, "provider_configs.0.id": "pc-1", "provider_configs.0.nrn": testAccountNRN,
		"provider_configs.0.specification_id": "spec-9", "provider_configs.0.dimensions.environment": "production",
	})
}

func TestDataSourceParametersRead(t *testing.T) {
	readListDataSource(t, dataSourceParameters(), adapt(dataSourceParametersRead), "/parameter", []any{
		Parameter{Id: 40, Name: "DB password", Nrn: testAppNRN, Type: "environment", Encoding: "plaintext", Variable: "DB_PASSWORD", Secret: true, ReadOnly: true},
	}, map[string]any{"nrn": testAppNRN}, testAppNRN, map[string]any{
		"parameters.#": 1, "parameters.0.id": 40, "parameters.0.name": "DB password", "parameters.0.nrn": testAppNRN,
		"parameters.0.type": "environment", "parameters.0.encoding": "plaintext", "parameters.0.variable": "DB_PASSWORD",
		"parameters.0.secret": true, "parameters.0.read_only": true,
	})
}

// Every list data source surfaces the API's refusal verbatim.
func TestHierarchyListDataSources_SurfaceTheAPIsRefusal(t *testing.T) {
	cases := []struct {
		name   string
		res    *schema.Resource
		read   func(context.Context, *schema.ResourceData, any) diagLike
		config map[string]any
	}{
		{"applications", dataSourceApplications(), adapt(dataSourceApplicationsRead), map[string]any{"namespace_id": 3}},
		{"scopes", dataSourceScopes(), adapt(dataSourceScopesRead), map[string]any{"application_id": 4}},
		{"services", dataSourceServices(), adapt(dataSourceServicesRead), map[string]any{"nrn": testAppNRN}},
		{"dimensions", dataSourceDimensions(), adapt(dataSourceDimensionsRead), map[string]any{"nrn": testAppNRN}},
		{"provider_configs", dataSourceProviderConfigs(), adapt(dataSourceProviderConfigsRead), map[string]any{"nrn": testAccountNRN}},
		{"parameters", dataSourceParameters(), adapt(dataSourceParametersRead), map[string]any{"nrn": testAppNRN}},
	}
	server := forbiddenServer()
	defer server.Close()
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := schema.TestResourceDataRaw(t, tc.res.Schema, tc.config)
			diags := tc.read(context.Background(), d, newTestClient(server))
			if !diags.HasError() || !strings.Contains(fmt.Sprint(diags), "You don't have permission") {
				t.Fatalf("want the API's message, got %v", diags)
			}
		})
	}
}

type listsApplications struct {
	NullOps
	apps []*Application
}

func (l listsApplications) ListApplications(string) ([]*Application, error) { return l.apps, nil }

func TestDataSourceApplicationsRead_UnserializableTagsIsAnError(t *testing.T) {
	d := schema.TestResourceDataRaw(t, dataSourceApplications().Schema, map[string]any{"namespace_id": 3})
	ops := listsApplications{apps: []*Application{{Id: 12, Tags: map[string]any{"bad": make(chan int)}}}}

	diags := dataSourceApplicationsRead(context.Background(), d, ops)
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "error serializing tags for application 12") {
		t.Fatalf("want a tags serialization error, got %v", diags)
	}
}

// Real scope rows carry capabilities whose shape varies between scopes — on
// one application, 24 of 37 had capabilities.visibility as a string and 4 as
// an object. The listing reads only the fields it exposes, so no row's
// capabilities can fail the whole listing (the first live apply failed with
// "cannot unmarshal string into ... capabilities.visibility").
func TestListScopes_ToleratesEveryCapabilitiesShape(t *testing.T) {
	var requests []url.Values
	server := stableQueryServer(t, "/scope", []any{
		map[string]any{"id": 1, "slug": "a", "status": "active", "capabilities": map[string]any{"visibility": "public"}},
		map[string]any{"id": 2, "slug": "b", "status": "deleted", "capabilities": map[string]any{"visibility": map[string]any{"reachability": "account"}}},
		map[string]any{"id": 3, "slug": "c", "status": "deleted"},
	}, &requests, nil)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourceScopes().Schema, map[string]any{"application_id": 4})
	if diags := dataSourceScopesRead(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if got := d.Get("scopes.#"); got != 3 {
		t.Errorf("scopes.# = %v, want 3", got)
	}
}
