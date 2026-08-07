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
	ArchivedAt             string                 `json:"archived_at,omitempty"`
	ActionsInProgress      []ActionInProgress     `json:"actions_in_progress,omitempty"`
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
		// Keep the response body in the error: the refusals a link create can
		// answer with — the duplicate-link block naming an ARCHIVED twin and how
		// to resolve it among them — arrive as a 400 whose message is the only
		// explanation the operator gets. (The previous shape decoded into
		// NullErrors and printed its `Id` as a "status code", so the message
		// never surfaced at all.)
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("error creating link resource, got %d: %s", res.StatusCode, string(bodyBytes))
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
		// The link twin of PatchService's error shape: archive and restore
		// refusals ("cannot carry attributes", "use the 'archive' action
		// instead", "must be active to unarchive its links") all arrive as a 400
		// whose message is the only thing naming the guard, and Terraform shows
		// the operator the error, not the provider's stdout.
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("error patching link resource, got %d: %s", res.StatusCode, string(bodyBytes))
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

	// The API answers a link delete with 204 No Content (200 kept for
	// tolerance); treating 204 as failure errored every hard delete AFTER the
	// row was already gone. Same accepted set as DeleteService.
	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("error deleting link resource, got %d for %s: %s", res.StatusCode, linkId, string(bodyBytes))
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
