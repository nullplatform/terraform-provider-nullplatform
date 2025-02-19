package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	LINK_SPECIFICATION_PATH = "/link_specification"
)

type LinkSpecification struct {
	Id              string                 `json:"id,omitempty"`
	Name            string                 `json:"name,omitempty"`
	Slug            string                 `json:"slug,omitempty"`
	Unique          bool                   `json:"unique"`
	SpecificationId string                 `json:"specification_id,omitempty"`
	VisibleTo       []string               `json:"visible_to"`
	Dimensions      map[string]interface{} `json:"dimensions,omitempty"`
	AssignableTo    string                 `json:"assignable_to"`
	Attributes      map[string]interface{} `json:"attributes,omitempty"`
	Selectors       Selectors              `json:"selectors,omitempty"`
}

func (c *NullClient) CreateLinkSpecification(s *LinkSpecification) (*LinkSpecification, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)
	if err != nil {
		return nil, fmt.Errorf("failed to encode link specification: %v", err)
	}

	res, err := c.MakeRequest("POST", LINK_SPECIFICATION_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create link specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	sRes := &LinkSpecification{}
	if err := json.NewDecoder(res.Body).Decode(sRes); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return sRes, nil
}

func (c *NullClient) GetLinkSpecification(specId string) (*LinkSpecification, error) {
	path := fmt.Sprintf("%s/%s", LINK_SPECIFICATION_PATH, specId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get link specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	spec := &LinkSpecification{}
	if err := json.NewDecoder(res.Body).Decode(spec); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return spec, nil
}

func (c *NullClient) PatchLinkSpecification(specId string, s *LinkSpecification) error {
	path := fmt.Sprintf("%s/%s", LINK_SPECIFICATION_PATH, specId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)
	if err != nil {
		return fmt.Errorf("failed to encode link specification: %v", err)
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to patch link specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) DeleteLinkSpecification(specId string) error {
	path := fmt.Sprintf("%s/%s", LINK_SPECIFICATION_PATH, specId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete link specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}
