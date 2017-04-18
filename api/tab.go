// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

//A TabSummary is thebasci configuration for a tab
type TabSummary struct {
	ID    int64  `json:"id"  db:"id"`
	Title string `json:"title"  db:"title"`
}

//A Tab is a collection of widgets to be displayed together
type Tab struct {
	TabSummary
	Widgets [][]Widget `json:"widgets,omitempty"`
}

//A Widget is a standalone item in a tab. It can either contains emails or feed items.
type Widget struct {
	ID     int64       `json:"id" db:"id"`
	Type   string      `json:"widgetType" db:"type"`
	Config interface{} `json:"config"`
}

//WidgetFeedType is the widget type for feed widgets
const WidgetFeedType = "feed"

//WidgetEmailType is the widget type for email widgets
const WidgetEmailType = "email"

//WidgetConfig is the basic configuration for a widget
type WidgetConfig struct {
	Title        string `json:"title" db:"title"`
	DisplayCount int    `json:"display_count,omitempty"`
	Link         string `json:"link,omitempty"`
}

//ConfigFeed is the configuration for a feed widget
type ConfigFeed struct {
	WidgetConfig
	FeedID int64  `json:"feed_id"`
	URL    string `json:"url"`
}

//NewWidgetFeed creates a new feed widget witn the given configuration
func NewWidgetFeed(id int64, cfg ConfigFeed) Widget {
	return Widget{
		ID:     id,
		Type:   WidgetFeedType,
		Config: cfg,
	}
}

//ConfigEmail is the widget configuration for an email widget
type ConfigEmail struct {
	WidgetConfig
	AccountID int64 `json:"account_id"`
}

//NewWidgetEmail creates a new email widget witn the given configuration
func NewWidgetEmail(id int64, cfg ConfigEmail) Widget {
	return Widget{
		ID:     id,
		Type:   WidgetEmailType,
		Config: cfg,
	}
}

//SetupTypedConfig recreate the typed config from a map[string]interface{}
func (w *Widget) SetupTypedConfig() {

	if cfg, ok := w.Config.(map[string]interface{}); ok {

		widgetConfig := WidgetConfig{}
		if v, ok := cfg["title"]; ok {
			if s, ok := v.(string); ok {
				widgetConfig.Title = s
			}
		}
		if v, ok := cfg["display_count"]; ok {
			if f, ok := v.(float64); ok {
				widgetConfig.DisplayCount = int(f)
			}
		}
		if v, ok := cfg["link"]; ok {
			if s, ok := v.(string); ok {
				widgetConfig.Link = s
			}
		}

		switch w.Type {
		case WidgetEmailType:
			newCfg := ConfigEmail{
				WidgetConfig: widgetConfig,
			}
			if v, ok := cfg["account_id"]; ok {
				if f, ok := v.(float64); ok {
					newCfg.AccountID = int64(f)
				}
			}
			w.Config = newCfg
		case WidgetFeedType:
			newCfg := ConfigFeed{
				WidgetConfig: widgetConfig,
			}
			if v, ok := cfg["url"]; ok {
				if s, ok := v.(string); ok {
					newCfg.URL = s
				}
			}
			if v, ok := cfg["feed_id"]; ok {
				if f, ok := v.(float64); ok {
					newCfg.FeedID = int64(f)
				}
			}
			w.Config = newCfg
		}
	}
}
