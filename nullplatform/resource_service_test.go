package nullplatform

import (
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/stretchr/testify/assert"
)

type MockNullOps struct{}

func (m *MockNullOps) CreateService(service *Service) (*Service, error) {
	return &Service{Id: "123"}, nil
}

func (m *MockNullOps) GetService(serviceID string) (*Service, error) {
	return &Service{
		Id:                     "123",
		Name:                   "Test Service",
		SpecificationId:        "spec123",
		DesiredSpecificationId: "desiredSpec123",
		EntityNrn:              "entity123",
		Status:                 "active",
		LinkableTo:             []interface{}{"link1", "link2"},
		Dimensions:             map[string]interface{}{"key1": "value1", "key2": "value2"},
		Messages:               []interface{}{},
		Selectors:              map[string]interface{}{"selector1": "value1", "selector2": "value2"},
		Attributes:             map[string]interface{}{"attr1": "value1", "attr2": "value2"},
	}, nil
}

func TestServiceCreate(t *testing.T) {
	mockNullOps := &MockNullOps{}
	d := &schema.ResourceData{}
	d.Set("name", "Test Service")
	d.Set("specification_id", "spec123")
	d.Set("entity_nrn", "entity123")

	err := ServiceCreate(d, mockNullOps)

	assert.NoError(t, err)
	assert.Equal(t, "123", d.Id())
}

func TestServiceRead(t *testing.T) {
	mockNullOps := &MockNullOps{}
	d := &schema.ResourceData{Id: "123"}

	err := ServiceRead(d, mockNullOps)

	assert.NoError(t, err)
	assert.Equal(t, "Test Service", d.Get("name"))
	assert.Equal(t, "spec123", d.Get("specification_id"))
	assert.Equal(t, "desiredSpec123", d.Get("desired_specification_id"))
	assert.Equal(t, "entity123", d.Get("entity_nrn"))
	assert.Equal(t, []interface{}{"link1", "link2"}, d.Get("linkable_to"))
	assert.Equal(t, "active", d.Get("status"))
	assert.Equal(t, map[string]interface{}{"key1": "value1", "key2": "value2"}, d.Get("dimensions"))
	assert.Equal(t, []interface{}{}, d.Get("messages"))
	assert.Equal(t, map[string]interface{}{"selector1": "value1", "selector2": "value2"}, d.Get("selectors"))
	assert.Equal(t, map[string]interface{}{"attr1": "value1", "attr2": "value2"}, d.Get("attributes"))
}
