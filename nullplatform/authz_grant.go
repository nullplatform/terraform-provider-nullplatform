package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	AUTHZ_GRANT_PATH = "/authz/grants"
)

type AuthzGrant struct {
	ID       int    `json:"id,omitempty"`
	UserID   int    `json:"user_id"`
	RoleSlug string `json:"role_slug"`
	NRN      string `json:"nrn"`
}

func (c *NullClient) CreateAuthzGrant(g *AuthzGrant) (*AuthzGrant, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(g)
	if err != nil {
		return nil, fmt.Errorf("failed to encode authz grant: %v", err)
	}

	res, err := c.MakeRequest("POST", AUTHZ_GRANT_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create authz grant: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	grant := &AuthzGrant{}
	if err := json.NewDecoder(res.Body).Decode(grant); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return grant, nil
}

func (c *NullClient) GetAuthzGrant(grantID string) (*AuthzGrant, error) {
	path := fmt.Sprintf("%s/%s", AUTHZ_GRANT_PATH, grantID)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get authz grant: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	grant := &AuthzGrant{}
	if err := json.NewDecoder(res.Body).Decode(grant); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return grant, nil
}

func (c *NullClient) DeleteAuthzGrant(grantID string) error {
	path := fmt.Sprintf("%s/%s", AUTHZ_GRANT_PATH, grantID)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete authz grant: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}
