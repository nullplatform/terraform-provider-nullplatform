package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const ACCOUNT_PATH = "/account"

type Account struct {
	Id                 int    `json:"id,omitempty"`
	Name               string `json:"name,omitempty"`
	OrganizationId     int    `json:"organization_id,omitempty"`
	RepositoryPrefix   string `json:"repository_prefix,omitempty"`
	RepositoryProvider string `json:"repository_provider,omitempty"`
	Slug               string `json:"slug,omitempty"`
	Status             string `json:"status,omitempty"`
	Nrn                string `json:"nrn,omitempty"`
}

func (c *NullClient) CreateAccount(account *Account) (*Account, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*account)

	if err != nil {
		return nil, err
	}

	res, err := c.MakeRequest("POST", ACCOUNT_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return nil, fmt.Errorf("failed to decode null error response: %w", err)
		}
		return nil, fmt.Errorf("error creating account resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	accountRes := &Account{}
	derr := json.NewDecoder(res.Body).Decode(accountRes)

	if derr != nil {
		return nil, derr
	}

	return accountRes, nil
}

func (c *NullClient) PatchAccount(accountId string, account *Account) error {
	path := fmt.Sprintf("%s/%s", ACCOUNT_PATH, accountId)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*account)

	if err != nil {
		return err
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return fmt.Errorf("failed to decode null error response: %w", err)
		}
		return fmt.Errorf("error updating account resource, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}

func (c *NullClient) GetAccount(accountId string) (*Account, error) {
	path := fmt.Sprintf("%s/%s", ACCOUNT_PATH, accountId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	account := &Account{}
	derr := json.NewDecoder(res.Body).Decode(account)

	if derr != nil {
		return nil, derr
	}

	if account.Status == "deleted" {
		return account, fmt.Errorf("error getting account resource, the status is %s", account.Status)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting account resource, got %d for %s", res.StatusCode, accountId)
	}

	return account, nil
}

func (c *NullClient) DeleteAccount(accountId string) error {
	path := fmt.Sprintf("%s/%s", ACCOUNT_PATH, accountId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("error making DELETE request: %w", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return fmt.Errorf("failed to decode error response: %w", err)
		}
		return fmt.Errorf("error deleting account, got status code: %d, message: %s", res.StatusCode, nErr.Message)
	}

	return nil
}
