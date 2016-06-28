// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

//User represents the basic configuration for a user
type User struct {
	UserID      string `json:"user_id" db:"id"`
	DisplayName string `json:"display_name" db:"display_name"`
	Email       string `json:"email" db:"email"`

	IsAdmin bool `json:"is_admin,omitempty" db:"isadmin"`
}

//AnonymousUserID is the ID to be used when dealin with anonymous acces to the application
const AnonymousUserID = "<anonymous>"

//AnonymousUser is the user to be used when dealin with anonymous acces to the application
var AnonymousUser = User{
	UserID: AnonymousUserID,
}
