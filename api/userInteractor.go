// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"context"
)

//UserInfo allow access to basic user information
type UserInfo interface {
	ID() string
	DisplayName() string
	Email() string
}

//UserInteractor allows interactions with the User connected to the application
type UserInteractor interface {
	CurrentUserIsAdmin(ctx context.Context) bool
	CurrentUserID(ctx context.Context) (string, error)
	CurrentUser(ctx context.Context) (UserInfo, error)
}
