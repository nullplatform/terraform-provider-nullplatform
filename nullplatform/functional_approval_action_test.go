package nullplatform

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/config"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

const approvalNrn = "organization=1:account=2:namespace=3:application=4"

func newApprovalFake(t *testing.T) (*fakeplatform.Server, *fakeplatform.Approvals) {
	t.Helper()
	fake := fakeplatform.New()
	approvals := fakeplatform.RegisterApproval(fake)
	t.Cleanup(fake.Close)
	return fake, approvals
}

// seedAction stores an action as the API answers one created with
// on_policy_success = "approve" and on_policy_fail = "manual".
func seedAction(fake *fakeplatform.Server, id, entity, action string) {
	fake.Seed(fakeplatform.ApprovalAction, id, fakeplatform.Item{
		"nrn": approvalNrn, "entity": entity, "action": action,
		"dimensions": fakeplatform.Item{"environment": "production"}, "status": "active",
		"on_policy_success": "approve", "on_policy_fail": "manual", "on_checklist_fail": nil,
		"checklist_specification_id": nil, "checklist_template_id": nil,
	})
}

func seedPolicy(fake *fakeplatform.Server, id, name string, conditions fakeplatform.Item) {
	fake.Seed(fakeplatform.ApprovalPolicy, id, fakeplatform.Item{
		"nrn": approvalNrn, "name": name, "conditions": conditions, "selector": fakeplatform.Item{},
	})
}

var (
	coverageConditions = fakeplatform.Item{"build.metadata.coverage.code.coverage": fakeplatform.Item{"$gte": 80}}
	securityConditions = fakeplatform.Item{"build.metadata.security.security.vulnerabilities.critical": fakeplatform.Item{"$eq": 0}}
)

// upgradeStep plans a K row's .tf, verbatim, over the state 0.0.105 left after
// applying it (testdata/approval_upgrade, synthetic ids), and expects no
// changes: a local backend reads a copy of that state, so the plan is the one
// an upgrade sees. The config is a directory and the step brings its own
// providers because only then does the harness init with the step's config,
// and init is what reads the backend. The state names the provider
// registry.terraform.io/nullplatform/nullplatform, so the harness serves it
// under that namespace.
func upgradeStep(t *testing.T, fake *fakeplatform.Server, row string) resource.TestStep {
	t.Helper()
	t.Setenv(resource.EnvTfAccProviderNamespace, "nullplatform")
	fixtures := filepath.Join("testdata", "approval_upgrade")
	state, err := os.ReadFile(filepath.Join(fixtures, row+".tfstate.json"))
	if err != nil {
		t.Fatal(err)
	}
	statePath := filepath.Join(t.TempDir(), "terraform.tfstate")
	if err := os.WriteFile(statePath, state, 0o600); err != nil {
		t.Fatal(err)
	}
	tf, err := os.ReadFile(filepath.Join(fixtures, row+".tf"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	harness := fmt.Sprintf(`
terraform {
  required_providers {
    nullplatform = {
      source = "nullplatform/nullplatform"
    }
  }
  backend "local" {
    path = %[1]q
  }
}

provider "nullplatform" {}

variable "nrn" {
  default = %[2]q
}

variable "nrn_namespace" {
  default = %[2]q
}
`, statePath, approvalNrn)
	for name, content := range map[string]string{"harness.tf": harness, row + ".tf": string(tf)} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return resource.TestStep{
		ProviderFactories: functionalFactories(fake),
		ConfigDirectory:   config.StaticDirectory(dir),
		ConfigPlanChecks:  resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
	}
}

func TestFunctionalApprovalAction_UpgradeAssociationsNoDiff(t *testing.T) {
	fake, approvals := newApprovalFake(t)
	seedPolicy(fake, "2001", "PCI", fakeplatform.Item{"application.metadata.metadata_application.PCI": "No"})
	seedPolicy(fake, "2002", "Code Coverage", coverageConditions)
	seedPolicy(fake, "2003", "Security", securityConditions)
	seedPolicy(fake, "2004", "Respect Budget", fakeplatform.Item{"application": fakeplatform.Item{"$expr": fakeplatform.Item{"$lte": []any{
		"$application.metadata.finops.current_total_cost", "$application.metadata.finops.budget_assigned",
	}}}})
	seedAction(fake, "1001", "deployment", "deployment:create")
	seedAction(fake, "1002", "scope", "scope:create")
	for _, policy := range []string{"2001", "2002", "2003"} {
		approvals.SeedAssociation("1001", policy, false)
	}
	approvals.SeedAssociation("1002", "2004", false)

	resource.UnitTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			upgradeStep(t, fake, "associations"),
		},
	})
}

