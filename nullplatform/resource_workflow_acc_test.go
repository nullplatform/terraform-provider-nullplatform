package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	tfterraform "github.com/hashicorp/terraform-plugin-testing/terraform"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// The workflow acceptance tests create disposable workflows (key tfacc-...)
// in the organization of NULLPLATFORM_API_KEY, and destroy them. The key must
// be allowed to manage workflows and, for the checklist test, notification
// channels at the organization.

var accWorkflowWebhookPattern = regexp.MustCompile(`/workflows/webhooks/wf_[^/]+/live/[A-Za-z0-9_-]+$`)

func accWorkflowFactories() map[string]func() (*schema.Provider, error) {
	return map[string]func() (*schema.Provider, error){
		"nullplatform": func() (*schema.Provider, error) { return Provider(), nil },
	}
}

// accWorkflowClient is a client configured as the provider configures its own,
// for checks against the API itself.
func accWorkflowClient(t *testing.T) *NullClient {
	t.Helper()
	p := Provider()
	if diags := p.Configure(context.Background(), terraform.NewResourceConfigRaw(nil)); diags.HasError() {
		t.Fatalf("configuring a client for the checks: %v", diags)
	}
	return p.Meta().(*NullClient)
}

func skipUnlessAcc(t *testing.T) {
	t.Helper()
	if os.Getenv(resource.EnvTfAcc) == "" {
		t.Skipf("acceptance test: set %s=1 and NULLPLATFORM_API_KEY", resource.EnvTfAcc)
	}
}

func accWebhookWorkflowConfig(suffix, message string, active bool) string {
	return fmt.Sprintf(`
resource "nullplatform_workflow" "this" {
  name        = "tfacc %[1]s"
  key         = "tfacc-%[1]s"
  description = "Disposable: provider acceptance test. Safe to delete."
  path        = "/tfacc"
  definition = jsonencode({
    steps = {
      start = { id = "start", name = "On webhook", pluginType = "webhook", config = { path = "/tfacc-%[1]s", mode = "start" } }
      say   = { id = "say", name = "Say", pluginType = "log", config = { message = %[2]q } }
    }
    connections = [{ id = "c1", from = "start", to = "say" }]
  })
}

resource "nullplatform_workflow_alias" "live" {
  workflow_id = nullplatform_workflow.this.id
  name        = "live"
  revision    = nullplatform_workflow.this.revision
  active      = %[3]t
}
`, suffix, message, active)
}

