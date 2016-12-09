// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package outlook

import (
	"context"
	"encoding/json"
	"io"
	"io/ioutil"
	"time"

	"golang.org/x/oauth2"

	"github.com/pkg/errors"

	"github.com/oki-apps/okihome/api"
)

type provider struct {
	desc api.ProviderDescription
	cfg  *oauth2.Config
	r    api.Repository
}

//Config is the configuration of the app that will access Outlook API
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

var description = api.ProviderDescription{
	Name:              "outlook",
	Title:             "Outlook.com",
	Link:              "http://outlook.live.com",
	AvailableServices: []api.Service{api.ServiceEmail},
}

//New creates a new email provider that is able to access the Outlook API
func New(cfg Config, r api.Repository) api.EmailProvider {
	p := provider{
		desc: description,
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Scopes: []string{
				"offline_access",
				"https://outlook.office.com/mail.read",
			},
			RedirectURL: cfg.RedirectURL,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://login.microsoftonline.com/common/oauth2/v2.0/authorize",
				TokenURL: "https://login.microsoftonline.com/common/oauth2/v2.0/token",
			},
		},
		r: r,
	}
	return p
}

func (p provider) Description() api.ProviderDescription {
	return p.desc
}

func (p provider) Config() *oauth2.Config {
	return p.cfg
}

func (p provider) get(ctx context.Context, account api.ExternalAccount, url string, jsonData interface{}) error {
	client := p.cfg.Client(ctx, account.Token)

	r, err := client.Get(url)
	if err != nil {
		return errors.Wrap(err, "Call to Outlook api failed")
	}

	defer r.Body.Close()
	body, err := ioutil.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		return errors.Wrap(err, "Unable to read response body")
	}

	//TODO check err code

	if err = json.Unmarshal(body, jsonData); err != nil {
		return errors.Wrap(err, "Unable to connect to decode JSON")
	}

	return nil
}

func (p provider) GetCurrentEmailAddress(ctx context.Context, account api.ExternalAccount) (string, error) {

	url := "https://outlook.office.com/api/v2.0/me"

	var responseJSON struct {
		//Id string
		EmailAddress string
		//DisplayName string
		//Alias string
		//MailboxGuid string
	}

	err := p.get(ctx, account, url, &responseJSON)
	if err != nil {
		return "", errors.Wrap(err, "Unable to retrieve response")
	}

	return responseJSON.EmailAddress, nil
}

func (p provider) GetItems(ctx context.Context, account api.ExternalAccount, q api.EmailQuery, pageToken *string) (*api.EmailPage, error) {

	if q.Category == "" {
		q.Category = "inbox"
	}

	url := "https://outlook.office.com/api/v2.0/me/mailfolders/" + q.Category + "/messages?" +
		"$count=true&$top=30&$select=Subject,Sender,ReceivedDateTime,BodyPreview,IsRead,Weblink"

	if pageToken != nil {
		url = *pageToken
	}

	var responseJSON struct {
		Count int64  `json:"@odata.count"`
		Next  string `json:"@odata.nextLink"`
		Value []struct {
			ID               string `json:"Id"`
			ReceivedDateTime time.Time
			Subject          string
			BodyPreview      string
			Sender           struct {
				EmailAddress struct {
					Name    string
					Address string
				}
			}
			IsRead  bool
			WebLink string
		} `json:"value"`
	}

	err := p.get(ctx, account, url, &responseJSON)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to retrieve response")
	}

	res := api.EmailPage{
		NextPageToken:      responseJSON.Next,
		ResultSizeEstimate: responseJSON.Count,
		Items:              make([]api.EmailItem, 0, len(responseJSON.Value)),
	}

	for _, item := range responseJSON.Value {

		res.Items = append(res.Items, api.EmailItem{
			ItemForUser: api.ItemForUser{
				FeedItem: api.FeedItem{
					GUID:      item.ID,
					Title:     item.Subject,
					Published: item.ReceivedDateTime,
					Link:      item.WebLink,
				},
				Read: item.IsRead,
			},
			From:    item.Sender.EmailAddress.Name,
			Snippet: item.BodyPreview,
		})
	}

	return &res, nil
}