func TestFunctionalApprovalAction_UpgradeInlinePoliciesNoDiff(t *testing.T) {
	fake, approvals := newApprovalFake(t)
	seedPolicy(fake, "2001", "coverage", coverageConditions)
	seedAction(fake, "1001", "deployment", "deployment:create")
	approvals.SeedAssociation("1001", "2001", false)
	upgrade := upgradeStep(t, fake, "inline")
	// The same action, migrated to a specification with the API afterwards;
	// the .tf is never touched.
	migrated := upgrade
	migrated.PreConfig = func() { approvals.Migrate("1001", "spec_functional0001") }

	resource.UnitTest(t, resource.TestCase{
		Steps: []resource.TestStep{upgrade, migrated},
	})
}

func TestFunctionalApprovalAction_UpgradeAPIMigratedNoDiff(t *testing.T) {
	fake, approvals := newApprovalFake(t)
	seedPolicy(fake, "2001", "coverage", coverageConditions)
	seedPolicy(fake, "2002", "security", securityConditions)
	seedAction(fake, "1001", "deployment", "deployment:create")
	approvals.SeedAssociation("1001", "2001", false)
	approvals.SeedAssociation("1001", "2002", false)
	approvals.Migrate("1001", "spec_functional0001")

	resource.UnitTest(t, resource.TestCase{
		Steps: []resource.TestStep{
			upgradeStep(t, fake, "migrated"),
		},
	})
}

// sent is a request as the fake logs it: json.Encoder ends every body with a
// newline.
func sent(method, path, body string) string {
	if body != "" {
		body += "\n"
	}
	return method + " " + path + " " + body
}

// writeLog reads the fake's writes step by step: each check sees only the
// requests since the previous one, GETs left out (how often Terraform
// refreshes is not the provider's), sorted (neither is the order of
// independent resources).
type writeLog struct {
	fake *fakeplatform.Server
	seen int
}

func (l *writeLog) expect(want ...string) func(*terraform.State) error {
	return func(*terraform.State) error {
		requests := l.fake.Requests()
		var got []string
		for _, r := range requests[l.seen:] {
			if r.Method != http.MethodGet {
				got = append(got, r.Method+" "+r.Path+" "+r.Body)
			}
		}
		l.seen = len(requests)
		sort.Strings(got)
		expected := append([]string(nil), want...)
		sort.Strings(expected)
		if strings.Join(got, "|") != strings.Join(expected, "|") {
			return fmt.Errorf("writes:\n%q\nwant:\n%q", got, expected)
		}
		return nil
	}
}

const approvalPolicyBlock = `
provider "nullplatform" {}

resource "nullplatform_approval_policy" "coverage" {
  nrn        = "` + approvalNrn + `"
  name       = "coverage"
  conditions = jsonencode({ "build.metadata.coverage.code.coverage" = { "$gte" = 80 } })
}
`

// approvalActionConfig is the policy plus an action: attributes and extra go
// inside the action's block, after follows it.
func approvalActionConfig(attributes, extra, after string) string {
	return approvalPolicyBlock + fmt.Sprintf(`
resource "nullplatform_approval_action" "deployment_create" {
  nrn        = %q
  entity     = "deployment"
  dimensions = { environment = "production" }
%s
%s
}
%s`, approvalNrn, attributes, extra, after)
}

const (
	approvalAttributes = `
  action            = "deployment:create"
  on_policy_success = "approve"
  on_policy_fail    = "manual"`
	ignorePolicies   = `  lifecycle { ignore_changes = [policies] }`
	associationBlock = `
resource "nullplatform_approval_action_policy_association" "coverage" {
  approval_action_id = nullplatform_approval_action.deployment_create.id
  approval_policy_id = nullplatform_approval_policy.coverage.id
}`
	createdPolicy = `{"nrn":"` + approvalNrn + `","name":"coverage","conditions":{"build.metadata.coverage.code.coverage":{"$gte":80}},"selector":{}}`
	createdAction = `{"nrn":"` + approvalNrn + `","entity":"deployment","action":"deployment:create","dimensions":{"environment":"production"},"on_policy_success":"approve","on_policy_fail":"manual"}`
)

