// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package contextUser

import (
	"context"

	"github.com/oki-apps/okihome/api"
	"github.com/oki-apps/server"
)

type interactor struct {
}

//New creates a new user interactor compatible with server storing the current user in the context
func New() api.UserInteractor {
	return &interactor{}
}

//CurrentUserIsAdmin returns true if the current user is an administrator
func (i *interactor) CurrentUserIsAdmin(ctx context.Context) bool {
	userID, err := i.CurrentUserID(ctx)
	if err != nil {
		return false
	}

	return userID == "admin"
}

//CurrentUserID returns the info of the current user.
//Returns an nil value if not logged in.
func (i *interactor) CurrentUserID(ctx context.Context) (string, error) {
	u, err := i.CurrentUser(ctx)
	if err != nil {
		return "", err
	}
	return u.ID(), nil
}

//CurrentUserID returns the info of the current user.
//Returns an nil value if not logged in.
func (i *interactor) CurrentUser(ctx context.Context) (api.UserInfo, error) {
	return server.GetUserInfo(ctx)
}