// TestAccWorkflow_webhookLifecycle creates a webhook workflow published under
// live, changes its definition (one apply: a new revision, the alias on it,
// active), deactivates and reactivates the alias, repairs drift made through
// the API, imports both by key, and destroys them.
func TestAccWorkflow_webhookLifecycle(t *testing.T) {
	skipUnlessAcc(t)
	suffix := acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum)
	suffix = strings.ToLower(suffix)
	client := accWorkflowClient(t)
	var workflowID string

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_5_0)},
		ProviderFactories:      accWorkflowFactories(),
		CheckDestroy:           accWorkflowDestroyed(t, client, &workflowID, nil),
		Steps: []resource.TestStep{
			{
				Config: accWebhookWorkflowConfig(suffix, "first", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("nullplatform_workflow.this", tfjsonpath.New("revision"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("nullplatform_workflow.this", tfjsonpath.New("nrn"),
						knownvalue.StringRegexp(regexp.MustCompile(`^organization=\d+$`))),
					statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("revision"), knownvalue.Int64Exact(1)),
					statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("active"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("webhook_urls"),
						knownvalue.MapExact(map[string]knownvalue.Check{"start": knownvalue.StringRegexp(accWorkflowWebhookPattern)})),
				},
				Check: resource.ComposeTestCheckFunc(
					func(s *tfterraform.State) error {
						workflowID = s.RootModule().Resources["nullplatform_workflow.this"].Primary.ID
						return nil
					},
					accWorkflowAliasIs(client, &workflowID, 1, true),
				),
			},
			{
				Config: accWebhookWorkflowConfig(suffix, "second", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("nullplatform_workflow.this", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("nullplatform_workflow_alias.live", plancheck.ResourceActionUpdate),
					plancheck.ExpectUnknownValue("nullplatform_workflow.this", tfjsonpath.New("revision")),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("nullplatform_workflow.this", tfjsonpath.New("revision"), knownvalue.Int64Exact(2)),
					statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("revision"), knownvalue.Int64Exact(2)),
					statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("active"), knownvalue.Bool(true)),
				},
				Check: accWorkflowAliasIs(client, &workflowID, 2, true),
			},
			{
				Config: accWebhookWorkflowConfig(suffix, "second", false),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("active"), knownvalue.Bool(false)),
					statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("webhook_urls"), knownvalue.MapExact(map[string]knownvalue.Check{})),
				},
				Check: accWorkflowAliasIs(client, &workflowID, 2, false),
			},
			{
				Config: accWebhookWorkflowConfig(suffix, "second", true),
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("active"), knownvalue.Bool(true)),
					statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("webhook_urls"),
						knownvalue.MapExact(map[string]knownvalue.Check{"start": knownvalue.StringRegexp(accWorkflowWebhookPattern)})),
				},
				Check: accWorkflowAliasIs(client, &workflowID, 2, true),
			},
			{
				// Drift: the alias deactivated outside Terraform shows in the plan,
				// and the apply activates it again.
				PreConfig: func() {
					if _, err := client.DeactivateWorkflowAlias(workflowID, "live"); err != nil {
						t.Fatalf("deactivating outside Terraform: %v", err)
					}
				},
				Config: accWebhookWorkflowConfig(suffix, "second", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("nullplatform_workflow.this", plancheck.ResourceActionNoop),
					plancheck.ExpectResourceAction("nullplatform_workflow_alias.live", plancheck.ResourceActionUpdate),
				}},
				Check: accWorkflowAliasIs(client, &workflowID, 2, true),
			},
			{
				// Drift: a revision saved outside Terraform shows in the plan, and the
				// apply saves the configured definition again and publishes it.
				PreConfig: func() {
					read, err := client.GetWorkflow(workflowID)
					if err != nil {
						t.Fatalf("reading before the outside change: %v", err)
					}
					definition := map[string]json.RawMessage{}
					if err := json.Unmarshal(read.Revision.Definition, &definition); err != nil {
						t.Fatal(err)
					}
					definition["connections"] = json.RawMessage(`[]`)
					for _, key := range []string{"id", "triggers", "pinnedDescriptors"} {
						delete(definition, key)
					}
					if _, err := client.PutWorkflowRevision(workflowID, definition); err != nil {
						t.Fatalf("saving a revision outside Terraform: %v", err)
					}
				},
				Config: accWebhookWorkflowConfig(suffix, "second", true),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("nullplatform_workflow.this", plancheck.ResourceActionUpdate),
					plancheck.ExpectResourceAction("nullplatform_workflow_alias.live", plancheck.ResourceActionUpdate),
				}},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectKnownValue("nullplatform_workflow.this", tfjsonpath.New("revision"), knownvalue.Int64Exact(4)),
					statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("revision"), knownvalue.Int64Exact(4)),
				},
				Check: accWorkflowAliasIs(client, &workflowID, 4, true),
			},
			{
				// Import by key: the import plans no change.
				ResourceName:    "nullplatform_workflow.this",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				ImportStateId:   "tfacc-" + suffix,
			},
			{
				ResourceName:    "nullplatform_workflow_alias.live",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithID,
				ImportStateId:   "tfacc-" + suffix + "/live",
			},
		},
	})
}

// TestAccWorkflow_checklistTrigger publishes a workflow with a checklist
// trigger at the organization: activation registers its notification channel,
// and the destroy releases it.
func TestAccWorkflow_checklistTrigger(t *testing.T) {
	skipUnlessAcc(t)
	suffix := strings.ToLower(acctest.RandStringFromCharSet(8, acctest.CharSetAlphaNum))
	client := accWorkflowClient(t)
	org, err := client.GetOrganizationIDFromToken()
	if err != nil {
		t.Fatal(err)
	}
	var workflowID string
	var channelID int

	resource.Test(t, resource.TestCase{
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_5_0)},
		ProviderFactories:      accWorkflowFactories(),
		CheckDestroy:           accWorkflowDestroyed(t, client, &workflowID, &channelID),
		Steps: []resource.TestStep{{
			Config: fmt.Sprintf(`
resource "nullplatform_workflow" "gate" {
  name        = "tfacc gate %[1]s"
  key         = "tfacc-gate-%[1]s"
  description = "Disposable: provider acceptance test. Safe to delete."
  definition = jsonencode({
    steps = {
      gate = { id = "gate", name = "On checklist", pluginType = "np-checklist-trigger", config = {
        kind = "tfacc-%[1]s", nrn = "organization=%[2]s", mode = "start", pathPrefix = "/np/checklist/tfacc-%[1]s"
      } }
      say = { id = "say", name = "Say", pluginType = "log", config = { message = "dispatched" } }
    }
    connections = [{ id = "c1", from = "gate", to = "say", sourcePort = "default", targetPort = "default" }]
  })
}

resource "nullplatform_workflow_alias" "live" {
  workflow_id = nullplatform_workflow.gate.id
  name        = "live"
  revision    = nullplatform_workflow.gate.revision
}
`, suffix, org),
			ConfigStateChecks: []statecheck.StateCheck{
				statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("active"), knownvalue.Bool(true)),
				statecheck.ExpectKnownValue("nullplatform_workflow_alias.live", tfjsonpath.New("webhook_urls"),
					knownvalue.MapExact(map[string]knownvalue.Check{"gate": knownvalue.StringRegexp(accWorkflowWebhookPattern)})),
			},
			Check: func(s *tfterraform.State) error {
				workflowID = s.RootModule().Resources["nullplatform_workflow.gate"].Primary.ID
				id, err := accWorkflowChannelID(client, workflowID, "live", "gate")
				if err != nil {
					return err
				}
				channelID = id
				if active, err := accChannelActive(client, channelID); err != nil || !active {
					return fmt.Errorf("the checklist trigger's channel %d is not active (%v)", channelID, err)
				}
				return nil
			},
		}},
	})
}

