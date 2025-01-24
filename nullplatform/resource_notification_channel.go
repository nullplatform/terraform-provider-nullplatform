package nullplatform

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceNotificationChannel() *schema.Resource {
	return &schema.Resource{
		Description: "The notification channel resource allows you to configure a nullplatform notification channel",

		Create: NotificationChannelCreate,
		Read:   NotificationChannelRead,
		Update: NotificationChannelUpdate,
		Delete: NotificationChannelDelete,

		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				d.Set("id", d.Id())
				return []*schema.ResourceData{d}, nil
			},
		},

		Schema: map[string]*schema.Schema{
			"nrn": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "The NRN identifier",
			},
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Notification type: slack or http",
			},
			"source": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"configuration": {
				Type:     schema.TypeList,
				Required: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"channels": {
							Type:     schema.TypeList,
							Optional: true,
							Elem: &schema.Schema{
								Type: schema.TypeString,
							},
						},
						"url": {
							Type:     schema.TypeString,
							Optional: true,
						},
					},
				},
			},
			"filters": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "JSON encoded filters",
			},
		},
	}
}

func NotificationChannelCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	nrn := d.Get("nrn").(string)
	notificationType := d.Get("type").(string)

	sources := d.Get("source").([]interface{})
	notificationSource := make([]string, len(sources))
	for i, source := range sources {
		notificationSource[i] = source.(string)
	}

	configList := d.Get("configuration").([]interface{})
	if len(configList) == 0 {
		return fmt.Errorf("configuration must be set with channels or url")
	}
	config := configList[0].(map[string]interface{})

	newNotificationChannel := &NotificationChannel{
		Nrn:           nrn,
		Type:          notificationType,
		Source:        notificationSource,
		Configuration: &NotificationChannelConfiguration{},
	}

	if channels, ok := config["channels"]; ok {
		channelList := channels.([]interface{})
		strChannels := make([]string, len(channelList))
		for i, ch := range channelList {
			strChannels[i] = ch.(string)
		}
		newNotificationChannel.Configuration.Channels = strChannels
	}
	if url, ok := config["url"]; ok {
		newNotificationChannel.Configuration.Url = url.(string)
	}

	channel, err := nullOps.CreateNotificationChannel(newNotificationChannel)
	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(channel.Id))
	return NotificationChannelRead(d, m)
}

func NotificationChannelRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	notificationChannelId := d.Id()

	notificationChannel, err := nullOps.GetNotificationChannel(notificationChannelId)
	if err != nil {
		if notificationChannel.Status == "inactive" {
			d.SetId("")
			return nil
		}
		return err
	}

	if notificationChannel == nil {
		d.SetId("")
		return nil
	}

	if err := d.Set("nrn", notificationChannel.Nrn); err != nil {
		return err
	}

	if err := d.Set("type", notificationChannel.Type); err != nil {
		return err
	}

	if err := d.Set("source", notificationChannel.Source); err != nil {
		return err
	}

	config := make(map[string]interface{})
	if len(notificationChannel.Configuration.Channels) > 0 {
		config["channels"] = notificationChannel.Configuration.Channels
	}
	if notificationChannel.Configuration.Url != "" {
		config["url"] = notificationChannel.Configuration.Url
	}

	if err := d.Set("configuration", []interface{}{config}); err != nil {
		return err
	}

	return nil
}

func NotificationChannelUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	notificationChannelId := d.Id()

	updateNotificationChannel := &NotificationChannel{}
	needsUpdate := false

	if d.HasChange("type") {
		updateNotificationChannel.Type = d.Get("type").(string)
		needsUpdate = true
	}

	if d.HasChange("configuration") {
		configList := d.Get("configuration").([]interface{})
		if len(configList) > 0 {
			config := configList[0].(map[string]interface{})
			updateNotificationChannel.Configuration = &NotificationChannelConfiguration{}

			if channels, ok := config["channels"]; ok {
				channelList := channels.([]interface{})
				strChannels := make([]string, len(channelList))
				for i, ch := range channelList {
					strChannels[i] = ch.(string)
				}
				updateNotificationChannel.Configuration.Channels = strChannels
			}
			if url, ok := config["url"]; ok {
				updateNotificationChannel.Configuration.Url = url.(string)
			}
			needsUpdate = true
		}
	}

	if d.HasChange("filters") {
		if filters, ok := d.GetOk("filters"); ok {
			var filtersMap map[string]interface{}
			if err := json.Unmarshal([]byte(filters.(string)), &filtersMap); err != nil {
				return fmt.Errorf("invalid filters JSON: %v", err)
			}
			updateNotificationChannel.Filters = filtersMap
			needsUpdate = true
		}
	}

	if needsUpdate {
		if err := nullOps.UpdateNotificationChannel(notificationChannelId, updateNotificationChannel); err != nil {
			return err
		}
	}

	return NotificationChannelRead(d, m)
}

func NotificationChannelDelete(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	notificationChannelId := d.Id()

	err := nullOps.DeleteNotificationChannel(notificationChannelId)
	if err != nil {
		return err
	}

	d.SetId("")

	return nil
}
