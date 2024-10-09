package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const DIMENSION_PATH = "/runtime_configuration/dimension"

type Dimension struct {
	ID     int               `json:"id,omitempty"`
	Name   string            `json:"name"`
	NRN    string            `json:"nrn"`
	Slug   string            `json:"slug,omitempty"`
	Status string            `json:"status,omitempty"`
	Order  int               `json:"order,omitempty"`
	Values map[string]string `json:"values,omitempty"`
}

type ErrorResponse struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

func (c *NullClient) CreateDimension(d *Dimension) (*Dimension, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(d)
	if err != nil {
		return nil, fmt.Errorf("error encoding dimension: %v", err)
	}

	res, err := c.MakeRequest("POST", DIMENSION_PATH, &buf)
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
			return nil, fmt.Errorf("API error creating dimension: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return nil, fmt.Errorf("error creating dimension resource, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	createdDimension := &Dimension{}
	err = json.Unmarshal(body, createdDimension)
	if err != nil {
		return nil, fmt.Errorf("error decoding created dimension: %v", err)
	}

	return createdDimension, nil
}

func (c *NullClient) GetDimension(dimensionID string) (*Dimension, error) {
	path := fmt.Sprintf("%s/%s", DIMENSION_PATH, dimensionID)

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
			return nil, fmt.Errorf("API error getting dimension: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return nil, fmt.Errorf("error getting dimension resource, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	dimension := &Dimension{}
	err = json.Unmarshal(body, dimension)
	if err != nil {
		return nil, fmt.Errorf("error decoding dimension: %v", err)
	}

	return dimension, nil
}

func (c *NullClient) UpdateDimension(dimensionID string, d *Dimension) error {
	path := fmt.Sprintf("%s/%s", DIMENSION_PATH, dimensionID)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(d)
	if err != nil {
		return fmt.Errorf("error encoding dimension: %v", err)
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
			return fmt.Errorf("API error updating dimension: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return fmt.Errorf("error updating dimension resource, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	return nil
}

func (c *NullClient) DeleteDimension(dimensionID string) error {
	path := fmt.Sprintf("%s/%s", DIMENSION_PATH, dimensionID)

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
			return fmt.Errorf("API error deleting dimension: %s (Code: %s)", errResp.Message, errResp.Code)
		}
		return fmt.Errorf("error deleting dimension resource, got status code: %d, body: %s", res.StatusCode, string(body))
	}

	return nil
}
