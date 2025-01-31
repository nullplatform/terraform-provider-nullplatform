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
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*link)
	if err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", LINK_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		nErr := &NullErrors{}
		dErr := json.NewDecoder(res.Body).Decode(nErr)
		if res.StatusCode == http.StatusBadRequest {
			if dErr != nil {
				return nil, fmt.Errorf("an error happened: %s", dErr)
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
	path := fmt.Sprintf("%s/%s", LINK_PATH, linkId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*link)
	if err != nil {
		return err
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
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
	path := fmt.Sprintf("%s/%s", LINK_PATH, linkId)

	res, err := c.MakeRequest("DELETE", path, nil)
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
	path := fmt.Sprintf("%s/%s", LINK_PATH, linkId)

	res, err := c.MakeRequest("GET", path, nil)
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
