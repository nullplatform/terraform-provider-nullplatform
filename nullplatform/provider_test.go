package nullplatform

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestProvider_HasChildResources(t *testing.T) {
	expectedResources := []string{
		"nullplatform_scope",
	}

	resources := Provider().ResourcesMap

	for _, resource := range expectedResources {
		require.Contains(t, resources, resource, "An expected resource was not registered")
		require.NotNil(t, resources[resource], "A resource cannot have a nil schema")
	}
	require.Equal(t, len(expectedResources), len(resources), "There are an unexpected number of registered resources")
}

func TestProvider_HasChildDataSources(t *testing.T) {
	expectedDataSources := []string{
		"nullplatform_scope",
	}

	dataSources := Provider().DataSourcesMap

	for _, resource := range expectedDataSources {
		require.Contains(t, dataSources, resource, "An expected data source was not registered")
		require.NotNil(t, dataSources[resource], "A data source cannot have a nil schema")
	}
	require.Equal(t, len(expectedDataSources), len(dataSources), "There are an unexpected number of registered data sources")
}
