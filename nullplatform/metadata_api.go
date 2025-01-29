package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type Metadata struct {
	Value interface{} `json:"value"`
}

func getMetadataPath(entity, entityId, metadataType string) string {
	return fmt.Sprintf("/%s/%s/%s", entity, entityId, metadataType)
}

func (c *NullClient) CreateMetadata(entity, entityId, metadataType string, m *Metadata) error {
	path := getMetadataPath(entity, entityId, metadataType)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(m.Value)
	if err != nil {
		return fmt.Errorf("failed to encode metadata: %v", err)
	}

	res, err := c.MakeRequest("POST", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to create metadata: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) GetMetadata(entity, entityId, metadataType string) (*Metadata, error) {
	path := getMetadataPath(entity, entityId, metadataType)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get metadata: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	var value interface{}
	if err := json.NewDecoder(res.Body).Decode(&value); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return &Metadata{Value: value}, nil
}

func (c *NullClient) UpdateMetadata(entity, entityId, metadataType string, m *Metadata) error {
	path := getMetadataPath(entity, entityId, metadataType)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(m.Value)
	if err != nil {
		return fmt.Errorf("failed to encode metadata: %v", err)
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to update metadata: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) DeleteMetadata(entity, entityId, metadataType string) error {
	path := getMetadataPath(entity, entityId, metadataType)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete metadata: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}
