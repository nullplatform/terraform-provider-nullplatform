package nullplatform_test

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

// The request body of an agent channel carries the customer api_key: it must
// never be printed (provider stdout ends up in Terraform logs).
func TestNotificationChannel_DoesNotPrintTheRequestBody(t *testing.T) {
	var body map[string]interface{}
	client := agentChannelServer(t, managedAgentChannelResponse, &body)

	original := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = writer

	create := schema.TestResourceDataRaw(t, agentChannelSchema(), agentChannelConfig("dummy-secret-key"))
	createErr := nullplatform.NotificationChannelCreate(create, client)
	update := schema.TestResourceDataRaw(t, agentChannelSchema(), agentChannelConfig("dummy-secret-key"))
	update.SetId("123")
	updateErr := nullplatform.NotificationChannelUpdate(update, client)

	writer.Close()
	os.Stdout = original
	printed, _ := io.ReadAll(reader)

	if createErr != nil || updateErr != nil {
		t.Fatalf("unexpected errors: create=%v update=%v", createErr, updateErr)
	}
	if strings.Contains(string(printed), "dummy-secret-key") {
		t.Errorf("the api_key was printed to stdout: %s", printed)
	}
}
