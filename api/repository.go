// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"context"
)

//Repository is the interface allowing usage of any data store for tabs, widgets, read flags and all other data.
type Repository interface {
	//RunInTransaction(ctx context.Context, f func(repo Repository) error) error

	IsNotFound(err error) bool

	GetUser(ctx context.Context, userID string) (User, error)
	StoreUser(ctx context.Context, user *User) error
	//DeleteUser(ctx context.Context, userID string) error

	GetTabs(ctx context.Context, userID string) ([]TabSummary, error)
	IsTabAccessAllowed(ctx context.Context, userID string, tabID int64) error
	AllowTabAccess(ctx context.Context, userID string, tabID int64) error

	GetTab(ctx context.Context, tabID int64) (Tab, error)
	StoreTab(ctx context.Context, tab *Tab) error
	DeleteTab(ctx context.Context, tabID int64) error

	GetWidget(ctx context.Context, tabID int64, widgetID int64) (Widget, error)
	StoreWidget(ctx context.Context, tabID int64, widget *Widget) error
	DeleteWidget(ctx context.Context, tabID int64, widgetID int64) error

	UpdateTabLayout(ctx context.Context, tabID int64, layout [][]int64) error
	DeleteWidgetFromTab(ctx context.Context, tabID int64, widgetID int64) error

	GetOrCreateFeedID(ctx context.Context, URL string) (int64, error)
	GetFeed(ctx context.Context, feedID int64) (Feed, error)
	GetFeedItems(ctx context.Context, feedID int64) ([]FeedItem, error)
	StoreFeed(ctx context.Context, feed *Feed, feedItems []FeedItem) error
	//DeleteFeed(ctx context.Context, feedID int64) error

	AreItemsRead(ctx context.Context, userID string, feedID int64, guids []string) ([]bool, error)
	SetItemRead(ctx context.Context, userID string, feedID int64, guid string, read bool) error

	GetAccount(ctx context.Context, userID string, accountID int64) (ExternalAccount, error)
	GetAccounts(ctx context.Context, userID string) ([]ExternalAccount, error)
	DeleteAccount(ctx context.Context, userID string, accountID int64) error
	StoreAccount(ctx context.Context, userID string, account *ExternalAccount) error

	GetUserFromTemporaryCode(ctx context.Context, serviceName string, code string) (string, error)
	StoreTemporaryCode(ctx context.Context, userID string, serviceName string, code string) error
	DeleteTemporaryCode(ctx context.Context, userID string, serviceName string) error

	GetEmailItem(ctx context.Context, account ExternalAccount, guid string, minVersion uint64) (EmailItem, error)
	StoreEmailItem(ctx context.Context, account ExternalAccount, version uint64, item EmailItem) error
}
