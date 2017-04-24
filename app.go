// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package okihome

import (
	"context"
	"fmt"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"

	"github.com/oki-apps/okihome/api"
)

//App is the main application.
//
//Usually, a single app is created and runned.
type App struct {
	repository     api.Repository
	userInteractor api.UserInteractor
	logInteractor  api.LogInteractor
	providers      map[string]api.Provider
}

//NewApp creates a new App using the given services
func NewApp(r api.Repository, u api.UserInteractor, l api.LogInteractor, p []api.Provider) *App {
	app := &App{
		repository:     r,
		userInteractor: u,
		logInteractor:  l,
		providers:      make(map[string]api.Provider),
	}

	for _, provider := range p {
		app.providers[provider.Description().Name] = provider
	}

	return app
}

// Infof formats its arguments according to the format, analogous to fmt.Printf,
// and records the text as a log message at Info level.
func (app *App) Infof(ctx context.Context, format string, args ...interface{}) {
	app.logInteractor.Infof(ctx, format, args...)
}

// Errorf is like Infof, but at Error level.
func (app *App) Errorf(ctx context.Context, format string, args ...interface{}) {
	app.logInteractor.Errorf(ctx, format, args...)
}

func (app *App) Error(ctx context.Context, err error) {
	app.logInteractor.Errorf(ctx, "%s", err)
}

type notAuthorized string

func (err notAuthorized) IsNotAuthorized() bool {
	return true
}
func (err notAuthorized) Error() string {
	return string(err)
}

//UserData contains the basic user information
type UserData struct {
	User api.User         `json:"user"`
	Tabs []api.TabSummary `json:"tabs"`
}

//User returns the basic user information for the user with the given id
func (app App) User(ctx context.Context, userID string) (UserData, error) {

	//Check that a user is logged
	loggedInUser, err := app.userInteractor.CurrentUser(ctx)
	if err != nil {
		return UserData{}, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	if userID != loggedInUser.ID() {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return UserData{}, errors.Wrap(notAuthorized("access denied to user: "+userID), "access by "+loggedInUser.ID())
		}
	}

	data := UserData{}

	//Get the user in datastore
	data.User, err = app.repository.GetUser(ctx, userID)
	if err != nil {
		if app.repository.IsNotFound(err) {
			data.User.UserID = loggedInUser.ID()
			data.User.DisplayName = loggedInUser.DisplayName()
			data.User.Email = loggedInUser.Email()
			data.User.IsAdmin = false

			err = app.repository.StoreUser(ctx, &data.User)
		}

		if err != nil {
			return UserData{}, errors.Wrap(err, "retrieving user from datastore failed")
		}
	}

	data.Tabs, err = app.repository.GetTabs(ctx, userID)
	if err != nil {
		return UserData{}, errors.Wrap(err, "retrieving tab ids from datastore failed")
	}

	return data, nil
}

//BackupUser returns the configuration of a given user (used for backup and restore)
func (app App) BackupUser(ctx context.Context, userID string) (api.Snapshot, error) {

	//Check that a user is logged
	loggedInUser, err := app.userInteractor.CurrentUser(ctx)
	if err != nil {
		return api.Snapshot{}, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	if userID != loggedInUser.ID() {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return api.Snapshot{}, errors.Wrap(notAuthorized("access denied to user: "+userID), "access by "+loggedInUser.ID())
		}
	}

	data := api.Snapshot{}

	//Get the user in datastore
	data.User, err = app.repository.GetUser(ctx, userID)
	if err != nil {
		return api.Snapshot{}, errors.Wrap(err, "retrieving user from datastore failed")
	}

	//Get the tabs
	tabs, err := app.repository.GetTabs(ctx, userID)
	if err != nil {
		return api.Snapshot{}, errors.Wrap(err, "retrieving tab ids from datastore failed")
	}

	for _, t := range tabs {
		tab, err := app.repository.GetTab(ctx, t.ID)
		if err != nil {
			return api.Snapshot{}, errors.Wrap(err, "retrieving tab from datastore failed")
		}
		data.Tabs = append(data.Tabs, tab)
	}

	//Get the feeds
	feedIDs := make(map[int64]bool)
	for _, t := range data.Tabs {
		for _, col := range t.Widgets {
			for _, w := range col {
				if w.Type == api.WidgetFeedType {
					cfg := w.Config.(api.ConfigFeed)

					feedIDs[cfg.FeedID] = true
				}
			}
		}
	}
	for feedID := range feedIDs {
		feed, err := app.repository.GetFeed(ctx, feedID)
		if err != nil {
			return api.Snapshot{}, errors.Wrap(err, "retrieving feed from datastore failed")
		}
		data.Feeds = append(data.Feeds, feed)
	}

	//Get the accounts
	data.Accounts, err = app.repository.GetAccounts(ctx, userID)
	if err != nil {
		return api.Snapshot{}, errors.Wrap(err, "retrieving accounts from datastore failed")
	}

	return data, nil
}

