package nullplatform

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// orderSensitiveListServer answers GET /account and GET /namespace the way
// the real API does (observed 2026-11-03): pages by limit/offset, and only
// in a stable order when asked for sort=id:asc. Unsorted, each page is cut
// from a different ordering of the rows, so consecutive pages repeat some
// rows and never return others — on a real 6-namespace account, offsets
// 0/2/4 at limit 2 returned one namespace three times and two never.
// items must be given in id order.
func orderSensitiveListServer(t *testing.T, path string, items []any, requests *[]url.Values) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != path {
			t.Errorf("unexpected request: %s", r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
			return
		}
		q := r.URL.Query()
		*requests = append(*requests, q)
		limit, offset := 30, 0
		if v := q.Get("limit"); v != "" {
			limit, _ = strconv.Atoi(v)
		}
		if v := q.Get("offset"); v != "" {
			offset, _ = strconv.Atoi(v)
		}
		n := len(items)
		page := []any{}
		for i := offset; i < offset+limit && i < n; i++ {
			if q.Get("sort") == "id:asc" {
				page = append(page, items[i])
			} else {
				page = append(page, items[(offset+i)%n])
			}
		}
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"paging":  map[string]any{"total": n, "limit": limit, "offset": offset},
			"results": page,
		})
	}))
}

