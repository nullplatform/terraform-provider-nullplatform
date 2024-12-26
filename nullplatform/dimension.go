package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
)

const DIMENSION_PATH = "/runtime_configuration/dimension"

type Dimension struct {
	ID     int                      `json:"id,omitempty"`
	Name   string                   `json:"name"`
	NRN    string                   `json:"nrn"`
	Slug   string                   `json:"slug,omitempty"`
	Status string                   `json:"status,omitempty"`
	Order  int                      `json:"order,omitempty"`
	Values []map[string]interface{} `json:"values,omitempty"`
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

func (c *NullClient) GetDimension(ID *string, name *string, slug *string, status *string, nrn *string) (*Dimension, error) {
	params := map[string]string{}

	if ID != nil && *ID != "" {
		if id, err := strconv.Atoi(*ID); err == nil && id > 0 {
			params["id"] = *ID
		}
	}

	if nrn != nil && *nrn != "" {
		params["nrn"] = *nrn
	}

	if name != nil && *name != "" {
		params["name"] = *name
	}

	if slug != nil && *slug != "" {
		params["slug"] = *slug
	}

	if status != nil && *status != "" {
		params["status"] = *status
	}

	queryString := c.PrepareQueryString(params)
	path := fmt.Sprintf("%s%s", DIMENSION_PATH, queryString)

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

	response := &map[string]interface{}{}
	err = json.Unmarshal(body, response)
	if err != nil {
		return nil, fmt.Errorf("error decoding dimension: %v", err)
	}

	results, ok := (*response)["results"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("the data has returnned no occurence")
	}

	// Check if "results" has exactly one element
	if len(results) != 1 {
		return nil, fmt.Errorf("result expected returned %d elements", len(results))
	}

	dimension := &Dimension{}
	rawDimension := results[0].(map[string]interface{})
	dimension.ID = int(rawDimension["id"].(float64))
	dimension.Name = rawDimension["name"].(string)
	dimension.Status = rawDimension["status"].(string)
	dimension.NRN = rawDimension["nrn"].(string)
	dimension.Order = int(rawDimension["order"].(float64))
	dimension.Slug = rawDimension["slug"].(string)
	values := rawDimension["values"].([]interface{})
	dimension.Values = make([]map[string]interface{}, len(values))
	for i, v := range values {
		dimension.Values[i] = v.(map[string]interface{})
	}
	if err != nil {
		return nil, fmt.Errorf("error asserting result to map %v", results[0])
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