//RestoreUser restores the configuration of a given user (used for backup and restore)
func (app App) RestoreUser(ctx context.Context, userID string, s api.Snapshot) error {

	//Check that a user is logged
	loggedInUser, err := app.userInteractor.CurrentUser(ctx)
	if err != nil {
		return errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	if userID != loggedInUser.ID() {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return errors.Wrap(notAuthorized("access denied to user: "+userID), "access by "+loggedInUser.ID())
		}
	}

	//UserID should match
	if userID != s.User.UserID {
		return errors.New(fmt.Sprintf("User IDs do not match: '%s' '%s'", userID, s.User.UserID))
	}

	//No tabs defined for user to update
	tabs, err := app.repository.GetTabs(ctx, userID)
	if err != nil {
		return errors.Wrap(err, "retrieving tab ids from datastore failed")
	}
	if len(tabs) > 0 {
		return errors.New(fmt.Sprintf("Restore not possible due %d to existing tabs", len(tabs)))
	}

	//Get account matching
	accounts, err := app.repository.GetAccounts(ctx, userID)
	if err != nil {
		return errors.Wrap(err, "retrieving accounts from datastore failed")
	}
	existingAccounts := make(map[string]int64)
	for _, a := range accounts {
		existingAccounts[a.Key()] = a.ID
	}
	allAccounts := make(map[int64]int64)
	for _, a := range s.Accounts {
		existingID, ok := existingAccounts[a.Key()]
		if !ok {
			return errors.New("Restore not possible due to missing account: " + a.Key())
		}
		allAccounts[a.ID] = existingID
	}

	//Add all feeds and keep matching
	allFeeds := make(map[int64]int64)
	for _, f := range s.Feeds {
		id, err := app.repository.GetOrCreateFeedID(ctx, f.URL)
		if err != nil {
			return errors.Wrap(err, "retrieving feed id from datastore failed")
		}
		allFeeds[f.ID] = id
	}

	//Create all tabs and add widgets
	for _, t := range s.Tabs {

		newTab, err := app.NewTab(ctx, t.TabSummary)
		if err != nil {
			return errors.Wrap(err, "creating tab failed")
		}

		for i, c := range t.Widgets {
			newTab.Widgets = append(newTab.Widgets, nil)

			for _, w := range c {

				newWidget := w
				newWidget.ID = 0
				newWidget.SetupTypedConfig()

				//Map account id/feed id in widget configs
				switch newWidget.Type {
				case api.WidgetFeedType:
					cfg := newWidget.Config.(api.ConfigFeed)
					var ok bool
					cfg.FeedID, ok = allFeeds[cfg.FeedID]
					if !ok {
						return errors.New("Unknown feed ID")
					}
					newWidget.Config = cfg

				case api.WidgetEmailType:
					cfg := newWidget.Config.(api.ConfigEmail)
					var ok bool
					cfg.AccountID, ok = allAccounts[cfg.AccountID]
					if !ok {
						return errors.New("Unknown account ID")
					}
					newWidget.Config = cfg
				}

				//Store updated widget
				err := app.repository.StoreWidget(ctx, newTab.ID, &newWidget)
				if err != nil {
					return errors.Wrap(err, "creating widget failed")
				}

				newTab.Widgets[i] = append(newTab.Widgets[i], newWidget)
			}
		}

		err = app.repository.StoreTab(ctx, &newTab)
		if err != nil {
			return errors.Wrap(err, "creating tab layout failed")
		}
	}

	return nil
}

