package nullplatform

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

const NOTIFICATION_CHANNEL_PATH = "/notification/channel"

type NotificationChannel struct {
	Id            int                               `json:"id,omitempty"`
	Nrn           string                            `json:"nrn,omitempty"`
	Type          string                            `json:"type,omitempty"`
	Source        []string                          `json:"source,omitempty"`
	Configuration *NotificationChannelConfiguration `json:"configuration,omitempty"`
	Status        string                            `json:"status,omitempty"`
}

type NotificationChannelConfiguration struct {
	Channels []string `json:"channels,omitempty"`
	Url      string   `json:"url,omitempty"`
	Token    string   `json:"token,omitempty"`
}

func (c *NullClient) CreateNotificationChannel(notification *NotificationChannel) (*NotificationChannel, error) {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*notification)

	if err != nil {
		return nil, err
	}

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
		defer res.Body.Close()
		return nil, fmt.Errorf("error creating notification channel, got status code: %d, %s", res.StatusCode, nErr.Message)
	}

	resNotification := &NotificationChannel{}
	derr := json.NewDecoder(res.Body).Decode(resNotification)

	if derr != nil {
		return nil, derr
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

	notification := &NotificationChannel{}
	derr := json.NewDecoder(res.Body).Decode(notification)

	if derr != nil {
		return nil, derr
	}

	if notification.Status == "inactive" {
		return notification, fmt.Errorf("error getting notification channel, the status is %s", notification.Status)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("error getting notification channel, got %d for %s", res.StatusCode, notificationId)
	}

	return notification, nil
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

func (c *NullClient) UpdateNotificationChannel(notificationId string, notification *NotificationChannel) error {
	var buf bytes.Buffer
	err := json.NewEncoder(&buf).Encode(*notification)
	if err != nil {
		return err
	}

	path := fmt.Sprintf("%s/%s", NOTIFICATION_CHANNEL_PATH, notificationId)
	res, err := c.MakeRequest("PATCH", path, &buf)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("error updating notification channel, got %d", res.StatusCode)
	}

	return nil
}
