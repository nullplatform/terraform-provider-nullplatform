package nullplatform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// packagePublishFake is a stateful fake of the package publish contract the
// resource drives, copied from main-service-api's PackageService.update:
//
//   - PUT /packages on an existing (nrn, slug) publishes a new revision.
//   - merge_components defaults to TRUE: the body's components are overlaid,
//     by name, on the BOM of the package's default revision (latest when there
//     is no default), so every component the body leaves out carries over.
//   - a resolved BOM equal to the latest revision's is a no-op: no revision is
//     published, whatever version the body names.
//
// It records each PUT body so tests can assert the wire contract too.
type packagePublishFake struct {
	mu        sync.Mutex
	pkg       *Package
	revisions []*packageFakeRevision
	bodies    []map[string]interface{}
}

type packageFakeRevision struct {
	id         string
	version    string
	components []PackageComponent
}

func (f *packagePublishFake) serve(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		switch {
		case r.Method == http.MethodPut && r.URL.Path == PACKAGE_PATH:
			f.publish(t, w, r)
		case r.Method == http.MethodGet && f.pkg != nil && r.URL.Path == PACKAGE_PATH+"/"+f.pkg.ID:
			_ = json.NewEncoder(w).Encode(f.pkg)
		case r.Method == http.MethodGet && f.pkg != nil && r.URL.Path == PACKAGE_PATH+"/"+f.pkg.ID+"/revisions":
			results := []*PackageRevision{}
			for _, revision := range f.revisions {
				results = append(results, &PackageRevision{ID: revision.id, PackageID: f.pkg.ID, Version: revision.version})
			}
			_ = json.NewEncoder(w).Encode(packageRevisionListResponse{Results: results})
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
}

func (f *packagePublishFake) publish(t *testing.T, w http.ResponseWriter, r *http.Request) {
	var raw map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		t.Fatalf("decoding PUT /packages body: %v", err)
	}
	f.bodies = append(f.bodies, raw)
	encoded, _ := json.Marshal(raw)
	var body PackageUpsert
	_ = json.Unmarshal(encoded, &body)

	if f.pkg == nil {
		f.pkg = &Package{ID: "pkg-1", Nrn: body.Nrn, Slug: body.Slug, Name: body.Name}
		revision := f.addRevision(body.Version, body.Components)
		f.pkg.DefaultRevisionID = revision.id
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(f.pkg)
		return
	}

	components := body.Components
	if merge, sent := raw["merge_components"]; !sent || merge != false {
		components = mergeComponentsByName(f.revision(f.resolvedRevisionID()).components, body.Components)
	}
	latest := f.revision(f.pkg.LatestRevisionID)
	if !sameComponentSet(components, latest.components) {
		revision := f.addRevision(body.Version, components)
		if body.Default {
			f.pkg.DefaultRevisionID = revision.id
		}
	}
	_ = json.NewEncoder(w).Encode(f.pkg)
}

// seedPackage arranges a package that already exists on the platform — e.g.
// one created from its specification in the UI — without going through PUT.
func (f *packagePublishFake) seedPackage(nrn, slug, version string, components []PackageComponent) {
	f.pkg = &Package{ID: "pkg-1", Nrn: nrn, Slug: slug, Name: slug}
	f.pkg.DefaultRevisionID = f.addRevision(version, components).id
}

func (f *packagePublishFake) addRevision(version string, components []PackageComponent) *packageFakeRevision {
	revision := &packageFakeRevision{id: fmt.Sprintf("rev-%d", len(f.revisions)+1), version: version, components: components}
	f.revisions = append(f.revisions, revision)
	f.pkg.LatestRevisionID = revision.id
	return revision
}

func (f *packagePublishFake) resolvedRevisionID() string {
	if f.pkg.DefaultRevisionID != "" {
		return f.pkg.DefaultRevisionID
	}
	return f.pkg.LatestRevisionID
}

func (f *packagePublishFake) revision(id string) *packageFakeRevision {
	for _, revision := range f.revisions {
		if revision.id == id {
			return revision
		}
	}
	return &packageFakeRevision{}
}

func (f *packagePublishFake) componentNames(version string) ([]string, bool) {
	for _, revision := range f.revisions {
		if revision.version == version {
			names := []string{}
			for _, component := range revision.components {
				names = append(names, component.Name)
			}
			sort.Strings(names)
			return names, true
		}
	}
	return nil, false
}

func (f *packagePublishFake) versions() []string {
	versions := []string{}
	for _, revision := range f.revisions {
		versions = append(versions, revision.version)
	}
	return versions
}

func mergeComponentsByName(base, overlay []PackageComponent) []PackageComponent {
	merged := []PackageComponent{}
	index := map[string]int{}
	for _, component := range append(append([]PackageComponent{}, base...), overlay...) {
		if at, seen := index[component.Name]; seen {
			merged[at] = component
			continue
		}
		index[component.Name] = len(merged)
		merged = append(merged, component)
	}
	return merged
}