//Services returns the list of all available providers
func (app App) Services(ctx context.Context) ([]api.ProviderDescription, error) {

	services := make([]api.ProviderDescription, 0, len(app.providers))

	for _, provider := range app.providers {
		services = append(services, provider.Description())
	}

	return services, nil
}

//AssociatedAccount returns the information related to the given account, including the authentication tokens
func (app App) AssociatedAccount(ctx context.Context, userID string, accountID int64) (api.ExternalAccount, error) {

	//Check that a user is logged
	loggedInUserID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return api.ExternalAccount{}, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	if userID != loggedInUserID {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return api.ExternalAccount{}, errors.Wrap(notAuthorized("access denied to user: "+userID), "access by "+loggedInUserID)
		}
	}

	data, err := app.repository.GetAccount(ctx, userID, accountID)
	if err != nil {
		return api.ExternalAccount{}, errors.Wrap(err, "retrieving account from datastore failed")
	}

	return data, nil
}

//AssociatedAccounts returns the list of accounts available for the given user
func (app App) AssociatedAccounts(ctx context.Context, userID string) ([]api.ExternalAccount, error) {

	//Check that a user is logged
	loggedInUserID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	if userID != loggedInUserID {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return nil, errors.Wrap(notAuthorized("access denied to user: "+userID), "access by "+loggedInUserID)
		}
	}

	data, err := app.repository.GetAccounts(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving accounts from datastore failed")
	}

	return data, nil
}

//AssociatedServiceAccounts returns the list of accounts available for the given user for a specific provider
func (app App) AssociatedServiceAccounts(ctx context.Context, userID string, serviceName string) ([]api.ExternalAccount, error) {

	accounts, err := app.AssociatedAccounts(ctx, userID)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving all accounts failed")
	}

	serviceAccounts := make([]api.ExternalAccount, 0, len(accounts))
	for _, a := range accounts {
		if a.ProviderName == serviceName {
			serviceAccounts = append(serviceAccounts, a)
		}
	}

	return serviceAccounts, nil
}

//RevokeAccount permanently removes access to the given account
func (app App) RevokeAccount(ctx context.Context, userID string, accountID int64) (bool, error) {

	//Check that a user is logged
	loggedInUserID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return false, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	if userID != loggedInUserID {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return false, errors.Wrap(notAuthorized("access denied to user: "+userID), "access by "+loggedInUserID)
		}
	}

	//Delete the account and associated token
	err = app.repository.DeleteAccount(ctx, userID, accountID)
	if err != nil {
		return false, errors.Wrap(err, "removing account from datastore failed")
	}

	return true, nil
}

//Tab returns the configuration and layout for the given tab
func (app App) Tab(ctx context.Context, tabID int64) (api.Tab, error) {

	//Check that a user is logged
	userID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return api.Tab{}, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	err = app.repository.IsTabAccessAllowed(ctx, userID, tabID)
	if err != nil {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return api.Tab{}, errors.Wrap(err, "access by "+userID)
		}
	}

	//Get the tab in datastore
	tab, err := app.repository.GetTab(ctx, tabID)
	if err != nil {
		return tab, errors.Wrap(err, "retrieving tab from datastore failed")
	}

	return tab, nil
}

//EditTab updates the tab with the given configuration
func (app App) EditTab(ctx context.Context, tabID int64, newSummary api.TabSummary) (api.Tab, error) {

	//Check that a user is logged
	userID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return api.Tab{}, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	err = app.repository.IsTabAccessAllowed(ctx, userID, tabID)
	if err != nil {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return api.Tab{}, errors.Wrap(err, "access by "+userID)
		}
	}

	//Get the tab from datastore
	tab, err := app.repository.GetTab(ctx, tabID)
	if err != nil {
		return tab, errors.Wrap(err, "retrieving tab from datastore failed")
	}

	newSummary.ID = tabID
	tab.TabSummary = newSummary

	err = app.repository.StoreTab(ctx, &tab)
	if err != nil {
		return tab, errors.Wrap(err, "storing tab into datastore failed")
	}

	return tab, nil
}

//DeleteTab permanently removes the given tab
func (app App) DeleteTab(ctx context.Context, tabID int64) (bool, error) {

	//Check that a user is logged
	userID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return false, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	err = app.repository.IsTabAccessAllowed(ctx, userID, tabID)
	if err != nil {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return false, errors.Wrap(err, "access by "+userID)
		}
	}

	//Remove the tab from datastore
	err = app.repository.DeleteTab(ctx, tabID)
	if err != nil {
		return false, errors.Wrap(err, "removing tab from datastore failed")
	}

	return true, nil
}

