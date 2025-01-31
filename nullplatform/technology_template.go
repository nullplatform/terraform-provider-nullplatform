package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	TECHNOLOGY_TEMPLATE_PATH = "/template"
)

type TechnologyTemplate struct {
	Id           json.Number              `json:"id,omitempty"`
	Name         string                   `json:"name"`
	Status       string                   `json:"status,omitempty"`
	Organization json.Number              `json:"organization,omitempty"`
	Account      json.Number              `json:"account,omitempty"`
	URL          string                   `json:"url"`
	Provider     map[string]interface{}   `json:"provider"`
	Components   []map[string]interface{} `json:"components"`
	Tags         []string                 `json:"tags,omitempty"`
	Metadata     map[string]interface{}   `json:"metadata,omitempty"`
	Rules        map[string]interface{}   `json:"rules,omitempty"`
	CreatedAt    string                   `json:"created_at,omitempty"`
	UpdatedAt    string                   `json:"updated_at,omitempty"`
}

func (t *TechnologyTemplate) GetId() string {
	if t.Id == "" {
		return ""
	}
	return t.Id.String()
}

func (t *TechnologyTemplate) GetOrganization() string {
	if t.Organization == "" {
		return ""
	}
	return t.Organization.String()
}

func (t *TechnologyTemplate) GetAccount() string {
	if t.Account == "" {
		return ""
	}
	return t.Account.String()
}

func (c *NullClient) CreateTechnologyTemplate(t *TechnologyTemplate) (*TechnologyTemplate, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*t)
	if err != nil {
		return nil, fmt.Errorf("failed to encode technology template: %v", err)
	}

	res, err := c.MakeRequest("POST", TECHNOLOGY_TEMPLATE_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create technology template: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	tRes := &TechnologyTemplate{}
	if err := json.NewDecoder(res.Body).Decode(tRes); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return tRes, nil
}

func (c *NullClient) GetTechnologyTemplate(templateId string) (*TechnologyTemplate, error) {
	path := fmt.Sprintf("%s/%s", TECHNOLOGY_TEMPLATE_PATH, templateId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get technology template: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	template := &TechnologyTemplate{}
	if err := json.NewDecoder(res.Body).Decode(template); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return template, nil
}

func (c *NullClient) PatchTechnologyTemplate(templateId string, t *TechnologyTemplate) error {
	path := fmt.Sprintf("%s/%s", TECHNOLOGY_TEMPLATE_PATH, templateId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*t)
	if err != nil {
		return fmt.Errorf("failed to encode technology template: %v", err)
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to patch technology template: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}
