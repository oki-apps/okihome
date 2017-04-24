// Copyright 2017 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"golang.org/x/oauth2"

	"github.com/jmoiron/sqlx"
	"github.com/pkg/errors"

	"github.com/oki-apps/okihome/api"
)

var rwMutex sync.RWMutex

//Config is the configuration to access the SQLite database
type Config struct {
	DriverName       string
	ConnectionString string
}

//New creates a new repository that stores data in a SQLite database
func New(cfg Config) (api.Repository, error) {

	db, err := sqlx.Connect(cfg.DriverName, cfg.ConnectionString)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to connect to database")
	}

	r := &repo{
		DB: db,
		Tx: nil,
	}
	return r, nil
}

type repo struct {
	DB *sqlx.DB
	Tx *sqlx.Tx
}

func (r *repo) runInTransaction(ctx context.Context, f func(repo api.Repository) error) error {

	if r.Tx != nil {
		return errors.New("Nested transactions are prohibited")
	}

	tx, err := r.DB.Beginx()
	if err != nil {
		return errors.Wrap(err, "Unable to start transaction")
	}
	defer tx.Rollback()

	txRepo := *r
	txRepo.Tx = tx

	err = f(&txRepo)

	if err != nil {
		if err2 := tx.Rollback(); err2 != nil {
			return errors.Wrap(err, "Rollback failed")
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return errors.Wrap(err, "Commit failed")
	}

	return nil
}

func (r *repo) IsNotFound(err error) bool {

	return errors.Cause(err) == sql.ErrNoRows

}

func (r *repo) Queryer() sqlx.Queryer {
	if r.Tx != nil {
		return r.Tx
	}

	return r.DB
}
func (r *repo) Execer() sqlx.Execer {
	if r.Tx != nil {
		return r.Tx
	}

	return r.DB
}

func (r *repo) GetUser(ctx context.Context, userID string) (api.User, error) {

	log.Println("GetUser", "Waiting for lock", userID)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("GetUser", "Lock", userID)

	var u api.User
	err := sqlx.Get(
		r.Queryer(), &u,
		"SELECT id, display_name, email, isadmin FROM t_user WHERE id=$1",
		userID)

	if err != nil {
		log.Printf("GetUser failed: %+v", err)
		return api.User{}, errors.Wrap(err, "Fetching user failed")
	}

	log.Println("GetUser", "UnLock", userID)
	return u, nil
}

func (r *repo) StoreUser(ctx context.Context, user *api.User) error {

	log.Println("StoreUser", "Waiting for lock")
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("StoreUser", "Lock")

	_, err := r.Execer().Exec(
		"INSERT INTO t_user(id,display_name,email,isadmin) VALUES ($1,$2,$3,$4)",
		user.UserID, user.DisplayName, user.Email, user.IsAdmin)
	if err != nil {
		return errors.Wrap(err, "Inserting user failed")
	}

	log.Println("StoreUser", "UnLock")
	return nil
}

func (r *repo) GetTabs(ctx context.Context, userID string) ([]api.TabSummary, error) {

	log.Println("GetTabs", "Waiting for lock", userID)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("GetTabs", "Lock", userID)

	var tabs []api.TabSummary

	err := sqlx.Select(
		r.Queryer(), &tabs,
		`SELECT t_tab.id, t_tab.title 
FROM t_tab 
JOIN tj_tabaccess ON t_tab.id = tj_tabaccess.tab_id 
WHERE tj_tabaccess.user_id=$1`,
		userID)

	if err != nil {
		return nil, errors.Wrap(err, "Fetching tabs failed")
	}

	log.Println("GetTabs", "UnLock", userID)
	return tabs, nil
}
func (r *repo) IsTabAccessAllowed(ctx context.Context, userID string, tabID int64) error {

	log.Println("IsTabAccessAllowed", "Waiting for lock", userID, tabID)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("IsTabAccessAllowed", "Lock", userID, tabID)

	var count int64
	err := sqlx.Get(
		r.Queryer(), &count,
		`SELECT count(*) FROM tj_tabaccess WHERE user_id=$1 AND tab_id=$2`,
		userID, tabID)

	if err != nil {
		return errors.Wrap(err, "Checking tab access failed")
	}

	if count != 1 {
		return errors.New("Tab access not allowed")
	}

	log.Println("IsTabAccessAllowed", "UnLock", userID, tabID)
	return nil

}
func (r *repo) AllowTabAccess(ctx context.Context, userID string, tabID int64) error {

	log.Println("AllowTabAccess", "Waiting for lock", userID, tabID)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("AllowTabAccess", "Lock", userID, tabID)

	_, err := r.Execer().Exec(
		"INSERT INTO tj_tabaccess(user_id,tab_id) VALUES ($1,$2)",
		userID, tabID)

	if err != nil {
		return errors.Wrap(err, "Adding tab access failed")
	}

	log.Println("AllowTabAccess", "UnLock", userID, tabID)
	return nil
}

func (r *repo) GetTab(ctx context.Context, tabID int64) (api.Tab, error) {

	log.Println("GetTab", "Waiting for lock", tabID)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("GetTab", "Lock", tabID)

	var t struct {
		api.Tab
		Layout []byte `db:"layout"`
	}

	//Get the tab
	err := sqlx.Get(
		r.Queryer(), &t,
		`SELECT id, title, layout FROM t_tab WHERE id=$1`,
		tabID)

	if err != nil {
		return api.Tab{}, errors.Wrap(err, "Retrieving tab failed")
	}

	//Get the widgets
	if t.Layout != nil {
		widgetIDs := [][]int64{}
		err := json.Unmarshal(t.Layout, &widgetIDs)
		if err != nil {
			return api.Tab{}, errors.Wrap(err, "Retrieving tab widgets layout failed")
		}

		t.Tab.Widgets = make([][]api.Widget, len(widgetIDs))

		for i, col := range widgetIDs {
			t.Tab.Widgets[i] = make([]api.Widget, len(col))

			for j, id := range col {

				widget, err := r.getWidgetWithoutLock(ctx, tabID, id)
				if err != nil {
					return api.Tab{}, errors.Wrap(err, "Retrieving widget failed")
				}

				t.Tab.Widgets[i][j] = widget
			}
		}

	}

	log.Println("GetTab", "UnLock", tabID)
	return t.Tab, nil
}
func (r *repo) StoreTab(ctx context.Context, tab *api.Tab) error {

	log.Println("StoreTab", "Waiting for lock")
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("StoreTab", "Lock")

	layout := "["
	for i, col := range tab.Widgets {
		if i > 0 {
			layout += ","
		}
		layout += "["
		for j, w := range col {
			if j > 0 {
				layout += ","
			}
			layout += fmt.Sprint(w.ID)
		}
		layout += "]"
	}
	layout += "]"

	if tab.ID > 0 {
		//Update
		_, err := r.Execer().Exec(
			"UPDATE t_tab SET title=$1, layout=$2 WHERE id=$3",
			tab.Title, layout, tab.ID)
		if err != nil {
			return errors.Wrap(err, "Updating tab failed "+layout)
		}
	} else {
		//Insert
		res, err := r.Execer().Exec(
			"INSERT INTO t_tab(title,layout) VALUES ($1,$2)",
			tab.Title, layout)
		if err != nil {
			return errors.Wrap(err, "Inserting tab failed")
		}
		tab.ID, err = res.LastInsertId()
		if err != nil {
			return errors.Wrap(err, "Retreieving inserted tab ID failed")
		}
	}

	log.Println("StoreTab", "UnLock")
	return nil
}

func (r *repo) DeleteTab(ctx context.Context, tabID int64) error {

	log.Println("DeleteTab", "Waiting for lock", tabID)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("DeleteTab", "Lock", tabID)

	_, err := r.Execer().Exec(
		"DELETE FROM t_tab WHERE id=$1",
		tabID)
	if err != nil {
		return errors.Wrap(err, "Removing tab failed")
	}
	log.Println("DeleteTab", "UnLock", tabID)
	return nil
}
func (r *repo) GetWidget(ctx context.Context, tabID int64, widgetID int64) (api.Widget, error) {

	log.Println("GetWidget", "Waiting for lock", tabID, widgetID)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("GetWidget", "Lock", tabID, widgetID)

	w, err := r.getWidgetWithoutLock(ctx, tabID, widgetID)

	log.Println("GetWidget", "UnLock", tabID, widgetID)

	return w, err
}

func (r *repo) getWidgetWithoutLock(ctx context.Context, tabID int64, widgetID int64) (api.Widget, error) {

	var w struct {
		Cfg []byte `db:"cfg"`
		api.Widget
	}
	err := sqlx.Get(
		r.Queryer(), &w,
		`SELECT id, type, config as cfg FROM t_widget WHERE id=$1 and tab_id=$2`,
		widgetID, tabID)

	//Create empty config based on type
	switch w.Widget.Type {
	case api.WidgetFeedType:
		config := api.ConfigFeed{}

		err = json.Unmarshal(w.Cfg, &config)
		if err != nil {
			return api.Widget{}, errors.Wrap(err, "Unmarshaling widget config failed")
		}

		w.Widget.Config = config

	case api.WidgetEmailType:
		config := api.ConfigEmail{}

		err = json.Unmarshal(w.Cfg, &config)
		if err != nil {
			return api.Widget{}, errors.Wrap(err, "Unmarshaling widget config failed")
		}

		w.Widget.Config = config

	}

	return w.Widget, nil
}

func (r *repo) StoreWidget(ctx context.Context, tabID int64, widget *api.Widget) error {

	log.Println("StoreWidget", "Waiting for lock", tabID)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("StoreWidget", "Lock", tabID)

	configJSON, err := json.Marshal(widget.Config)
	if err != nil {
		return errors.Wrap(err, "Marshaling widget config failed")
	}

	if widget.ID > 0 {
		//Update
		_, err := r.Execer().Exec(
			"UPDATE t_widget SET type=$1,config=$2 WHERE id=$3 AND tab_id=$4",
			widget.Type, configJSON, widget.ID, tabID)
		if err != nil {
			return errors.Wrap(err, "Updating widget failed")
		}
	} else {
		//Insert
		res, err := r.Execer().Exec(
			"INSERT INTO t_widget(type,config,tab_id) VALUES ($1,$2,$3)",
			widget.Type, configJSON, tabID)
		if err != nil {
			return errors.Wrap(err, "Inserting widget failed")
		}
		widget.ID, err = res.LastInsertId()
		if err != nil {
			return errors.Wrap(err, "Retrieving last inserted widget ID failed")
		}
	}

	log.Println("StoreWidget", "UnLock", tabID)
	return nil
}

func (r *repo) DeleteWidget(ctx context.Context, tabID int64, widgetID int64) error {

	log.Println("DeleteWidget", "Waiting for lock", tabID, widgetID)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("DeleteWidget", "Lock", tabID, widgetID)

	_, err := r.Execer().Exec(
		"DELETE FROM t_widget WHERE id=$1 AND tab_id=$2",
		widgetID, tabID)
	if err != nil {
		return errors.Wrap(err, "Removing widget failed")
	}
	log.Println("DeleteWidget", "UnLock", tabID, widgetID)
	return nil
}

func (r *repo) UpdateTabLayout(ctx context.Context, tabID int64, layout [][]int64) error {
	return r.runInTransaction(ctx, func(repo api.Repository) error {

		tab, err := repo.GetTab(ctx, tabID)
		if err != nil {
			return errors.Wrap(err, "retrieving tab from datastore failed")
		}

		allWidgets := make(map[int64]api.Widget)
		for _, column := range tab.Widgets {
			for _, w := range column {
				allWidgets[w.ID] = w
			}
		}

		tab.Widgets = nil

		for _, column := range layout {
			newCol := []api.Widget{}

			for _, widgetID := range column {
				w, ok := allWidgets[widgetID]
				if !ok {
					return errors.New("Unable to find widget in tab")
				}
				newCol = append(newCol, w)
				delete(allWidgets, widgetID)
			}

			tab.Widgets = append(tab.Widgets, newCol)
		}

		if len(allWidgets) > 0 {
			return errors.New("Not all widgets used in new layout")
		}

		err = repo.StoreTab(ctx, &tab)
		if err != nil {
			return errors.Wrap(err, "saving tab in datastore failed")
		}

		return nil
	})
}

func (r *repo) DeleteWidgetFromTab(ctx context.Context, tabID int64, widgetID int64) error {

	return r.runInTransaction(ctx, func(repo api.Repository) error {

		found := false

		tab, err := repo.GetTab(ctx, tabID)
		if err != nil {
			return errors.Wrap(err, "retrieving tab from datastore failed")
		}

		iFound, jFound := 0, 0
		for i, column := range tab.Widgets {
			for j, w := range column {
				if w.ID == widgetID {
					iFound = i
					jFound = j
					found = true
				}
			}
		}

		if !found {
			return errors.New("widget not found")
		}

		tab.Widgets[iFound] = append(tab.Widgets[iFound][:jFound], tab.Widgets[iFound][jFound+1:]...)

		err = repo.StoreTab(ctx, &tab)
		if err != nil {
			return errors.Wrap(err, "saving tab in datastore failed")
		}

		return nil
	})
}

func (r *repo) GetOrCreateFeedID(ctx context.Context, URL string) (int64, error) {

	log.Println("GetOrCreateFeedID", "Waiting for lock", URL)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("GetOrCreateFeedID", "Lock", URL)

	var feedID int64
	err := sqlx.Get(
		r.Queryer(), &feedID,
		`SELECT id FROM t_feed WHERE url=$1`,
		URL)

	if err == nil {
		return feedID, nil
	}

	if err != sql.ErrNoRows {
		return 0, errors.Wrap(err, "Getting feed failed")
	}

	res, err := r.Execer().Exec(
		"INSERT INTO t_feed(url,next_retrieval) VALUES ($1,(date('now')))",
		URL)
	if err != nil {
		return 0, errors.Wrap(err, "Inserting feed failed")
	}
	feedID, err = res.LastInsertId()
	if err != nil {
		return 0, errors.Wrap(err, "Retreiving last inserted feed ID failed")
	}

	log.Println("GetOrCreateFeedID", "UnLock", URL)
	return feedID, nil

}

func (r *repo) GetFeed(ctx context.Context, feedID int64) (api.Feed, error) {

	log.Println("GetFeed", "Waiting for lock", feedID)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("GetFeed", "Lock", feedID)

	var feed struct {
		ID            int64          `db:"id"`
		URL           string         `db:"url"`
		NextRetrieval sql.NullString `db:"next_retrieval"`
		Title         *string        `db:"title"`
	}

	//Get the feed
	err := sqlx.Get(
		r.Queryer(), &feed,
		`SELECT id, url, next_retrieval, title FROM t_feed WHERE id=$1`,
		feedID)

	if err != nil {
		return api.Feed{}, errors.Wrap(err, "Retrieving feed failed")
	}

	var f api.Feed
	f.ID = feed.ID
	f.URL = feed.URL
	if feed.NextRetrieval.Valid {
		t, err := time.Parse("2006-01-02 15:04:05", feed.NextRetrieval.String)
		if err != nil {
			f.NextRetrieval = t
		}
	}
	if feed.Title != nil {
		f.Title = *feed.Title
	}

	log.Println("GetFeed", "UnLock", feedID)
	return f, nil
}

func (r *repo) GetFeedItems(ctx context.Context, feedID int64) ([]api.FeedItem, error) {

	log.Println("GetFeedItems", "Waiting for lock", feedID)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("GetFeedItems", "Lock", feedID)

	var items []api.FeedItem

	//Get the feed
	err := sqlx.Select(
		r.Queryer(), &items,
		`SELECT guid, title, published, link FROM t_feeditem WHERE feed_id=$1 ORDER BY published DESC`,
		feedID)

	if err != nil {
		return nil, errors.Wrap(err, "Retrieving feed items failed")
	}

	log.Println("GetFeedItems", "UnLock", feedID)
	return items, nil
}
func (r *repo) StoreFeed(ctx context.Context, feed *api.Feed, feedItems []api.FeedItem) error {

	log.Println("StoreFeed", "Waiting for lock")
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("StoreFeed", "Lock")

	if feed.ID > 0 {
		//Update
		_, err := r.Execer().Exec(
			"UPDATE t_feed SET url=$1, next_retrieval=$2, title=$3 WHERE id=$4",
			feed.URL, feed.NextRetrieval, feed.Title, feed.ID)
		if err != nil {
			return errors.Wrap(err, "Updating feed failed")
		}

		_, err = r.Execer().Exec(
			"DELETE FROM t_feeditem WHERE feed_id=$1",
			feed.ID)
		if err != nil {
			return errors.Wrap(err, "Cleaning existing feed items failed")
		}

	} else {
		//Insert
		res, err := r.Execer().Exec(
			"INSERT INTO t_feed(url, next_retrieval, title) VALUES ($1,$2,$3)",
			feed.URL, feed.NextRetrieval, feed.Title)
		if err != nil {
			return errors.Wrap(err, "Inserting feed failed")
		}
		feed.ID, err = res.LastInsertId()
		if err != nil {
			return errors.Wrap(err, "Retrieving last inserted feed ID failed")
		}
	}

	//Store or update items
	for _, item := range feedItems {

		_, err := r.Execer().Exec(
			"INSERT INTO t_feeditem (feed_id, guid, title, published, link) VALUES ($1,$2,$3,$4,$5)",
			feed.ID, item.GUID, item.Title, item.Published, item.Link)
		if err != nil {
			return errors.Wrap(err, "Inserrting new feed items failed")
		}

	}

	log.Println("StoreFeed", "UnLock")
	return nil
}

func (r *repo) AreItemsRead(ctx context.Context, userID string, feedID int64, guids []string) ([]bool, error) {

	log.Println("AreItemsRead", "Waiting for lock", userID, feedID)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("AreItemsRead", "Lock", userID, feedID)

	res := make([]bool, len(guids))

	for i, guid := range guids {
		read := false
		err := sqlx.Get(
			r.Queryer(), &read,
			"SELECT read FROM tj_feeditem_user WHERE user_id=$1 AND feed_id=$2 AND guid=$3",
			userID, feedID, guid)
		if err != nil && err != sql.ErrNoRows {
			return nil, errors.Wrap(err, "Getting read status failed")
		}

		res[i] = read
	}

	log.Println("AreItemsRead", "UnLock", userID, feedID)
	return res, nil
}
func (r *repo) setItemRead(ctx context.Context, userID string, feedID int64, guid string, read bool) error {

	err := sqlx.Get(
		r.Queryer(), &read,
		"SELECT read FROM tj_feeditem_user WHERE user_id=$1 AND feed_id=$2 AND guid=$3",
		userID, feedID, guid)
	if err != nil && err != sql.ErrNoRows {
		return errors.Wrap(err, "Getting read status failed")
	}

	if err == sql.ErrNoRows {
		_, err := r.Execer().Exec(
			"INSERT INTO tj_feeditem_user (user_id, feed_id, guid, read) VALUES ($1,$2,$3,$4)",
			userID, feedID, guid, read)
		if err != nil {
			return errors.Wrap(err, "Inserting read status failed")
		}
	} else {
		_, err := r.Execer().Exec(
			"UPDATE tj_feeditem_user SET read=$4 WHERE user_id=$1 AND feed_id=$2 AND guid=$3",
			userID, feedID, guid, read)
		if err != nil {
			return errors.Wrap(err, "Updating read status failed")
		}
	}

	return nil
}
func (r *repo) SetItemRead(ctx context.Context, userID string, feedID int64, guid string, read bool) error {

	log.Println("SetItemRead", "Waiting for lock", userID, feedID, guid)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("SetItemRead", "Lock", userID, feedID, guid)

	err := r.setItemRead(ctx, userID, feedID, guid, read)
	if err != nil {
		return err
	}

	log.Println("SetItemRead", "UnLock", userID, feedID, guid)
	return nil
}

func (r *repo) SetItemsRead(ctx context.Context, userID string, feedID int64, guids []string, read bool) error {

	log.Println("SetItemsRead", "Waiting for lock", userID, feedID)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("SetItemsRead", "Lock", userID, feedID)

	for _, guid := range guids {
		err := r.setItemRead(ctx, userID, feedID, guid, read)
		if err != nil {
			return err
		}
	}

	log.Println("SetItemsRead", "UnLock", userID, feedID)
	return nil
}

func (r *repo) GetAccount(ctx context.Context, userID string, accountID int64) (api.ExternalAccount, error) {

	log.Println("GetAccount", "Waiting for lock", userID, accountID)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("GetAccount", "Lock", userID, accountID)

	var acc struct {
		Tokenjson []byte `db:"tokenjson"`
		api.ExternalAccount
	}
	err := sqlx.Get(
		r.Queryer(), &acc,
		`SELECT t_account.id, t_account.provider, t_account.account_id, t_account.token as tokenjson
FROM t_account 
WHERE t_account.id=$1 AND t_account.user_id=$2`,
		accountID, userID)

	if err != nil {
		return api.ExternalAccount{}, errors.Wrap(err, "Retrieving account failed")
	}

	acc.ExternalAccount.Token = &oauth2.Token{}
	err = json.Unmarshal(acc.Tokenjson, &acc.ExternalAccount.Token)
	if err != nil {
		return api.ExternalAccount{}, errors.Wrap(err, "Unmarshaling account token failed")
	}

	return acc.ExternalAccount, nil
}
func (r *repo) GetAccounts(ctx context.Context, userID string) ([]api.ExternalAccount, error) {

	log.Println("GetAccounts", "Waiting for lock", userID)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("GetAccounts", "Lock", userID)

	accounts := []struct {
		Tokenjson []byte `db:"tokenjson"`
		api.ExternalAccount
	}{}

	err := sqlx.Select(
		r.Queryer(), &accounts,
		`SELECT t_account.id, t_account.provider, t_account.account_id, t_account.token as tokenjson
FROM t_account 
WHERE t_account.user_id=$1`,
		userID)

	if err != nil {
		return nil, errors.Wrap(err, "Fetching accounts failed")
	}

	res := make([]api.ExternalAccount, len(accounts))
	for i, acc := range accounts {

		acc.ExternalAccount.Token = &oauth2.Token{}
		err = json.Unmarshal(acc.Tokenjson, &acc.ExternalAccount.Token)
		if err != nil {
			return nil, errors.Wrap(err, "Unmarshaling account token failed")
		}

		res[i] = acc.ExternalAccount
	}

	return res, nil
}
func (r *repo) DeleteAccount(ctx context.Context, userID string, accountID int64) error {

	log.Println("DeleteAccount", "Waiting for lock", userID, accountID)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("DeleteAccount", "Lock", userID, accountID)

	_, err := r.Execer().Exec(
		"DELETE FROM t_account WHERE id=$1 AND t_account.user_id=$2",
		accountID, userID)
	if err != nil {
		return errors.Wrap(err, "Removing account failed")
	}

	return nil

}

func (r *repo) StoreAccount(ctx context.Context, userID string, account *api.ExternalAccount) error {

	log.Println("StoreAccount", "Waiting for lock", userID)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("StoreAccount", "Lock", userID)

	tokenJSON, err := json.Marshal(account.Token)
	if err != nil {
		return errors.Wrap(err, "Marshaling account token failed")
	}

	if account.ID > 0 {
		//Update
		_, err := r.Execer().Exec(
			"UPDATE t_account SET provider=$1, account_id=$2, token=$3 WHERE id=$4 AND user_id=$5",
			account.ProviderName, account.AccountID, tokenJSON, account.ID, userID)
		if err != nil {
			return errors.Wrap(err, "Updating account failed")
		}

	} else {
		//Insert
		res, err := r.Execer().Exec(
			"INSERT INTO t_account(provider, account_id, token, user_id) VALUES ($1,$2,$3,$4)",
			account.ProviderName, account.AccountID, tokenJSON, userID)
		if err != nil {
			return errors.Wrap(err, "Inserting account failed")
		}
		account.ID, err = res.LastInsertId()
		if err != nil {
			return errors.Wrap(err, "Retrieving last inserted account ID failed")
		}
	}

	return nil
}

func (r *repo) GetUserFromTemporaryCode(ctx context.Context, serviceName string, code string) (string, error) {

	log.Println("GetUserFromTemporaryCode", "Waiting for lock", serviceName, code)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("GetUserFromTemporaryCode", "Lock", serviceName, code)

	var userID string
	err := sqlx.Get(
		r.Queryer(), &userID,
		"SELECT user_id FROM t_temporarycode WHERE provider=$1 AND code=$2",
		serviceName, code)

	if err != nil {
		return "", errors.Wrap(err, "Retrieving user failed")
	}

	return userID, nil
}
func (r *repo) StoreTemporaryCode(ctx context.Context, userID string, serviceName string, code string) error {

	log.Println("StoreTemporaryCode", "Waiting for lock", userID, serviceName, code)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("StoreTemporaryCode", "Lock", userID, serviceName, code)

	_, err := r.Execer().Exec(
		"INSERT INTO t_temporarycode(user_id, provider, code) VALUES ($1,$2,$3)",
		userID, serviceName, code)

	if err != nil {
		return errors.Wrap(err, "Storing temporary code failed")
	}

	return nil
}
func (r *repo) DeleteTemporaryCode(ctx context.Context, userID string, serviceName string) error {

	log.Println("DeleteTemporaryCode", "Waiting for lock", userID, serviceName)
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("DeleteTemporaryCode", "Lock", userID, serviceName)

	_, err := r.Execer().Exec(
		"DELETE FROM t_temporarycode WHERE user_id=$1 AND provider=$2",
		userID, serviceName)

	if err != nil {
		return errors.Wrap(err, "Deleting temporary code failed")
	}

	return nil
}

func (r *repo) GetEmailItem(ctx context.Context, account api.ExternalAccount, guid string, minVersion uint64) (api.EmailItem, error) {

	log.Println("GetEmailItem", "Waiting for lock", guid)
	rwMutex.RLock()
	defer rwMutex.RUnlock()
	log.Println("GetEmailItem", "Lock", guid)

	var emailItem api.EmailItem
	err := sqlx.Get(
		r.Queryer(), &emailItem,
		`SELECT guid, title, published, link, sender, snippet, read
FROM t_emailitem WHERE account_id=$1 AND guid=$2 AND version>=$3`,
		account.ID, guid, minVersion)

	if err != nil {
		if err == sql.ErrNoRows {
			return api.EmailItem{}, nil
		}

		return api.EmailItem{}, errors.Wrap(err, "Retrieving item failed")
	}

	return emailItem, nil
}
func (r *repo) StoreEmailItem(ctx context.Context, account api.ExternalAccount, version uint64, item api.EmailItem) error {

	log.Println("StoreEmailItem", "Waiting for lock")
	rwMutex.Lock()
	defer rwMutex.Unlock()
	log.Println("StoreEmailItem", "Lock")

	var currentVersion uint64
	err := sqlx.Get(
		r.Queryer(), &currentVersion,
		`SELECT version
FROM t_emailitem WHERE account_id=$1 AND guid=$2`,
		account.ID, item.GUID)
	if err != nil && err != sql.ErrNoRows {
		return errors.Wrap(err, "Getting current version failed")
	}

	if err == sql.ErrNoRows {

		_, err := r.Execer().Exec(
			`INSERT INTO t_emailitem(account_id, guid, title, published, link, 
sender, snippet, read, version) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`,
			account.ID, item.GUID, item.Title, item.Published, item.Link,
			item.From, item.Snippet, item.Read, version)

		if err != nil {
			return errors.Wrap(err, "Storing email item failed")
		}

	} else if currentVersion < version {

		_, err := r.Execer().Exec(
			`UPDATE t_emailitem SET title=$3, published=$4, link=$5, 
sender=$6, snippet=$7, read=$8, version=$9
WHERE account_id=$1 AND guid=$2`,
			account.ID, item.GUID, item.Title, item.Published, item.Link,
			item.From, item.Snippet, item.Read, version)

		if err != nil {
			return errors.Wrap(err, "Updating email item failed")
		}

	}

	log.Println("StoreEmailItem", "UnLock")
	return nil
}