//NewTab creates a new tab
func (app App) NewTab(ctx context.Context, tabDesc api.TabSummary) (api.Tab, error) {

	//Check that a user is logged
	userID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return api.Tab{}, errors.Wrap(err, "retrieving current user failed")
	}

	var tab api.Tab
	tab.Title = tabDesc.Title
	tab.Widgets = [][]api.Widget{
		[]api.Widget{},
		[]api.Widget{},
		[]api.Widget{},
		[]api.Widget{},
	}

	err = app.repository.StoreTab(ctx, &tab)
	if err != nil {
		return api.Tab{}, errors.Wrap(err, "saving tab in datastore failed")
	}

	err = app.repository.AllowTabAccess(ctx, userID, tab.ID)
	if err != nil {
		return api.Tab{}, errors.Wrap(err, "saving tab access rules in datastore failed")
	}

	return tab, nil
}

//NewWidget adds a widget to the current tab
func (app App) NewWidget(ctx context.Context, tabID int64, widget api.Widget) (api.Widget, error) {

	//Check that a user is logged
	userID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return api.Widget{}, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	err = app.repository.IsTabAccessAllowed(ctx, userID, tabID)
	if err != nil {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return api.Widget{}, errors.Wrap(err, "access by "+userID)
		}
	}

	switch widget.Type {
	case api.WidgetFeedType:
		cfg := widget.Config.(api.ConfigFeed)
		cfg.FeedID = 0
		if cfg.DisplayCount <= 0 {
			cfg.DisplayCount = 5 //TODO use configurable constante
		}

		//Get or create the feed
		cfg.FeedID, err = app.repository.GetOrCreateFeedID(ctx, cfg.URL)
		if err != nil {
			return api.Widget{}, errors.Wrap(err, "unable to create feed")
		}

		if len(cfg.Title) == 0 {

			//Get the title from existing feed
			feed, _, err := app.feed(ctx, cfg.FeedID, false)
			if err != nil {
				return api.Widget{}, errors.Wrap(err, "feed retrieval failed")
			}

			cfg.Title = feed.Title
		}

		widget.Config = cfg

	case api.WidgetEmailType:
		cfg := widget.Config.(api.ConfigEmail)
		if cfg.DisplayCount <= 0 {
			cfg.DisplayCount = 5 //TODO use configurable constante
		}

		account, err := app.repository.GetAccount(ctx, userID, cfg.AccountID)
		if err != nil {
			return api.Widget{}, errors.Wrap(err, "account retrieval failed")
		}

		provider, ok := app.providers[account.ProviderName]
		if !ok {
			return api.Widget{}, errors.New("Unknown service: " + account.ProviderName)
		}

		if len(cfg.Title) == 0 {
			cfg.Title = provider.Description().Title
		}
		if len(cfg.Link) == 0 {
			cfg.Link = provider.Description().Link
		}

		widget.Config = cfg
	}

	//Store the new widget within the tab
	tab, err := app.repository.GetTab(ctx, tabID)
	if err != nil {
		return api.Widget{}, errors.Wrap(err, "retrieving tab from datastore failed")
	}

	err = app.repository.StoreWidget(ctx, tabID, &widget)
	if err != nil {
		return api.Widget{}, errors.Wrap(err, "saving widget in datastore failed")
	}

	if len(tab.Widgets) == 0 {
		tab.Widgets = [][]api.Widget{[]api.Widget{}}
	}
	tab.Widgets[0] = append(tab.Widgets[0], widget)

	err = app.repository.StoreTab(ctx, &tab)
	if err != nil {
		return api.Widget{}, errors.Wrap(err, "saving tab in datastore failed")
	}

	return widget, nil
}

