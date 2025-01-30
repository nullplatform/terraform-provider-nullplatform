package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type ActionSpecification struct {
	Id                     string                 `json:"id,omitempty"`
	Name                   string                 `json:"name"`
	Type                   string                 `json:"type,omitempty"`
	Parameters             map[string]interface{} `json:"parameters,omitempty"`
	Results                map[string]interface{} `json:"results,omitempty"`
	ServiceSpecificationId string                 `json:"service_specification_id,omitempty"`
	Slug                   string                 `json:"slug,omitempty"`
	LinkSpecificationId    string                 `json:"link_specification_id,omitempty"`
	Retryable              bool                   `json:"retryable,omitempty"`
}

func getActionSpecificationPath(parentType, parentId string) string {
	if parentType == "service" {
		return fmt.Sprintf("/service_specification/%s/action_specification", parentId)
	}
	return fmt.Sprintf("/link_specification/%s/action_specification", parentId)
}

func (c *NullClient) CreateActionSpecification(s *ActionSpecification) (*ActionSpecification, error) {
	// Determine which parent ID to use
	var parentType, parentId string
	if s.ServiceSpecificationId != "" {
		parentType = "service"
		parentId = s.ServiceSpecificationId
	} else {
		parentType = "link"
		parentId = s.LinkSpecificationId
	}

	path := getActionSpecificationPath(parentType, parentId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)
	if err != nil {
		return nil, fmt.Errorf("failed to encode action specification: %v", err)
	}

	res, err := c.MakeRequest("POST", path, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create action specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	sRes := &ActionSpecification{}
	if err := json.NewDecoder(res.Body).Decode(sRes); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return sRes, nil
}

func (c *NullClient) GetActionSpecification(specId, parentType, parentId string) (*ActionSpecification, error) {
	basePath := getActionSpecificationPath(parentType, parentId)
	path := fmt.Sprintf("%s/%s", basePath, specId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get action specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	spec := &ActionSpecification{}
	if err := json.NewDecoder(res.Body).Decode(spec); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return spec, nil
}

func (c *NullClient) PatchActionSpecification(specId string, s *ActionSpecification, parentType, parentId string) error {
	basePath := getActionSpecificationPath(parentType, parentId)
	path := fmt.Sprintf("%s/%s", basePath, specId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)
	if err != nil {
		return fmt.Errorf("failed to encode action specification: %v", err)
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to patch action specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) DeleteActionSpecification(specId string, parentType, parentId string) error {
	basePath := getActionSpecificationPath(parentType, parentId)
	path := fmt.Sprintf("%s/%s", basePath, specId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete action specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}
