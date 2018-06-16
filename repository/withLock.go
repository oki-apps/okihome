package repository

import (
	"context"
	"log"
	"sync"

	"github.com/oki-apps/okihome/api"
)

//WithLock wraps a repository with read/write locking mechanism
func WithLock(r api.Repository) api.Repository {
	return &lockedRepo{
		repo: r,
	}
}

type lockedRepo struct {
	repo    api.Repository
	rwMutex sync.RWMutex
}

func (r *lockedRepo) IsNotFound(err error) bool {
	return r.repo.IsNotFound(err)
}

func (r *lockedRepo) rlock(args ...interface{}) {
	log.Println("Waiting for read lock", args)
	r.rwMutex.RLock()
	log.Println("Read lock", args)
}
func (r *lockedRepo) runlock(args ...interface{}) {
	r.rwMutex.RUnlock()
	log.Println("Read unlock", args)
}
func (r *lockedRepo) lock(args ...interface{}) {
	log.Println("Waiting for write lock", args)
	r.rwMutex.Lock()
	log.Println("Write lock", args)
}
func (r *lockedRepo) unlock(args ...interface{}) {
	r.rwMutex.Unlock()
	log.Println("Write unlock", args)
}

func (r *lockedRepo) GetUser(ctx context.Context, userID string) (api.User, error) {
	r.rlock("GetUser", userID)
	defer r.runlock("GetUser", userID)
	return r.repo.GetUser(ctx, userID)
}
func (r *lockedRepo) StoreUser(ctx context.Context, user *api.User) error {
	r.lock("StoreUSer")
	defer r.unlock("StoreUSer")
	return r.repo.StoreUser(ctx, user)
}

func (r *lockedRepo) GetTabs(ctx context.Context, userID string) ([]api.TabSummary, error) {
	r.rlock("GetTabs", userID)
	defer r.runlock("GetTabs", userID)
	return r.repo.GetTabs(ctx, userID)
}
func (r *lockedRepo) IsTabAccessAllowed(ctx context.Context, userID string, tabID int64) error {
	r.rlock("IsTabAccessAllowed", userID, tabID)
	defer r.runlock("IsTabAccessAllowed", userID, tabID)
	return r.repo.IsTabAccessAllowed(ctx, userID, tabID)
}
func (r *lockedRepo) AllowTabAccess(ctx context.Context, userID string, tabID int64) error {
	r.lock("AllowTabAccess", userID, tabID)
	defer r.unlock("AllowTabAccess", userID, tabID)
	return r.repo.AllowTabAccess(ctx, userID, tabID)
}

func (r *lockedRepo) GetTab(ctx context.Context, tabID int64) (api.Tab, error) {
	r.rlock("GetTab", tabID)
	defer r.runlock("GetTab", tabID)
	return r.repo.GetTab(ctx, tabID)
}
func (r *lockedRepo) StoreTab(ctx context.Context, tab *api.Tab) error {
	r.lock("StoreTab")
	defer r.unlock("StoreTab")
	return r.repo.StoreTab(ctx, tab)
}
func (r *lockedRepo) DeleteTab(ctx context.Context, tabID int64) error {
	r.lock("DeleteTab", tabID)
	defer r.unlock("DeleteTab", tabID)
	return r.repo.DeleteTab(ctx, tabID)
}

func (r *lockedRepo) GetWidget(ctx context.Context, tabID int64, widgetID int64) (api.Widget, error) {
	r.rlock("GetWidget", tabID, widgetID)
	defer r.runlock("GetWidget", tabID, widgetID)
	return r.repo.GetWidget(ctx, tabID, widgetID)
}
func (r *lockedRepo) StoreWidget(ctx context.Context, tabID int64, widget *api.Widget) error {
	r.lock("StoreWidget", tabID)
	defer r.unlock("StoreWidget", tabID)
	return r.repo.StoreWidget(ctx, tabID, widget)
}
func (r *lockedRepo) DeleteWidget(ctx context.Context, tabID int64, widgetID int64) error {
	r.lock("DeleteWidget", tabID, widgetID)
	defer r.unlock("DeleteWidget", tabID, widgetID)
	return r.repo.DeleteWidget(ctx, tabID, widgetID)
}

func (r *lockedRepo) UpdateTabLayout(ctx context.Context, tabID int64, layout [][]int64) error {
	r.lock("UpdateTabLayout", tabID)
	defer r.unlock("UpdateTabLayout", tabID)
	return r.repo.UpdateTabLayout(ctx, tabID, layout)
}
func (r *lockedRepo) DeleteWidgetFromTab(ctx context.Context, tabID int64, widgetID int64) error {
	r.lock("DeleteWidgetFromTab", tabID, widgetID)
	defer r.unlock("DeleteWidgetFromTab", tabID, widgetID)
	return r.repo.DeleteWidgetFromTab(ctx, tabID, widgetID)
}

