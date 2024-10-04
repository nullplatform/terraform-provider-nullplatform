package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
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
		return nil, err
	}

	res, err := c.MakeRequest("POST", path, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error creating dimension value, got status code: %d", res.StatusCode)
	}

	createdValue := &DimensionValue{}
	err = json.NewDecoder(res.Body).Decode(createdValue)
	if err != nil {
		return nil, err
	}

	return createdValue, nil
}

func (c *NullClient) GetDimensionValue(dimensionID, valueID string) (*DimensionValue, error) {
	path := fmt.Sprintf("%s/%s/value/%s", DIMENSION_PATH, dimensionID, valueID)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting dimension value, got status code: %d", res.StatusCode)
	}

	value := &DimensionValue{}
	err = json.NewDecoder(res.Body).Decode(value)
	if err != nil {
		return nil, err
	}

	return value, nil
}

func (c *NullClient) UpdateDimensionValue(dimensionID, valueID string, dv *DimensionValue) error {
	path := fmt.Sprintf("%s/%s/value/%s", DIMENSION_PATH, dimensionID, valueID)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(dv)
	if err != nil {
		return err
	}

	res, err := c.MakeRequest("PUT", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("error updating dimension value, got status code: %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) DeleteDimensionValue(dimensionID, valueID string) error {
	path := fmt.Sprintf("%s/%s/value/%s", DIMENSION_PATH, dimensionID, valueID)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("error deleting dimension value, got status code: %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) ListDimensionValues(dimensionID string) ([]*DimensionValue, error) {
	path := fmt.Sprintf("%s/%s/value", DIMENSION_PATH, dimensionID)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error listing dimension values, got status code: %d", res.StatusCode)
	}

	var values []*DimensionValue
	err = json.NewDecoder(res.Body).Decode(&values)
	if err != nil {
		return nil, err
	}

	return values, nil
}
