// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strconv"

	"github.com/pkg/errors"

	_ "github.com/lib/pq"
	"github.com/oki-apps/okihome"
	"github.com/oki-apps/okihome/api"
	"github.com/oki-apps/okihome/logInteractor/console"
	"github.com/oki-apps/okihome/providers/gmail"
	"github.com/oki-apps/okihome/providers/outlook"
	"github.com/oki-apps/okihome/repository/postgresql"
	"github.com/oki-apps/okihome/userInteractor/contextUser"
	"github.com/oki-apps/server"
)

type config struct {
	Server     server.Config
	Postgresql *postgresql.Config
	Gmail      *gmail.Config
	Outlook    *outlook.Config
}

func readConfig() config {
	var cfg config

	path := "okihome.json"
	if len(os.Args) >= 2 {
		path = os.Args[1]
	}

	b, err := ioutil.ReadFile(path)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	err = json.Unmarshal(b, &cfg)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	fmt.Println("Configuration read from ", path)

	return cfg
}

func main() {

	cfg := readConfig()

	//Instantiate all components

	//DatabaseConnector
	var repo api.Repository
	if cfg.Postgresql != nil {
		var err error
		repo, err = postgresql.New(*cfg.Postgresql)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Println("Missing PostgreSQL configuration")
		os.Exit(1)
	}

	//Log
	logInteractor := console.New()

	//User
	userInteractor := contextUser.New()

	//Services provider
	var providers []api.Provider
	if cfg.Gmail != nil {
		gmailProvider := gmail.New(*cfg.Gmail, repo)
		providers = append(providers, gmailProvider)
	}
	if cfg.Outlook != nil {
		outlookProvider := outlook.New(*cfg.Outlook, repo)
		providers = append(providers, outlookProvider)
	}

	app := okihome.NewApp(repo, userInteractor, logInteractor, providers)

	webApp, err := newWebApp(app)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	//Server
	s, err := server.New(cfg.Server)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	s.Public(server.NewJSONRoute("GET", "/api/version", webApp.GetVersion))

	s.Private(server.NewJSONRoute("GET", "/api/users/{userID}", webApp.GetUser))

	s.Private(server.NewRouteFunc("GET", "/pages/services/{serviceName}/callback", webApp.ServiceCallback))
	s.Private(server.NewRouteFunc("GET", "/pages/services/{serviceName}/register", webApp.ServiceRegister))
	s.Private(server.NewRouteFunc("GET", "/pages/users/{userID}/accounts/{accountID}", webApp.AccountStatus))

	s.Private(server.NewJSONRoute("GET", "/api/services", webApp.GetServices))

	s.Private(server.NewJSONRoute("POST", "/api/tabs", webApp.NewTab))
	s.Private(server.NewJSONRoute("GET", "/api/tabs/{tabID}", webApp.GetTab))
	s.Private(server.NewJSONRoute("POST", "/api/tabs/{tabID}", webApp.EditTab))
	s.Private(server.NewJSONRoute("DELETE", "/api/tabs/{tabID}", webApp.DeleteTab))

	s.Private(server.NewJSONRoute("POST", "/api/tabs/{tabID}/widgets", webApp.NewWidget))
	s.Private(server.NewJSONRoute("POST", "/api/tabs/{tabID}/widgets/{widgetID}", webApp.EditWidget))
	s.Private(server.NewJSONRoute("DELETE", "/api/tabs/{tabID}/widgets/{widgetID}", webApp.DeleteWidget))
	s.Private(server.NewJSONRoute("POST", "/api/tabs/{tabID}/layout", webApp.UpdateLayout))

	s.Private(server.NewJSONRoute("GET", "/api/users/{userID}/feeds/{feedID}/items", webApp.GetFeedItems))
	s.Private(server.NewJSONRoute("POST", "/api/users/{userID}/feeds/{feedID}", webApp.MarkAsRead))

	s.Private(server.NewJSONRoute("GET", "/api/users/{userID}/accounts", webApp.GetAssociatedAccounts))
	s.Private(server.NewJSONRoute("DELETE", "/api/users/{userID}/accounts/{accountID}", webApp.RevokeAccount))

	s.Private(server.NewJSONRoute("GET", "/api/users/{userID}/accounts/{accountID}/emails", webApp.GetEmails))

	s.Private(server.NewJSONRoute("POST", "/api/preview", webApp.Preview))

	//Start web app
	if err := s.Run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

type invalidEntry struct {
	err error
}

func (e invalidEntry) Error() string {
	return fmt.Sprintf("Invalid entry: %s", e.err)
}
func (e invalidEntry) IsNotFound() bool {
	return true
}

type webApp struct {
	app *okihome.App
}

func newWebApp(app *okihome.App) (webApp, error) {
	return webApp{app: app}, nil
}

func (wa webApp) ServiceCallback(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	serviceName := server.Param(r, "serviceName")

	state := r.FormValue("state")
	code := r.FormValue("code")
	wa.app.Infof(ctx, "Callback received: %s", state)

	err := wa.app.HandleOauth2Callback(ctx, serviceName, state, code)
	if err != nil {
		e := errors.Wrap(err, "Unable to handle callback")
		wa.app.Error(ctx, e)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	//Get userID from context
	userInfo, err := server.GetUserInfo(ctx)
	if err != nil {
		e := errors.Wrap(err, "Unable to retrieve userID")
		wa.app.Error(ctx, e)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	accounts, err := wa.app.AssociatedServiceAccounts(ctx, userInfo.ID(), serviceName)
	if err != nil {
		e := errors.Wrap(err, "AssociatedServiceAccounts failed")
		wa.app.Error(ctx, e)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}
	if len(accounts) > 0 {
		//Redirect to the status page
		url := fmt.Sprintf("/pages/users/%s/accounts/%d", userInfo.ID(), accounts[len(accounts)-1].ID)
		wa.app.Infof(ctx, "Redirecting to %s", url)
		http.Redirect(w, r, url, http.StatusFound)
	} else {
		//Redirect to the register page
		url := "/pages/services/" + serviceName + "/register"
		wa.app.Infof(ctx, "Redirecting to %s", url)
		http.Redirect(w, r, url, http.StatusFound)
	}

}

func (wa webApp) ServiceRegister(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	serviceName := server.Param(r, "serviceName")

	//Get userID from context
	userInfo, err := server.GetUserInfo(ctx)
	if err != nil {
		e := errors.Wrap(err, "Unable to retrieve userID")
		wa.app.Error(ctx, e)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	accounts, err := wa.app.AssociatedServiceAccounts(ctx, userInfo.ID(), serviceName)
	if err != nil {
		e := errors.Wrap(err, "GetServiceToken failed")
		wa.app.Error(ctx, e)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if len(accounts) == 0 {
		authURL, err := wa.app.ServiceRegister(ctx, serviceName)
		if err != nil {
			e := errors.Wrap(err, "ServiceRegister failed")
			wa.app.Error(ctx, e)
			http.Error(w, "Internal server error", http.StatusInternalServerError)
			return
		}

		wa.app.Infof(ctx, "Redirect: %s", authURL)
		http.Redirect(w, r, authURL, http.StatusFound)
		return
	}

	//Redirect to the status page
	url := fmt.Sprintf("/pages/users/%s/accounts/%d", userInfo.ID(), accounts[0].ID)
	wa.app.Infof(ctx, "Redirecting to %s", url)
	http.Redirect(w, r, url, http.StatusFound)
}

func (wa webApp) AccountStatus(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := server.Param(r, "userID")
	accountIDstr := server.Param(r, "accountID")
	accountID, err := strconv.ParseInt(accountIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Account ID error")
		wa.app.Error(ctx, e)
	}

	account, err := wa.app.AssociatedAccount(ctx, userID, accountID)
	if err != nil {
		e := errors.Wrap(err, "Getting associated account failed")
		wa.app.Error(ctx, e)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	if account.Token == nil {
		url := "/pages/services/" + account.ProviderName + "/register"

		fmt.Fprintf(w, `
<html>
	Service %s not authorized yet<br /><a href="%s">Register</a>
</html>
`, account.ProviderName, url)

	} else {
		fmt.Fprintf(w, `
<html>
	<script type='text/javascript'>
		opener.top.location.reload();
		self.close();
	</script>
	<h3>Success</h3>
	<p>Okihome is now authorized to access your data on %s.</p>
	<p>You may close this window.</p>
</html>
`, account.ProviderName)
	}

}

func (wa webApp) GetVersion(req *http.Request) (interface{}, error) {
	return struct {
		Version string `json:"version"`
	}{Version: "0.8-beta"}, nil
}

func (wa webApp) GetServices(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	data, err := wa.app.Services(ctx)
	if err != nil {
		e := errors.Wrap(err, "Unable to retrieve services")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) GetUser(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	userID := server.Param(req, "userID")

	data, err := wa.app.User(ctx, userID)
	if err != nil {
		e := errors.Wrap(err, "Unable to retrieve user")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) GetAssociatedAccounts(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	userID := server.Param(req, "userID")

	data, err := wa.app.AssociatedAccounts(ctx, userID)
	if err != nil {
		e := errors.Wrap(err, "Unable to retrieve associated accounts")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) RevokeAccount(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	userID := server.Param(req, "userID")
	accountIDstr := server.Param(req, "accountID")
	accountID, err := strconv.ParseInt(accountIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Account ID error")
		wa.app.Error(ctx, e)
	}

	data, err := wa.app.RevokeAccount(ctx, userID, accountID)
	if err != nil {
		e := errors.Wrap(err, "Unable to revoke account")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) GetTab(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	tabIDstr := server.Param(req, "tabID")
	tabID, err := strconv.ParseInt(tabIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}

	data, err := wa.app.Tab(ctx, tabID)
	if err != nil {
		e := errors.Wrap(err, "Unable to retrieve tab")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) DeleteTab(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	tabIDstr := server.Param(req, "tabID")
	tabID, err := strconv.ParseInt(tabIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}

	data, err := wa.app.DeleteTab(ctx, tabID)
	if err != nil {
		e := errors.Wrap(err, "Unable to delete tab")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) EditTab(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	tabIDstr := server.Param(req, "tabID")
	tabID, err := strconv.ParseInt(tabIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}

	body, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab edited items are missing")
		wa.app.Error(ctx, e)
		return nil, e
	}

	var newSummary api.TabSummary
	if err := json.Unmarshal(body, &newSummary); err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab edited items are invalid")
		wa.app.Error(ctx, e)
		return nil, e
	}

	data, err := wa.app.EditTab(ctx, tabID, newSummary)
	if err != nil {
		e := errors.Wrap(err, "Unable to edit tab")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) NewTab(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	body, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab description is missing")
		wa.app.Error(ctx, e)
		return nil, e
	}

	var tab api.TabSummary
	if err := json.Unmarshal(body, &tab); err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab description is invalid")
		wa.app.Error(ctx, e)
		return nil, e
	}

	data, err := wa.app.NewTab(ctx, tab)
	if err != nil {
		e := errors.Wrap(err, "Unable to add tab")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) NewWidget(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	tabIDstr := server.Param(req, "tabID")
	tabID, err := strconv.ParseInt(tabIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}

	body, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Widget description is missing")
		wa.app.Error(ctx, e)
		return nil, e
	}

	var widget api.Widget
	if err := json.Unmarshal(body, &widget); err != nil {
		e := errors.Wrap(invalidEntry{err}, "Widget description is invalid")
		wa.app.Error(ctx, e)
		return nil, e
	}

	//Convert widget.Config from map to the correct config
	options, ok := widget.Config.(map[string]interface{})
	if !ok {
		e := errors.New("Widget configuration is invalid")
		wa.app.Error(ctx, e)
		return nil, e
	}
	switch widget.Type {
	case api.WidgetFeedType:
		cfg := api.ConfigFeed{}
		cfg.URL = options["url"].(string)

		widget.Config = cfg
	case api.WidgetEmailType:
		cfg := api.ConfigEmail{}
		var accountIDvalue int64
		switch accountID := options["account_id"].(type) {
		case string:
			accountIDvalue, err = strconv.ParseInt(accountID, 10, 64)
			if err != nil {
				e := errors.Wrap(invalidEntry{err}, "Account ID error")
				wa.app.Error(ctx, e)
				return nil, e
			}
		case int64:
			accountIDvalue = accountID
		case int32:
			accountIDvalue = int64(accountID)
		case int:
			accountIDvalue = int64(accountID)
		case float64:
			accountIDvalue = int64(accountID)
		default:
			e := errors.New("Account ID is invalid")
			wa.app.Infof(ctx, "Options %#v", options)
			wa.app.Infof(ctx, "accountID %#v", accountID)
			wa.app.Error(ctx, e)
			return nil, e
		}

		cfg.AccountID = accountIDvalue

		widget.Config = cfg
	}

	data, err := wa.app.NewWidget(ctx, tabID, widget)
	if err != nil {
		e := errors.Wrap(err, "Unable to add widget")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) EditWidget(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	tabIDstr := server.Param(req, "tabID")
	tabID, err := strconv.ParseInt(tabIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}
	widgetIDstr := server.Param(req, "widgetID")
	widgetID, err := strconv.ParseInt(widgetIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Widget ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}

	body, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Widget config is missing")
		wa.app.Error(ctx, e)
		return nil, e
	}

	var editedConfig api.WidgetConfig
	if err := json.Unmarshal(body, &editedConfig); err != nil {
		e := errors.Wrap(invalidEntry{err}, "Widget config is invalid")
		wa.app.Error(ctx, e)
		return nil, e
	}

	data, err := wa.app.EditWidget(ctx, tabID, widgetID, editedConfig)
	if err != nil {
		e := errors.Wrap(err, "Unable to edit widget")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) DeleteWidget(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	tabIDstr := server.Param(req, "tabID")
	tabID, err := strconv.ParseInt(tabIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}
	widgetIDstr := server.Param(req, "widgetID")
	widgetID, err := strconv.ParseInt(widgetIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Widget ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}

	data, err := wa.app.DeleteWidget(ctx, tabID, widgetID)
	if err != nil {
		e := errors.Wrap(err, "Unable to delete widget")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) UpdateLayout(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	tabIDstr := server.Param(req, "tabID")
	tabID, err := strconv.ParseInt(tabIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Tab ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}

	body, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Widgets layout is missing")
		wa.app.Error(ctx, e)
		return nil, e
	}

	var layout [][]int64
	if err := json.Unmarshal(body, &layout); err != nil {
		e := errors.Wrap(invalidEntry{err}, "Widgets layout is invalid")
		wa.app.Error(ctx, e)
		return nil, e
	}

	data, err := wa.app.UpdateLayout(ctx, tabID, layout)
	if err != nil {
		e := errors.Wrap(err, "Unable to update layout")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) Preview(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	url := req.FormValue("url")
	if len(url) == 0 && req.Body != nil {
		if body, err := ioutil.ReadAll(req.Body); err == nil {
			defer req.Body.Close()
			var jsonItem struct {
				URL string `json:"url"`
			}
			if err := json.Unmarshal(body, &jsonItem); err == nil {
				url = jsonItem.URL
			}
		}
	}

	data, err := wa.app.Preview(ctx, url)
	if err != nil {
		e := errors.Wrap(err, "Unable to retrieve items for preview")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) GetFeedItems(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	userID := server.Param(req, "userID")

	feedIDstr := server.Param(req, "feedID")
	feedID, err := strconv.ParseInt(feedIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Feed ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}

	data, err := wa.app.FeedItems(ctx, userID, feedID)
	if err != nil {
		e := errors.Wrap(err, "Unable to retrieve items")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}

func (wa webApp) MarkAsRead(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	userID := server.Param(req, "userID")

	feedIDstr := server.Param(req, "feedID")
	feedID, err := strconv.ParseInt(feedIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Feed ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}

	body, err := ioutil.ReadAll(req.Body)
	defer req.Body.Close()
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "GUIDs error")
		wa.app.Error(ctx, e)
		return nil, e
	}
	var jsonItem struct {
		GUIDs []string `json:"guids"`
	}
	if err := json.Unmarshal(body, &jsonItem); err != nil {
		e := errors.Wrap(invalidEntry{err}, "GUIDs decoding failed")
		wa.app.Error(ctx, e)
		return nil, e
	}

	err = wa.app.MarkAsRead(ctx, userID, feedID, jsonItem.GUIDs)
	if err != nil {
		e := errors.Wrap(err, "Unable to retrieve items")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return nil, nil
}

func (wa webApp) GetEmails(req *http.Request) (interface{}, error) {
	ctx := req.Context()

	userID := server.Param(req, "userID")

	accountIDstr := server.Param(req, "accountID")
	accountID, err := strconv.ParseInt(accountIDstr, 10, 64)
	if err != nil {
		e := errors.Wrap(invalidEntry{err}, "Account ID error")
		wa.app.Error(ctx, e)
		return nil, e
	}

	data, err := wa.app.GetEmails(ctx, userID, accountID)
	if err != nil {
		e := errors.Wrap(err, "Unable to retrieve items")
		wa.app.Error(ctx, e)
		return nil, e
	}

	return data, nil
}