func (r *lockedRepo) GetOrCreateFeedID(ctx context.Context, URL string) (int64, error) {
	r.lock("GetOrCreateFeedID", URL)
	defer r.unlock("GetOrCreateFeedID", URL)
	return r.repo.GetOrCreateFeedID(ctx, URL)
}
func (r *lockedRepo) GetFeed(ctx context.Context, feedID int64) (api.Feed, error) {
	r.rlock("GetFeed", feedID)
	defer r.runlock("GetFeed", feedID)
	return r.repo.GetFeed(ctx, feedID)
}
func (r *lockedRepo) GetFeedItems(ctx context.Context, feedID int64) ([]api.FeedItem, error) {
	r.rlock("GetFeedItems", feedID)
	defer r.runlock("GetFeedItems", feedID)
	return r.repo.GetFeedItems(ctx, feedID)
}
func (r *lockedRepo) StoreFeed(ctx context.Context, feed *api.Feed, feedItems []api.FeedItem) error {
	r.lock("StoreFeed")
	defer r.unlock("StoreFeed")
	return r.repo.StoreFeed(ctx, feed, feedItems)
}

func (r *lockedRepo) AreItemsRead(ctx context.Context, userID string, feedID int64, guids []string) ([]bool, error) {
	r.rlock("AreItemsRead", userID, feedID)
	defer r.runlock("AreItemsRead", userID, feedID)
	return r.repo.AreItemsRead(ctx, userID, feedID, guids)
}
func (r *lockedRepo) SetItemRead(ctx context.Context, userID string, feedID int64, guid string, read bool) error {
	r.lock("SetItemRead", userID, feedID, guid)
	defer r.unlock("SetItemRead", userID, feedID, guid)
	return r.repo.SetItemRead(ctx, userID, feedID, guid, read)
}
func (r *lockedRepo) SetItemsRead(ctx context.Context, userID string, feedID int64, guid []string, read bool) error {
	r.lock("SetItemsRead", userID, feedID)
	defer r.unlock("SetItemsRead", userID, feedID)
	return r.repo.SetItemsRead(ctx, userID, feedID, guid, read)
}

func (r *lockedRepo) GetAccount(ctx context.Context, userID string, accountID int64) (api.ExternalAccount, error) {
	r.rlock("GetAccount", userID, accountID)
	defer r.runlock("GetAccount", userID, accountID)
	return r.repo.GetAccount(ctx, userID, accountID)
}
func (r *lockedRepo) GetAccounts(ctx context.Context, userID string) ([]api.ExternalAccount, error) {
	r.rlock("GetAccounts", userID)
	defer r.runlock("GetAccounts", userID)
	return r.repo.GetAccounts(ctx, userID)
}
func (r *lockedRepo) DeleteAccount(ctx context.Context, userID string, accountID int64) error {
	r.lock("DeleteAccount", userID, accountID)
	defer r.unlock("DeleteAccount", userID, accountID)
	return r.repo.DeleteAccount(ctx, userID, accountID)
}
func (r *lockedRepo) StoreAccount(ctx context.Context, userID string, account *api.ExternalAccount) error {
	r.lock("StoreAccount", userID)
	defer r.unlock("StoreAccount", userID)
	return r.repo.StoreAccount(ctx, userID, account)
}

func (r *lockedRepo) GetUserFromTemporaryCode(ctx context.Context, serviceName string, code string) (string, error) {
	r.rlock("GetUserFromTemporaryCode", serviceName)
	defer r.runlock("GetUserFromTemporaryCode", serviceName)
	return r.repo.GetUserFromTemporaryCode(ctx, serviceName, code)
}
func (r *lockedRepo) StoreTemporaryCode(ctx context.Context, userID string, serviceName string, code string) error {
	r.lock("StoreTemporaryCode", userID, serviceName)
	defer r.unlock("StoreTemporaryCode", userID)
	return r.repo.StoreTemporaryCode(ctx, userID, serviceName, code)
}
func (r *lockedRepo) DeleteTemporaryCode(ctx context.Context, userID string, serviceName string) error {
	r.lock("DeleteTemporaryCode", userID, serviceName)
	defer r.unlock("DeleteTemporaryCode", userID, serviceName)
	return r.repo.DeleteTemporaryCode(ctx, userID, serviceName)
}

func (r *lockedRepo) GetEmailItem(ctx context.Context, account api.ExternalAccount, guid string, minVersion uint64) (api.EmailItem, error) {
	r.rlock("GetEmailItem")
	defer r.runlock("GetEmailItem")
	return r.repo.GetEmailItem(ctx, account, guid, minVersion)
}
func (r *lockedRepo) StoreEmailItem(ctx context.Context, account api.ExternalAccount, version uint64, item api.EmailItem) error {
	r.lock("StoreEmailItem")
	defer r.unlock("StoreEmailItem")
	return r.repo.StoreEmailItem(ctx, account, version, item)
}