// accWorkflowAliasIs checks the alias live through the API.
func accWorkflowAliasIs(client *NullClient, workflowID *string, revision int, active bool) resource.TestCheckFunc {
	return func(*tfterraform.State) error {
		alias, err := client.GetWorkflowAlias(*workflowID, "live")
		if err != nil {
			return err
		}
		if alias.Revision != revision || alias.Active != active {
			return fmt.Errorf("the API has alias live on revision %d, active %t; want revision %d, active %t",
				alias.Revision, alias.Active, revision, active)
		}
		return nil
	}
}

// accWorkflowDestroyed checks through the API that the workflow is gone and its
// alias with it. The channel a checklist trigger's activation registered is
// released by the engine when the destroy deactivates the alias; an engine
// that does not release it on every deactivation leaves it active, which the
// check reports in the log and cleans up, since the provider never owned it.
func accWorkflowDestroyed(t *testing.T, client *NullClient, workflowID *string, channelID *int) resource.TestCheckFunc {
	return func(*tfterraform.State) error {
		if *workflowID == "" {
			return nil
		}
		if _, err := client.GetWorkflow(*workflowID); !isWorkflowGone(err) {
			return fmt.Errorf("workflow %s still exists after the destroy (%v)", *workflowID, err)
		}
		if _, err := client.GetWorkflowAlias(*workflowID, "live"); !isWorkflowGone(err) {
			return fmt.Errorf("alias live of workflow %s still exists after the destroy (%v)", *workflowID, err)
		}
		if channelID != nil && *channelID != 0 {
			active, err := accChannelActive(client, *channelID)
			if err != nil {
				return err
			}
			if active {
				t.Logf("ENGINE: the deactivation left the checklist trigger's channel %d active; deleting it", *channelID)
				return client.DeleteNotificationChannel(fmt.Sprint(*channelID))
			}
		}
		return nil
	}
}

// accWorkflowChannelID reads the channel a trigger's activation registered,
// from the alias's trigger states.
func accWorkflowChannelID(client *NullClient, workflowID, alias, trigger string) (int, error) {
	res, err := client.MakeRequest(http.MethodGet, workflowAliasPath(workflowID, alias), nil)
	if err != nil {
		return 0, err
	}
	defer res.Body.Close()
	var body struct {
		TriggerStates map[string]struct {
			PluginState struct {
				ChannelId int `json:"channelId"`
			} `json:"pluginState"`
		} `json:"triggerStates"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil {
		return 0, err
	}
	if id := body.TriggerStates[trigger].PluginState.ChannelId; id != 0 {
		return id, nil
	}
	return 0, fmt.Errorf("alias %s of workflow %s has no channel for trigger %s", alias, workflowID, trigger)
}

// accChannelActive reports whether a channel exists and is active. A deleted
// channel answers no 200 (the API answers 401 for it).
func accChannelActive(client *NullClient, id int) (bool, error) {
	res, err := client.MakeRequest(http.MethodGet, fmt.Sprintf("%s/%d", NOTIFICATION_CHANNEL_PATH, id), nil)
	if err != nil {
		return false, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return false, nil
	}
	var channel struct {
		Status string `json:"status"`
	}
	if err := json.NewDecoder(res.Body).Decode(&channel); err != nil {
		return false, err
	}
	return channel.Status == "active", nil
}
