package nullplatform

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceNotificationChannel() *schema.Resource {
	return &schema.Resource{
		Description: "The notification channel resource allows you to configure a nullplatform notification channel",

		Create: NotificationChannelCreate,
		Read:   NotificationChannelRead,
		//Update: NotificationChannelUpdate,
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
				Description: "The NRN of the resource (including children resources) where the action will apply.",
			},
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Possible values: [`slack`, `http`].",
			},
			"source": {
				Type:        schema.TypeList,
				Required:    true,
				ForceNew:    true,
				Description: "Possible values: [`approval`, `service`, `audit`]",

				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"configuration": {
				Type:        schema.TypeList,
				MaxItems:    1,
				Required:    true,
				ForceNew:    true,
				Description: "Channels as an array of Slack channels or Http as the URL where the notifications will be sent.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"channels": {
							Type:     schema.TypeSet,
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
		newNotificationChannel.Configuration.Channels = expandStringSet(channels.(*schema.Set))
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

func expandStringSet(set *schema.Set) []string {
	list := set.List()
	result := make([]string, len(list))
	for i, v := range list {
		result[i] = v.(string)
	}
	return result
}