//DeleteWidget permanently removes a widget
func (app App) DeleteWidget(ctx context.Context, tabID int64, widgetID int64) (bool, error) {
	//Check that a user is logged
	userID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return false, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	err = app.repository.IsTabAccessAllowed(ctx, userID, tabID)
	if err != nil {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return false, errors.Wrap(err, "access by "+userID)
		}
	}

	app.Infof(ctx, "Removing widget %d %d", tabID, widgetID)

	//Update the tab layout
	err = app.repository.DeleteWidgetFromTab(ctx, tabID, widgetID)
	if err != nil {
		return false, errors.Wrap(err, "removing widget from tab failed")
	}

	err = app.repository.DeleteWidget(ctx, tabID, widgetID)
	if err != nil {
		return false, errors.Wrap(err, "removing widget from datastore failed")
	}

	return true, nil

}

//EditWidget updates the widget configuration
func (app App) EditWidget(ctx context.Context, tabID int64, widgetID int64, newConfig api.WidgetConfig) (api.Widget, error) {

	//Check that a user is logged
	userID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return api.Widget{}, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	err = app.repository.IsTabAccessAllowed(ctx, userID, tabID)
	if err != nil {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return api.Widget{}, errors.Wrap(err, "access by "+userID)
		}
	}

	app.Infof(ctx, "Editing widget %d %d", tabID, widgetID)

	//Get current version
	widget, err := app.repository.GetWidget(ctx, tabID, widgetID)
	if err != nil {
		return api.Widget{}, errors.Wrap(err, "retrieving widget from datastore failed")
	}

	switch widget.Type {
	case api.WidgetFeedType:
		cfg, ok := widget.Config.(api.ConfigFeed)
		if !ok {
			return api.Widget{}, errors.New("Invalid widget config type")
		}

		cfg.Title = newConfig.Title
		cfg.DisplayCount = newConfig.DisplayCount

		widget.Config = cfg
	case api.WidgetEmailType:
		cfg, ok := widget.Config.(api.ConfigEmail)
		if !ok {
			return api.Widget{}, errors.New("Invalid widget config type")
		}

		cfg.Title = newConfig.Title
		cfg.DisplayCount = newConfig.DisplayCount

		widget.Config = cfg
	}

	err = app.repository.StoreWidget(ctx, tabID, &widget)
	if err != nil {
		return api.Widget{}, errors.Wrap(err, "updating widget in datastore failed")
	}

	return widget, nil

}

//UpdateLayout reorganises the content of a tab, based on the given widget id lists
func (app App) UpdateLayout(ctx context.Context, tabID int64, layout [][]int64) ([][]int64, error) {

	//Check that a user is logged
	userID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	err = app.repository.IsTabAccessAllowed(ctx, userID, tabID)
	if err != nil {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return nil, errors.Wrap(err, "access by "+userID)
		}
	}

	//Update the tab layout
	err = app.repository.UpdateTabLayout(ctx, tabID, layout)
	if err != nil {
		return nil, errors.Wrap(err, "saving tab in datastore failed")
	}

	return layout, nil
}

//PreviewItem contains the basic information for a retrieved post
type PreviewItem struct {
	Title     string    `json:"title"`
	Published time.Time `json:"published"`
	Link      string    `json:"link"`
}

//PreviewResult contains the basic information for a retrieved feed
type PreviewResult struct {
	Title string        `json:"title"`
	Items []PreviewItem `json:"items"`
}

//Preview returns the content of the feed at the given URL
func (app App) Preview(ctx context.Context, URL string) (PreviewResult, error) {

	//Check that a user is logged
	_, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return PreviewResult{}, errors.Wrap(err, "retrieving current user failed")
	}

	//Get external feed
	fp := gofeed.NewParser()
	extFeed, err := fp.ParseURL(URL)
	if err != nil {
		return PreviewResult{}, errors.Wrap(err, "retrieving feed failed")
	}

	var res PreviewResult
	res.Title = extFeed.Title

	for _, item := range extFeed.Items {

		if item.PublishedParsed == nil {
			tNow := time.Now()
			item.PublishedParsed = &tNow
		}

		res.Items = append(res.Items, PreviewItem{
			Title:     item.Title,
			Published: *item.PublishedParsed,
			Link:      item.Link,
		})
	}

	return res, nil
}

