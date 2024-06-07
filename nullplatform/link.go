package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
)

const LINK_PATH = "/link"

type Link struct {
	Id                     string                 `json:"id,omitempty"`
	Slug                   string                 `json:"slug,omitempty"`
	Name                   string                 `json:"name,omitempty"`
	ServiceId              string                 `json:"service_id,omitempty"`
	SpecificationId        string                 `json:"specification_id,omitempty"`
	DesiredSpecificationId string                 `json:"desired_specification_id,omitempty"`
	EntityNrn              string                 `json:"entity_nrn,omitempty"`
	LinkableTo             []interface{}          `json:"linkable_to,omitempty"`
	Status                 string                 `json:"status,omitempty"`
	Selectors              map[string]interface{} `json:"selectors,omitempty"`
	Dimensions             map[string]interface{} `json:"dimensions,omitempty"`
	Attributes             map[string]interface{} `json:"attributes,omitempty"`
}

func (c *NullClient) CreateLink(link *Link) (*Link, error) {
	url := fmt.Sprintf("https://%s%s", c.ApiURL, LINK_PATH)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*link)

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
		return nil, fmt.Errorf("error creating link resource, got status code: %d", nErr.Id)
	}

	linkRes := &Link{}
	derr := json.NewDecoder(res.Body).Decode(linkRes)

	if derr != nil {
		return nil, derr
	}

	return linkRes, nil
}

func (c *NullClient) PatchLink(linkId string, link *Link) error {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, LINK_PATH, linkId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*link)

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
		return fmt.Errorf("error patching link resource, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) DeleteLink(linkId string) error {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, LINK_PATH, linkId)

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
		return fmt.Errorf("error deleting link resource, got %d for %s", res.StatusCode, linkId)
	}

	return nil
}

func (c *NullClient) GetLink(linkId string) (*Link, error) {
	url := fmt.Sprintf("https://%s%s/%s", c.ApiURL, LINK_PATH, linkId)

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

	link := &Link{}
	derr := json.NewDecoder(res.Body).Decode(link)

	if derr != nil {
		return nil, derr
	}

	if res.StatusCode != http.StatusOK {
		io.Copy(os.Stdout, res.Body)
		return nil, fmt.Errorf("error getting link resource, got %d for %s", res.StatusCode, linkId)
	}

	return link, nil
}
