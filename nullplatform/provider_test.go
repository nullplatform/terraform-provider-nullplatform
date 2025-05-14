package nullplatform_test

import (
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
		"nullplatform_api_key",
		"nullplatform_approval_action",
		"nullplatform_approval_policy",
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
		"nullplatform_action_specification",
		"nullplatform_service_specification",
		"nullplatform_link_specification",
		"nullplatform_authz_grant",
		"nullplatform_user",
		"nullplatform_technology_template",
		"nullplatform_metadata",
		"nullplatform_scope_type",
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
		"nullplatform_scope",
		"nullplatform_service",
		"nullplatform_application",
		"nullplatform_parameter",
		"nullplatform_parameter_by_name",
		"nullplatform_dimension",
	}

	dataSources := nullplatform.Provider().DataSourcesMap

	for _, resource := range expectedDataSources {
		require.Contains(t, dataSources, resource, "An expected data source was not registered")
		require.NotNil(t, dataSources[resource], "A data source cannot have a nil schema")
	}
	require.Equal(t, len(expectedDataSources), len(dataSources), "There are an unexpected number of registered data sources")
}