// An old configuration sends the same requests, byte for byte: the creates,
// a PATCH carrying only what changed, the association POST/DELETE and
// the destroy. Neither form ever sends a PATCH on create or destroy.
func TestFunctionalApprovalAction_SameRequestsForOldConfigs(t *testing.T) {
	t.Run("associations", func(t *testing.T) {
		fake, _ := newApprovalFake(t)
		log := &writeLog{fake: fake}

		resource.UnitTest(t, resource.TestCase{
			ProviderFactories: functionalFactories(fake),
			CheckDestroy: log.expect(
				sent("DELETE", "/approval/action/1", ""),
				sent("DELETE", "/approval/policy/1", ""),
			),
			Steps: []resource.TestStep{
				{
					Config: approvalActionConfig(approvalAttributes, ignorePolicies, associationBlock),
					Check: log.expect(
						sent("POST", "/approval/policy", createdPolicy),
						sent("POST", "/approval/action", createdAction),
						sent("POST", "/approval/action/1/policy", `{"policy_id":"1"}`),
					),
				},
				{
					Config: approvalActionConfig(strings.Replace(approvalAttributes, `"manual"`, `"deny"`, 1), ignorePolicies, associationBlock),
					Check:  log.expect(sent("PATCH", "/approval/action/1", `{"on_policy_fail":"deny"}`)),
				},
				{
					Config: approvalActionConfig(strings.Replace(approvalAttributes, `"manual"`, `"deny"`, 1), ignorePolicies, ""),
					Check:  log.expect(sent("DELETE", "/approval/action/1/policy/1", "")),
				},
			},
		})
	})

	// The action with association resources, migrated with the API: the
	// server seeded on_checklist_fail, and a change of on_policy_fail still
	// sends only that.
	t.Run("migrated", func(t *testing.T) {
		fake, approvals := newApprovalFake(t)
		log := &writeLog{fake: fake}
		config := func(fail string) string {
			return approvalActionConfig(strings.Replace(approvalAttributes, `"manual"`, fail, 1), ignorePolicies, associationBlock)
		}

		resource.UnitTest(t, resource.TestCase{
			ProviderFactories: functionalFactories(fake),
			Steps: []resource.TestStep{
				{Config: config(`"manual"`)},
				{
					PreConfig: func() {
						approvals.Migrate("1", "spec_functional0001")
						log.seen = len(fake.Requests())
					},
					Config: config(`"deny"`),
					Check:  log.expect(sent("PATCH", "/approval/action/1", `{"on_policy_fail":"deny"}`)),
				},
			},
		})
	})

	t.Run("inline policies", func(t *testing.T) {
		fake, _ := newApprovalFake(t)
		log := &writeLog{fake: fake}

		resource.UnitTest(t, resource.TestCase{
			ProviderFactories: functionalFactories(fake),
			CheckDestroy: log.expect(
				sent("DELETE", "/approval/action/1", ""),
				sent("DELETE", "/approval/policy/1", ""),
			),
			Steps: []resource.TestStep{
				{
					Config: approvalActionConfig(approvalAttributes, `  policies = [nullplatform_approval_policy.coverage.id]`, ""),
					Check: log.expect(
						sent("POST", "/approval/policy", createdPolicy),
						sent("POST", "/approval/action", createdAction),
						sent("POST", "/approval/action/1/policy", `{"policy_id":"1"}`),
					),
				},
			},
		})
	})
}

// An action deleted outside Terraform (the API's soft delete: GET answers 200
// with status "deleted") leaves the state, and the plan offers to create it.
func TestFunctionalApprovalAction_DeletedOutsideTerraform(t *testing.T) {
	fake, _ := newApprovalFake(t)
	config := approvalActionConfig(approvalAttributes, "", "")

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					if err := newTestClient(fake.HTTP()).DeleteApprovalAction("1"); err != nil {
						t.Fatal(err)
					}
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("nullplatform_approval_action.deployment_create", plancheck.ResourceActionCreate),
				}},
				Check: resource.TestCheckResourceAttr("nullplatform_approval_action.deployment_create", "id", "2"),
			},
		},
	})
}

