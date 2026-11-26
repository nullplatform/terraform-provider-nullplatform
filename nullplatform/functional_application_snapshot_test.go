package nullplatform

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"

	"github.com/nullplatform/terraform-provider-nullplatform/internal/fakeplatform"
)

// Real plan/apply over the fake platform: a change of a type's schema cuts a
// snapshot and leaves its applications where they are, and an application
// moves to another snapshot when `application_type_snapshot_id` says so.

func snapshotConfig(typeSchema, snapshotLine, attributes string) string {
	return applicationTypeConfig("AI Agent", typeSchema, "") + fmt.Sprintf(`
resource "nullplatform_application" "bot" {
  name                = "support-bot"
  namespace_id        = 3
  application_type_id = nullplatform_application_type.agent.id
  attributes          = jsonencode(%s)
%s}
`, attributes, snapshotLine)
}

const (
	onTheLastSnapshot = "  application_type_snapshot_id = nullplatform_application_type.agent.last_snapshot_id\n"
	lowAutonomy       = `{ model = "fable", autonomy = "low" }`
)

func onSnapshot(id string) resource.TestCheckFunc {
	return resource.TestCheckResourceAttr("nullplatform_application.bot", "application_type_snapshot_id", id)
}

func patchesOfTheApplication(fake *fakeplatform.Server) []fakeplatform.Request {
	return requestsTo(fake, http.MethodPatch, "/application/1")
}

func TestFunctionalApplication_FollowsItsTypesLastSnapshot(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{
				// A new application is on the last snapshot: there is nowhere to move.
				Config: snapshotConfig(agentSchema, onTheLastSnapshot, lowAutonomy),
				Check: resource.ComposeTestCheckFunc(onSnapshot("snapshot-1"), func(*terraform.State) error {
					if sent := patchesOfTheApplication(fake); len(sent) != 0 {
						return fmt.Errorf("want no edit of a new application, got %v", sent)
					}
					return nil
				}),
			},
			{
				// The schema changes: the snapshot it cuts is not known when the plan is
				// made, and the same apply moves the application to it. No attribute
				// changes, so the move carries an empty edit of them.
				Config: snapshotConfig(agentSchemaReordered, onTheLastSnapshot, lowAutonomy),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{
						plancheck.ExpectResourceAction("nullplatform_application.bot", plancheck.ResourceActionUpdate),
					},
				},
				Check: resource.ComposeTestCheckFunc(onSnapshot("snapshot-2"),
					lastPatchIs(fake, 1, `{"desired_application_type_snapshot_id":"snapshot-2","attributes":{}}`)),
			},
			{
				// Back to an earlier snapshot, in one request with the attributes that change.
				Config: snapshotConfig(agentSchemaReordered, `  application_type_snapshot_id = "snapshot-1"`+"\n", `{ model = "opus", autonomy = "low" }`),
				Check: resource.ComposeTestCheckFunc(onSnapshot("snapshot-1"),
					resource.TestCheckResourceAttr("nullplatform_application.bot", "attributes", `{"autonomy":"low","model":"opus"}`),
					lastPatchIs(fake, 2, `{"desired_application_type_snapshot_id":"snapshot-1","attributes":{"model":"opus"}}`)),
			},
			{
				// A snapshot that is not of the type is refused with the API's reason.
				Config:      snapshotConfig(agentSchemaReordered, `  application_type_snapshot_id = "snapshot-99"`+"\n", `{ model = "opus", autonomy = "low" }`),
				ExpectError: regexp.MustCompile(`snapshot-99 is not a snapshot of the application's type`),
			},
			{
				// The refused edit left the state as the platform has it.
				ResourceName:      "nullplatform_application.bot",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestFunctionalApplication_StaysOnItsSnapshotWhenNoneIsSet(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: snapshotConfig(agentSchema, "", lowAutonomy), Check: onSnapshot("snapshot-1")},
			{
				Config: snapshotConfig(agentSchemaReordered, "", lowAutonomy),
				Check: resource.ComposeTestCheckFunc(onSnapshot("snapshot-1"), func(*terraform.State) error {
					if sent := patchesOfTheApplication(fake); len(sent) != 0 {
						return fmt.Errorf("want the application left alone, got %v", sent)
					}
					return nil
				}),
			},
		},
	})
}

