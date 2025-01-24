package nullplatform

import (
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceNotificationChannel() *schema.Resource {
	return &schema.Resource{
		Description: "The notification channel resource allows you to configure a nullplatform notification channel",
		Create:      NotificationChannelCreate,
		Read:        NotificationChannelRead,
		Update:      NotificationChannelUpdate,
		Delete:      NotificationChannelDelete,

		Schema: AddNRNSchema(map[string]*schema.Schema{
			"type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Channel type (slack, http, gitlab, github, azure)",
			},
			"source": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Schema{
					Type: schema.TypeString,
				},
			},
			"configuration": {
				Type:     schema.TypeList,
				Required: true,
				MinItems: 1,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"slack": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"channels": {
										Type:     schema.TypeList,
										Required: true,
										MinItems: 1,
										Elem:     &schema.Schema{Type: schema.TypeString},
									},
								},
							},
						},
						"http": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"url": {
										Type:     schema.TypeString,
										Required: true,
									},
									"headers": {
										Type:     schema.TypeMap,
										Optional: true,
										Elem:     &schema.Schema{Type: schema.TypeString},
									},
								},
							},
						},
						"gitlab": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"project_id": {
										Type:     schema.TypeString,
										Required: true,
									},
									"reference": {
										Type:     schema.TypeString,
										Required: true,
									},
								},
							},
						},
						"azure": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"project": {
										Type:     schema.TypeString,
										Required: true,
									},
									"reference": {
										Type:     schema.TypeString,
										Required: true,
									},
									"pipeline_id": {
										Type:     schema.TypeInt,
										Required: true,
									},
									"organization": {
										Type:     schema.TypeString,
										Required: true,
									},
								},
							},
						},
						"github": {
							Type:     schema.TypeList,
							Optional: true,
							MaxItems: 1,
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"account": {
										Type:     schema.TypeString,
										Required: true,
									},
									"reference": {
										Type:     schema.TypeString,
										Required: true,
									},
									"repository": {
										Type:     schema.TypeString,
										Required: true,
									},
									"workflow_id": {
										Type:     schema.TypeString,
										Required: true,
									},
									"installation_id": {
										Type:     schema.TypeString,
										Required: true,
									},
								},
							},
						},
					},
				},
			},
			"filters": {
				Type:     schema.TypeString,
				Optional: true,
			},
		}),
	}
}

func NotificationChannelCreate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	configList := d.Get("configuration").([]interface{})
	config := configList[0].(map[string]interface{})

	flatConfig := make(map[string]interface{})
	switch d.Get("type").(string) {
	case "slack":
		if slackConfig, ok := config["slack"].([]interface{}); ok && len(slackConfig) > 0 {
			slackMap := slackConfig[0].(map[string]interface{})
			if channels, ok := slackMap["channels"]; ok {
				flatConfig["channels"] = channels
			}
		}
	case "http":
		if httpConfig, ok := config["http"].([]interface{}); ok && len(httpConfig) > 0 {
			httpMap := httpConfig[0].(map[string]interface{})
			flatConfig["url"] = httpMap["url"]
			if headers, ok := httpMap["headers"].(map[string]interface{}); ok {
				flatConfig["headers"] = headers
			}
		}
	case "gitlab":
		if gitlabConfig, ok := config["gitlab"].([]interface{}); ok && len(gitlabConfig) > 0 {
			gitlabMap := gitlabConfig[0].(map[string]interface{})
			flatConfig["project_id"] = gitlabMap["project_id"]
			flatConfig["reference"] = gitlabMap["reference"]
		}
	case "azure":
		if azureConfig, ok := config["azure"].([]interface{}); ok && len(azureConfig) > 0 {
			azureMap := azureConfig[0].(map[string]interface{})
			flatConfig["project"] = azureMap["project"]
			flatConfig["reference"] = azureMap["reference"]
			flatConfig["pipeline_id"] = azureMap["pipeline_id"]
			flatConfig["organization"] = azureMap["organization"]
		}
	case "github":
		if githubConfig, ok := config["github"].([]interface{}); ok && len(githubConfig) > 0 {
			githubMap := githubConfig[0].(map[string]interface{})
			flatConfig["account"] = githubMap["account"]
			flatConfig["reference"] = githubMap["reference"]
			flatConfig["repository"] = githubMap["repository"]
			flatConfig["workflow_id"] = githubMap["workflow_id"]
			flatConfig["installation_id"] = githubMap["installation_id"]
		}
	}

	var nrn string
	var err error
	if v, ok := d.GetOk("nrn"); ok {
		nrn = v.(string)
	} else {
		nrn, err = ConstructNRNFromComponents(d, nullOps)
		if err != nil {
			return fmt.Errorf("error constructing NRN: %v", err)
		}
	}

	newChannel := &NotificationChannel{
		Nrn:           nrn,
		Type:          d.Get("type").(string),
		Configuration: flatConfig,
	}

	if v, ok := d.GetOk("source"); ok {
		sources := v.([]interface{})
		strSources := make([]string, len(sources))
		for i, source := range sources {
			strSources[i] = source.(string)
		}
		newChannel.Source = strSources
	}

	if v, ok := d.GetOk("filters"); ok {
		var filtersMap map[string]interface{}
		if err := json.Unmarshal([]byte(v.(string)), &filtersMap); err != nil {
			return fmt.Errorf("invalid filters JSON: %v", err)
		}
		newChannel.Filters = filtersMap
	} else {
		newChannel.Filters = make(map[string]interface{})
	}

	channel, err := nullOps.CreateNotificationChannel(newChannel)
	if err != nil {
		return err
	}

	d.SetId(strconv.Itoa(channel.Id))
	return NotificationChannelRead(d, m)
}

