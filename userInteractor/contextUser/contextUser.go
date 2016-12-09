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

//CurrentUserID returns the ID of the current user.
//Returns an empty string if not logged in.
func (i *interactor) CurrentUserID(ctx context.Context) (string, error) {
	return server.GetUserID(ctx)
}
