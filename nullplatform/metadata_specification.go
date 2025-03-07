package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	METADATA_SPECIFICATION_PATH = "/metadata/metadata_specification"
)

type MetadataSpecification struct {
	Id          string                 `json:"id,omitempty"`
	Name        string                 `json:"name,omitempty"`
	Description string                 `json:"description,omitempty"`
	Nrn         string                 `json:"nrn,omitempty"`
	Entity      string                 `json:"entity,omitempty"`
	Metadata    string                 `json:"metadata,omitempty"`
	Schema      map[string]interface{} `json:"schema,omitempty"`
}

func (c *NullClient) CreateMetadataSpecification(m *MetadataSpecification) (*MetadataSpecification, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*m)
	if err != nil {
		return nil, fmt.Errorf("failed to encode metadata specification: %v", err)
	}

	res, err := c.MakeRequest("POST", METADATA_SPECIFICATION_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create metadata specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	mRes := &MetadataSpecification{}
	if err := json.NewDecoder(res.Body).Decode(mRes); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return mRes, nil
}

func (c *NullClient) UpdateMetadataSpecification(id string, m *MetadataSpecification) (*MetadataSpecification, error) {
	path := fmt.Sprintf("%s/%s", METADATA_SPECIFICATION_PATH, id)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*m)
	if err != nil {
		return nil, fmt.Errorf("failed to encode metadata specification: %v", err)
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to update metadata specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	mRes := &MetadataSpecification{}
	if err := json.NewDecoder(res.Body).Decode(mRes); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return mRes, nil
}

func (c *NullClient) GetMetadataSpecification(id string) (*MetadataSpecification, error) {
	path := fmt.Sprintf("%s/%s", METADATA_SPECIFICATION_PATH, id)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get metadata specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	m := &MetadataSpecification{}
	if err := json.NewDecoder(res.Body).Decode(m); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return m, nil
}

func (c *NullClient) DeleteMetadataSpecification(id string) error {
	path := fmt.Sprintf("%s/%s", METADATA_SPECIFICATION_PATH, id)

	emptyBody := bytes.NewBuffer([]byte("{}"))

	res, err := c.MakeRequest("DELETE", path, emptyBody)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete metadata specification: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}
