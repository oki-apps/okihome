package datastore

import (
	"context"

	"cloud.google.com/go/datastore"
	"github.com/oki-apps/okihome/api"
	"github.com/pkg/errors"
)

type repo struct {
	datastoreClient *datastore.Client
	tx              *datastore.Transaction
	onCommit        map[*datastore.PendingKey]func(*datastore.Key)
}

func (r *repo) Get(ctx context.Context, key *datastore.Key, dst interface{}) error {
	if r.tx != nil {
		return r.tx.Get(key, dst)
	}

	return r.datastoreClient.Get(ctx, key, dst)
}

func (r *repo) Put(ctx context.Context, key *datastore.Key, src interface{}, onCommit func(key *datastore.Key)) error {
	if r.tx != nil {
		k, err := r.tx.Put(key, src)
		if err != nil {
			return err
		}

		if onCommit != nil {
			r.onCommit[k] = onCommit
		}
		return nil
	}

	k, err := r.datastoreClient.Put(ctx, key, src)
	if err != nil {
		return err
	}

	if onCommit != nil {
		onCommit(k)
	}

	return nil
}

func (r *repo) Delete(ctx context.Context, key *datastore.Key) error {
	if r.tx != nil {
		return r.tx.Delete(key)
	}

	return r.datastoreClient.Delete(ctx, key)
}

//New creates a new repository that stores data in an appengine datastore
func New(projectID string) (api.Repository, error) {

	ctx := context.Background()

	datastoreClient, err := datastore.NewClient(ctx, projectID)
	if err != nil {
		return nil, errors.Wrap(err, "Unable to create datastore client")
	}

	r := &repo{
		datastoreClient: datastoreClient,
		tx:              nil,
	}
	return r, nil
}

func (r *repo) IsNotFound(err error) bool {
	return err == datastore.ErrNoSuchEntity
}

func userKey(userID string) *datastore.Key {
	return datastore.NameKey("User", userID, nil)
}

func (r *repo) GetUser(ctx context.Context, userID string) (api.User, error) {

	var result api.User
	key := userKey(userID)

	err := r.Get(ctx, key, &result)

	return result, err
}

func (r *repo) StoreUser(ctx context.Context, user *api.User) error {

	key := datastore.NameKey("User", user.UserID, nil)

	return r.Put(ctx, key, user, nil)
}

func (r *repo) GetTabs(ctx context.Context, userID string) ([]api.TabSummary, error) {
	return nil, errors.New("Not implemented")
}
func (r *repo) IsTabAccessAllowed(ctx context.Context, userID string, tabID int64) error {
	return errors.New("Not implemented")
}
func (r *repo) AllowTabAccess(ctx context.Context, userID string, tabID int64) error {
	return errors.New("Not implemented")
}

func (r *repo) GetTab(ctx context.Context, tabID int64) (api.Tab, error) {
	return api.Tab{}, errors.New("Not implemented")
}
func (r *repo) StoreTab(ctx context.Context, tab *api.Tab) error {
	return errors.New("Not implemented")
}
func (r *repo) DeleteTab(ctx context.Context, tabID int64) error {
	return errors.New("Not implemented")
}

func (r *repo) GetWidget(ctx context.Context, tabID int64, widgetID int64) (api.Widget, error) {
	return api.Widget{}, errors.New("Not implemented")
}
func (r *repo) StoreWidget(ctx context.Context, tabID int64, widget *api.Widget) error {
	return errors.New("Not implemented")
}
func (r *repo) DeleteWidget(ctx context.Context, tabID int64, widgetID int64) error {
	return errors.New("Not implemented")
}

func (r *repo) UpdateTabLayout(ctx context.Context, tabID int64, layout [][]int64) error {
	return errors.New("Not implemented")
}

func (r *repo) DeleteWidgetFromTab(ctx context.Context, tabID int64, widgetID int64) error {
	return errors.New("Not implemented")
}

func (r *repo) GetOrCreateFeedID(ctx context.Context, URL string) (int64, error) {
	return 0, errors.New("Not implemented")
}
func (r *repo) GetFeed(ctx context.Context, feedID int64) (api.Feed, error) {
	return api.Feed{}, errors.New("Not implemented")
}
func (r *repo) GetFeedItems(ctx context.Context, feedID int64) ([]api.FeedItem, error) {
	return nil, errors.New("Not implemented")
}
func (r *repo) StoreFeed(ctx context.Context, feed *api.Feed, feedItems []api.FeedItem) error {
	return errors.New("Not implemented")
}

func (r *repo) AreItemsRead(ctx context.Context, userID string, feedID int64, guids []string) ([]bool, error) {
	return nil, errors.New("Not implemented")
}
func (r *repo) SetItemRead(ctx context.Context, userID string, feedID int64, guid string, read bool) error {
	return errors.New("Not implemented")
}
func (r *repo) SetItemsRead(ctx context.Context, userID string, feedID int64, guids []string, read bool) error {
	return errors.New("Not implemented")
}

func (r *repo) GetAccount(ctx context.Context, userID string, accountID int64) (api.ExternalAccount, error) {
	return api.ExternalAccount{}, errors.New("Not implemented")
}
func (r *repo) GetAccounts(ctx context.Context, userID string) ([]api.ExternalAccount, error) {
	return nil, errors.New("Not implemented")
}
func (r *repo) DeleteAccount(ctx context.Context, userID string, accountID int64) error {
	return errors.New("Not implemented")
}
func (r *repo) StoreAccount(ctx context.Context, userID string, account *api.ExternalAccount) error {
	return errors.New("Not implemented")
}

func (r *repo) GetUserFromTemporaryCode(ctx context.Context, serviceName string, code string) (string, error) {
	return "", errors.New("Not implemented")
}
func (r *repo) StoreTemporaryCode(ctx context.Context, userID string, serviceName string, code string) error {
	return errors.New("Not implemented")
}
func (r *repo) DeleteTemporaryCode(ctx context.Context, userID string, serviceName string) error {
	return errors.New("Not implemented")
}

func (r *repo) GetEmailItem(ctx context.Context, account api.ExternalAccount, guid string, minVersion uint64) (api.EmailItem, error) {
	return api.EmailItem{}, errors.New("Not implemented")
}
func (r *repo) StoreEmailItem(ctx context.Context, account api.ExternalAccount, version uint64, item api.EmailItem) error {
	return errors.New("Not implemented")
}
