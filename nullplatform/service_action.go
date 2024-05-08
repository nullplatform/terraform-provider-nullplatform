package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const SERVICE_ACTION_PATH = "/service/%s/action"

type ActionService struct {
	ServiceId              string                   `json:"service_id,omitempty"`
	Id                     string                   `json:"id,omitempty"`
	Status                 string                   `json:"status,omitempty"`
	Name                   string                   `json:"name,omitempty"`
	SpecificationId        string                   `json:"specification_id,omitempty"`
	Parameters             map[string]interface{}   `json:"parameters,omitempty"`
	Results                map[string]interface{}   `json:"results,omitempty"`
}

func (c *NullClient) CreateServiceAction(sAction *ActionService, id string, action string) (*ActionService, error) {
	url := fmt.Sprintf("https://%s%s", c.ApiURL, fmt.Sprintf(SERVICE_PATH, id))

	sAction.Name = action + "-" + sAction.Name

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*sAction)

	if err != nil {
		return nil, err
	}

	r, err := http.NewRequest("POST", url, &buf)
	if err != nil {
		return nil, err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		nErr := &NullErrors{}
		dErr := json.NewDecoder(res.Body).Decode(nErr)
		if res.StatusCode == http.StatusBadRequest {
			if dErr != nil {
				return nil, fmt.Errorf("An error happened: %s", dErr)
			}
		}
		return nil, fmt.Errorf("error creating action service resource, got: %d", nErr)
	}

	sActionRes := &ActionService{}
	decErr := json.NewDecoder(res.Body).Decode(sActionRes)

	if decErr != nil {
		return nil, decErr
	}

	return sActionRes, nil
}


func (c *NullClient) GetServiceAction(actionId string, serviceId string) (*ActionService, error) {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, fmt.Sprintf(SERVICE_PATH, serviceId), actionId)

	r, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(r)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	action := &ActionService{}
	derr := json.NewDecoder(res.Body).Decode(action)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		io.Copy(os.Stdout, res.Body)
		return nil, fmt.Errorf("error getting action service resource, got %d for %s", res.StatusCode, actionId)
	}

	return action, nil
}
