package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const NOTIFICATION_CHANNEL_PATH = "/notification/channel"

type NotificationChannel struct {
	Id            int                    `json:"id,omitempty"`
	Nrn           string                 `json:"nrn,omitempty"`
	Type          string                 `json:"type,omitempty"`
	Source        []string               `json:"source,omitempty"`
	Configuration map[string]interface{} `json:"configuration,omitempty"`
	Status        string                 `json:"status,omitempty"`
	Filters       map[string]interface{} `json:"filters"`
}

func (c *NullClient) CreateNotificationChannel(notification *NotificationChannel) (*NotificationChannel, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*notification)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Request body: %s\n", buf.String())

	res, err := c.MakeRequest("POST", NOTIFICATION_CHANNEL_PATH, &buf)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		var nErr NullErrors
		if err := json.NewDecoder(res.Body).Decode(&nErr); err != nil {
			return nil, fmt.Errorf("failed to decode error response: %w", err)
		}
		return nil, fmt.Errorf("error creating notification channel, got status code: %d, %s", res.StatusCode, nErr.Message)
	}

	resNotification := &NotificationChannel{}
	if err := json.NewDecoder(res.Body).Decode(resNotification); err != nil {
		return nil, err
	}

	return resNotification, nil
}

func (c *NullClient) GetNotificationChannel(notificationId string) (*NotificationChannel, error) {
	path := fmt.Sprintf("%s/%s", NOTIFICATION_CHANNEL_PATH, notificationId)

	res, err := c.MakeRequest("GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting notification channel, got %d for %s", res.StatusCode, notificationId)
	}

	notification := &NotificationChannel{}
	if err := json.NewDecoder(res.Body).Decode(notification); err != nil {
		return nil, err
	}

	return notification, nil
}

func (c *NullClient) UpdateNotificationChannel(notificationId string, notification *NotificationChannel) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*notification)
	if err != nil {
		return err
	}

	fmt.Printf("Request body: %s\n", buf.String())

	path := fmt.Sprintf("%s/%s", NOTIFICATION_CHANNEL_PATH, notificationId)
	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusNoContent {
		return fmt.Errorf("error updating notification channel, got %d", res.StatusCode)
	}

	return nil
}

func (c *NullClient) DeleteNotificationChannel(notificationId string) error {
	path := fmt.Sprintf("%s/%s", NOTIFICATION_CHANNEL_PATH, notificationId)

	res, err := c.MakeRequest("DELETE", path, nil)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if (res.StatusCode != http.StatusOK) && (res.StatusCode != http.StatusNoContent) {
		return fmt.Errorf("error deleting notification channel, got %d", res.StatusCode)
	}

	return nil
}
