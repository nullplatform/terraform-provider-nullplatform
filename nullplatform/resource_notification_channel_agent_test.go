package nullplatform_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

// An agent channel created without api_key gets a platform-managed credential
// (main-notifications-api PR #205). The API rejects an empty api_key, so the
// provider must omit the field instead of sending "".

const managedAgentChannelResponse = `{
	"id": 123,
	"nrn": "organization=1:account=2",
	"type": "agent",
	"source": ["service"],
	"configuration": {
		"command": {"type": "exec", "data": {"cmdline": "echo dummy"}},
		"selector": {"environment": "test"}
	},
	"status": "active",
	"filters": {}
}`

func agentChannelConfig(apiKey string) map[string]interface{} {
	agent := map[string]interface{}{
		"command": []interface{}{
			map[string]interface{}{
				"type": "exec",
				"data": map[string]interface{}{"cmdline": `"echo dummy"`},
			},
		},
		"selector": map[string]interface{}{"environment": "test"},
	}
	if apiKey != "" {
		agent["api_key"] = apiKey
	}
	return map[string]interface{}{
		"nrn":    "organization=1:account=2",
		"type":   "agent",
		"source": []interface{}{"service"},
		"configuration": []interface{}{
			map[string]interface{}{"agent": []interface{}{agent}},
		},
	}
}

// agentChannelServer records the JSON body of the last POST or PATCH and answers
// GET with the given channel.
func agentChannelServer(t *testing.T, getResponse string, body *map[string]interface{}) *nullplatform.NullClient {
	t.Helper()
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost, http.MethodPatch:
			if err := json.NewDecoder(r.Body).Decode(body); err != nil {
				t.Errorf("failed to decode %s body: %v", r.Method, err)
			}
			if r.Method == http.MethodPatch {
				w.WriteHeader(http.StatusNoContent)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, getResponse)
		case http.MethodGet:
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, getResponse)
		default:
			t.Errorf("unexpected request method: %s", r.Method)
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	}))
	t.Cleanup(server.Close)
	return &nullplatform.NullClient{
		Client: server.Client(),
		ApiURL: strings.TrimPrefix(server.URL, "https://"),
		Token:  nullplatform.Token{AccessToken: "test-token"},
	}
}

func agentChannelSchema() map[string]*schema.Schema {
	return nullplatform.Provider().ResourcesMap["nullplatform_notification_channel"].Schema
}

func sentConfiguration(t *testing.T, body map[string]interface{}) map[string]interface{} {
	t.Helper()
	configuration, ok := body["configuration"].(map[string]interface{})
	if !ok {
		t.Fatalf("request body has no configuration object: %v", body)
	}
	return configuration
}

func TestNotificationChannelSchema_AgentApiKeyIsOptionalSensitiveAndDocumented(t *testing.T) {
	agent := agentChannelSchema()["configuration"].Elem.(*schema.Resource).Schema["agent"].Elem.(*schema.Resource)
	apiKey := agent.Schema["api_key"]

	if apiKey.Required || !apiKey.Optional {
		t.Errorf("api_key must be Optional (Required=%v, Optional=%v)", apiKey.Required, apiKey.Optional)
	}
	if !apiKey.Sensitive {
		t.Error("api_key must stay Sensitive")
	}
	if !strings.Contains(apiKey.Description, "controlplane:agent-dispatcher") {
		t.Errorf("api_key Description must explain the managed credential, got %q", apiKey.Description)
	}
}

func TestNotificationChannelCreate_AgentWithoutApiKeyOmitsTheField(t *testing.T) {
	var body map[string]interface{}
	client := agentChannelServer(t, managedAgentChannelResponse, &body)
	d := schema.TestResourceDataRaw(t, agentChannelSchema(), agentChannelConfig(""))

	if err := nullplatform.NotificationChannelCreate(d, client); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, present := sentConfiguration(t, body)["api_key"]; present {
		t.Errorf("create body must not carry api_key when it is not configured: %v", body["configuration"])
	}
}

func TestNotificationChannelCreate_AgentWithApiKeySendsIt(t *testing.T) {
	var body map[string]interface{}
	client := agentChannelServer(t, managedAgentChannelResponse, &body)
	d := schema.TestResourceDataRaw(t, agentChannelSchema(), agentChannelConfig("dummy-customer-key"))

	if err := nullplatform.NotificationChannelCreate(d, client); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got := sentConfiguration(t, body)["api_key"]; got != "dummy-customer-key" {
		t.Errorf("got api_key %v, want %q", got, "dummy-customer-key")
	}
}

func TestNotificationChannelUpdate_AgentWithoutApiKeyOmitsTheField(t *testing.T) {
	var body map[string]interface{}
	client := agentChannelServer(t, managedAgentChannelResponse, &body)
	d := schema.TestResourceDataRaw(t, agentChannelSchema(), agentChannelConfig(""))
	d.SetId("123")

	if err := nullplatform.NotificationChannelUpdate(d, client); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, present := sentConfiguration(t, body)["api_key"]; present {
		t.Errorf("update body must not carry api_key when it is not configured: %v", body["configuration"])
	}
}

func TestNotificationChannelRead_ManagedAgentChannelLeavesApiKeyEmpty(t *testing.T) {
	var body map[string]interface{}
	client := agentChannelServer(t, managedAgentChannelResponse, &body)
	d := schema.TestResourceDataRaw(t, agentChannelSchema(), map[string]interface{}{})
	d.SetId("123")

	if err := nullplatform.NotificationChannelRead(d, client); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// "" equals an omitted api_key in configuration, so the next plan is clean.
	if got := d.Get("configuration.0.agent.0.api_key"); got != "" {
		t.Errorf("got api_key %q, want empty", got)
	}
	if got := d.Get("configuration.0.agent.0.selector.environment"); got != "test" {
		t.Errorf("got selector.environment %q, want %q", got, "test")
	}
}
