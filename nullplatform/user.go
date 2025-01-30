package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	USER_PATH = "/user"
)

type User struct {
	ID             string `json:"id,omitempty"`
	Email          string `json:"email"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Avatar         string `json:"avatar,omitempty"`
	OrganizationID int    `json:"organization_id"`
}

func (c *NullClient) CreateUser(u *User) (*User, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(u)
	if err != nil {
		return nil, fmt.Errorf("failed to encode user: %v", err)
	}

	res, err := c.MakeRequest("POST", USER_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to create user: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	user := &User{}
	if err := json.NewDecoder(res.Body).Decode(user); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return user, nil
}

func (c *NullClient) GetUser(userID string) (*User, error) {
	path := fmt.Sprintf("%s/%s", USER_PATH, userID)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to get user: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	user := &User{}
	if err := json.NewDecoder(res.Body).Decode(user); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	return user, nil
}

func (c *NullClient) UpdateUser(userID string, u *User) error {
	path := fmt.Sprintf("%s/%s", USER_PATH, userID)

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(u)
	if err != nil {
		return fmt.Errorf("failed to encode user: %v", err)
	}

	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to update user: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) DeleteUser(userID string) error {
	path := fmt.Sprintf("%s/%s", USER_PATH, userID)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to delete user: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}
