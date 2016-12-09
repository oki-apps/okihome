// Copyright 2016 Simon HEGE. All rights reserved.
// Use of this source code is governed by a MIT-style
// license that can be found in the LICENSE file.

package api

import (
	"context"
)

//LogInteractor allows logging of application messages
type LogInteractor interface {
	// Infof formats its arguments according to the format, analogous to fmt.Printf,
	// and records the text as a log message at Info level.
	Infof(ctx context.Context, format string, args ...interface{})

	// Errorf is like Infof, but at Error level.
	Errorf(ctx context.Context, format string, args ...interface{})
}
