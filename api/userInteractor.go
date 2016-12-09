// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"context"
)

//UserInteractor allows interactions with the User connected to the application
type UserInteractor interface {
	CurrentUserIsAdmin(ctx context.Context) bool
	CurrentUserID(ctx context.Context) (string, error)
}
