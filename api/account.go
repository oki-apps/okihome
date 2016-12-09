// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"context"

	"golang.org/x/oauth2"
)

//Service represents a kind of service provided bu third parties
type Service string

const (
	//ServiceEmail is the Service for email providers (such as Outlook, Gmail, ...)
	ServiceEmail Service = "EMAIL"
	//ServiceSocialFeed is the Service for social feeds providers (such as Facebook, Twitter, ...)
	ServiceSocialFeed Service = "SOCIAL_FEED"
)

//ProviderDescription is the basic information regarding a service provider
type ProviderDescription struct {
	Name              string    `json:"name"`
	Title             string    `json:"title"`
	Link              string    `json:"link"`
	AvailableServices []Service `json:"services"`
}

//Provider is the interface to be implemented by service provider libraries
type Provider interface {
	Description() ProviderDescription
	Config() *oauth2.Config
}

//Category represents a group of related emails (it can be a folder or a tag based on the provider)
type Category struct {
	Name  string `json:"name"`
	Title string `json:"title"`
}

//An EmailProvider is provider related to email service
type EmailProvider interface {
	Provider

	GetCurrentEmailAddress(ctx context.Context, account ExternalAccount) (string, error)

	//GetAvailableCategories(ctx context.Context, account ExternalAccount) ([]Category, error)

	GetItems(ctx context.Context, account ExternalAccount, q EmailQuery, pageToken *string) (*EmailPage, error)
}

//A SocialFeedProvider is provider related to social feeds service
type SocialFeedProvider interface {
	Provider

	GetItems(account ExternalAccount) ([]ItemForUser, error)
}

//An EmailItem is the representation of a email or conversation
type EmailItem struct {
	ItemForUser

	From    string `json:"from" db:"sender"`
	Snippet string `json:"snippet" db:"snippet"`
}

//EmailQuery contains the request parameter when retrieving data from a provider
type EmailQuery struct {
	Category string `json:"category"`
}

//EmailPage is a batch of results for a query
type EmailPage struct {
	Items              []EmailItem `json:"items"`
	NextPageToken      string      `json:"nextpage,omitempty"`
	ResultSizeEstimate int64       `json:"result_size_estimate"`
}

//ExternalAccount is the basic information required to access an account on external service
type ExternalAccount struct {
	ID           int64         `json:"id" db:"id"`
	ProviderName string        `json:"provider_name" db:"provider"`
	AccountID    string        `json:"account_id" db:"account_id"`
	Token        *oauth2.Token `json:"-" db:"token"`
}
