package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

type DimensionValue struct {
	ID          int    `json:"id,omitempty"`
	DimensionID int    `json:"dimensionId,omitempty"`
	Name        string `json:"name"`
	NRN         string `json:"nrn"`
	Slug        string `json:"slug,omitempty"`
	Status      string `json:"status,omitempty"`
}

func (c *NullClient) CreateDimensionValue(dimensionID string, dv *DimensionValue) (*DimensionValue, error) {
	path := fmt.Sprintf("%s/%s/value", DIMENSION_PATH, dimensionID)
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(dv)
	if err != nil {
		return nil, fmt.Errorf("error encoding dimension value: %v", err)
	}

	res, err := c.MakeRequest("POST", path, &buf)
	if err != nil {
		return nil, fmt.Errorf("error making POST request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("API error creating dimension value: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return nil, fmt.Errorf("error creating dimension value, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	createdValue := &DimensionValue{}
	err = json.Unmarshal(body, createdValue)
	if err != nil {
		return nil, fmt.Errorf("error decoding created dimension value: %v", err)
	}

	return createdValue, nil
}

func (c *NullClient) GetDimensionValue(dimensionID, valueID string) (*DimensionValue, error) {
	path := fmt.Sprintf("%s/%s/value/%s", DIMENSION_PATH, dimensionID, valueID)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("error making GET request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("API error getting dimension value: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return nil, fmt.Errorf("error getting dimension value, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	value := &DimensionValue{}
	err = json.Unmarshal(body, value)
	if err != nil {
		return nil, fmt.Errorf("error decoding dimension value: %v", err)
	}

	return value, nil
}

func (c *NullClient) UpdateDimensionValue(dimensionID, valueID string, dv *DimensionValue) error {
	path := fmt.Sprintf("%s/%s/value/%s", DIMENSION_PATH, dimensionID, valueID)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(dv)
	if err != nil {
		return fmt.Errorf("error encoding dimension value: %v", err)
	}

	res, err := c.MakeRequest("PUT", path, &buf)
	if err != nil {
		return fmt.Errorf("error making PUT request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return fmt.Errorf("API error updating dimension value: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return fmt.Errorf("error updating dimension value, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	return nil
}

func (c *NullClient) DeleteDimensionValue(dimensionID, valueID string) error {
	path := fmt.Sprintf("%s/%s/value/%s", DIMENSION_PATH, dimensionID, valueID)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("error making DELETE request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return fmt.Errorf("error reading response body: %v", err)
	}

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return fmt.Errorf("API error deleting dimension value: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return fmt.Errorf("error deleting dimension value, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	return nil
}

func (c *NullClient) ListDimensionValues(dimensionID string) ([]*DimensionValue, error) {
	path := fmt.Sprintf("%s/%s/value", DIMENSION_PATH, dimensionID)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("error making GET request: %v", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response body: %v", err)
	}

	if res.StatusCode != http.StatusOK {
		var errResp ErrorResponse
		if err := json.Unmarshal(body, &errResp); err == nil {
			return nil, fmt.Errorf("API error listing dimension values: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return nil, fmt.Errorf("error listing dimension values, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	var values []*DimensionValue
	err = json.Unmarshal(body, &values)
	if err != nil {
		return nil, fmt.Errorf("error decoding dimension values: %v", err)
	}

	return values, nil
}
