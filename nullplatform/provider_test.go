package nullplatform_test

import (
	"context"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
	"github.com/stretchr/testify/require"

	"github.com/nullplatform/terraform-provider-nullplatform/nullplatform"
)

var testAccProviders map[string]*schema.Provider

func provider() *schema.Provider {
	return nullplatform.Provider()
}

func init() {
	testAccProviders = map[string]*schema.Provider{
		"nullplatform": provider(),
	}
}

func testAccPreCheck(t *testing.T) {
	if v := os.Getenv("NULLPLATFORM_APPLICATION_ID"); v == "" {
		t.Fatal("NULLPLATFORM_APPLICATION_ID must be set for acceptance tests")
	}
}

func TestProvider_HasChildResources(t *testing.T) {
	expectedResources := []string{
		"nullplatform_account",
		"nullplatform_application",
		"nullplatform_api_key",
		"nullplatform_approval_action",
		"nullplatform_approval_policy",
		"nullplatform_approval_action_policy_association",
		"nullplatform_capability",
		"nullplatform_deployment_strategy",
		"nullplatform_scope_domain",
		"nullplatform_dimension",
		"nullplatform_dimension_value",
		"nullplatform_link",
		"nullplatform_metadata_specification",
		"nullplatform_namespace",
		"nullplatform_notification_channel",
		"nullplatform_parameter",
		"nullplatform_parameter_value",
		"nullplatform_provider_config",
		"nullplatform_runtime_configuration",
		"nullplatform_scope",
		"nullplatform_service",
		"nullplatform_service_action",
		"nullplatform_action_specification",
		"nullplatform_service_specification",
		"nullplatform_link_specification",
		"nullplatform_authz_grant",
		"nullplatform_user",
		"nullplatform_technology_template",
		"nullplatform_metadata",
		"nullplatform_scope_type",
		"nullplatform_entity_hook_action",
		"nullplatform_provider_specification",
		"nullplatform_artifact",
		"nullplatform_package",
	}

	resources := nullplatform.Provider().ResourcesMap

	for _, resource := range expectedResources {
		require.Contains(t, resources, resource, "An expected resource was not registered")
		require.NotNil(t, resources[resource], "A resource cannot have a nil schema")
	}
	require.Equal(t, len(expectedResources), len(resources), "There are an unexpected number of registered resources")
}

func TestProvider_HasChildDataSources(t *testing.T) {
	expectedDataSources := []string{
		"nullplatform_account",
		"nullplatform_namespace",
		"nullplatform_scope",
		"nullplatform_service",
		"nullplatform_application",
		"nullplatform_parameter",
		"nullplatform_parameter_by_name",
		"nullplatform_dimension",
		"nullplatform_service_specification",
		"nullplatform_scope_type",
		"nullplatform_action_specification",
		"nullplatform_action_specifications",
		"nullplatform_artifact",
		"nullplatform_package",
	}

	dataSources := nullplatform.Provider().DataSourcesMap

	for _, resource := range expectedDataSources {
		require.Contains(t, dataSources, resource, "An expected data source was not registered")
		require.NotNil(t, dataSources[resource], "A data source cannot have a nil schema")
	}
	require.Equal(t, len(expectedDataSources), len(dataSources), "There are an unexpected number of registered data sources")
}

// A deprecated attribute backed by a DefaultFunc is reported by the SDK as configured
// as soon as the environment variable exists, so every plan warned about 'np_apikey'
// even for configurations that only set 'api_key'.
func TestProvider_LegacyEnvVarsDoNotDeprecateCurrentConfig(t *testing.T) {
	t.Setenv("NP_API_KEY", "legacy-env-key")
	t.Setenv("NP_API_HOST", "legacy.nullplatform.com")

	diags := provider().Validate(terraform.NewResourceConfigRaw(map[string]any{
		nullplatform.API_KEY: "current-key",
	}))

	require.Empty(t, diags, "a configuration using only current attributes must not raise diagnostics")
}

func TestProvider_DeprecatedAttributesInConfigStillWarn(t *testing.T) {
	for _, attribute := range []string{nullplatform.NP_API_KEY, nullplatform.NP_API_HOST} {
		t.Run(attribute, func(t *testing.T) {
			diags := provider().Validate(terraform.NewResourceConfigRaw(map[string]any{
				attribute: "some-value",
			}))

			require.Len(t, diags, 1)
			require.Equal(t, diag.Warning, diags[0].Severity)
			require.Equal(t, "Argument is deprecated", diags[0].Summary)
		})
	}
}

func TestProvider_ConfigureResolvesAPIKeyAndHost(t *testing.T) {
	envNames := []string{"NULLPLATFORM_API_KEY", "NULLPLATFORM_HOST", "NP_API_KEY", "NP_API_HOST"}

	cases := []struct {
		name         string
		config       map[string]any
		env          map[string]string
		wantAPIKey   string
		wantHost     string
		wantWarnings []string
	}{
		{
			name:       "current attribute wins over the legacy environment variable",
			config:     map[string]any{nullplatform.API_KEY: "config-key"},
			env:        map[string]string{"NP_API_KEY": "legacy-env-key"},
			wantAPIKey: "config-key",
			wantHost:   nullplatform.DEFAULT_HOST,
		},
		{
			name:       "current environment variables",
			config:     map[string]any{},
			env:        map[string]string{"NULLPLATFORM_API_KEY": "env-key", "NULLPLATFORM_HOST": "custom.nullplatform.com"},
			wantAPIKey: "env-key",
			wantHost:   "custom.nullplatform.com",
		},
		{
			name:         "legacy environment variables are honored with a warning",
			config:       map[string]any{},
			env:          map[string]string{"NP_API_KEY": "legacy-env-key", "NP_API_HOST": "legacy.nullplatform.com"},
			wantAPIKey:   "legacy-env-key",
			wantHost:     "legacy.nullplatform.com",
			wantWarnings: []string{"Deprecated API Key Environment Variable", "Deprecated Host Environment Variable"},
		},
		{
			name:         "legacy attributes are honored with a warning",
			config:       map[string]any{nullplatform.NP_API_KEY: "legacy-key", nullplatform.NP_API_HOST: "legacy-attr.nullplatform.com"},
			wantAPIKey:   "legacy-key",
			wantHost:     "legacy-attr.nullplatform.com",
			wantWarnings: []string{"Deprecated API Key Usage", "Deprecated Host Usage"},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Clear every variable first: the test host may export the legacy ones.
			for _, name := range envNames {
				t.Setenv(name, tc.env[name])
			}

			p := provider()
			diags := p.Configure(context.Background(), terraform.NewResourceConfigRaw(tc.config))
			require.False(t, diags.HasError(), "configure must not fail: %v", diags)

			client, ok := p.Meta().(*nullplatform.NullClient)
			require.True(t, ok, "provider meta must be a *NullClient")
			require.Equal(t, tc.wantAPIKey, client.ApiKey)
			require.Equal(t, tc.wantHost, client.ApiURL)

			summaries := make([]string, 0, len(diags))
			for _, d := range diags {
				summaries = append(summaries, d.Summary)
			}
			require.ElementsMatch(t, tc.wantWarnings, summaries)
		})
	}
}

func TestProvider_ConfigureFailsWithoutAPIKey(t *testing.T) {
	for _, name := range []string{"NULLPLATFORM_API_KEY", "NP_API_KEY"} {
		t.Setenv(name, "")
	}

	diags := provider().Configure(context.Background(), terraform.NewResourceConfigRaw(map[string]any{}))

	require.True(t, diags.HasError())
	require.Equal(t, "Missing API Key", diags[0].Summary)
}
