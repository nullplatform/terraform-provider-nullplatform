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


type Message struct {
	Level            string `json:"level,omitempty"`
	Message          string `json:"message"`
}

type Selector struct {
	Imported    bool     `json:"imported,omitempty"`
	Provider    string   `json:"provider,omitempty"`
	Category    string   `json:"category,omitempty"`
	SubCategory string   `json:"sub_category,omitempty"`
}

type Service struct {
	Id                     int              `json:"id,omitempty"`
	Name                   string           `json:"name,omitempty"`
	SpecificationId        int              `json:"specification_id,omitempty"`
	EntityNrn              string           `json:"entity_nrn,omitempty"`
	LinkableTo             []string         `json:"linkable_to,omitempty"`
	Status                 string           `json:"status,omitempty"`
	Slug                   string           `json:"slug,omitempty"`
	Messages              *Message          `json:"messages,omitempty"`
	Selectors             *Selector        `json:"selectors,omitempty"`
	Dimensions            map[string]string `json:"dimensions,omitempty"`
	Attributes            map[string]string `json:"attributes,omitempty"`
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
		if res.StatusCode == http.StatusBadRequest {
			nErr := &NullErrors{}
			dErr := json.NewDecoder(res.Body).Decode(nErr)
			if dErr != nil {
				return nil, fmt.Errorf("An error happened: %s", dErr)
			}
			
		}
		return nil, fmt.Errorf("error creating service resource, got status code: %d", res.StatusCode)
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