func (app App) feed(ctx context.Context, feedID int64, loadItems bool) (api.Feed, []api.FeedItem, error) {

	//Get the feed from datastore
	feed, err := app.repository.GetFeed(ctx, feedID)
	if err != nil {
		return feed, nil, errors.Wrap(err, "retrieving feed from datastore failed")
	}

	//Retrieve latest version
	tNow := time.Now()

	if tNow.After(feed.NextRetrieval) {

		fp := gofeed.NewParser()
		extFeed, err := fp.ParseURL(feed.URL)
		if err != nil {
			return feed, nil, errors.Wrap(err, "retrieving feed failed")
		}

		feed.NextRetrieval = tNow.Add(time.Duration(15) * time.Minute) //TODO get this from http client
		feed.Title = extFeed.Title

		feedItems := make([]api.FeedItem, 0, len(extFeed.Items))
		for _, extItem := range extFeed.Items {

			if extItem.PublishedParsed == nil {
				tNow := time.Now()
				extItem.PublishedParsed = &tNow
			}

			feedItems = append(feedItems, api.FeedItem{
				GUID:      extItem.GUID,
				Title:     extItem.Title,
				Published: *extItem.PublishedParsed,
				Link:      extItem.Link,
			})
		}

		//Store in datastore
		go func() {
			err := app.repository.StoreFeed(context.Background(), &feed, feedItems)
			if err != nil {
				app.Error(ctx, errors.Wrap(err, "storage of feed failed"))
			}
		}()

		return feed, feedItems, nil
	}

	var feedItems []api.FeedItem
	if loadItems {
		feedItems, err = app.repository.GetFeedItems(ctx, feedID)
		if err != nil {
			return feed, nil, errors.Wrap(err, "retrieving feed items from datastore failed")
		}
	}

	return feed, feedItems, nil
}

//Widget returns the widget configuration
func (app App) Widget(ctx context.Context, tabID int64, widgetID int64) (api.Widget, error) {

	tab, err := app.Tab(ctx, tabID)
	if err != nil {
		return api.Widget{}, errors.Wrap(err, "retrieving tab failed")
	}

	for _, l := range tab.Widgets {
		for _, w := range l {
			if w.ID == widgetID {
				return w, nil
			}
		}
	}

	return api.Widget{}, errors.Wrap(errors.New("widget not found"), "invalid widget id") //TODO: manage in datastore or send a NotFound error
}

//FeedItems returns the items of a feed and the reading status for the given user
func (app App) FeedItems(ctx context.Context, userID string, feedID int64) ([]api.ItemForUser, error) {

	app.Infof(ctx, "Getting items for %s feed %d", userID, feedID)

	//Check that a user is logged
	loggedInUserID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	if userID != loggedInUserID {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return nil, errors.Wrap(notAuthorized("access denied to user: "+userID), "access by "+loggedInUserID)
		}
	}

	//Get the feed from datastore and/or URL
	feed, feeditems, err := app.feed(ctx, feedID, true)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving feed items failed")
	}

	//Get the read status
	count := len(feeditems)
	if count == 0 {
		return nil, errors.New("No items in feed " + feed.URL)
	}
	if count > 100 { //Arbritary limitation to avoid memory bump
		count = 100
	}
	guids := make([]string, count)
	for itemIdx := 0; itemIdx < count; itemIdx++ {
		guids[itemIdx] = feeditems[itemIdx].GUID
	}
	readStatus, err := app.repository.AreItemsRead(ctx, userID, feedID, guids)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving reading status failed")
	}

	var items []api.ItemForUser

	for itemIdx := 0; itemIdx < count; itemIdx++ {

		read := false
		if itemIdx < len(readStatus) {
			read = readStatus[itemIdx]
		}

		items = append(items, api.ItemForUser{
			FeedItem: feeditems[itemIdx],
			Read:     read,
		})
	}

	app.Infof(ctx, "Done with %d items", len(items))
	return items, nil
}

//MarkAsRead marks one or multiple feed items as read for the given user
func (app App) MarkAsRead(ctx context.Context, userID string, feedID int64, guids []string) error {

	//Check that a user is logged
	loggedInUserID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	if userID != loggedInUserID {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return errors.Wrap(notAuthorized("access denied to user: "+userID), "access by "+loggedInUserID)
		}
	}

	//Store th new status in datastore
	for _, guid := range guids {
		err = app.repository.SetItemRead(ctx, userID, feedID, guid, true)
		if err != nil {
			return errors.Wrap(err, "saving read status failed")
		}
	}

	return nil
}

