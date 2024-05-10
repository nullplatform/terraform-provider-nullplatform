package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const SERVICE_PATH = "/service"


type Service struct {
	Id                     string                  `json:"id,omitempty"`
	Name                   string                  `json:"name,omitempty"`
	SpecificationId        string                  `json:"specification_id,omitempty"`
	DesiredSpecificationId string                  `json:"desired_specification_id,omitempty"`
	EntityNrn              string                  `json:"entity_nrn,omitempty"`
	LinkableTo             []interface{}           `json:"linkable_to,omitempty"`
	Status                 string                  `json:"status,omitempty"`
	Slug                   string                  `json:"slug,omitempty"`
	Messages               []interface{}           `json:"messages,omitempty"`
	Selectors              map[string]interface{}  `json:"selectors,omitempty"`
	Dimensions             map[string]interface{}  `json:"dimensions,omitempty"`
	Attributes             map[string]interface{}  `json:"attributes,omitempty"`
}

func (c *NullClient) CreateService(s *Service) (*Service, error) {
	url := fmt.Sprintf("https://%s%s", c.ApiURL, SERVICE_PATH)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)

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
		return nil, fmt.Errorf("error creating service resource, got status code: %d", nErr)
	}

	sRes := &Service{}
	derr := json.NewDecoder(res.Body).Decode(sRes)

	if derr != nil {
		return nil, derr
	}

	return sRes, nil
}

func (c *NullClient) PatchService(serviceId string, s *Service) error {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, SERVICE_PATH, serviceId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*s)

	if err != nil {
		return err
	}

	r, err := http.NewRequest("PATCH", url, &buf)
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		io.Copy(os.Stdout, res.Body)
		return fmt.Errorf("error patching service resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) DeleteService(serviceId string) error {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, SERVICE_PATH, serviceId)

	r, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Authorization", fmt.Sprintf("Bearer %s", c.Token.AccessToken))

	res, err := c.Client.Do(r)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		io.Copy(os.Stdout, res.Body)
		return fmt.Errorf("error deleting service resource, got %d for %s", res.StatusCode, serviceId)
	}

	return nil
}

func (c *NullClient) GetService(serviceId string) (*Service, error) {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, SERVICE_PATH, serviceId)

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

	s := &Service{}
	derr := json.NewDecoder(res.Body).Decode(s)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		io.Copy(os.Stdout, res.Body)
		return nil, fmt.Errorf("error getting service resource, got %d for %s", res.StatusCode, serviceId)
	}

	return s, nil
}
