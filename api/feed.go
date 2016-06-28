// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"time"
)

//A Feed is an timely ordered colletion of links to articles elsewhere on the web
type Feed struct {
	ID            int64     `json:"id" db:"id"`
	URL           string    `json:"url" db:"url"`
	NextRetrieval time.Time `json:"next_retrieval" db:"next_retrieval"`
	Title         string    `json:"title" db:"title"`
}

//A FeedItem is an item on a feed.
//The GUID should be unique within a feed
type FeedItem struct {
	GUID      string    `json:"guid" db:"guid"`
	Title     string    `json:"title" db:"title"`
	Published time.Time `json:"published" db:"published"`
	Link      string    `json:"link" db:"link"`
}

//An ItemForUser is a feed item with reading status for a given user added
type ItemForUser struct {
	FeedItem

	Read bool `json:"read"`
}
