// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package console

import (
	"golang.org/x/net/context"
	"log"

	"github.com/oki-apps/okihome/api"
)

type console struct{}

//New creates a new LogInteractor which prints everything in the standard output
func New() api.LogInteractor {
	return &console{}
}

// Infof formats its arguments according to the format, analogous to fmt.Printf,
// and records the text as a log message at Info level.
func (c *console) Infof(ctx context.Context, format string, args ...interface{}) {
	log.Printf("INF "+format, args...)
}

// Errorf is like Infof, but at Error level.
func (c *console) Errorf(ctx context.Context, format string, args ...interface{}) {
	log.Printf("ERR "+format, args...)
}
