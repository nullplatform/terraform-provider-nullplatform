package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	USER_PATH        = "/user"
	USER_INVITE_PATH = "/user/invite"
)

type User struct {
	ID             int    `json:"id,omitempty"`
	Email          string `json:"email"`
	FirstName      string `json:"first_name"`
	LastName       string `json:"last_name"`
	Avatar         string `json:"avatar,omitempty"`
	OrganizationID int    `json:"organization_id"`
}

type ListPaging struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
}

type UserList struct {
	Paging  ListPaging `json:"paging"`
	Results []User     `json:"results"`
}

type createUserRequest struct {
	Email          string        `json:"email"`
	FirstName      string        `json:"first_name"`
	LastName       string        `json:"last_name"`
	Avatar         string        `json:"avatar,omitempty"`
	OrganizationID int           `json:"organization_id"`
	Grants         []interface{} `json:"grants"`
}

type deactivateUserRequest struct {
	Status string `json:"status"`
}

func (c *NullClient) CreateUser(u *User) (*User, error) {
	req := &createUserRequest{
		Email:          u.Email,
		FirstName:      u.FirstName,
		LastName:       u.LastName,
		Avatar:         u.Avatar,
		OrganizationID: u.OrganizationID,
		Grants:         []interface{}{},
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode user: %v", err)
	}

	res, err := c.MakeRequest("POST", USER_INVITE_PATH, &buf)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		apiError := &ApiError{}
		if err := json.NewDecoder(res.Body).Decode(apiError); err == nil {
			bodyBytes, _ := io.ReadAll(res.Body)
			return nil, fmt.Errorf("failed to create user: status code %d, response: %s", res.StatusCode, string(bodyBytes))
		}

		if apiError.Message == "The user already exists" {
			return nil, &ResourceExistsError{"user", apiError.ID, apiError.Message}
		}

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

	deactivateReq := &deactivateUserRequest{
		Status: "inactive",
	}

	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(deactivateReq)
	if err != nil {
		return fmt.Errorf("failed to encode deactivate request: %v", err)
	}

	res, err := c.MakeRequest("PUT", path, &buf)
	if err != nil {
		return fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent && res.StatusCode != http.StatusNotFound {
		bodyBytes, _ := io.ReadAll(res.Body)
		return fmt.Errorf("failed to deactivate user: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	return nil
}

func (c *NullClient) LookupUser(userValues *User) (*User, error) {
	path := fmt.Sprintf("%s?organization_id=%d&email=%s", USER_PATH, userValues.OrganizationID, userValues.Email)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to make API request: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(res.Body)
		return nil, fmt.Errorf("failed to lookup user: status code %d, response: %s", res.StatusCode, string(bodyBytes))
	}

	listResult := &UserList{}
	if err := json.NewDecoder(res.Body).Decode(listResult); err != nil {
		return nil, fmt.Errorf("failed to decode API response: %v", err)
	}

	users := listResult.Results
	if len(users) == 0 {
		return nil, &ResourceNotFoundError{ApiType: "user", ID: 0, Message: "user not found"}
	}

	user := (users)[0]

	return &user, nil
}