func NotificationChannelRead(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)
	channel, err := nullOps.GetNotificationChannel(d.Id())
	if err != nil {
		if channel != nil && channel.Status == "inactive" {
			d.SetId("")
			return nil
		}
		return err
	}

	if channel == nil {
		d.SetId("")
		return nil
	}

	d.Set("nrn", channel.Nrn)
	d.Set("type", channel.Type)
	d.Set("source", channel.Source)

	config := make(map[string]interface{})
	switch channel.Type {
	case "slack":
		if channels, ok := channel.Configuration["channels"]; ok {
			config["slack"] = []interface{}{
				map[string]interface{}{
					"channels": channels,
				},
			}
		}
	case "http":
		httpConfig := make(map[string]interface{})
		if url, ok := channel.Configuration["url"]; ok {
			httpConfig["url"] = url
		}
		if headers, ok := channel.Configuration["headers"]; ok {
			httpConfig["headers"] = headers
		}
		config["http"] = []interface{}{httpConfig}
	case "gitlab":
		config["gitlab"] = []interface{}{
			map[string]interface{}{
				"project_id": channel.Configuration["project_id"],
				"reference":  channel.Configuration["reference"],
			},
		}
	case "azure":
		config["azure"] = []interface{}{
			map[string]interface{}{
				"project":      channel.Configuration["project"],
				"reference":    channel.Configuration["reference"],
				"pipeline_id":  channel.Configuration["pipeline_id"],
				"organization": channel.Configuration["organization"],
			},
		}
	case "github":
		config["github"] = []interface{}{
			map[string]interface{}{
				"account":         channel.Configuration["account"],
				"reference":       channel.Configuration["reference"],
				"repository":      channel.Configuration["repository"],
				"workflow_id":     channel.Configuration["workflow_id"],
				"installation_id": channel.Configuration["installation_id"],
			},
		}
	}

	if err := d.Set("configuration", []interface{}{config}); err != nil {
		return err
	}

	if channel.Filters != nil && len(channel.Filters) > 0 {
		filtersJson, err := json.Marshal(channel.Filters)
		if err != nil {
			return err
		}
		d.Set("filters", string(filtersJson))
	}

	return nil
}

func NotificationChannelUpdate(d *schema.ResourceData, m any) error {
	nullOps := m.(NullOps)

	if d.HasChanges("type", "configuration", "filters") {
		configList := d.Get("configuration").([]interface{})
		config := configList[0].(map[string]interface{})

		flatConfig := make(map[string]interface{})
		switch d.Get("type").(string) {
		case "slack":
			if slackConfig, ok := config["slack"].([]interface{}); ok && len(slackConfig) > 0 {
				slackMap := slackConfig[0].(map[string]interface{})
				if channels, ok := slackMap["channels"]; ok {
					flatConfig["channels"] = channels
				}
			}
		case "http":
			if httpConfig, ok := config["http"].([]interface{}); ok && len(httpConfig) > 0 {
				httpMap := httpConfig[0].(map[string]interface{})
				flatConfig["url"] = httpMap["url"]
				if headers, ok := httpMap["headers"].(map[string]interface{}); ok {
					flatConfig["headers"] = headers
				}
			}
		case "gitlab":
			if gitlabConfig, ok := config["gitlab"].([]interface{}); ok && len(gitlabConfig) > 0 {
				gitlabMap := gitlabConfig[0].(map[string]interface{})
				flatConfig["project_id"] = gitlabMap["project_id"]
				flatConfig["reference"] = gitlabMap["reference"]
			}
		case "azure":
			if azureConfig, ok := config["azure"].([]interface{}); ok && len(azureConfig) > 0 {
				azureMap := azureConfig[0].(map[string]interface{})
				flatConfig["project"] = azureMap["project"]
				flatConfig["reference"] = azureMap["reference"]
				flatConfig["pipeline_id"] = azureMap["pipeline_id"]
				flatConfig["organization"] = azureMap["organization"]
			}
		case "github":
			if githubConfig, ok := config["github"].([]interface{}); ok && len(githubConfig) > 0 {
				githubMap := githubConfig[0].(map[string]interface{})
				flatConfig["account"] = githubMap["account"]
				flatConfig["reference"] = githubMap["reference"]
				flatConfig["repository"] = githubMap["repository"]
				flatConfig["workflow_id"] = githubMap["workflow_id"]
				flatConfig["installation_id"] = githubMap["installation_id"]
			}
		}

		updateChannel := &NotificationChannel{
			Type:          d.Get("type").(string),
			Configuration: flatConfig,
			Filters:       make(map[string]interface{}),
		}

		if v, ok := d.GetOk("filters"); ok {
			var filtersMap map[string]interface{}
			if err := json.Unmarshal([]byte(v.(string)), &filtersMap); err != nil {
				return fmt.Errorf("invalid filters JSON: %v", err)
			}
			updateChannel.Filters = filtersMap
		}

		if err := nullOps.UpdateNotificationChannel(d.Id(), updateChannel); err != nil {
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
