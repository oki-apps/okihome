// Copyright 2017 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

//Snapshot represents the configuration of a given user (used for backup and restore)
//Widget items (including read status) are not part of this
type Snapshot struct {
	User     User
	Tabs     []Tab
	Feeds    []Feed
	Accounts []ExternalAccount
}