func TestFunctionalApplication_IsCreatedOnAnEarlierSnapshot(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{
			{Config: applicationTypeConfig("AI Agent", agentSchema, "")},
			{Config: applicationTypeConfig("AI Agent", agentSchemaReordered, "")},
			{
				// The platform creates it on the last snapshot; the move follows.
				Config: snapshotConfig(agentSchemaReordered, `  application_type_snapshot_id = "snapshot-1"`+"\n", lowAutonomy),
				Check: resource.ComposeTestCheckFunc(onSnapshot("snapshot-1"),
					lastPatchIs(fake, 1, `{"desired_application_type_snapshot_id":"snapshot-1","attributes":{}}`)),
			},
		},
	})
}

func TestFunctionalApplication_CreatedOnASnapshotThatIsNotOfItsTypeIsAnError(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactories(fake),
		Steps: []resource.TestStep{{
			Config:      snapshotConfig(agentSchema, `  application_type_snapshot_id = "snapshot-99"`+"\n", lowAutonomy),
			ExpectError: regexp.MustCompile(`snapshot-99 is not a snapshot of the application's type`),
		}},
	})
}

// An application whose creation failed is not moved: the reason it failed is
// what the apply says.
func TestFunctionalApplication_CreatedOnAnEarlierSnapshotFailsWithItsCreation(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactoriesWith(failedCreationOps{newTestClient(fake.HTTP())}),
		Steps: []resource.TestStep{
			{Config: applicationTypeConfig("AI Agent", agentSchema, "")},
			{Config: applicationTypeConfig("AI Agent", agentSchemaReordered, "")},
			{
				Config:      snapshotConfig(agentSchemaReordered, `  application_type_snapshot_id = "snapshot-1"`+"\n", lowAutonomy),
				ExpectError: regexp.MustCompile(`its creation failed(.|\s)*no channel takes it`),
			},
		},
	})
}

// failedCreationOps is the fake's client with applications whose type's
// workflow answered their creation with a failure.
type failedCreationOps struct {
	*NullClient
}

func (o failedCreationOps) GetApplication(id string) (*Application, error) {
	app, err := o.NullClient.GetApplication(id)
	if err == nil && app.Status == "active" {
		app.Status = "failed"
		app.Messages = []interface{}{map[string]interface{}{"level": "ERROR", "message": "no channel takes it"}}
	}
	return app, err
}

// The wait for the creation's answer reads the application; a read that fails
// ends it, and the application stays in the state to be dealt with.
func TestFunctionalApplication_CreatedOnAnEarlierSnapshotFailsWhenItCannotBeRead(t *testing.T) {
	fake := newApplicationTypeFake(t)

	resource.UnitTest(t, resource.TestCase{
		ProviderFactories: functionalFactoriesWith(&unreadableOnceOps{NullClient: newTestClient(fake.HTTP())}),
		Steps: []resource.TestStep{
			{Config: applicationTypeConfig("AI Agent", agentSchema, "")},
			{Config: applicationTypeConfig("AI Agent", agentSchemaReordered, "")},
			{
				Config:      snapshotConfig(agentSchemaReordered, `  application_type_snapshot_id = "snapshot-1"`+"\n", lowAutonomy),
				ExpectError: regexp.MustCompile(`still waiting for its type's workflow to answer its creation(.|\s)*unreadable`),
			},
		},
	})
}

// unreadableOnceOps is the fake's client whose first read of an application fails.
type unreadableOnceOps struct {
	*NullClient
	failed bool
}

func (o *unreadableOnceOps) GetApplication(id string) (*Application, error) {
	if !o.failed {
		o.failed = true
		return nil, fmt.Errorf("unreadable")
	}
	return o.NullClient.GetApplication(id)
}