func entityActionConfig(entity, action string) string {
	return fmt.Sprintf(`
provider "nullplatform" {}

resource "nullplatform_approval_action" "deployment_create" {
  nrn               = %q
  entity            = %q
  action            = %q
  on_policy_success = "approve"
  on_policy_fail    = "manual"
}
`, approvalNrn, entity, action)
}

// The API never applies a new entity or action (its PATCH ignores both,
// observed against the API), so changing either replaces the action — and the
// next plan is clean.
func TestFunctionalApprovalAction_EntityActionForceNew(t *testing.T) {
	fake, _ := newApprovalFake(t)
	log := &writeLog{fake: fake}
	const address = "nullplatform_approval_action.deployment_create"
	replaced := resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
		plancheck.ExpectResourceAction(address, plancheck.ResourceActionDestroyBeforeCreate),
	}}

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: entityActionConfig("deployment", "deployment:create")},
			{
				PreConfig:        func() { log.seen = len(fake.Requests()) },
				Config:           entityActionConfig("deployment", "deployment:rollback"),
				ConfigPlanChecks: replaced,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(address, "id", "2"),
					resource.TestCheckResourceAttr(address, "action", "deployment:rollback"),
					log.expect(
						sent("DELETE", "/approval/action/1", ""),
						sent("POST", "/approval/action", `{"nrn":"`+approvalNrn+`","entity":"deployment","action":"deployment:rollback","on_policy_success":"approve","on_policy_fail":"manual"}`),
					),
				),
			},
			{
				Config:           entityActionConfig("scope", "deployment:rollback"),
				ConfigPlanChecks: replaced,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr(address, "id", "3"),
					resource.TestCheckResourceAttr(address, "entity", "scope"),
				),
			},
		},
	})
}

const securityPolicyBlock = `
resource "nullplatform_approval_policy" "security" {
  nrn        = "` + approvalNrn + `"
  name       = "security"
  conditions = jsonencode({ "build.metadata.security.security.vulnerabilities.critical" = { "$eq" = 0 } })
}
`

// An update that changes no on_policy_* sends no PATCH; one that changes
// one sends only that one.
func TestFunctionalApprovalAction_NoEmptyPatch(t *testing.T) {
	fake, _ := newApprovalFake(t)
	log := &writeLog{fake: fake}
	both := `  policies = [nullplatform_approval_policy.coverage.id, nullplatform_approval_policy.security.id]`

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: approvalActionConfig(approvalAttributes, `  policies = [nullplatform_approval_policy.coverage.id]`, "")},
			{
				// Only policies change: a second one, created in this step.
				PreConfig: func() { log.seen = len(fake.Requests()) },
				Config:    approvalActionConfig(approvalAttributes, both, securityPolicyBlock),
				Check: log.expect(
					sent("POST", "/approval/policy", `{"nrn":"`+approvalNrn+`","name":"security","conditions":{"build.metadata.security.security.vulnerabilities.critical":{"$eq":0}},"selector":{}}`),
					sent("POST", "/approval/action/1/policy", `{"policy_id":"2"}`),
				),
			},
			{
				Config: approvalActionConfig(strings.Replace(approvalAttributes, `"approve"`, `"manual"`, 1), both, securityPolicyBlock),
				Check:  log.expect(sent("PATCH", "/approval/action/1", `{"on_policy_success":"manual"}`)),
			},
			{
				// With on_checklist_fail in the state, changing on_policy_fail
				// still sends only on_policy_fail.
				Config: approvalActionConfig(strings.Replace(approvalAttributes, `"approve"`, `"manual"`, 1), both+`
  on_checklist_fail = "manual"`, securityPolicyBlock),
				Check: log.expect(sent("PATCH", "/approval/action/1", `{"on_checklist_fail":"manual"}`)),
			},
			{
				Config: approvalActionConfig(`
  action            = "deployment:create"
  on_policy_success = "manual"
  on_policy_fail    = "deny"
  on_checklist_fail = "manual"`, both, securityPolicyBlock),
				Check: log.expect(sent("PATCH", "/approval/action/1", `{"on_policy_fail":"deny"}`)),
			},
		},
	})
}

