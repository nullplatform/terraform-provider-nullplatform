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
		"nullplatform_scope",
		"nullplatform_service",
		"nullplatform_link",
		"nullplatform_parameter",
		"nullplatform_parameter_value",
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
	}

	dataSources := nullplatform.Provider().DataSourcesMap

	for _, resource := range expectedDataSources {
		require.Contains(t, dataSources, resource, "An expected data source was not registered")
		require.NotNil(t, dataSources[resource], "A data source cannot have a nil schema")
	}
	require.Equal(t, len(expectedDataSources), len(dataSources), "There are an unexpected number of registered data sources")
}