func sameComponentSet(left, right []PackageComponent) bool {
	key := func(components []PackageComponent) string {
		entries := []string{}
		for _, c := range components {
			entries = append(entries, c.Name+"|"+c.ResourceType+"|"+c.ResourceID+"|"+c.ResourceRevisionID)
		}
		sort.Strings(entries)
		return strings.Join(entries, ",")
	}
	return key(left) == key(right)
}

func packageConfig(version string, components ...map[string]interface{}) map[string]interface{} {
	list := []interface{}{}
	for _, component := range components {
		list = append(list, component)
	}
	return map[string]interface{}{
		"nrn":        "organization=1:account=2",
		"slug":       "k8s-containers",
		"name":       "Containers",
		"version":    version,
		"default":    true,
		"components": list,
	}
}

func specComponent(name string) map[string]interface{} {
	return map[string]interface{}{
		"name":                 name,
		"resource_type":        "service_specification",
		"resource_id":          "spec-1",
		"resource_revision_id": "spec-snap-1",
	}
}

func artifactComponent(name, artifactID, revisionID string) map[string]interface{} {
	return map[string]interface{}{
		"name":                 name,
		"resource_type":        "artifact",
		"resource_id":          artifactID,
		"resource_revision_id": revisionID,
	}
}

// Removing a component from `components` and bumping `version` must publish a
// revision without it. Without merge_components=false the API overlays the
// config on the previous BOM, the removed git source comes back, the result
// equals the latest revision, and the bump publishes nothing at all.
func TestPackageUpdate_ComponentRemovedFromConfigIsDroppedFromTheNewRevision(t *testing.T) {
	fake := &packagePublishFake{}
	server := fake.serve(t)
	defer server.Close()
	client := newTestClient(server)

	first := schema.TestResourceDataRaw(t, resourcePackage().Schema, packageConfig("1.0.0",
		specComponent("scope"),
		artifactComponent("source", "git-artifact", "git-rev-1"),
		artifactComponent("worker-image", "oci-artifact", "oci-rev-1"),
	))
	if err := PackageCreate(first, client); err != nil {
		t.Fatalf("create: %v", err)
	}

	second := schema.TestResourceDataRaw(t, resourcePackage().Schema, packageConfig("1.1.0",
		specComponent("scope"),
		artifactComponent("worker-image", "oci-artifact", "oci-rev-1"),
	))
	second.SetId(first.Id())
	if err := PackageUpdate(second, client); err != nil {
		t.Fatalf("update: %v", err)
	}

	names, published := fake.componentNames("1.1.0")
	if !published {
		t.Fatalf("version 1.1.0 was never published; published versions: %v", fake.versions())
	}
	if want := []string{"scope", "worker-image"}; strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("1.1.0 BOM = %v, want exactly %v (the removed git source must not carry over)", names, want)
	}
	for i, body := range fake.bodies {
		if merge, sent := body["merge_components"]; !sent || merge != false {
			t.Errorf("PUT #%d merge_components = %v (sent: %v), want false", i+1, merge, sent)
		}
	}
}

// Adopting a package that already exists — e.g. created from its spec in the
// UI with a git source — publishes exactly the configured BOM: the existing
// revision's spec and source components must not leak into it.
func TestPackageCreate_OverAnExistingPackagePublishesOnlyTheConfiguredBOM(t *testing.T) {
	fake := &packagePublishFake{}
	fake.seedPackage("organization=1:account=2", "k8s-containers", "1.0.0", []PackageComponent{
		{Name: "spec", ResourceType: "service_specification", ResourceID: "spec-1", ResourceRevisionID: "spec-snap-1"},
		{Name: "source", ResourceType: "artifact", ResourceID: "git-artifact", ResourceRevisionID: "git-rev-1"},
	})
	server := fake.serve(t)
	defer server.Close()

	d := schema.TestResourceDataRaw(t, resourcePackage().Schema, packageConfig("1.1.0",
		specComponent("scope"),
		artifactComponent("worker-image", "oci-artifact", "oci-rev-1"),
	))
	if err := PackageCreate(d, newTestClient(server)); err != nil {
		t.Fatalf("create: %v", err)
	}

	names, published := fake.componentNames("1.1.0")
	if !published {
		t.Fatalf("version 1.1.0 was never published; published versions: %v", fake.versions())
	}
	if want := []string{"scope", "worker-image"}; strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("1.1.0 BOM = %v, want exactly %v", names, want)
	}
	if got := d.Get("published_revision_id").(string); got != fake.pkg.LatestRevisionID {
		t.Errorf("published_revision_id = %q, want %q", got, fake.pkg.LatestRevisionID)
	}
}