// A refused PATCH carries the API's message, not just its status code.
func TestFunctionalApprovalAction_PatchErrorCarriesAPIMessage(t *testing.T) {
	fake, _ := newApprovalFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: approvalActionConfig(approvalAttributes, "", "")},
			{
				Config:      approvalActionConfig(strings.Replace(approvalAttributes, `"manual"`, `"bogus"`, 1), "", ""),
				ExpectError: regexp.MustCompile(`error patching approval action resource, got 400: Invalid policy callback\.\s+It\s+must\s+be\s+one\s+of\s+the\s+following:\s+manual,\s+approve,\s+deny`),
			},
		},
	})
}

// Importing an action that does not exist is an error, never a panic.
// The API answers a missing numeric id with 401 — it cannot tell it apart
// from one this key cannot read, so it stays an error and nothing leaves the
// state on a permission problem; a non-numeric id matches no route, and the
// 404 is Terraform's own "Cannot import non-existent remote object".
func TestFunctionalApprovalAction_ImportMissingIsError(t *testing.T) {
	fake, _ := newApprovalFake(t)
	config := approvalActionConfig(approvalAttributes, "", "")
	const address = "nullplatform_approval_action.deployment_create"

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config:        config,
				ResourceName:  address,
				ImportState:   true,
				ImportStateId: "999999999",
				ExpectError:   regexp.MustCompile(`approval action 999999999 not found or not readable with this API key:\s+You're\s+not\s+authorized\s+to\s+perform\s+this\s+operation\.`),
			},
			{
				Config:        config,
				ResourceName:  address,
				ImportState:   true,
				ImportStateId: "not-a-number",
				ExpectError:   regexp.MustCompile(`Cannot import non-existent remote object`),
			},
		},
	})
}

// What the API sets is no diff: on_policy_* when the configuration omits
// them, on_checklist_fail when a specification is linked.
func TestFunctionalApprovalAction_ServerSeededValuesNoDiff(t *testing.T) {
	fake, _, client := newLinkFake(t)
	bare := approvalActionConfig(`  action = "deployment:create"`, "", "")

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				Config: bare,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("nullplatform_approval_action.deployment_create", "on_policy_success", "manual"),
					resource.TestCheckResourceAttr("nullplatform_approval_action.deployment_create", "on_policy_fail", "manual"),
				),
			},
			{
				Config: bare + specAndLinkBlocks,
				Check: func(*terraform.State) error {
					if action, err := client.GetApprovalAction("1"); err != nil || action.OnChecklistFail != "pending" {
						return fmt.Errorf("action %+v, %v; want on_checklist_fail seeded pending", action, err)
					}
					return nil
				},
			},
		},
	})
}

// A fail path outside the list fails in the plan.
func TestFunctionalApprovalAction_InvalidOnChecklistFail(t *testing.T) {
	fake, _ := newApprovalFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config:      approvalActionConfig(approvalAttributes, `  on_checklist_fail = "foo"`, ""),
			ExpectError: regexp.MustCompile(`expected on_checklist_fail to be one of \["deny" "pending" "manual"\], got foo`),
		}},
	})
}

// With a specification linked the API keeps listing policies the action no
// longer uses: the refresh does not read them, so dropping inline
// policies from a migrated action converges instead of coming back.
func TestFunctionalApprovalAction_LinkedReadLeavesPolicies(t *testing.T) {
	fake, approvals, client := newLinkFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: approvalActionConfig(approvalAttributes, `  policies = [nullplatform_approval_policy.coverage.id]`, "")},
			{
				PreConfig: func() {
					spec, err := client.CreateChecklistSpecification(&ChecklistSpecification{Nrn: approvalNrn, Name: "spec", Definition: json.RawMessage(specJSONV1)})
					if err != nil {
						t.Fatal(err)
					}
					approvals.Migrate("1", spec.Id)
				},
				Config: approvalActionConfig(approvalAttributes, "", ""),
				Check:  resource.TestCheckResourceAttr("nullplatform_approval_action.deployment_create", "policies.#", "0"),
			},
		},
	})
}
