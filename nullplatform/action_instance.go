package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const ACTION_INSTANCE_PATH = "/service/%s/action"
const ACTION_INSTANCE_ITEM_PATH = "/service/%s/action/%s"

type ActionInstance struct {
	Id              string                 `json:"id,omitempty"`
	Status          string                 `json:"status,omitempty"`
	SpecificationId string                 `json:"specification_id,omitempty"`
	ServiceId       string                 `json:"service_id,omitempty"`
	Parameters      map[string]interface{} `json:"parameters,omitempty"`
	Results         map[string]interface{} `json:"results,omitempty"`
	Messages        []interface{}          `json:"messages,omitempty"`
}

func (c *NullClient) CreateServiceAction(serviceID string, a *ActionInstance) (*ActionInstance, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(*a); err != nil {
		return nil, err
	}
	path := fmt.Sprintf(ACTION_INSTANCE_PATH, serviceID)

	res, err := c.MakeRequest("POST", path, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("error creating service action: status=%d body=%s", res.StatusCode, string(bodyBytes))
	}

	out := &ActionInstance{}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *NullClient) GetServiceAction(serviceID, actionID string) (*ActionInstance, error) {
	path := fmt.Sprintf(ACTION_INSTANCE_ITEM_PATH, serviceID, actionID)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("error getting service action: status=%d body=%s", res.StatusCode, string(bodyBytes))
	}

	out := &ActionInstance{}
	if err := json.NewDecoder(res.Body).Decode(out); err != nil {
		return nil, err
	}
	return out, nil
}
