package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
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

func (c *NullClient) CreateDimension(d *Dimension) (*Dimension, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(d)
	if err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", DIMENSION_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error creating dimension resource, got status code: %d", res.StatusCode)
	}

	createdDimension := &Dimension{}
	err = json.NewDecoder(res.Body).Decode(createdDimension)
	if err != nil {
		return nil, err
	}

	return createdDimension, nil
}

func (c *NullClient) GetDimension(dimensionID string) (*Dimension, error) {
	path := fmt.Sprintf("%s/%s", DIMENSION_PATH, dimensionID)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting dimension resource, got status code: %d", res.StatusCode)
	}

	dimension := &Dimension{}
	err = json.NewDecoder(res.Body).Decode(dimension)
	if err != nil {
		return nil, err
	}

	return dimension, nil
}

func (c *NullClient) UpdateDimension(dimensionID string, d *Dimension) error {
	path := fmt.Sprintf("%s/%s", DIMENSION_PATH, dimensionID)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(d)
	if err != nil {
		return err
	}

	res, err := c.MakeRequest("PUT", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("error updating dimension resource, got status code: %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) DeleteDimension(dimensionID string) error {
	path := fmt.Sprintf("%s/%s", DIMENSION_PATH, dimensionID)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("error deleting dimension resource, got status code: %d", res.StatusCode)
	}

	return nil
}