func forbiddenServer() *httptest.Server {
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"status":403,"code":"forbidden","message":"You don't have permission to perform this operation on this resource."}`))
	}))
}

func testAccounts(n int) []any {
	items := []any{}
	for i := 1; i <= n; i++ {
		items = append(items, Account{Id: i, Name: "acc", Slug: "acc-" + strconv.Itoa(i), OrganizationId: 42, Status: "active",
			Nrn: "organization=42:account=" + strconv.Itoa(i)})
	}
	return items
}

func testNamespaces(n int) []any {
	items := []any{}
	for i := 1; i <= n; i++ {
		items = append(items, Namespace{Id: i, Name: "ns", Slug: "ns-" + strconv.Itoa(i), AccountId: 7, Status: "active",
			Nrn: "organization=42:account=7:namespace=" + strconv.Itoa(i)})
	}
	return items
}

func assertListQueries(t *testing.T, requests []url.Values, filter, value string) {
	t.Helper()
	if len(requests) < 2 {
		t.Fatalf("got %d requests, want the listing walked across pages", len(requests))
	}
	for i, q := range requests {
		if q.Get(filter) != value || q.Get("sort") != "id:asc" || q.Get("limit") == "" || q.Get("offset") == "" {
			t.Errorf("request %d = %v, want %s=%s, sort=id:asc, limit and offset", i, q, filter, value)
		}
	}
}

// Every account comes back once, in id order, even past the first page: the
// walk asks for sort=id:asc, without which the API's pages overlap.
func TestListAccounts_WalksEveryPageInIDOrder(t *testing.T) {
	setPlatformArtifactPageSize(t, 2)
	var requests []url.Values
	server := orderSensitiveListServer(t, "/account", testAccounts(5), &requests)
	defer server.Close()

	accounts, err := newTestClient(server).ListAccounts("42")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var ids []int
	for _, a := range accounts {
		ids = append(ids, a.Id)
	}
	if fmtInts(ids) != "[1 2 3 4 5]" {
		t.Errorf("got accounts %v, want 1..5 each once in id order", ids)
	}
	assertListQueries(t, requests, "organization_id", "42")
}

// Every namespace of the account comes back once, in id order, past the
// first page.
func TestListNamespaces_WalksEveryPageInIDOrder(t *testing.T) {
	setPlatformArtifactPageSize(t, 2)
	var requests []url.Values
	server := orderSensitiveListServer(t, "/namespace", testNamespaces(5), &requests)
	defer server.Close()

	namespaces, err := newTestClient(server).ListNamespaces("7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var ids []int
	for _, n := range namespaces {
		ids = append(ids, n.Id)
	}
	if fmtInts(ids) != "[1 2 3 4 5]" {
		t.Errorf("got namespaces %v, want 1..5 each once in id order", ids)
	}
	assertListQueries(t, requests, "account_id", "7")
}

func fmtInts(ids []int) string {
	parts := make([]string, len(ids))
	for i, id := range ids {
		parts[i] = strconv.Itoa(id)
	}
	return "[" + strings.Join(parts, " ") + "]"
}

// Without organization_id the data source lists the organization of the
// provider's credentials — GET /account itself refuses an unfiltered listing.
func TestDataSourceAccountsRead_DefaultsToTheTokensOrganization(t *testing.T) {
	var requests []url.Values
	server := orderSensitiveListServer(t, "/account", []any{
		Account{Id: 1, Name: "Main", Slug: "main", OrganizationId: 42, Status: "active", Nrn: "organization=42:account=1",
			RepositoryPrefix: "main", RepositoryProvider: "github", Settings: map[string]any{"tier": "gold"}},
		Account{Id: 2, Name: "Bare", Slug: "bare", OrganizationId: 42, Status: "active", Nrn: "organization=42:account=2"},
	}, &requests)
	defer server.Close()

	c := newTestClient(server)
	c.cachedOrgID = "42"
	d := schema.TestResourceDataRaw(t, dataSourceAccounts().Schema, map[string]any{})

	if diags := dataSourceAccountsRead(context.Background(), d, c); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if requests[0].Get("organization_id") != "42" {
		t.Errorf("listed organization %q, want the token's 42", requests[0].Get("organization_id"))
	}
	if d.Id() != "42" || d.Get("organization_id").(int) != 42 {
		t.Errorf("id = %q, organization_id = %d, want 42 and 42", d.Id(), d.Get("organization_id").(int))
	}
	want := map[string]any{
		"accounts.#": 2, "accounts.0.id": 1, "accounts.0.name": "Main", "accounts.0.slug": "main",
		"accounts.0.organization_id": 42, "accounts.0.status": "active", "accounts.0.nrn": "organization=42:account=1",
		"accounts.0.repository_prefix": "main", "accounts.0.repository_provider": "github",
		"accounts.0.settings": `{"tier":"gold"}`, "accounts.1.slug": "bare", "accounts.1.settings": "",
	}
	for key, value := range want {
		if got := d.Get(key); got != value {
			t.Errorf("%s = %v, want %v", key, got, value)
		}
	}
}

// A configured organization_id wins over the token's organization.
func TestDataSourceAccountsRead_ConfiguredOrganizationWins(t *testing.T) {
	var requests []url.Values
	server := orderSensitiveListServer(t, "/account", []any{}, &requests)
	defer server.Close()

	c := newTestClient(server)
	c.cachedOrgID = "42"
	d := schema.TestResourceDataRaw(t, dataSourceAccounts().Schema, map[string]any{"organization_id": 9})

	if diags := dataSourceAccountsRead(context.Background(), d, c); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if requests[0].Get("organization_id") != "9" || d.Id() != "9" {
		t.Errorf("listed organization %q with id %q, want 9", requests[0].Get("organization_id"), d.Id())
	}
	if got := d.Get("accounts.#"); got != 0 {
		t.Errorf("accounts.# = %v, want 0", got)
	}
}

func TestDataSourceAccountsRead_SurfacesTheAPIsRefusal(t *testing.T) {
	server := forbiddenServer()
	defer server.Close()

	c := newTestClient(server)
	d := schema.TestResourceDataRaw(t, dataSourceAccounts().Schema, map[string]any{"organization_id": 9})

	diags := dataSourceAccountsRead(context.Background(), d, c)
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "You don't have permission") {
		t.Fatalf("want the API's message, got %v", diags)
	}
}

type orgFromToken struct {
	NullOps
	orgID    string
	err      error
	accounts []*Account
}

func (o orgFromToken) GetOrganizationIDFromToken() (string, error) { return o.orgID, o.err }
func (o orgFromToken) ListAccounts(string) ([]*Account, error)     { return o.accounts, nil }

func TestDataSourceAccountsRead_NoOrganizationAnywhereIsAnError(t *testing.T) {
	d := schema.TestResourceDataRaw(t, dataSourceAccounts().Schema, map[string]any{})

	diags := dataSourceAccountsRead(context.Background(), d, orgFromToken{err: errors.New("organization ID not found in token")})
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "organization_id is not set") {
		t.Fatalf("want an error naming organization_id, got %v", diags)
	}
}

func TestDataSourceAccountsRead_NonNumericTokenOrganizationIsAnError(t *testing.T) {
	d := schema.TestResourceDataRaw(t, dataSourceAccounts().Schema, map[string]any{})

	diags := dataSourceAccountsRead(context.Background(), d, orgFromToken{orgID: "abc"})
	if !diags.HasError() || !strings.Contains(diags[0].Summary, `invalid organization ID "abc"`) {
		t.Fatalf("want an invalid organization error, got %v", diags)
	}
}

func TestDataSourceAccountsRead_UnserializableSettingsIsAnError(t *testing.T) {
	d := schema.TestResourceDataRaw(t, dataSourceAccounts().Schema, map[string]any{})
	ops := orgFromToken{orgID: "42", accounts: []*Account{{Id: 3, Settings: map[string]any{"bad": make(chan int)}}}}

	diags := dataSourceAccountsRead(context.Background(), d, ops)
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "error serializing settings for account 3") {
		t.Fatalf("want a settings serialization error, got %v", diags)
	}
}

func TestDataSourceNamespacesRead_ListsTheAccountsNamespaces(t *testing.T) {
	var requests []url.Values
	server := orderSensitiveListServer(t, "/namespace", []any{
		Namespace{Id: 11, Name: "Dev", Slug: "dev", AccountId: 7, Status: "active", Nrn: "organization=42:account=7:namespace=11"},
		Namespace{Id: 12, Name: "Prod", Slug: "prod", AccountId: 7, Status: "active", Nrn: "organization=42:account=7:namespace=12"},
	}, &requests)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourceNamespaces().Schema, map[string]any{"account_id": 7})

	if diags := dataSourceNamespacesRead(context.Background(), d, newTestClient(server)); diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	if requests[0].Get("account_id") != "7" || d.Id() != "7" {
		t.Errorf("listed account %q with id %q, want 7", requests[0].Get("account_id"), d.Id())
	}
	want := map[string]any{
		"namespaces.#": 2, "namespaces.0.id": 11, "namespaces.0.name": "Dev", "namespaces.0.slug": "dev",
		"namespaces.0.account_id": 7, "namespaces.0.status": "active",
		"namespaces.0.nrn": "organization=42:account=7:namespace=11", "namespaces.1.slug": "prod",
	}
	for key, value := range want {
		if got := d.Get(key); got != value {
			t.Errorf("%s = %v, want %v", key, got, value)
		}
	}
}

func TestDataSourceNamespacesRead_SurfacesTheAPIsRefusal(t *testing.T) {
	server := forbiddenServer()
	defer server.Close()

	d := schema.TestResourceDataRaw(t, dataSourceNamespaces().Schema, map[string]any{"account_id": 7})

	diags := dataSourceNamespacesRead(context.Background(), d, newTestClient(server))
	if !diags.HasError() || !strings.Contains(diags[0].Summary, "You don't have permission") {
		t.Fatalf("want the API's message, got %v", diags)
	}
}
