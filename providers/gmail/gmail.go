// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package gmail

import (
	"fmt"
	"strings"
	"time"

	"golang.org/x/net/context"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/gmail/v1"

	"github.com/pkg/errors"

	"github.com/oki-apps/okihome/api"
)

type provider struct {
	desc api.ProviderDescription
	cfg  *oauth2.Config
	r    api.Repository
}

//Config is the configuration of the app that will access Gmail API
type Config struct {
	ClientID     string
	ClientSecret string
	RedirectURL  string
}

var description = api.ProviderDescription{
	Name:              "google",
	Title:             "Gmail",
	Link:              "https://gmail.com",
	AvailableServices: []api.Service{api.ServiceEmail},
}

//New creates a new email provider that is able to access the Gmail API
func New(cfg Config, r api.Repository) api.EmailProvider {
	p := provider{
		desc: description,
		cfg: &oauth2.Config{
			ClientID:     cfg.ClientID,
			ClientSecret: cfg.ClientSecret,
			Scopes: []string{
				gmail.GmailReadonlyScope,
			},
			RedirectURL: cfg.RedirectURL,
			Endpoint:    google.Endpoint,
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

func (p provider) getService(ctx context.Context, account api.ExternalAccount) (*gmail.Service, error) {
	client := p.cfg.Client(ctx, account.Token)

	srv, err := gmail.New(client)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create Gmail client")
	}

	return srv, nil
}

func (p provider) GetCurrentEmailAddress(ctx context.Context, account api.ExternalAccount) (string, error) {

	srv, err := p.getService(ctx, account)
	if err != nil {
		return "", errors.Wrap(err, "Unable to connect to the Gmail service")
	}
	user := "me"

	profile, err := srv.Users.GetProfile(user).Do()
	if err != nil {
		return "", errors.Wrap(err, "Unable to retrieve profile")
	}

	return profile.EmailAddress, nil
}

func (p provider) GetAvailableCategories(ctx context.Context, account api.ExternalAccount) ([]api.Category, error) {

	srv, err := p.getService(ctx, account)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to connect to the Gmail service")
	}
	user := "me"
	r, err := srv.Users.Labels.List(user).Do()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to retrieve label list")
	}

	var categories []api.Category
	for _, l := range r.Labels {
		categories = append(categories, api.Category{
			Name:  l.Id,
			Title: l.Name,
		})
	}

	return categories, nil
}

func (p provider) GetItems(ctx context.Context, account api.ExternalAccount, q api.EmailQuery, pageToken *string) (*api.EmailPage, error) {

	srv, err := p.getService(ctx, account)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to connect to the Gmail service")
	}
	user := "me"

	req := srv.Users.Threads.List(user).MaxResults(30).LabelIds("INBOX")
	if pageToken != nil {
		req = req.PageToken(*pageToken)
	}
	if len(q.Category) > 0 {
		req = req.LabelIds(q.Category)
	}

	r, err := req.Do()
	if err != nil {
		return nil, errors.Wrap(err, "Unable to retrieve threads list")
	}

	res := api.EmailPage{
		NextPageToken:      r.NextPageToken,
		ResultSizeEstimate: r.ResultSizeEstimate,
	}

	fmt.Println("Got ", len(r.Threads), " threads")

	for _, thread := range r.Threads {

		emailItem, err := p.r.GetEmailItem(ctx, account, thread.Id, thread.HistoryId)
		if err != nil {
			return nil, errors.Wrap(err, "Unable to retrieve prefetched thread "+thread.Id)
		}
		if emailItem.GUID == "" {
			emailItem, err = p.createEmailItem(ctx, srv, user, account, *thread)
			if err != nil {
				fmt.Println("Thread ", *thread)
				return nil, errors.Wrap(err, "Unable to create and cache thread "+thread.Id)
			}
		}

		if emailItem.GUID != "" {
			res.Items = append(res.Items, emailItem)
		}
	}

	return &res, nil
}

func getHeader(msg *gmail.Message, key string) (string, error) {

	for _, h := range msg.Payload.Headers {
		if h.Name == key {
			return h.Value, nil
		}
	}

	return "", errors.New("Header " + key + " not found on message " + msg.Id)
}

func (p provider) createEmailItem(ctx context.Context, srv *gmail.Service, user string, account api.ExternalAccount, thread gmail.Thread) (api.EmailItem, error) {

	var res api.EmailItem
	res.GUID = thread.Id
	res.Link = "https://mail.google.com/mail/#inbox/" + thread.Id

	if len(thread.Messages) == 0 {
		r, err := srv.Users.Threads.Get(user, thread.Id).Do()
		if err != nil {
			//TODO:: notify app
			return api.EmailItem{}, nil
		}
		thread = *r
	}

	if len(thread.Messages) == 0 {
		return api.EmailItem{}, errors.New("No message in thread " + thread.Id)
	}

	firstMessage := thread.Messages[0]
	lastMessage := thread.Messages[len(thread.Messages)-1]
	var firstUnread *gmail.Message
	var lastUnread *gmail.Message

	froms := make(map[string]bool)
	unreadCount := 0

	for _, m := range thread.Messages {
		from, err := getHeader(m, "From")
		if err != nil {
			return api.EmailItem{}, errors.Wrap(err, "Unable to retrieve thread sender for msg "+m.Id)
		}
		froms[from] = true

		for _, label := range m.LabelIds {
			if label == "UNREAD" {
				unreadCount++
				lastUnread = m
				if firstUnread == nil {
					firstUnread = m
				}
				break
			}
		}
	}

	var err error

	res.Title, err = getHeader(firstMessage, "Subject")
	if err != nil {
		res.Title = ""
	}
	res.Read = (unreadCount == 0)

	mainMessage := lastMessage
	if !res.Read {
		mainMessage = lastUnread
	}

	res.Snippet = mainMessage.Snippet
	if unreadCount > 1 {
		res.Snippet = "[...] - " + res.Snippet
	}

	res.Published = time.Unix(int64(time.Duration(lastMessage.InternalDate)/(time.Second/time.Millisecond)), 0)

	//From
	res.From, err = getHeader(mainMessage, "From")
	if err != nil {
		return api.EmailItem{}, errors.Wrap(err, "Unable to retrieve thread sender for "+thread.Id)
	}
	if strings.Index(res.From, "<") > 1 {
		res.From = res.From[:strings.Index(res.From, "<")]
	}
	if len(froms) > 1 {
		res.From = fmt.Sprintf("%s (%d)", res.From, len(froms))
	}

	//Saveit with historyId
	err = p.r.StoreEmailItem(ctx, account, thread.HistoryId, res)
	if err != nil {
		fmt.Println(err)
	}

	return res, nil
}