//GetEmails returns the list of email in a given account
func (app App) GetEmails(ctx context.Context, userID string, accountID int64) (*api.EmailPage, error) {

	app.Infof(ctx, "Getting items for %s feed %d", userID, accountID)

	//Check that a user is logged
	loggedInUserID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving current user failed")
	}

	//Check authorization
	if userID != loggedInUserID {
		if !app.userInteractor.CurrentUserIsAdmin(ctx) {
			return nil, errors.Wrap(notAuthorized("access denied to user: "+userID), "access by "+loggedInUserID)
		}
	}

	//Get the account from datastore
	account, err := app.repository.GetAccount(ctx, userID, accountID)
	if err != nil {
		return nil, errors.Wrap(err, "retrieving account failed")
	}

	//Get the provider
	emailProvider, err := app.getEmailProvider(account.ProviderName)
	if err != nil {
		return nil, errors.Wrap(err, "Email provider not found")
	}

	return emailProvider.GetItems(ctx, account, api.EmailQuery{}, nil)
}

func (app App) getEmailProvider(serviceName string) (api.EmailProvider, error) {

	provider, ok := app.providers[serviceName]

	if !ok {
		return nil, errors.New("Unknown service: " + serviceName)
	}

	emailProvider, ok := provider.(api.EmailProvider)
	if !ok {
		return nil, errors.New("Email service not available: " + serviceName)
	}

	return emailProvider, nil
}

func (app App) getServiceConfig(serviceName string) (*oauth2.Config, error) {

	provider, ok := app.providers[serviceName]

	if !ok {
		return nil, errors.New("Unknown service: " + serviceName)
	}

	return provider.Config(), nil
}

//ServiceRegister computes the AuthCodeURL for the given service
func (app App) ServiceRegister(ctx context.Context, serviceName string) (string, error) {

	//Check that a user is logged
	loggedInUserID, err := app.userInteractor.CurrentUserID(ctx)
	if err != nil {
		return "", errors.Wrap(err, "retrieving current user failed")
	}

	//Generate code
	randState := fmt.Sprintf("oki%d", time.Now().UnixNano())

	//Store it
	err = app.repository.StoreTemporaryCode(ctx, loggedInUserID, serviceName, randState)
	if err != nil {
		return "", errors.Wrap(err, "saving temporary code failed")
	}

	//Get the URL
	config, err := app.getServiceConfig(serviceName)
	if err != nil {
		return "", errors.Wrap(err, "Unable to retrieve service configuration")
	}
	authURL := config.AuthCodeURL(randState, oauth2.AccessTypeOffline)
	fmt.Println("AuthCodeURL", authURL)

	return authURL, nil
}

//HandleOauth2Callback manages the Oauth2 flow and creates a new account for the user who started the flow.
func (app App) HandleOauth2Callback(ctx context.Context, serviceName string, state, code string) error {

	//Check state
	userID, err := app.repository.GetUserFromTemporaryCode(ctx, serviceName, state)
	if err != nil {
		return errors.Wrap(err, "retrieving user failed")
	}

	if len(userID) == 0 {
		return errors.Wrap(notAuthorized("access denied"), "invalid oauth2 state")
	}

	if code == "" {
		return errors.New("Empty code received")
	}

	//Get the provider
	emailProvider, err := app.getEmailProvider(serviceName)
	if err != nil {
		return errors.Wrap(err, "Email provider not found")
	}

	token, err := emailProvider.Config().Exchange(ctx, code)
	if err != nil {
		return errors.Wrap(err, "Exchange failed")
	}

	err = app.repository.DeleteTemporaryCode(ctx, userID, serviceName)
	if err != nil {
		return errors.Wrap(err, "erasing temporary code failed")
	}

	app.logInteractor.Infof(ctx, "New account on %s for %s: %v", serviceName, userID, *token)

	account := api.ExternalAccount{
		ProviderName: serviceName,
		Token:        token,
	}

	email, err := emailProvider.GetCurrentEmailAddress(ctx, account)
	if err != nil {
		return errors.Wrap(err, "retrieving email failed")
	}

	account.AccountID = email

	err = app.repository.StoreAccount(ctx, userID, &account)
	if err != nil {
		return errors.Wrap(err, "saving token failed")
	}

	return nil
}
