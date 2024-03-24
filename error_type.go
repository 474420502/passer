package passer

import (
	"context"
	"errors"
)

var ErrUnknown = errors.New("unknown data type, unable to execute")
var ErrTimeout = context.DeadlineExceeded
var ErrCanceled = context.Canceled

// var ErrTimeout = errors.New("context deadline exceeded: timeout")
