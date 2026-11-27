package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

const APPLICATION_TYPE_PATH = "/application_type"

// ApplicationType is the body of a create, the partial body of a patch and
// the answer to both and to a read. Every field is omitempty so a patch names
// only what changed.
//
// Schema is raw JSON on purpose: the order of a schema's properties is the
// order of the form the platform renders, and decoding it into a Go map would
// send it back sorted. Description is a pointer so a patch can clear it with
// an empty string, which omitempty would otherwise drop.
type ApplicationType struct {
	Id             string          `json:"id,omitempty"`
	Nrn            string          `json:"nrn,omitempty"`
	Name           string          `json:"name,omitempty"`
	Slug           string          `json:"slug,omitempty"`
	Description    *string         `json:"description,omitempty"`
	Status         string          `json:"status,omitempty"`
	VisibleTo      []string        `json:"visible_to,omitempty"`
	LastSnapshotId string          `json:"last_snapshot_id,omitempty"`
	Schema         json.RawMessage `json:"schema,omitempty"`
}

// sendApplicationType sends body, when there is one, and decodes the type the
// API answers. A refusal carries the API's whole response body: its message is
// the only thing naming the rule that refused.
func (c *NullClient) sendApplicationType(method, path, doing string, body *ApplicationType) (*ApplicationType, error) {
	var buf *bytes.Buffer
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to encode application type: %w", err)
		}
		buf = bytes.NewBuffer(encoded)
	}

	res, err := c.MakeRequest(method, path, buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %w", err)
	}
	defer res.Body.Close()

	answer, _ := io.ReadAll(res.Body)
	if method == http.MethodGet && applicationTypeWasDeleted(res.StatusCode, answer) {
		return nil, &ResourceNotFoundError{ApiType: "application type", Key: path[strings.LastIndex(path, "/")+1:], Message: string(answer)}
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to %s application type: status code %d, response: %s", doing, res.StatusCode, string(answer))
	}

	applicationType := &ApplicationType{}
	if err := json.Unmarshal(answer, applicationType); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %w", err)
	}
	return applicationType, nil
}

func (c *NullClient) CreateApplicationType(applicationType *ApplicationType) (*ApplicationType, error) {
	return c.sendApplicationType("POST", APPLICATION_TYPE_PATH, "create", applicationType)
}

func (c *NullClient) GetApplicationType(applicationTypeId string) (*ApplicationType, error) {
	return c.sendApplicationType("GET", fmt.Sprintf("%s/%s", APPLICATION_TYPE_PATH, applicationTypeId), "get", nil)
}

// applicationTypeWasDeleted tells the API's answer for a deleted type, which
// only a caller allowed on the type gets, from any other 404: a route an older
// API or a gateway does not have says nothing about the type. (An id the API
// never had answers 403.)
func applicationTypeWasDeleted(status int, answer []byte) bool {
	var refusal struct {
		Code string `json:"code"`
	}
	_ = json.Unmarshal(answer, &refusal)
	return status == http.StatusNotFound && refusal.Code == "APPLICATION_TYPE.NOT_FOUND"
}

// DeleteApplicationType deletes the type. One already deleted is done; any
// other refusal carries the API's whole response body.
func (c *NullClient) DeleteApplicationType(applicationTypeId string) error {
	res, err := c.MakeRequest("DELETE", fmt.Sprintf("%s/%s", APPLICATION_TYPE_PATH, applicationTypeId), nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %w", err)
	}
	defer res.Body.Close()

	answer, _ := io.ReadAll(res.Body)
	if res.StatusCode == http.StatusNoContent || applicationTypeWasDeleted(res.StatusCode, answer) {
		return nil
	}
	return fmt.Errorf("failed to delete application type: status code %d, response: %s", res.StatusCode, string(answer))
}

func (c *NullClient) PatchApplicationType(applicationTypeId string, applicationType *ApplicationType) (*ApplicationType, error) {
	return c.sendApplicationType("PATCH", fmt.Sprintf("%s/%s", APPLICATION_TYPE_PATH, applicationTypeId), "update", applicationType)
}

// suppressSameID is no diff between two spellings of one id: the platform
// reads the ids of application types and of their snapshots in lower case.
func suppressSameID(_, old, new string, _ *schema.ResourceData) bool {
	return strings.EqualFold(old, new)
}
