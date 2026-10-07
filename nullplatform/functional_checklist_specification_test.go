package nullplatform

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const (
	specAddress = "nullplatform_checklist_specification.spec"
	specHCLV1   = `jsonencode({ items = [{ id = "manual_review", type = "manual", title = "Manual review" }], execution_trigger = "explicit" })`
	specHCLV2   = `jsonencode({ items = [{ id = "manual_review", type = "manual", title = "Manual review" }, { id = "security_review", type = "manual", title = "Security review" }], execution_trigger = "explicit" })`
	// specJSONV1 and specJSONV2 are what jsonencode makes of them.
	specJSONV1 = `{"execution_trigger":"explicit","items":[{"id":"manual_review","title":"Manual review","type":"manual"}]}`
	specJSONV2 = `{"execution_trigger":"explicit","items":[{"id":"manual_review","title":"Manual review","type":"manual"},{"id":"security_review","title":"Security review","type":"manual"}]}`
)

func specConfig(name, attributes string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_checklist_specification" "spec" {
  nrn  = %q
  name = %q
%s
}
`, approvalNrn, name, attributes)
}

func specVersionID(n int) string { return fmt.Sprintf("spec_%016d", n) }

// wrapped matches text as Terraform prints an error: wrapped at any space.
func wrapped(text string) *regexp.Regexp {
	return regexp.MustCompile(strings.ReplaceAll(regexp.QuoteMeta(text), " ", `\s+`))
}

var emptyPlan = resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}}

// newVersionPlanned is the plan of an edit: an update whose current version is
// unknown until the apply creates it.
var newVersionPlanned = resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
	plancheck.ExpectResourceAction(specAddress, plancheck.ResourceActionUpdate),
	plancheck.ExpectUnknownValue(specAddress, tfjsonpath.New("current_version_id")),
	plancheck.ExpectUnknownValue(specAddress, tfjsonpath.New("version")),
}}

func checkCurrentVersion(n int) resource.TestCheckFunc {
	return resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(specAddress, "id", approvalNrn+"/spec"),
		resource.TestCheckResourceAttr(specAddress, "current_version_id", specVersionID(n)),
		resource.TestCheckResourceAttr(specAddress, "version", strconv.Itoa(n)),
	)
}

// specStatuses is every stored version's status, by id.
func specStatuses(fake *fakeplatform.Server) map[string]string {
	statuses := map[string]string{}
	for _, item := range fake.Items(fakeplatform.ChecklistSpecification) {
		statuses[fakeplatform.Str(item, "id")] = fakeplatform.Str(item, "status")
	}
	return statuses
}

func requestsOf(fake *fakeplatform.Server, method string) []fakeplatform.Request {
	var out []fakeplatform.Request
	for _, r := range fake.Requests() {
		if r.Method == method {
			out = append(out, r)
		}
	}
	return out
}

// linkActions stores live approval actions on a version, ids from first on.
func linkActions(fake *fakeplatform.Server, versionID string, first, count int) {
	for id := first; id < first+count; id++ {
		fake.Seed(fakeplatform.ApprovalAction, strconv.Itoa(id), fakeplatform.Item{
			"nrn": approvalNrn, "status": "active", "checklist_specification_id": versionID,
		})
	}
}

// Create, a new version for a definition change and another for a description
// change — each sending only description and definition —, a clean plan, and a
// destroy that deletes every version, newest first.
func TestFunctionalChecklistSpecification_Lifecycle(t *testing.T) {
	fake, _ := newChecklistFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		CheckDestroy: func(*terraform.State) error {
			var deleted []string
			for _, r := range requestsOf(fake, http.MethodDelete) {
				deleted = append(deleted, strings.TrimPrefix(r.Path, CHECKLIST_SPECIFICATION_PATH+"/"))
			}
			want := []string{specVersionID(3), specVersionID(2), specVersionID(1)}
			if fmt.Sprint(deleted) != fmt.Sprint(want) || fmt.Sprint(specStatuses(fake)) != fmt.Sprint(map[string]string{
				specVersionID(1): "deleted", specVersionID(2): "deleted", specVersionID(3): "deleted"}) {
				return fmt.Errorf("deleted %v (statuses %v), want %v, all deleted", deleted, specStatuses(fake), want)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: specConfig("spec", "  definition = "+specHCLV1),
				Check:  checkCurrentVersion(1),
			},
			{
				Config:           specConfig("spec", "  definition = "+specHCLV2),
				ConfigPlanChecks: newVersionPlanned,
				Check: resource.ComposeTestCheckFunc(checkCurrentVersion(2), func(*terraform.State) error {
					patches := requestsOf(fake, http.MethodPatch)
					if len(patches) != 1 || patches[0].Path != CHECKLIST_SPECIFICATION_PATH+"/"+specVersionID(1) ||
						patches[0].Body != `{"description":null,"definition":`+specJSONV2+"}\n" || specStatuses(fake)[specVersionID(1)] != "active" {
						return fmt.Errorf("patches %v, version 1 %s; want one on version 1 with the definition, version 1 kept",
							patches, specStatuses(fake)[specVersionID(1)])
					}
					return nil
				}),
			},
			{
				Config:           specConfig("spec", "  description = \"Two reviews\"\n  definition = "+specHCLV2),
				ConfigPlanChecks: newVersionPlanned,
				Check: resource.ComposeTestCheckFunc(
					checkCurrentVersion(3),
					resource.TestCheckResourceAttr(specAddress, "description", "Two reviews"),
					func(*terraform.State) error {
						if last := requestsOf(fake, http.MethodPatch)[1]; last.Body != `{"description":"Two reviews","definition":`+specJSONV2+"}\n" {
							return fmt.Errorf("patched %s, want the description and the definition, no name", last.Body)
						}
						return nil
					},
				),
			},
			{
				Config:           specConfig("spec", "  description = \"Two reviews\"\n  definition = "+specHCLV2),
				ConfigPlanChecks: emptyPlan,
			},
		},
	})
}

// The API answers the definition in JSONB key order: the same JSON written in
// another order or spacing is not a change, and nothing is sent.
func TestFunctionalChecklistSpecification_EquivalentDefinitionNoDiff(t *testing.T) {
	fake, _ := newChecklistFake(t)
	reordered := `  definition = <<-EOT
    { "items": [ { "type": "manual",  "id": "manual_review", "title": "Manual review" } ],
      "execution_trigger": "explicit" }
  EOT`

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: specConfig("spec", "  definition = "+specHCLV1)},
			{
				Config:           specConfig("spec", reordered),
				ConfigPlanChecks: emptyPlan,
				Check: resource.ComposeTestCheckFunc(checkCurrentVersion(1), func(*terraform.State) error {
					if patches := requestsOf(fake, http.MethodPatch); len(patches) != 0 {
						return fmt.Errorf("sent %v, want no PATCH", patches)
					}
					return nil
				}),
			},
		},
	})
}

// newLineage creates versions 1 and 2 of "spec" through the API, outside
// Terraform.
func newLineage(t *testing.T, client *NullClient) (v1, v2 *ChecklistSpecification) {
	t.Helper()
	v1, err := client.CreateChecklistSpecification(&ChecklistSpecification{Nrn: approvalNrn, Name: "spec", Definition: json.RawMessage(specJSONV1)})
	if err != nil {
		t.Fatal(err)
	}
	if v2, err = client.PatchChecklistSpecification(v1.Id, &ChecklistSpecification{Definition: json.RawMessage(specJSONV2)}); err != nil {
		t.Fatal(err)
	}
	return v1, v2
}

func importStep(id string, check resource.ImportStateCheckFunc, expectError *regexp.Regexp) resource.TestStep {
	return resource.TestStep{
		Config: specConfig("spec", "  definition = "+specHCLV2), ResourceName: specAddress,
		ImportState: true, ImportStateId: id, ImportStatePersist: check != nil, ImportStateCheck: check, ExpectError: expectError,
	}
}

// importedAtVersion2 checks an import landed on the lineage's current version.
func importedAtVersion2(states []*terraform.InstanceState) error {
	if len(states) != 1 || states[0].ID != approvalNrn+"/spec" || states[0].Attributes["current_version_id"] != specVersionID(2) ||
		states[0].Attributes["version"] != "2" {
		return fmt.Errorf("imported %+v, want %s/spec at version 2", states, approvalNrn)
	}
	return nil
}

// An import by the id of an older version lands on the current one, and the
// configuration plans nothing; an id that does not exist is Terraform's
// "Cannot import non-existent remote object".
func TestFunctionalChecklistSpecification_ImportByVersionID(t *testing.T) {
	fake, client := newChecklistFake(t)
	v1, _ := newLineage(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			importStep("spec_missing", nil, regexp.MustCompile(`Cannot import non-existent remote object`)),
			importStep(v1.Id, importedAtVersion2, nil),
			{Config: specConfig("spec", "  definition = "+specHCLV2), ConfigPlanChecks: emptyPlan},
		},
	})
}

// An import by "<nrn>/<name>" finds the lineage at exactly that nrn; a name
// with no live version there, or an id of neither form, is refused.
func TestFunctionalChecklistSpecification_ImportByNrnName(t *testing.T) {
	fake, client := newChecklistFake(t)
	newLineage(t, client)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			importStep(approvalNrn+"/other", nil, wrapped(`cannot import checklist specification "`+approvalNrn+`/other": `+approvalNrn+` has no live version named "other"`)),
			importStep("other", nil, wrapped(`checklist specification import id "other" is neither a version id (spec_…) nor "<nrn>/<name>"`)),
			importStep(approvalNrn+"/spec", importedAtVersion2, nil),
			{Config: specConfig("spec", "  definition = "+specHCLV2), ConfigPlanChecks: emptyPlan},
		},
	})
}

// A version created outside Terraform shows as a definition to take back; the
// apply creates the next version with the configuration's.
func TestFunctionalChecklistSpecification_EditedOutsideTerraform(t *testing.T) {
	fake, client := newChecklistFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: specConfig("spec", "  definition = "+specHCLV1)},
			{
				PreConfig: func() {
					if _, err := client.PatchChecklistSpecification(specVersionID(1), &ChecklistSpecification{Definition: json.RawMessage(specJSONV2)}); err != nil {
						t.Fatal(err)
					}
				},
				Config:           specConfig("spec", "  definition = "+specHCLV1),
				ConfigPlanChecks: newVersionPlanned,
				Check:            resource.ComposeTestCheckFunc(checkCurrentVersion(3), resource.TestCheckResourceAttr(specAddress, "definition", specJSONV1)),
			},
		},
	})
}

// With the current version deleted outside Terraform, the refresh follows the
// newest one left and the apply brings the configuration back as a new one.
func TestFunctionalChecklistSpecification_CurrentVersionDeletedOutside(t *testing.T) {
	fake, client := newChecklistFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		// Version 2, deleted outside, is not deleted again.
		CheckDestroy: func(*terraform.State) error {
			var deleted []string
			for _, r := range requestsOf(fake, http.MethodDelete) {
				deleted = append(deleted, strings.TrimPrefix(r.Path, CHECKLIST_SPECIFICATION_PATH+"/"))
			}
			if want := []string{specVersionID(2), specVersionID(3), specVersionID(1)}; fmt.Sprint(deleted) != fmt.Sprint(want) {
				return fmt.Errorf("deleted %v, want %v: the outside delete, then 3 and 1", deleted, want)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: specConfig("spec", "  definition = "+specHCLV1)},
			{Config: specConfig("spec", "  definition = "+specHCLV2), Check: checkCurrentVersion(2)},
			{
				PreConfig: func() {
					if err := client.DeleteChecklistSpecification(specVersionID(2)); err != nil {
						t.Fatal(err)
					}
				},
				Config:           specConfig("spec", "  definition = "+specHCLV2),
				ConfigPlanChecks: newVersionPlanned,
				Check:            checkCurrentVersion(3),
			},
		},
	})
}

// A lineage deleted whole outside Terraform leaves the state and the plan
// offers to create it, which the API refuses: a deleted name is never created
// again.
func TestFunctionalChecklistSpecification_LineageDeleted(t *testing.T) {
	fake, client := newChecklistFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: specConfig("spec", "  definition = "+specHCLV1)},
			{
				PreConfig: func() {
					if err := client.DeleteChecklistSpecification(specVersionID(1)); err != nil {
						t.Fatal(err)
					}
				},
				Config: specConfig("spec", "  definition = "+specHCLV1),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(specAddress, plancheck.ResourceActionCreate),
				}},
				ExpectError: wrapped("got 400: A checklist specification with the same nrn/name/version already exists"),
			},
		},
	})
}

// A destroy, or a rename without create_before_destroy, while approval actions
// are linked to any version is refused before any DELETE, naming the
// actions — 50 per version and the rest counted, as the API lists them.
func TestFunctionalChecklistSpecification_DestroyInUseRefused(t *testing.T) {
	fake, _ := newChecklistFake(t)
	ids := []string{"1"}
	for id := 2; id <= 51; id++ {
		ids = append(ids, strconv.Itoa(id))
	}
	inUse := func(name string) *regexp.Regexp {
		return wrapped(`cannot delete checklist specification "` + approvalNrn + `/` + name + `": Checklist specification is in use by ` +
			`one or more approval actions (approval actions ` + strings.Join(ids, ", ") + ` and 1 more). Unlink them first; to rename ` +
			`a linked specification, add lifecycle { create_before_destroy = true }`)
	}
	nothingDeleted := func() {
		if deletes := requestsOf(fake, http.MethodDelete); len(deletes) != 0 {
			t.Errorf("sent %v, want no DELETE", deletes)
		}
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: specConfig("spec", "  definition = "+specHCLV1)},
			{Config: specConfig("spec", "  definition = "+specHCLV2)},
			{
				// Action 1 on version 1; 52 to 2 on version 2, whose read lists 50.
				PreConfig: func() {
					linkActions(fake, specVersionID(1), 1, 1)
					linkActions(fake, specVersionID(2), 2, 51)
				},
				Config:      specConfig("spec", "  definition = "+specHCLV2),
				Destroy:     true,
				ExpectError: inUse("spec"),
			},
			{
				PreConfig:   nothingDeleted,
				Config:      specConfig("renamed", "  definition = "+specHCLV2),
				ExpectError: inUse("spec"),
			},
			{
				PreConfig: func() {
					nothingDeleted()
					linkActions(fake, "", 1, 52)
				},
				Config:           specConfig("spec", "  definition = "+specHCLV2),
				ConfigPlanChecks: emptyPlan,
				Check: func(*terraform.State) error {
					if len(specStatuses(fake)) != 2 {
						return fmt.Errorf("versions %v, want spec's two and no renamed one", specStatuses(fake))
					}
					return nil
				},
			},
		},
	})
}

// A rename with create_before_destroy creates the new lineage, re-links the
// action to it and only then deletes every version of the old one.
func TestFunctionalChecklistSpecification_RenameCreateBeforeDestroy(t *testing.T) {
	fake, _, client := newLinkFake(t)
	spec := func(name string) string {
		return fmt.Sprintf("  name       = %q\n  definition = %s\n  lifecycle {\n    create_before_destroy = true\n  }", name, specHCLV1)
	}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: linkedSpecConfig(spec("spec"), "", "")},
			{
				Config: linkedSpecConfig(spec("renamed"), "", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(specAddress, plancheck.ResourceActionCreateBeforeDestroy),
					plancheck.ExpectResourceAction(linkAddress, plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(specAddress, "id", approvalNrn+"/renamed"),
					resource.TestCheckResourceAttr(linkAddress, "checklist_specification_id", specVersionID(2)),
					func(*terraform.State) error {
						action, err := client.GetApprovalAction("1")
						if err != nil || action.ChecklistSpecificationId != specVersionID(2) ||
							fmt.Sprint(specStatuses(fake)) != fmt.Sprint(map[string]string{specVersionID(1): "deleted", specVersionID(2): "active"}) {
							return fmt.Errorf("action %+v (%v), versions %v; want the action on renamed's, spec's deleted", action, err, specStatuses(fake))
						}
						return nil
					},
				),
			},
		},
	})
}

// A specification, an action that declares no on_policy_* and its link
// are created in one apply; what the API sets is no diff.
func TestFunctionalChecklistSpecification_CreateWithLinkedAction(t *testing.T) {
	fake, _, _ := newLinkFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config: linkedConfig(`  on_checklist_fail = "manual"`, ""),
			ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
				plancheck.ExpectResourceAction(specAddress, plancheck.ResourceActionCreate),
				plancheck.ExpectResourceAction(actionAddress, plancheck.ResourceActionCreate),
				plancheck.ExpectResourceAction(linkAddress, plancheck.ResourceActionCreate),
				plancheck.ExpectUnknownValue(actionAddress, tfjsonpath.New("on_policy_success")),
				plancheck.ExpectUnknownValue(actionAddress, tfjsonpath.New("checklist_specification_id")),
			}},
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(actionAddress, "on_policy_success", "manual"),
				resource.TestCheckResourceAttr(actionAddress, "on_policy_fail", "manual"),
				resource.TestCheckResourceAttr(actionAddress, "on_checklist_fail", "manual"),
				resource.TestCheckResourceAttrPair(linkAddress, "checklist_specification_id", specAddress, "current_version_id"),
			),
		}},
	})
}

// relinkTest edits the specification of a linked action: the new version and
// the re-link come in the same apply.
func relinkTest(t *testing.T, edited string) {
	fake, _, client := newLinkFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: linkedConfig("", "")},
			{
				Config: linkedSpecConfig(edited, "", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction(linkAddress, plancheck.ResourceActionUpdate),
					plancheck.ExpectUnknownValue(linkAddress, tfjsonpath.New("checklist_specification_id")),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(linkAddress, "checklist_specification_id", specVersionID(2)),
					func(*terraform.State) error {
						if action, err := client.GetApprovalAction("1"); err != nil || action.ChecklistSpecificationId != specVersionID(2) {
							return fmt.Errorf("action %+v, %v; want it on version 2", action, err)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestFunctionalChecklistSpecification_UpdateRelinks(t *testing.T) {
	relinkTest(t, `  name       = "spec"
  definition = `+specHCLV2)
}

func TestFunctionalChecklistSpecification_DescriptionChangeRelinks(t *testing.T) {
	relinkTest(t, `  name        = "spec"
  description = "Two reviews"
  definition  = `+specHCLV1)
}

// After a destroy, the same name cannot be created again: the apply fails
// with the API's refusal on the first try, without retrying or adopting the
// lineage.
func TestFunctionalChecklistSpecification_DeletedNameNotRecreated(t *testing.T) {
	fake, _ := newChecklistFake(t)
	config := specConfig("spec", "  definition = "+specHCLV1)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		CheckDestroy: func(*terraform.State) error {
			if posts := requestsOf(fake, http.MethodPost); len(posts) != 2 {
				return fmt.Errorf("sent %d creates, want 2: the first and the refused one", len(posts))
			}
			return nil
		},
		Steps: []resource.TestStep{
			{Config: config},
			{Config: config, Destroy: true},
			{Config: config, ExpectError: wrapped("got 400: A checklist specification with the same nrn/name/version already exists")},
		},
	})
}

// specReadFailsOnce fails the first read of a specification, as a transient
// error right after its create would.
type specReadFailsOnce struct {
	NullOps
	failed atomic.Bool
}

func (o *specReadFailsOnce) GetChecklistSpecification(id string) (*ChecklistSpecification, error) {
	if o.failed.CompareAndSwap(false, true) {
		return nil, errors.New("connection reset by peer")
	}
	return o.NullOps.GetChecklistSpecification(id)
}

// A read that fails right after the create leaves the specification in the
// state, whole: the apply succeeds, and the next plan neither replaces it nor
// finds a diff. Replaced, its only version would be deleted, and its name
// could never be created again.
func TestFunctionalChecklistSpecification_CreateReadFailsOnce(t *testing.T) {
	fake, client := newChecklistFake(t)
	ops := &specReadFailsOnce{NullOps: client}
	config := specConfig("spec", "  definition = "+specHCLV1)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactoriesWith(ops),
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(checkCurrentVersion(1), resource.TestCheckResourceAttr(specAddress, "definition", specJSONV1),
					func(*terraform.State) error {
						if !ops.failed.Load() {
							return fmt.Errorf("the read after the create never failed")
						}
						return nil
					}),
			},
			{
				Config:           config,
				ConfigPlanChecks: emptyPlan,
				Check: func(*terraform.State) error {
					if posts, deletes := len(requestsOf(fake, http.MethodPost)), len(requestsOf(fake, http.MethodDelete)); posts != 1 || deletes != 0 {
						return fmt.Errorf("sent %d creates and %d deletes, want one create and no delete", posts, deletes)
					}
					return nil
				},
			},
		},
	})
}

// A definition the API rejects fails the apply with its validation errors,
// creates no version, and a failed edit leaves the state on the version
// the API still has.
func TestFunctionalChecklistSpecification_InvalidDefinition(t *testing.T) {
	fake, _ := newChecklistFake(t)
	invalid := `  definition = jsonencode({ items = [] })`
	a1 := wrapped(`got 422: Checklist specification validation failed: [{"rule":"items.required","path":"definition.items","message":"items must be a non-empty array"}]`)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: specConfig("spec", invalid), ExpectError: a1},
			{Config: specConfig("spec", "  definition = "+specHCLV1), Check: checkCurrentVersion(1)},
			{Config: specConfig("spec", invalid), ExpectError: a1},
			{
				// The failed edit left the state as it was, not the rejected
				// definition: the refresh keeps its spelling.
				Config:           specConfig("spec", "  definition = "+specHCLV1),
				ConfigPlanChecks: emptyPlan,
				Check:            resource.ComposeTestCheckFunc(checkCurrentVersion(1), resource.TestCheckResourceAttr(specAddress, "definition", specJSONV1)),
			},
		},
	})
}

// Without nrn, the components resolve through the API before anything is
// written; with a token that names no organization, they cannot.
func TestFunctionalChecklistSpecification_UnresolvableNrnComponents(t *testing.T) {
	fake, _ := newChecklistFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config:      strings.Replace(specConfig("spec", "  definition = "+specHCLV1), "nrn  = \""+approvalNrn+"\"", `account = "acme"`, 1),
			ExpectError: regexp.MustCompile(`error constructing NRN`),
		}},
		CheckDestroy: func(*terraform.State) error {
			if posts := requestsOf(fake, http.MethodPost); len(posts) != 0 {
				return fmt.Errorf("sent %v, want nothing written", posts)
			}
			return nil
		},
	})
}

// lostPatch is a version the API created after the plan, whose answer never
// reached the provider: a PATCH cut on its way back.
type lostPatch struct {
	client     *NullClient
	definition string
}

func (p lostPatch) CheckPlan(_ context.Context, _ plancheck.CheckPlanRequest, resp *plancheck.CheckPlanResponse) {
	_, resp.Error = p.client.PatchChecklistSpecification(specVersionID(1), &ChecklistSpecification{Definition: json.RawMessage(p.definition)})
}

// When the lineage's newest version already is the desired one by apply time,
// the update adopts it instead of creating another.
func TestFunctionalChecklistSpecification_UpdateAdoptsTheDesiredVersion(t *testing.T) {
	fake, client := newChecklistFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: specConfig("spec", "  definition = "+specHCLV1)},
			{
				Config:           specConfig("spec", "  definition = "+specHCLV2),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{lostPatch{client, specJSONV2}}},
				Check: resource.ComposeTestCheckFunc(checkCurrentVersion(2), func(*terraform.State) error {
					if patches := requestsOf(fake, http.MethodPatch); len(patches) != 1 {
						return fmt.Errorf("sent %d PATCHes, want only the lost one", len(patches))
					}
					return nil
				}),
			},
		},
	})
}
