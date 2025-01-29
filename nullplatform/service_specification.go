package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	SERVICE_SPECIFICATION_PATH = "/service_specification"
)

type ServiceSpecification struct {
	Id           string                 `json:"id,omitempty"`
	Name         string                 `json:"name"`
	VisibleTo    []string               `json:"visible_to"`
	Dimensions   map[string]interface{} `json:"dimensions,omitempty"`
	AssignableTo string                 `json:"assignable_to,omitempty"`
	Type         string                 `json:"type,omitempty"`
	Attributes   map[string]interface{} `json:"attributes,omitempty"`
	Selectors    map[string]interface{} `json:"selectors,omitempty"`
}

func (c *NullClient) CreateServiceSpecification(s *ServiceSpecification) (*ServiceSpecification, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)
	if err != nil {
		return nil, fmt.Errorf("failed to encode service specification: %v", err)
	}

	res, err := c.MakeRequest("POST", SERVICE_SPECIFICATION_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create service specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	sRes := &ServiceSpecification{}
	if err := json.NewDecoder(res.Body).Decode(sRes); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return sRes, nil
}

func (c *NullClient) GetServiceSpecification(specId string) (*ServiceSpecification, error) {
	path := fmt.Sprintf("%s/%s", SERVICE_SPECIFICATION_PATH, specId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get service specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	spec := &ServiceSpecification{}
	if err := json.NewDecoder(res.Body).Decode(spec); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return spec, nil
}

func (c *NullClient) PatchServiceSpecification(specId string, s *ServiceSpecification) error {
	path := fmt.Sprintf("%s/%s", SERVICE_SPECIFICATION_PATH, specId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)
	if err != nil {
		return fmt.Errorf("failed to encode service specification: %v", err)
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to patch service specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) DeleteServiceSpecification(specId string) error {
	path := fmt.Sprintf("%s/%s", SERVICE_SPECIFICATION_PATH, specId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete service specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}
